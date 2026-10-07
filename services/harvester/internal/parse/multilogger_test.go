package parse

import (
	"reflect"
	"testing"
)

// captureLogger records every LogDrift call so a test can assert fan-out.
type captureLogger struct {
	calls int
	last  []string // [source, severity]
}

func (c *captureLogger) LogDrift(source, severity string, _, _, _ []string) {
	c.calls++
	c.last = []string{source, severity}
}

func TestMultiLogger_FansOutToEveryNonNilLogger(t *testing.T) {
	a := &captureLogger{}
	b := &captureLogger{}

	// A nil element must be skipped, not panic.
	ml := MultiLogger{a, nil, b}
	ml.LogDrift("yahoo", "breaking", []string{"x"}, nil, []string{"y"})

	if a.calls != 1 || b.calls != 1 {
		t.Fatalf("expected each logger called once, got a=%d b=%d", a.calls, b.calls)
	}
	want := []string{"yahoo", "breaking"}
	if !reflect.DeepEqual(a.last, want) || !reflect.DeepEqual(b.last, want) {
		t.Fatalf("loggers received wrong args: a=%v b=%v want=%v", a.last, b.last, want)
	}
}

func TestMultiLogger_EmptyIsNoOp(t *testing.T) {
	// Must not panic when empty or all-nil.
	MultiLogger{}.LogDrift("s", "benign", nil, nil, nil)
	MultiLogger{nil, nil}.LogDrift("s", "benign", nil, nil, nil)
}

// A MultiLogger must itself satisfy DriftLogger so it can be passed to
// DriftDetector.SetLogger.
var _ DriftLogger = MultiLogger{}
