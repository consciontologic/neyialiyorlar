package store

import (
	"testing"

	"github.com/neyialiyorlar/services/harvester/internal/parse"
)

// DriftStore must satisfy parse.DriftLogger so it can be wired into the
// detector via parse.MultiLogger.
var _ parse.DriftLogger = (*DriftStore)(nil)

func TestDriftSignature_StableAndOrderIndependent(t *testing.T) {
	a := DriftSignature("yahoo", "breaking", []string{"x", "y"}, []string{"b", "a"}, nil)
	b := DriftSignature("yahoo", "breaking", []string{"y", "x"}, []string{"a", "b"}, nil)
	if a != b {
		t.Fatalf("signature must be order-independent: %s vs %s", a, b)
	}
	if len(a) != 64 {
		t.Fatalf("expected 64-char sha256 hex, got %d", len(a))
	}
}

func TestDriftSignature_DistinguishesEvents(t *testing.T) {
	base := DriftSignature("yahoo", "breaking", []string{"x"}, nil, nil)
	cases := map[string]string{
		"different source":   DriftSignature("mkkvap", "breaking", []string{"x"}, nil, nil),
		"different severity": DriftSignature("yahoo", "benign", []string{"x"}, nil, nil),
		"different added":    DriftSignature("yahoo", "breaking", []string{"z"}, nil, nil),
		"diff in removed":    DriftSignature("yahoo", "breaking", []string{"x"}, []string{"r"}, nil),
		"diff in retyped":    DriftSignature("yahoo", "breaking", []string{"x"}, nil, []string{"t"}),
	}
	for name, sig := range cases {
		if sig == base {
			t.Errorf("%s: signature collided with base", name)
		}
	}
}

func TestDriftSignature_NilAndEmptyAreEqual(t *testing.T) {
	withNil := DriftSignature("s", "breaking", nil, nil, nil)
	withEmpty := DriftSignature("s", "breaking", []string{}, []string{}, []string{})
	if withNil != withEmpty {
		t.Fatalf("nil and empty diffs must hash identically: %s vs %s", withNil, withEmpty)
	}
}

func TestDriftSummary_OrdersRetypedRemovedAdded(t *testing.T) {
	got := DriftSummary("breaking", []string{"a"}, []string{"r"}, []string{"t"})
	want := "breaking: retyped=[t]; removed=[r]; added=[a]"
	if got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
	if only := DriftSummary("breaking", nil, nil, []string{"t1", "t2"}); only != "breaking: retyped=[t1,t2]" {
		t.Fatalf("retyped-only summary = %q", only)
	}
}

// LogDrift must ignore benign drift without touching the database, so calling
// it on a store with a nil *sql.DB must not panic.
func TestLogDrift_BenignIsIgnored(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("benign LogDrift must not touch the DB (panicked: %v)", r)
		}
	}()
	s := NewDriftStore(nil, nil)
	s.LogDrift("yahoo", "benign", []string{"x"}, nil, nil)
	s.LogDrift("yahoo", "none", nil, nil, nil)
}
