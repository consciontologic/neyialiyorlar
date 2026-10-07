package cli

import (
	"testing"
)

func TestParseRecomputeFlags(t *testing.T) {
	cmd := ParseRecomputeFlags()
	if cmd == nil {
		t.Errorf("ParseRecomputeFlags returned nil")
	}
}

func TestRecomputeCommandValidate(t *testing.T) {
	// Valid command
	cmd := &RecomputeCommand{
		MetricKey: "m01",
		FromTs:    "2026-01-01",
	}
	if err := cmd.Validate(); err != nil {
		t.Errorf("Validate: unexpected error on valid command: %v", err)
	}

	// Invalid: missing metric key
	cmd2 := &RecomputeCommand{
		MetricKey: "",
		FromTs:    "2026-01-01",
	}
	if err := cmd2.Validate(); err == nil {
		t.Errorf("Validate: expected error for missing metric_key")
	}
}
