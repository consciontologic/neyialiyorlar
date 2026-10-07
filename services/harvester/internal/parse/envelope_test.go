package parse

import (
	"testing"

	"github.com/neyialiyorlar/services/harvester/internal/model"
)

// TestEnvelopeValidate tests envelope shape validation.
func TestEnvelopeValidate(t *testing.T) {
	tests := []struct {
		name           string
		fieldNames     map[string]string
		requiredFields []string
		payload        string
		shouldPass     bool
		expectedReason string
	}{
		{
			name: "valid payload",
			fieldNames: map[string]string{
				"status": "string",
				"data":   "object",
			},
			requiredFields: []string{"status"},
			payload:        `{"status": "ok", "data": {}}`,
			shouldPass:     true,
		},
		{
			name: "missing required field",
			fieldNames: map[string]string{
				"status": "string",
			},
			requiredFields: []string{"status"},
			payload:        `{"data": {}}`,
			shouldPass:     false,
			expectedReason: "envelope_missing_required",
		},
		{
			name: "type mismatch",
			fieldNames: map[string]string{
				"status": "string",
			},
			requiredFields: []string{"status"},
			payload:        `{"status": 123}`,
			shouldPass:     false,
			expectedReason: "envelope_type_mismatch",
		},
		{
			name: "extra fields ignored",
			fieldNames: map[string]string{
				"status": "string",
			},
			requiredFields: []string{"status"},
			payload:        `{"status": "ok", "extra": "field", "another": 42}`,
			shouldPass:     true,
		},
		{
			name: "invalid JSON",
			fieldNames: map[string]string{
				"status": "string",
			},
			requiredFields: []string{"status"},
			payload:        `{invalid json`,
			shouldPass:     false,
			expectedReason: "envelope_json_decode_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewEnvelope(tt.fieldNames, tt.requiredFields)
			result := env.Validate([]byte(tt.payload))

			if tt.shouldPass {
				if result.Error != nil {
					t.Errorf("expected pass, got error: %v", result.Error)
				}
				if result.Confidence != model.Fresh {
					t.Errorf("expected Fresh confidence, got %v", result.Confidence)
				}
			} else {
				if result.Error == nil {
					t.Errorf("expected error, got none")
				}
				if result.Reason != tt.expectedReason {
					t.Errorf("expected reason %q, got %q", tt.expectedReason, result.Reason)
				}
			}
		})
	}
}

// TestEnvelopeFieldTypeChecking tests type validation.
func TestEnvelopeFieldTypeChecking(t *testing.T) {
	tests := []struct {
		name           string
		payload        string
		fieldNames     map[string]string
		shouldPass     bool
	}{
		{
			name:       "string field",
			payload:    `{"name": "foo"}`,
			fieldNames: map[string]string{"name": "string"},
			shouldPass: true,
		},
		{
			name:       "number field",
			payload:    `{"count": 42}`,
			fieldNames: map[string]string{"count": "number"},
			shouldPass: true,
		},
		{
			name:       "number field float",
			payload:    `{"value": 3.14}`,
			fieldNames: map[string]string{"value": "number"},
			shouldPass: true,
		},
		{
			name:       "boolean field",
			payload:    `{"enabled": true}`,
			fieldNames: map[string]string{"enabled": "boolean"},
			shouldPass: true,
		},
		{
			name:       "object field",
			payload:    `{"metadata": {"key": "value"}}`,
			fieldNames: map[string]string{"metadata": "object"},
			shouldPass: true,
		},
		{
			name:       "array field",
			payload:    `{"items": [1, 2, 3]}`,
			fieldNames: map[string]string{"items": "array"},
			shouldPass: true,
		},
		{
			name:       "null field",
			payload:    `{"optional": null}`,
			fieldNames: map[string]string{"optional": "null"},
			shouldPass: true,
		},
		{
			name:       "type mismatch string",
			payload:    `{"name": 123}`,
			fieldNames: map[string]string{"name": "string"},
			shouldPass: false,
		},
		{
			name:       "type mismatch number",
			payload:    `{"count": "not a number"}`,
			fieldNames: map[string]string{"count": "number"},
			shouldPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := NewEnvelope(tt.fieldNames, []string{})
			result := env.Validate([]byte(tt.payload))

			if tt.shouldPass {
				if result.Error != nil {
					t.Errorf("expected pass, got error: %v", result.Error)
				}
			} else {
				if result.Error == nil {
					t.Errorf("expected error, got none")
				}
			}
		})
	}
}

// TestEnvelopeFieldDrift tests tolerating field changes.
func TestEnvelopeFieldDrift(t *testing.T) {
	// Schema expects "status" and "data", but source is missing "data" (field drift).
	// This should pass because "data" is optional.
	env := NewEnvelope(
		map[string]string{
			"status": "string",
			"data":   "object",
		},
		[]string{"status"}, // only status is required
	)

	// Payload missing "data" field
	result := env.Validate([]byte(`{"status": "ok"}`))
	if result.Error != nil {
		t.Errorf("field drift (missing optional field) should not error: %v", result.Error)
	}

	// Payload has extra fields (should be ignored)
	result = env.Validate([]byte(`{"status": "ok", "new_field": "value"}`))
	if result.Error != nil {
		t.Errorf("extra fields should be ignored: %v", result.Error)
	}
}
