package parse

import (
	"sort"
	"sync"
	"testing"

	"github.com/neyialiyorlar/services/harvester/internal/model"
)

// ---- helpers ---------------------------------------------------------------

func mustFingerprint(t *testing.T, payload string) Shape {
	t.Helper()
	s, err := FingerprintJSON([]byte(payload))
	if err != nil {
		t.Fatalf("FingerprintJSON(%q) unexpected error: %v", payload, err)
	}
	return s
}

func shapeEqual(a, b Shape) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func sortedEqual(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	g := append([]string(nil), got...)
	w := append([]string(nil), want...)
	sort.Strings(g)
	sort.Strings(w)
	for i := range g {
		if g[i] != w[i] {
			return false
		}
	}
	return true
}

// ---- FingerprintJSON: stability (no false positives) -----------------------

func TestFingerprintJSON_ValueIndependent(t *testing.T) {
	a := mustFingerprint(t, `{"status":"ok","value":1.5}`)
	b := mustFingerprint(t, `{"status":"changed","value":9999.99}`)
	if !shapeEqual(a, b) {
		t.Errorf("different values must yield identical shape:\n a=%v\n b=%v", a, b)
	}
}

func TestFingerprintJSON_KeyOrderIndependent(t *testing.T) {
	a := mustFingerprint(t, `{"status":"ok","data":[]}`)
	b := mustFingerprint(t, `{"data":[],"status":"ok"}`)
	if !shapeEqual(a, b) {
		t.Errorf("key order must not affect shape:\n a=%v\n b=%v", a, b)
	}
}

func TestFingerprintJSON_WhitespaceIndependent(t *testing.T) {
	a := mustFingerprint(t, `{"status":"ok","data":[]}`)
	b := mustFingerprint(t, "  {\n  \"status\" : \"ok\" ,\n  \"data\" : [ ]\n }  ")
	if !shapeEqual(a, b) {
		t.Errorf("whitespace must not affect shape:\n a=%v\n b=%v", a, b)
	}
}

// ---- FingerprintJSON: structural detail ------------------------------------

func TestFingerprintJSON_RecordsKindsAndPaths(t *testing.T) {
	s := mustFingerprint(t, `{"status":"ok","n":3,"ok":true,"obj":{"x":1},"arr":[],"z":null}`)
	want := map[string]Kind{
		rootKey:  KindObject,
		"status": KindString,
		"n":      KindNumber,
		"ok":     KindBool,
		"obj":    KindObject,
		"obj.x":  KindNumber,
		"arr":    KindArray,
		"z":      KindNull,
	}
	if !shapeEqual(s, want) {
		t.Errorf("shape mismatch:\n got=%v\n want=%v", s, want)
	}
}

func TestFingerprintJSON_ArrayElementShape(t *testing.T) {
	s := mustFingerprint(t, `{"data":[{"indexname":"XU030","value":8500.25}]}`)
	for _, path := range []string{"$", "data", "data[]", "data[].indexname", "data[].value"} {
		if _, ok := s[path]; !ok {
			t.Errorf("expected path %q present in shape %v", path, s)
		}
	}
	if s["data[].value"] != KindNumber {
		t.Errorf("expected data[].value number, got %v", s["data[].value"])
	}
}

func TestFingerprintJSON_RootKindCapturesContainerType(t *testing.T) {
	obj := mustFingerprint(t, `{"a":1}`)
	arr := mustFingerprint(t, `[{"a":1}]`)
	if obj[rootKey] != KindObject {
		t.Errorf("expected object root, got %v", obj[rootKey])
	}
	if arr[rootKey] != KindArray {
		t.Errorf("expected array root, got %v", arr[rootKey])
	}
	// An object->array flip at the root is a retype of "$": breaking.
	rep := CompareShapes(obj, arr, nil)
	if rep.Severity != SeverityBreaking {
		t.Errorf("object->array root flip must be breaking, got %v (%s)", rep.Severity, rep.Summary)
	}
}

// ---- FingerprintJSON: hostile / edge inputs (no panic) ---------------------

func TestFingerprintJSON_EdgeInputs(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		wantErr bool
	}{
		{"empty object", `{}`, false},
		{"empty array", `[]`, false},
		{"top-level null", `null`, false},
		{"top-level string", `"hello"`, false},
		{"top-level number", `42`, false},
		{"top-level bool", `true`, false},
		{"empty bytes", ``, true},
		{"truncated", `{`, true},
		{"garbage", `<<<not json>>>`, true},
		{"trailing junk", `{"a":1}garbage`, false}, // decoder reads first value
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, err := FingerprintJSON([]byte(tc.payload))
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got shape %v", tc.payload, s)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error for %q: %v", tc.payload, err)
			}
			if s == nil {
				t.Errorf("expected non-nil shape for %q", tc.payload)
			}
		})
	}
}

func TestFingerprintJSON_DeeplyNestedDoesNotExplode(t *testing.T) {
	// Build a payload nested far deeper than maxShapeDepth.
	payload := ""
	for i := 0; i < 200; i++ {
		payload += `{"a":`
	}
	payload += `1`
	for i := 0; i < 200; i++ {
		payload += `}`
	}
	s, err := FingerprintJSON([]byte(payload))
	if err != nil {
		t.Fatalf("deep nest should decode: %v", err)
	}
	// Bounded depth: the recorded paths cannot exceed maxShapeDepth levels.
	for path := range s {
		depth := 0
		for _, r := range path {
			if r == '.' {
				depth++
			}
		}
		if depth > maxShapeDepth+1 {
			t.Errorf("path %q exceeds bounded depth", path)
		}
	}
}

// ---- ShapeFromFields / ShapeFromContract -----------------------------------

func TestShapeFromFields(t *testing.T) {
	s := ShapeFromFields(map[string]interface{}{
		"total_value": "1.234,56",
		"count":       float64(3),
	})
	if s[rootKey] != KindObject {
		t.Errorf("expected object root")
	}
	if s["total_value"] != KindString {
		t.Errorf("expected total_value string, got %v", s["total_value"])
	}
	if s["count"] != KindNumber {
		t.Errorf("expected count number, got %v", s["count"])
	}
}

func TestShapeFromContract(t *testing.T) {
	s := ShapeFromContract(map[string]string{
		"status": "string",
		"data":   "array",
		"n":      "number",
		"flag":   "boolean",
	})
	want := map[string]Kind{
		rootKey:  KindObject,
		"status": KindString,
		"data":   KindArray,
		"n":      KindNumber,
		"flag":   KindBool,
	}
	if !shapeEqual(s, want) {
		t.Errorf("contract shape mismatch:\n got=%v\n want=%v", s, want)
	}
}

// ---- CompareShapes severity classification ---------------------------------

func TestCompareShapes_Severity(t *testing.T) {
	base := Shape{rootKey: KindObject, "status": KindString, "data": KindArray}

	tests := []struct {
		name     string
		current  Shape
		required []string
		want     Severity
		added    []string
		removed  []string
		retyped  []string
	}{
		{
			name:    "identical",
			current: Shape{rootKey: KindObject, "status": KindString, "data": KindArray},
			want:    SeverityNone,
		},
		{
			name:    "additive field",
			current: Shape{rootKey: KindObject, "status": KindString, "data": KindArray, "extra": KindString},
			want:    SeverityBenign,
			added:   []string{"extra"},
		},
		{
			name:     "optional field removed",
			current:  Shape{rootKey: KindObject, "data": KindArray},
			required: []string{"data"},
			want:     SeverityBenign,
			removed:  []string{"status"},
		},
		{
			name:     "required field removed",
			current:  Shape{rootKey: KindObject, "status": KindString},
			required: []string{"data"},
			want:     SeverityBreaking,
			removed:  []string{"data"},
		},
		{
			name:    "field retyped",
			current: Shape{rootKey: KindObject, "status": KindNumber, "data": KindArray},
			want:    SeverityBreaking,
			retyped: []string{"status"},
		},
		{
			name:    "root retyped",
			current: Shape{rootKey: KindArray, "status": KindString, "data": KindArray},
			want:    SeverityBreaking,
			retyped: []string{rootKey},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rep := CompareShapes(base, tc.current, tc.required)
			if rep.Severity != tc.want {
				t.Errorf("severity: got %v want %v (%s)", rep.Severity, tc.want, rep.Summary)
			}
			if rep.Drifted != (tc.want != SeverityNone) {
				t.Errorf("drifted flag inconsistent with severity %v", tc.want)
			}
			if tc.added != nil && !sortedEqual(rep.Added, tc.added) {
				t.Errorf("added: got %v want %v", rep.Added, tc.added)
			}
			if tc.removed != nil && !sortedEqual(rep.Removed, tc.removed) {
				t.Errorf("removed: got %v want %v", rep.Removed, tc.removed)
			}
			if tc.retyped != nil && !sortedEqual(rep.Retyped, tc.retyped) {
				t.Errorf("retyped: got %v want %v", rep.Retyped, tc.retyped)
			}
		})
	}
}

func TestRecommendedConfidence(t *testing.T) {
	cases := map[Severity]model.Confidence{
		SeverityNone:     model.Fresh,
		SeverityBenign:   model.Approx,
		SeverityBreaking: model.Stale,
	}
	for sev, want := range cases {
		if got := RecommendedConfidence(sev); got != want {
			t.Errorf("RecommendedConfidence(%v): got %v want %v", sev, got, want)
		}
	}
}

// ---- DriftDetector behaviour -----------------------------------------------

func TestDriftDetector_FirstObservationEstablishesBaseline(t *testing.T) {
	d := NewDriftDetector(nil)
	rep := d.Observe("bist", mustFingerprint(t, `{"status":"ok","data":[]}`), []string{"data"})
	if rep.Drifted || rep.Severity != SeverityNone {
		t.Errorf("first observation must not report drift, got %v (%s)", rep.Severity, rep.Summary)
	}
	// A second identical observation also reports none.
	rep2 := d.Observe("bist", mustFingerprint(t, `{"status":"ok","data":[]}`), []string{"data"})
	if rep2.Drifted {
		t.Errorf("identical second observation must not drift, got %s", rep2.Summary)
	}
}

func TestDriftDetector_BenignDriftAdvancesBaseline(t *testing.T) {
	d := NewDriftDetector(nil)
	d.Observe("kap", mustFingerprint(t, `{"status":"ok","data":[]}`), []string{"data"})

	// A vendor adds an optional field: benign, and the baseline adapts.
	rep := d.Observe("kap", mustFingerprint(t, `{"status":"ok","data":[],"page":1}`), []string{"data"})
	if rep.Severity != SeverityBenign {
		t.Fatalf("additive change must be benign, got %v (%s)", rep.Severity, rep.Summary)
	}

	// Re-observing the new (additive) shape now reports no drift, proving
	// the baseline advanced — the system adapted automatically.
	rep2 := d.Observe("kap", mustFingerprint(t, `{"status":"ok","data":[],"page":1}`), []string{"data"})
	if rep2.Drifted {
		t.Errorf("baseline did not adapt to additive change: %s", rep2.Summary)
	}
}

func TestDriftDetector_BreakingDriftHoldsBaseline(t *testing.T) {
	d := NewDriftDetector(nil)
	good := `{"status":"ok","data":[]}`
	d.Observe("bist", mustFingerprint(t, good), []string{"data"})

	// "data" disappears: breaking.
	rep := d.Observe("bist", mustFingerprint(t, `{"status":"ok"}`), []string{"data"})
	if rep.Severity != SeverityBreaking {
		t.Fatalf("removing required field must be breaking, got %v (%s)", rep.Severity, rep.Summary)
	}

	// The baseline must NOT have adopted the broken shape: a subsequent
	// healthy payload compares clean against the held contract.
	rep2 := d.Observe("bist", mustFingerprint(t, good), []string{"data"})
	if rep2.Drifted {
		t.Errorf("baseline should have held the known-good contract, got %s", rep2.Summary)
	}
}

func TestDriftDetector_SeedCatchesFirstBreakingPayload(t *testing.T) {
	// Seeding from a declared contract lets the very first live payload be
	// judged against the intended shape rather than becoming the baseline.
	d := NewDriftDetector(nil)
	d.Seed("bist", ShapeFromContract(map[string]string{"status": "string", "data": "array"}))

	rep := d.Observe("bist", mustFingerprint(t, `{"status":"ok"}`), []string{"data"})
	if rep.Severity != SeverityBreaking {
		t.Errorf("seeded contract must flag first broken payload as breaking, got %v (%s)", rep.Severity, rep.Summary)
	}
}

func TestDriftDetector_ObserveJSONDecodeError(t *testing.T) {
	d := NewDriftDetector(nil)
	if _, err := d.ObserveJSON("bist", []byte(`{not json`), []string{"data"}); err == nil {
		t.Errorf("expected decode error for malformed JSON")
	}
}

// recordingDriftLogger captures LogDrift calls for assertions.
type recordingDriftLogger struct {
	severities []string
}

func (r *recordingDriftLogger) LogDrift(source, severity string, added, removed, retyped []string) {
	r.severities = append(r.severities, severity)
}

func TestDriftDetector_AlertsOnDriftOnly(t *testing.T) {
	rec := &recordingDriftLogger{}
	d := NewDriftDetector(nil)
	d.SetLogger(rec)

	good := `{"status":"ok","data":[]}`
	// First observation establishes the baseline: no alert.
	d.Observe("bist", mustFingerprint(t, good), []string{"data"})
	// Identical observation: no drift, no alert.
	d.Observe("bist", mustFingerprint(t, good), []string{"data"})
	if len(rec.severities) != 0 {
		t.Fatalf("expected no alerts for stable source, got %v", rec.severities)
	}

	// Additive change: benign alert.
	d.Observe("bist", mustFingerprint(t, `{"status":"ok","data":[],"page":1}`), []string{"data"})
	// Required field removed: breaking alert.
	d.Observe("bist", mustFingerprint(t, `{"status":"ok"}`), []string{"data"})

	if !sortedEqual(rec.severities, []string{"benign", "breaking"}) {
		t.Errorf("expected benign+breaking alerts, got %v", rec.severities)
	}
}

// ---- Hard breaking scenarios: realistic source payloads --------------------

func TestDrift_HardScenarios(t *testing.T) {
	tests := []struct {
		name     string
		baseline string
		current  string
		required []string
		want     Severity
	}{
		{
			name:     "BIST flips close from number to string",
			baseline: `{"status":"ok","data":[{"indexname":"XU030","value":8500.25}]}`,
			current:  `{"status":"ok","data":[{"indexname":"XU030","value":"8500.25"}]}`,
			required: []string{"data"},
			want:     SeverityBreaking,
		},
		{
			name:     "KAP renames data envelope to result",
			baseline: `{"status":"ok","data":[{"disclosureId":1}]}`,
			current:  `{"status":"ok","result":[{"disclosureId":1}]}`,
			required: []string{"data"},
			want:     SeverityBreaking,
		},
		{
			name:     "KAP nested record field removed",
			baseline: `{"data":[{"isin":"TR","close":1.0}]}`,
			current:  `{"data":[{"isin":"TR"}]}`,
			required: []string{"data[].close"},
			want:     SeverityBreaking,
		},
		{
			name:     "empty payload versus populated baseline",
			baseline: `{"status":"ok","data":[{"x":1}]}`,
			current:  `{}`,
			required: []string{"data"},
			want:     SeverityBreaking,
		},
		{
			name:     "vendor adds optional metadata",
			baseline: `{"status":"ok","data":[]}`,
			current:  `{"status":"ok","data":[],"generatedAt":"2026-06-26"}`,
			required: []string{"data"},
			want:     SeverityBenign,
		},
		{
			name:     "value churn only is stable",
			baseline: `{"status":"ok","data":[{"x":1}]}`,
			current:  `{"status":"different","data":[{"x":9999}]}`,
			required: []string{"data"},
			want:     SeverityNone,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			base := mustFingerprint(t, tc.baseline)
			cur := mustFingerprint(t, tc.current)
			rep := CompareShapes(base, cur, tc.required)
			if rep.Severity != tc.want {
				t.Errorf("got %v want %v\n baseline=%s\n current=%s\n summary=%s",
					rep.Severity, tc.want, tc.baseline, tc.current, rep.Summary)
			}
		})
	}
}

// ---- Concurrency safety (run under -race) ----------------------------------

func TestDriftDetector_ConcurrentObserveIsSafe(t *testing.T) {
	d := NewDriftDetector(nil)
	shape := mustFingerprint(t, `{"status":"ok","data":[{"x":1}]}`)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				d.Observe("concurrent", shape, []string{"data"})
			}
		}()
	}
	wg.Wait()
	// Final observation of the same shape must still be drift-free.
	if rep := d.Observe("concurrent", shape, []string{"data"}); rep.Drifted {
		t.Errorf("stable shape under concurrency reported drift: %s", rep.Summary)
	}
}

// ---- MemoryBaselineStore isolation -----------------------------------------

func TestMemoryBaselineStore_ReturnsCopy(t *testing.T) {
	store := NewMemoryBaselineStore()
	store.Set("s", Shape{rootKey: KindObject, "a": KindString})

	got, ok := store.Get("s")
	if !ok {
		t.Fatal("expected stored baseline")
	}
	// Mutating the returned copy must not corrupt the stored baseline.
	got["a"] = KindNumber
	got["injected"] = KindBool

	again, _ := store.Get("s")
	if again["a"] != KindString {
		t.Errorf("stored baseline was mutated via returned copy: %v", again)
	}
	if _, leaked := again["injected"]; leaked {
		t.Errorf("injected key leaked into stored baseline: %v", again)
	}
}

// ---- Fuzz: fingerprinting arbitrary bytes never panics ---------------------

func FuzzFingerprintJSON(f *testing.F) {
	seeds := []string{
		`{"status":"ok","data":[]}`,
		`{"data":[{"x":1,"y":"a","z":null}]}`,
		`[]`, `{}`, `null`, `true`, `42`, `"s"`,
		``, `{`, `<<<>>>`, `{"a":`,
		`{"deep":{"deeper":{"deepest":[1,2,3]}}}`,
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, input []byte) {
		shape, err := FingerprintJSON(input)
		if err != nil {
			return // controlled error path
		}
		// On success the shape must be usable and always carry a root kind.
		if shape == nil {
			t.Errorf("nil shape without error for %q", input)
		}
		if _, ok := shape[rootKey]; !ok {
			t.Errorf("shape missing root kind for %q: %v", input, shape)
		}
		// Comparing a shape with itself must be drift-free and must not panic.
		if rep := CompareShapes(shape, shape, nil); rep.Drifted {
			t.Errorf("self-comparison reported drift for %q: %s", input, rep.Summary)
		}
	})
}
