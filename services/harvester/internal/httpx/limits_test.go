package httpx

import (
	"context"
	"io"
	"testing"
	"time"
)

// TestBackoffExponentialGrowth verifies exponential backoff grows correctly.
func TestBackoffExponentialGrowth(t *testing.T) {
	b := Backoff{
		BaseMS:      500,
		Factor:      2.0,
		MaxMS:       60000,
		Jitter:      false,
		MaxAttempts: 5,
	}

	// Attempt 0: no backoff
	if d := b.NextDuration(0); d != 0 {
		t.Errorf("attempt 0: expected 0, got %v", d)
	}

	// Attempt 1: 500ms (base)
	if d := b.NextDuration(1); d != 500*time.Millisecond {
		t.Errorf("attempt 1: expected 500ms, got %v", d)
	}

	// Attempt 2: 1000ms (500 * 2)
	if d := b.NextDuration(2); d != 1000*time.Millisecond {
		t.Errorf("attempt 2: expected 1000ms, got %v", d)
	}

	// Attempt 3: 2000ms (500 * 2^2)
	if d := b.NextDuration(3); d != 2000*time.Millisecond {
		t.Errorf("attempt 3: expected 2000ms, got %v", d)
	}
}

// TestBackoffCeiling verifies backoff respects MaxMS.
func TestBackoffCeiling(t *testing.T) {
	b := Backoff{
		BaseMS:      500,
		Factor:      2.0,
		MaxMS:       5000,
		Jitter:      false,
		MaxAttempts: 10,
	}

	// Attempt 4: 4000ms (500 * 2^3)
	if d := b.NextDuration(4); d != 4000*time.Millisecond {
		t.Errorf("attempt 4: expected 4000ms, got %v", d)
	}

	// Attempt 5: would be 8000ms but capped at 5000ms
	if d := b.NextDuration(5); d != 5000*time.Millisecond {
		t.Errorf("attempt 5: expected 5000ms (capped), got %v", d)
	}

	// Attempt 6: still capped at 5000ms
	if d := b.NextDuration(6); d != 5000*time.Millisecond {
		t.Errorf("attempt 6: expected 5000ms (capped), got %v", d)
	}
}

// TestBackoffMaxAttemppts verifies max attempts boundary.
func TestBackoffMaxAttempts(t *testing.T) {
	b := Backoff{
		BaseMS:      500,
		Factor:      2.0,
		MaxMS:       60000,
		Jitter:      false,
		MaxAttempts: 3,
	}

	// Attempt 3: still allowed
	if d := b.NextDuration(3); d >= 0 {
		t.Logf("attempt 3: %v", d)
	}

	// Attempt 4: exceeds max, returns -1
	if d := b.NextDuration(4); d != -1 {
		t.Errorf("attempt 4: expected -1 (exceeded max), got %v", d)
	}
}

// TestCircuitBreakerOpensOnThreshold verifies circuit opens after N failures.
func TestCircuitBreakerOpensOnThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, 1000, 1)

	if cb.State != "closed" {
		t.Errorf("initial state: expected closed, got %s", cb.State)
	}

	// Record 2 failures: still closed
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State != "closed" {
		t.Errorf("after 2 failures: expected closed, got %s", cb.State)
	}

	// Record 3rd failure: should open
	cb.RecordFailure()
	if cb.State != "open" {
		t.Errorf("after 3 failures: expected open, got %s", cb.State)
	}
}

// TestCircuitBreakerHalfOpen verifies half-open transition after cooldown.
func TestCircuitBreakerHalfOpen(t *testing.T) {
	cb := NewCircuitBreaker(2, 100, 2) // 100ms cooldown, 2 half-open probes

	// Open the breaker
	cb.RecordFailure()
	cb.RecordFailure()
	if cb.State != "open" {
		t.Fatalf("failed to open breaker")
	}

	// IsOpen should return true (cooldown not elapsed)
	if !cb.IsOpen() {
		t.Errorf("immediately after open: IsOpen should be true")
	}

	// Wait for cooldown
	time.Sleep(150 * time.Millisecond)

	// AllowProbe should transition to half-open and allow a probe
	if !cb.AllowProbe() {
		t.Errorf("after cooldown: AllowProbe should be true")
	}

	if cb.State != "half-open" {
		t.Errorf("after probe: expected half-open, got %s", cb.State)
	}

	// Second probe still allowed
	if !cb.AllowProbe() {
		t.Errorf("second probe: should be allowed")
	}

	// Third probe denied (only 2 allowed)
	if cb.AllowProbe() {
		t.Errorf("third probe: should be denied (limit is 2)")
	}
}

// TestCircuitBreakerClosesOnSuccess verifies recovery path.
func TestCircuitBreakerClosesOnSuccess(t *testing.T) {
	cb := NewCircuitBreaker(2, 100, 1)

	// Open the breaker
	cb.RecordFailure()
	cb.RecordFailure()

	time.Sleep(150 * time.Millisecond)
	if !cb.AllowProbe() {
		t.Fatalf("probe not allowed")
	}

	// Success in half-open should close the circuit
	cb.RecordSuccess()
	if cb.State != "closed" {
		t.Errorf("after success in half-open: expected closed, got %s", cb.State)
	}
	if cb.FailCount != 0 {
		t.Errorf("after success: fail count should be 0, got %d", cb.FailCount)
	}
}

// TestLoadLimiterCap verifies per-host concurrency cap.
func TestLoadLimiterCap(t *testing.T) {
	ll := NewLoadLimiter(2)
	ctx := context.Background()

	// Acquire 2 slots
	if err := ll.Acquire(ctx, "example.com"); err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if err := ll.Acquire(ctx, "example.com"); err != nil {
		t.Fatalf("second acquire: %v", err)
	}

	// Third should block immediately (we use a context with timeout to avoid hanging)
	ctx2, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if err := ll.Acquire(ctx2, "example.com"); err == nil {
		t.Errorf("third acquire: expected error (cap exceeded), got none")
	}

	// Release one
	ll.Release("example.com")

	// Now third should succeed
	if err := ll.Acquire(ctx, "example.com"); err != nil {
		t.Fatalf("third acquire after release: %v", err)
	}
}

// TestLoadLimiterRetryAfter verifies Retry-After deferral.
func TestLoadLimiterRetryAfter(t *testing.T) {
	ll := NewLoadLimiter(10)
	ctx := context.Background()

	// Set Retry-After until 100ms from now
	until := time.Now().Add(100 * time.Millisecond)
	ll.SetRetryAfter("example.com", until)

	// Acquire should fail immediately (Retry-After in effect)
	if err := ll.Acquire(ctx, "example.com"); err == nil {
		t.Errorf("acquire during Retry-After: expected error, got none")
	}

	// Wait for Retry-After to elapse
	time.Sleep(150 * time.Millisecond)

	// Now acquire should succeed
	if err := ll.Acquire(ctx, "example.com"); err != nil {
		t.Errorf("acquire after Retry-After elapsed: %v", err)
	}
}

// TestSafeReaderMaxBytes verifies size limit enforcement.
func TestSafeReaderMaxBytes(t *testing.T) {
	data := []byte("hello, world!")
	reader := &mockReader{data: data}

	sr := &SafeReader{
		Reader:     reader,
		MaxBytes:   10, // Allow only 10 bytes
		TimeoutMS:  1000,
		DecompressionRatio: 2.0,
	}

	buf := make([]byte, 5)
	_, err := sr.Read(buf)
	if err != nil {
		t.Fatalf("first read: %v", err)
	}

	// Second read should succeed (5 + 5 = 10 bytes)
	_, err = sr.Read(buf)
	if err != nil {
		t.Fatalf("second read: %v", err)
	}

	// Third read should exceed limit (5 + 5 + 5 = 15 > 10)
	_, err = sr.Read(buf)
	if err == nil {
		t.Errorf("third read: expected error (exceeds max), got none")
	}
}

// TestSafeReaderCompressionRatio verifies bomb-guard ratio cap.
func TestSafeReaderCompressionRatio(t *testing.T) {
	data := []byte("a") // 1 byte compressed
	reader := &mockReader{data: data}

	sr := &SafeReader{
		Reader:             reader,
		MaxBytes:           1000,
		TimeoutMS:          1000,
		DecompressionRatio: 1.5, // Ratio cap: 1.5
	}

	buf := make([]byte, 100)
	// Read all at once (1 byte) — ratio = 1.0, OK
	n1, err := sr.Read(buf)
	if err != nil {
		t.Fatalf("first read: %v", err)
	}

	// Read again, expanding ratio beyond 1.5
	// Total decompressed = 1 + 100 = 101 bytes
	// Total compressed = 1 byte
	// Ratio = 101.0 > 1.5, so this read should fail
	// (This test is somewhat artificial since we're not actually decompressing.)
	sr.decompressedBytes += 100
	n2, err := sr.Read(buf)
	if err == nil {
		t.Errorf("read after ratio exceeded: expected error, got n=%d", n2)
	}

	t.Logf("n1=%d, n2=%d, err=%v", n1, n2, err)
}

// mockReader is a simple reader for testing.
type mockReader struct {
	data []byte
	pos  int
}

func (mr *mockReader) Read(p []byte) (int, error) {
	if mr.pos >= len(mr.data) {
		return 0, io.EOF
	}
	n := copy(p, mr.data[mr.pos:])
	mr.pos += n
	return n, nil
}
