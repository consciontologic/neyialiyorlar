package parse

import (
	"encoding/json"
	"fmt"

	"github.com/neyialiyorlar/services/harvester/internal/model"
)

// Envelope is a JSON shape guard that validates that the fetched payload
// has the expected top-level structure. It tolerates field drift (missing
// optional fields, extra fields ignored) but fails fast on type mismatches.
// Rationale (mandate 5): sources evolve; a missing field doesn't mean the
// data is useless — it just has unknown staleness. A wrong type is a sign
// of real corruption or source API change.
//
// Typical flow:
// 1. GET or POST fetch from source yields raw JSON bytes.
// 2. Envelope.Validate(bytes) checks shape.
// 3. If shape is valid, the field map is extracted and passed to domain parsers.
// 4. If shape is invalid, mark the raw as quarantined with reason "envelope_error".
type Envelope struct {
	// FieldNames: map of field name -> expected type. Example: {"status": "string", "data": "object", "error": "string"}
	FieldNames map[string]string

	// RequiredFields: names that MUST be present
	RequiredFields []string

	// Data: the extracted top-level fields after validation
	Data map[string]interface{}
}

// NewEnvelope creates an envelope validator with expected fields.
func NewEnvelope(fieldNames map[string]string, requiredFields []string) *Envelope {
	return &Envelope{
		FieldNames:     fieldNames,
		RequiredFields: requiredFields,
		Data:           make(map[string]interface{}),
	}
}

// Validate parses bytes as JSON and checks structure.
// Returns a ParseResult: on success, Data is populated; on failure, Error is set.
func (e *Envelope) Validate(payload []byte) model.ParseResult {
	// Unmarshal into a generic map to inspect structure.
	var raw map[string]interface{}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return model.ParseResult{
			Confidence: model.Stale,
			Error:      err,
			Reason:     "envelope_json_decode_error",
		}
	}

	// Check required fields are present.
	for _, requiredField := range e.RequiredFields {
		if _, ok := raw[requiredField]; !ok {
			return model.ParseResult{
				Confidence: model.Stale,
				Error:      fmt.Errorf("missing required field: %s", requiredField),
				Reason:     "envelope_missing_required",
			}
		}
	}

	// Validate type of each known field.
	for fieldName, expectedType := range e.FieldNames {
		if value, ok := raw[fieldName]; ok {
			// Perform type check.
			if !e.checkType(value, expectedType) {
				return model.ParseResult{
					Confidence: model.Stale,
					Error:      fmt.Errorf("field %s has wrong type: expected %s, got %T", fieldName, expectedType, value),
					Reason:     "envelope_type_mismatch",
				}
			}
		}
		// Missing optional fields are OK; ignored.
	}

	// Success: populate the data and return.
	e.Data = raw
	return model.ParseResult{
		Data:       e.Data,
		Confidence: model.Fresh,
	}
}

// checkType validates that a value matches an expected type string.
// Type names: "string", "number", "boolean", "object", "array", "null".
func (e *Envelope) checkType(value interface{}, expectedType string) bool {
	switch expectedType {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		// JSON numbers can be float64 in Go unmarshaling
		_, ok := value.(float64)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "object":
		_, ok := value.(map[string]interface{})
		return ok
	case "array":
		_, ok := value.([]interface{})
		return ok
	case "null":
		return value == nil
	default:
		// Unknown type; assume valid (soft failure)
		return true
	}
}

// AllFieldsMap returns a merged map of the provided fields and the envelope data.
// Used to pass extracted fields to downstream parsers.
func (e *Envelope) AllFieldsMap() map[string]interface{} {
	return e.Data
}
