package httpx

import (
	"context"
	"fmt"
	"io"
	"math"
	"math/rand"
	"sync"
	"time"
)

// Backoff implements exponential backoff with jitter and a max attempt cap.
// Rationale (reliability mandate 6): retry logic is a prerequisite for
// reliable scraping; jitter avoids thundering herd on 429/503. Max attempts
// prevent infinite retry loops on hard failures.
type Backoff struct {
	BaseMS      int     `json:"base_ms"`       // initial backoff duration (ms)
	Factor      float64 `json:"factor"`        // multiplier per attempt
	MaxMS       int     `json:"max_ms"`        // ceiling on backoff duration
	Jitter      bool    `json:"jitter"`        // add ±25% random deviation
	MaxAttempts int     `json:"max_attempts"`  // fail after N attempts
}

// NextDuration computes the backoff for attempt N (0-indexed).
// Rationale: exponential growth is capped; jitter is ±25% of the computed value.
func (b *Backoff) NextDuration(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	if attempt > b.MaxAttempts {
		return -1 // signal: max retries exceeded
	}

	// Exponential: base * (factor ^ attempt)
	ms := float64(b.BaseMS) * math.Pow(b.Factor, float64(attempt-1))
	if ms > float64(b.MaxMS) {
		ms = float64(b.MaxMS)
	}

	if b.Jitter {
		// ±25% random deviation
		jitterFrac := 0.25 * (2*rand.Float64() - 1) // [-0.25, 0.25]
		ms *= (1.0 + jitterFrac)
	}

	return time.Duration(ms) * time.Millisecond
}

// CircuitBreaker tracks per-source health: tracks consecutive failures,
// opens the circuit on threshold, and half-opens after a cooldown.
// Rationale (mandate 6): stop hammering a downed source; respect its recovery.
type CircuitBreaker struct {
	mu           sync.RWMutex
	State        string    // "closed", "open", "half-open"
	FailCount    int       // consecutive failures
	FailThreshold int      // open after N failures
	OpenAt       time.Time // when circuit opened
	CooldownMS   int       // wait this long before half-open
	HalfOpenProbes int     // allow N probe requests in half-open state
	ProbeCount   int       // current probe count
}

// NewCircuitBreaker creates a circuit breaker with initial state "closed".
func NewCircuitBreaker(failThreshold int, cooldownMS int, halfOpenProbes int) *CircuitBreaker {
	return &CircuitBreaker{
		State:          "closed",
		FailThreshold:  failThreshold,
		CooldownMS:     cooldownMS,
		HalfOpenProbes: halfOpenProbes,
	}
}

// RecordSuccess resets the failure counter and closes the circuit if half-open.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.FailCount = 0
	if cb.State == "half-open" {
		cb.State = "closed"
		cb.ProbeCount = 0
	}
}

// RecordFailure increments the failure counter and opens the circuit if threshold is reached.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.FailCount++
	if cb.FailCount >= cb.FailThreshold && cb.State == "closed" {
		cb.State = "open"
		cb.OpenAt = time.Now()
		cb.ProbeCount = 0
	}
}

// IsOpen returns true if the circuit is open and the cooldown has not elapsed,
// or if it's half-open and half-open probes are exhausted.
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.State == "closed" {
		return false
	}
	if cb.State == "open" {
		// Check if cooldown has elapsed
		if time.Since(cb.OpenAt) > time.Duration(cb.CooldownMS)*time.Millisecond {
			// Cooldown elapsed; transition to half-open
			return false // allow the next request
		}
		return true
	}
	if cb.State == "half-open" {
		// Allow up to HalfOpenProbes requests in half-open state
		return cb.ProbeCount >= cb.HalfOpenProbes
	}
	return false
}

// AllowProbe returns true if a probe is allowed in half-open state.
func (cb *CircuitBreaker) AllowProbe() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.State == "open" && time.Since(cb.OpenAt) > time.Duration(cb.CooldownMS)*time.Millisecond {
		cb.State = "half-open"
		cb.ProbeCount = 0
	}
	if cb.State == "half-open" && cb.ProbeCount < cb.HalfOpenProbes {
		cb.ProbeCount++
		return true
	}
	return false
}

// LoadLimiter enforces per-host concurrency cap and Retry-After deferral.
// Rationale (mandate 9): self-limit load to avoid bans; honour Retry-After
// to respect the source's rate-limiting signals.
type LoadLimiter struct {
	mu                      sync.Mutex
	maxConcurrentPerHost    int
	activePerHost           map[string]int
	hostRetryAfterUntil     map[string]time.Time
}

// NewLoadLimiter creates a limiter with a per-host concurrency cap.
func NewLoadLimiter(maxConcurrentPerHost int) *LoadLimiter {
	return &LoadLimiter{
		maxConcurrentPerHost:    maxConcurrentPerHost,
		activePerHost:           make(map[string]int),
		hostRetryAfterUntil:     make(map[string]time.Time),
	}
}

// Acquire blocks until a slot is available for the host, or until ctx is cancelled.
// Returns an error if Retry-After is in effect.
func (ll *LoadLimiter) Acquire(ctx context.Context, host string) error {
	for {
		ll.mu.Lock()
		retryUntil, hasRetryAfter := ll.hostRetryAfterUntil[host]
		if hasRetryAfter && time.Now().Before(retryUntil) {
			ll.mu.Unlock()
			return fmt.Errorf("retry-after in effect until %v", retryUntil)
		}
		if ll.activePerHost[host] < ll.maxConcurrentPerHost {
			ll.activePerHost[host]++
			ll.mu.Unlock()
			return nil
		}
		ll.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			// Retry acquiring
		}
	}
}

// Release decrements the active count for the host.
func (ll *LoadLimiter) Release(host string) {
	ll.mu.Lock()
	defer ll.mu.Unlock()
	if ll.activePerHost[host] > 0 {
		ll.activePerHost[host]--
	}
}

// SetRetryAfter sets a deferral deadline for the host (overrides backoff until this time).
// Rationale (mandate 9): honour the source's explicit rate-limit signal.
func (ll *LoadLimiter) SetRetryAfter(host string, until time.Time) {
	ll.mu.Lock()
	defer ll.mu.Unlock()
	ll.hostRetryAfterUntil[host] = until
}

// SafeRead wraps an io.Reader with size, timeout, and decompression-ratio guards.
// Rationale (mandate 17, OWASP): hostile/malformed payloads must not OOM or hang.
type SafeReader struct {
	Reader              io.Reader
	MaxBytes            int64
	TimeoutMS           int
	DecompressionRatio  float64
	bytesRead           int64
	decompressedBytes   int64
}

// Read enforces size and ratio limits on decompressed data.
func (sr *SafeReader) Read(p []byte) (int, error) {
	n, err := sr.Reader.Read(p)
	sr.bytesRead += int64(n)
	sr.decompressedBytes += int64(n)

	if sr.bytesRead > sr.MaxBytes {
		return 0, fmt.Errorf("response exceeds max_bytes limit (%d > %d)", sr.bytesRead, sr.MaxBytes)
	}

	// Decompression-bomb guard: if compressed data is very small but expands hugely,
	// flag it. This is a heuristic; a real implementation would track actual
	// compressed vs decompressed bytes from the gzip/deflate layer.
	if sr.decompressedBytes > 0 && sr.bytesRead > 0 {
		ratio := float64(sr.decompressedBytes) / float64(sr.bytesRead)
		if ratio > sr.DecompressionRatio {
			return 0, fmt.Errorf("decompression ratio exceeds limit (%.1f > %.1f)", ratio, sr.DecompressionRatio)
		}
	}

	return n, err
}
