package httpx

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ClientConfig bundles HTTP client configuration: backoff, breaker, load limits.
// Rationale (mandate 3): centralized config blueprint ensures consistency.
type ClientConfig struct {
	UserAgent              string
	MaxConcurrentPerHost   int
	TimeoutMS              int
	Backoff                Backoff
	CircuitBreaker         *CircuitBreaker
	MaxPayloadBytes        int64
	MaxRedirects           int
	AllowedHosts           []string // HTTPS-only allowlist; no SSRF pivot
	DecompressionRatio     float64
}

// Client is the main HTTP client with load-limiting, backoff, and breaker.
// Rationale (mandate 6, 9, 17): every fetch is self-limited and hardened.
type Client struct {
	config        ClientConfig
	httpClient    *http.Client
	limiter       *LoadLimiter
	breaker       *CircuitBreaker
	mu            sync.RWMutex
	sourceBreakers map[string]*CircuitBreaker // per-source circuit breakers
}

// NewClient creates a new HTTP client with the given config.
func NewClient(cfg ClientConfig) *Client {
	// Set up the underlying HTTP client with timeouts and connection pooling.
	transport := &http.Transport{
		MaxIdleConns:        32,
		MaxIdleConnsPerHost: 8,
		Dial: (&net.Dialer{
			Timeout:   time.Duration(cfg.TimeoutMS) * time.Millisecond,
			KeepAlive: 30 * time.Second,
		}).Dial,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
	}

	httpCli := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(cfg.TimeoutMS) * time.Millisecond,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Bounded redirect: fail if more than MaxRedirects have been followed.
			if len(via) >= cfg.MaxRedirects {
				return fmt.Errorf("too many redirects (%d >= %d)", len(via), cfg.MaxRedirects)
			}
			// Allowlist check: only HTTPS to configured hosts
			return checkAllowlistedHost(req.URL, cfg.AllowedHosts)
		},
	}

	return &Client{
		config:        cfg,
		httpClient:    httpCli,
		limiter:       NewLoadLimiter(cfg.MaxConcurrentPerHost),
		breaker:       cfg.CircuitBreaker,
		sourceBreakers: make(map[string]*CircuitBreaker),
	}
}

// checkAllowlistedHost validates that a URL is HTTPS and the host is in the allowlist.
// Rationale (mandate 17, OWASP): prevent SSRF pivots via open redirect.
func checkAllowlistedHost(u *url.URL, allowedHosts []string) error {
	if u.Scheme != "https" {
		return fmt.Errorf("redirect scheme not HTTPS: %s", u.Scheme)
	}

	host := strings.Split(u.Host, ":")[0] // strip port if present
	for _, allowed := range allowedHosts {
		if host == allowed {
			return nil
		}
	}
	return fmt.Errorf("redirect host not allowlisted: %s", host)
}

// GetBreaker returns the circuit breaker for a source, creating one if needed.
func (c *Client) GetBreaker(sourceID string) *CircuitBreaker {
	c.mu.Lock()
	defer c.mu.Unlock()

	if breaker, ok := c.sourceBreakers[sourceID]; ok {
		return breaker
	}

	// Create a new breaker with the same config as the global one.
	breaker := NewCircuitBreaker(
		c.config.CircuitBreaker.FailThreshold,
		c.config.CircuitBreaker.CooldownMS,
		c.config.CircuitBreaker.HalfOpenProbes,
	)
	c.sourceBreakers[sourceID] = breaker
	return breaker
}

// Do performs an HTTP request with load limiting, backoff, and circuit breaker.
// Rationale: every request is subject to the full suite of reliability primitives.
//
// The logic:
// 1. Check if the circuit breaker is open; if so, fail fast.
// 2. Acquire a load-limiter slot for the host (respects Retry-After).
// 3. Perform the request with exponential backoff on 5xx/timeout.
// 4. Return the response or error.
func (c *Client) Do(ctx context.Context, req *http.Request, sourceID string) (*http.Response, error) {
	breaker := c.GetBreaker(sourceID)

	// Check circuit-breaker state.
	if breaker.IsOpen() {
		if !breaker.AllowProbe() {
			return nil, fmt.Errorf("circuit breaker open for source %s", sourceID)
		}
	}

	// Acquire a load-limiter slot (respects Retry-After).
	host := req.URL.Host
	if err := c.limiter.Acquire(ctx, host); err != nil {
		return nil, err
	}
	defer c.limiter.Release(host)

	// Retry loop with exponential backoff.
	var lastErr error
	for attempt := 0; attempt < c.config.Backoff.MaxAttempts; attempt++ {
		resp, err := c.httpClient.Do(req.WithContext(ctx))

		// Check for Retry-After header and defer if present.
		if resp != nil && (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable) {
			retryAfter := resp.Header.Get("Retry-After")
			if retryAfter != "" {
				// Parse Retry-After (could be seconds or an HTTP date; simplify to seconds).
				if sec := parseRetryAfter(retryAfter); sec > 0 {
					c.limiter.SetRetryAfter(host, time.Now().Add(time.Duration(sec)*time.Second))
				}
			}
		}

		// Success: record and return.
		if err == nil && resp != nil && resp.StatusCode < 500 {
			breaker.RecordSuccess()
			return resp, nil
		}

		// Failure: record and retry.
		breaker.RecordFailure()
		lastErr = err
		if resp != nil {
			resp.Body.Close()
		}

		// Wait before retrying (exponential backoff).
		backoffDuration := c.config.Backoff.NextDuration(attempt + 1)
		if backoffDuration < 0 {
			break // max attempts exceeded
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoffDuration):
			// Retry
		}
	}

	breaker.RecordFailure()
	return nil, fmt.Errorf("max retries exceeded for %s: %w", sourceID, lastErr)
}

// parseRetryAfter parses a Retry-After header value (simplified: assume it's seconds).
// A full implementation would handle both "123" (seconds) and RFC1123 date format.
func parseRetryAfter(s string) int {
	var sec int
	if _, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &sec); err != nil {
		return 0
	}
	return sec
}

// GetWithBackoff is a convenience wrapper for GET requests.
func (c *Client) GetWithBackoff(ctx context.Context, url string, sourceID string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.config.UserAgent)
	return c.Do(ctx, req, sourceID)
}

// PostWithBackoff is a convenience wrapper for POST requests.
func (c *Client) PostWithBackoff(ctx context.Context, url string, contentType string, body io.Reader, sourceID string) (*http.Response, error) {
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.config.UserAgent)
	req.Header.Set("Content-Type", contentType)
	return c.Do(ctx, req, sourceID)
}

// ResponseBody wraps resp.Body with size/ratio/timeout guards.
// Rationale (mandate 17): limit buffer expansion from gzip/deflate.
func (c *Client) ResponseBody(resp *http.Response) io.Reader {
	return &SafeReader{
		Reader:             resp.Body,
		MaxBytes:           c.config.MaxPayloadBytes,
		TimeoutMS:          c.config.TimeoutMS,
		DecompressionRatio: c.config.DecompressionRatio,
	}
}
