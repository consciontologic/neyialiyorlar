package source

import (
	"testing"

	"github.com/neyialiyorlar/services/harvester/internal/parse"
)

// healthyYahooBody is a minimal but structurally complete Yahoo chart response,
// matching the shape NewYahooPriceFetcher decodes. Used to verify the drift
// backbone without a live HTTP call.
const healthyYahooBody = `{"chart":{"result":[{"meta":{"symbol":"X.IS","regularMarketPrice":1.0},` +
	`"timestamp":[1,2],"indicators":{"quote":[{"close":[1.0,2.0]}]}}],"error":null}}`

// driftCapture records the severity of each drift the detector emits.
type driftCapture struct {
	severities []string
	removed    [][]string
}

func (c *driftCapture) LogDrift(_, severity string, _, removed, _ []string) {
	c.severities = append(c.severities, severity)
	c.removed = append(c.removed, removed)
}

// The required backbone paths must actually exist in a healthy Yahoo
// fingerprint; otherwise the "required" markers are dead and a real removal
// of e.g. `indicators` would never be flagged breaking.
func TestYahooRequiredFields_PresentInHealthyFingerprint(t *testing.T) {
	shape, err := parse.FingerprintJSON([]byte(healthyYahooBody))
	if err != nil {
		t.Fatalf("fingerprint healthy body: %v", err)
	}
	for _, path := range yahooRequiredFields {
		if _, ok := shape[path]; !ok {
			t.Errorf("required path %q absent from healthy Yahoo fingerprint %v", path, shape)
		}
	}
}

// Seeding on a healthy payload then observing one with the indicators backbone
// removed must classify as breaking (a WARN alert), naming the removed path.
func TestYahooDrift_MissingBackboneIsBreaking(t *testing.T) {
	capture := &driftCapture{}
	det := parse.NewDriftDetector(nil)
	det.SetLogger(capture)

	// First observation seeds the baseline (no drift).
	rep, err := det.ObserveJSON("yahoo", []byte(healthyYahooBody), yahooRequiredFields)
	if err != nil {
		t.Fatalf("seed observe: %v", err)
	}
	if rep.Drifted {
		t.Fatalf("first observation must establish baseline, got drift: %+v", rep)
	}

	// Yahoo drops the `indicators` backbone — a breaking contract change.
	const missingIndicators = `{"chart":{"result":[{"meta":{"symbol":"X.IS"},` +
		`"timestamp":[1]}],"error":null}}`
	rep, err = det.ObserveJSON("yahoo", []byte(missingIndicators), yahooRequiredFields)
	if err != nil {
		t.Fatalf("observe missing indicators: %v", err)
	}
	if rep.Severity != parse.SeverityBreaking {
		t.Fatalf("expected breaking severity, got %q (%+v)", rep.Severity, rep)
	}
	if len(capture.severities) != 1 || capture.severities[0] != "breaking" {
		t.Fatalf("logger should have received one breaking alert, got %v", capture.severities)
	}
}

// A purely additive change (a new sibling field) is benign, not breaking, so it
// never floods the panel.
func TestYahooDrift_AdditiveIsBenign(t *testing.T) {
	det := parse.NewDriftDetector(nil)
	if _, err := det.ObserveJSON("yahoo", []byte(healthyYahooBody), yahooRequiredFields); err != nil {
		t.Fatalf("seed: %v", err)
	}
	const withExtra = `{"chart":{"result":[{"meta":{"symbol":"X.IS","regularMarketPrice":1.0},` +
		`"timestamp":[1,2],"indicators":{"quote":[{"close":[1.0]}]}}],"error":null},"newtop":true}`
	rep, err := det.ObserveJSON("yahoo", []byte(withExtra), yahooRequiredFields)
	if err != nil {
		t.Fatalf("observe additive: %v", err)
	}
	if rep.Severity != parse.SeverityBenign {
		t.Fatalf("expected benign severity for additive change, got %q (%+v)", rep.Severity, rep)
	}
}
