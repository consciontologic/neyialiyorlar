package synthetic

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/neyialiyorlar/services/api/internal/model"
)

// fakeStore is an in-memory Store so the engine is exercisable offline.
type fakeStore struct {
	mu          sync.Mutex
	entities    int
	sources     int
	rows        []MetricRow
	hasValues   bool
	countsValue map[string]int64
}

func (f *fakeStore) EnsureEntities(_ context.Context, entities []Entity) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.entities += len(entities)
	return nil
}

func (f *fakeStore) EnsureSources(_ context.Context, sources []string, _ time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sources += len(sources)
	return nil
}

func (f *fakeStore) UpsertMetric(_ context.Context, row MetricRow) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows = append(f.rows, row)
	return nil
}

func (f *fakeStore) HasMetricValues(_ context.Context) (bool, error) {
	return f.hasValues, nil
}

func (f *fakeStore) Counts(_ context.Context) (map[string]int64, error) {
	return f.countsValue, nil
}

type fakeBroadcaster struct {
	mu   sync.Mutex
	sent []model.MetricValue
}

func (f *fakeBroadcaster) Broadcast(v model.MetricValue) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, v)
}

type fakeCache struct {
	mu     sync.Mutex
	latest []model.MetricValue
}

func (f *fakeCache) SetLatest(_ context.Context, v model.MetricValue) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.latest = append(f.latest, v)
	return nil
}

func testConfig() Config {
	return Config{
		Enabled:       true,
		TickInterval:  time.Second,
		HistoryPoints: 3,
		Entities: []Entity{
			{ID: "banks", Type: "basket", DisplayTicker: "BANKS"},
			{ID: "GARAN", Type: "security", DisplayTicker: "GARAN", ISIN: "TRAGARAN91N1"},
		},
		Sources: []string{"bist", "kap"},
		Catalog: []Metric{
			{Key: "nimvi", Name: "Nimvi", Tier: model.CadenceTierDaily},
			{Key: "basis_spread", Name: "Basis Spread", Tier: model.CadenceTierDaily},
		},
	}
}

// TestBackfillPopulatesWhenEmpty verifies a cold start seeds reference data and
// generates metrics*entities*points historical rows.
func TestBackfillPopulatesWhenEmpty(t *testing.T) {
	store := &fakeStore{hasValues: false}
	eng := NewEngine(testConfig(), store, &fakeBroadcaster{}, &fakeCache{}, nil)

	if err := eng.Backfill(context.Background()); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	wantRows := 2 * 2 * 3 // metrics * entities * points
	if len(store.rows) != wantRows {
		t.Errorf("expected %d backfill rows, got %d", wantRows, len(store.rows))
	}
	if store.entities != 2 {
		t.Errorf("expected 2 entities ensured, got %d", store.entities)
	}
	if store.sources != 2 {
		t.Errorf("expected 2 sources ensured, got %d", store.sources)
	}
	for _, r := range store.rows {
		if r.Flag != model.ConfidenceApprox {
			t.Fatalf("backfill row not approx-flagged: %s", r.Flag)
		}
		if len(r.InputsHash) == 0 {
			t.Fatal("backfill row has empty inputs_hash")
		}
	}
}

// TestBackfillIdempotent verifies that when metric_value already holds data, the
// historical generation is skipped (no metric rows written) but reference data
// is still ensured.
func TestBackfillIdempotent(t *testing.T) {
	store := &fakeStore{hasValues: true}
	eng := NewEngine(testConfig(), store, nil, nil, nil)

	if err := eng.Backfill(context.Background()); err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if len(store.rows) != 0 {
		t.Errorf("expected no rows when already populated, got %d", len(store.rows))
	}
	if store.entities == 0 || store.sources == 0 {
		t.Error("expected reference data to be ensured even when populated")
	}
}

// TestTickEmitsBroadcastsAndCaches verifies one tick produces exactly one value
// per (metric, entity) pair, persisted, cached and broadcast, all approx-flagged.
func TestTickEmitsBroadcastsAndCaches(t *testing.T) {
	store := &fakeStore{hasValues: true}
	bc := &fakeBroadcaster{}
	cache := &fakeCache{}
	eng := NewEngine(testConfig(), store, bc, cache, nil)

	at := time.Date(2025, 6, 23, 10, 0, 0, 0, time.UTC)
	n, err := eng.Tick(context.Background(), at)
	if err != nil {
		t.Fatalf("tick: %v", err)
	}

	want := 2 * 2 // metrics * entities
	if n != want {
		t.Errorf("expected %d emitted, got %d", want, n)
	}
	if len(store.rows) != want {
		t.Errorf("expected %d upserts, got %d", want, len(store.rows))
	}
	if len(bc.sent) != want {
		t.Errorf("expected %d broadcasts, got %d", want, len(bc.sent))
	}
	if len(cache.latest) != want {
		t.Errorf("expected %d cache writes, got %d", want, len(cache.latest))
	}
	for _, v := range bc.sent {
		if v.Flag != model.ConfidenceApprox {
			t.Fatalf("broadcast value not approx-flagged: %s", v.Flag)
		}
	}
}

// TestTickWithoutOptionalDeps verifies broadcaster and cache are truly optional.
func TestTickWithoutOptionalDeps(t *testing.T) {
	store := &fakeStore{hasValues: true}
	eng := NewEngine(testConfig(), store, nil, nil, nil)

	n, err := eng.Tick(context.Background(), time.Now().UTC())
	if err != nil {
		t.Fatalf("tick: %v", err)
	}
	if n != 4 {
		t.Errorf("expected 4 emitted, got %d", n)
	}
}

// TestInputsHashDeterministic verifies the idempotency hash is stable and
// provenance-tagged (so synthetic rows never collide with real harvested rows).
func TestInputsHashDeterministic(t *testing.T) {
	ts := time.Date(2025, 6, 23, 10, 0, 0, 0, time.UTC)
	a := inputsHash("nimvi", "banks", ts)
	b := inputsHash("nimvi", "banks", ts)
	if string(a) != string(b) {
		t.Error("inputsHash not deterministic")
	}
	c := inputsHash("nimvi", "GARAN", ts)
	if string(a) == string(c) {
		t.Error("inputsHash should differ by entity")
	}
	if len(a) == 0 {
		t.Error("inputsHash must be non-empty")
	}
}
