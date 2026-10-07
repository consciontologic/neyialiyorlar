package source

import (
	"context"
	"testing"

	"github.com/neyialiyorlar/services/harvester/internal/model"
)

// TestSourceRegistry tests source registration and retrieval.
func TestSourceRegistry(t *testing.T) {
	registry := NewRegistry()

	// Create a test source
	testSource := &testSourceMock{
		nameVal: "test",
	}

	cfg := Config{
		SourceID:     "test-1",
		URL:          "http://localhost",
		Cadence:      model.Daily,
		Enabled:      true,
		WAFProtected: false,
	}

	err := registry.Register(testSource, cfg)
	if err != nil {
		t.Errorf("failed to register source: %v", err)
	}

	// Retrieve the source
	retrieved, ok := registry.Get("test")
	if !ok {
		t.Errorf("failed to retrieve registered source")
	}

	if retrieved.Name() != "test" {
		t.Errorf("expected name 'test', got %s", retrieved.Name())
	}
}

// TestRegistryWAFProtectedGuard tests that WAF-protected sources are rejected.
func TestRegistryWAFProtectedGuard(t *testing.T) {
	registry := NewRegistry()

	testSource := &testSourceMock{nameVal: "waf-protected"}

	cfg := Config{
		SourceID:     "protected-source",
		WAFProtected: true, // This should be rejected
	}

	err := registry.Register(testSource, cfg)
	if err == nil {
		t.Errorf("expected error when registering WAF-protected source")
	}

	if err.Error() == "" {
		t.Errorf("expected error message, got empty string")
	}
}

// TestRegistry ListSources tests listing all registered sources.
func TestRegistryListSources(t *testing.T) {
	registry := NewRegistry()

	src1 := &testSourceMock{nameVal: "src1"}
	src2 := &testSourceMock{nameVal: "src2"}

	registry.Register(src1, Config{SourceID: "s1", WAFProtected: false})
	registry.Register(src2, Config{SourceID: "s2", WAFProtected: false})

	sources := registry.ListSources()
	if len(sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(sources))
	}
}

// testSourceMock is a mock source for testing.
type testSourceMock struct {
	nameVal string
}

func (ts *testSourceMock) Name() string {
	return ts.nameVal
}

func (ts *testSourceMock) SourceID() string {
	return ts.nameVal + "-id"
}

func (ts *testSourceMock) Fetch(ctx context.Context, cursor string) ([]model.Raw, string, error) {
	return []model.Raw{}, "", nil
}

func (ts *testSourceMock) Parse(raw model.Raw) model.ParseResult {
	return model.ParseResult{Confidence: model.Fresh}
}

func (ts *testSourceMock) Cadence() model.Cadence {
	return model.Daily
}
