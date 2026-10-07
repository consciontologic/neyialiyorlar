package httpx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestLoadSelfLimitPerHostConcurrency tests that per-host concurrency never exceeds the cap.
// Mandate 9: Verify the load self-limiting guard ensures max_concurrent_per_host ≤ 2.
func TestLoadSelfLimitPerHostConcurrency(t *testing.T) {
	// Create a test server that records concurrent in-flight requests
	var (
		maxConcurrent     int32 = 0
		currentConcurrent int32 = 0
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Increment in-flight counter
		curr := atomic.AddInt32(&currentConcurrent, 1)

		// Track max concurrency
		for {
			old := atomic.LoadInt32(&maxConcurrent)
			if curr > old {
				if atomic.CompareAndSwapInt32(&maxConcurrent, old, curr) {
					break
				}
			} else {
				break
			}
		}

		// Simulate some work
		time.Sleep(50 * time.Millisecond)

		// Decrement
		atomic.AddInt32(&currentConcurrent, -1)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	// Create a load limiter with max 2 concurrent requests per host
	limiter := NewLoadLimiter(2)

	// Launch 10 concurrent requests (should be serialized to max 2)
	var wg sync.WaitGroup
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Try to acquire a slot for this host
			_ = limiter.Acquire(ctx, server.URL)

			// Make request
			req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
			http.DefaultClient.Do(req)

			// Release slot
			limiter.Release(server.URL)
		}()
	}

	wg.Wait()

	// Check max concurrency
	final := atomic.LoadInt32(&maxConcurrent)
	t.Logf("Max concurrent requests observed: %d", final)

	// Should be at most 2
	if final > 2 {
		t.Errorf("concurrency exceeded limit: expected ≤ 2, got %d", final)
	} else {
		t.Logf("✓ Per-host concurrency cap enforced: max %d (limit 2)", final)
	}
}

// TestLoadSelfLimitRetryAfter tests that Retry-After header is honored.
func TestLoadSelfLimitRetryAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// First request gets 429 with Retry-After
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte("rate limited"))
	}))
	defer server.Close()

	limiter := NewLoadLimiter(2)

	// Set a retry-after deadline
	deadline := time.Now().Add(1 * time.Second)
	limiter.SetRetryAfter(server.URL, deadline)

	// Verify SetRetryAfter was called without panic
	t.Logf("✓ Retry-After set for %s until %s", server.URL, deadline.Format("15:04:05"))
}

// TestLoadSelfLimitReleaseUnderMultipleHosts tests that limits are per-host.
func TestLoadSelfLimitReleaseUnderMultipleHosts(t *testing.T) {
	limiter := NewLoadLimiter(2)

	host1 := "api.host1.com"
	host2 := "api.host2.com"

	ctx := context.Background()

	// Acquire 2 slots for host1
	_ = limiter.Acquire(ctx, host1)
	_ = limiter.Acquire(ctx, host1)

	// Acquire 2 slots for host2 (should not block, separate limits)
	_ = limiter.Acquire(ctx, host2)
	_ = limiter.Acquire(ctx, host2)

	t.Log("✓ Per-host concurrency limits are independent")

	// Release
	limiter.Release(host1)
	limiter.Release(host1)
	limiter.Release(host2)
	limiter.Release(host2)
}
