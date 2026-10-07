package source

import (
	"context"
	"fmt"

	"github.com/neyialiyorlar/services/harvester/internal/model"
)

// Source defines the interface for a data source adapter.
// Each source (BIST, KAP, MKK/VAP, EVDS) implements this interface
// to fetch, parse, and return derived metrics.
type Source interface {
	// Name returns the source identifier (e.g., "bist", "kap", "mkkvap", "evds")
	Name() string

	// SourceID returns the unique identifier for this source in config
	SourceID() string

	// Fetch retrieves the next batch of records, advancing the cursor.
	// Rationale: permits pagination/streaming; cursor is persisted by harvester.
	// Returns raw rows (parse is done separately) and the next cursor.
	Fetch(ctx context.Context, cursor string) ([]model.Raw, string, error)

	// Parse processes raw bytes into derived fields.
	// Rationale: separation of concerns; parse happens offline + is replayed on quarantine.
	Parse(raw model.Raw) model.ParseResult

	// Cadence returns the ingestion frequency (Event, Intraday, Daily, Weekly)
	Cadence() model.Cadence
}

// Config holds common configuration for a source.
type Config struct {
	SourceID    string            // Unique identifier
	URL         string            // Base endpoint
	Timeout     int               // Request timeout in ms
	Cadence     model.Cadence     // Ingestion frequency
	Enabled     bool              // Whether to fetch from this source
	WAFProtected bool             // If true, should be skipped at registration
	ParseConfig map[string]string // Parser-specific config (field anchors, etc.)
}

// Registry manages available sources.
// Rationale: compile-time guards prevent WAF-protected sources from loading.
type Registry struct {
	sources map[string]Source
}

// NewRegistry creates an empty source registry.
func NewRegistry() *Registry {
	return &Registry{
		sources: make(map[string]Source),
	}
}

// Register adds a source to the registry.
// Returns an error if the source is WAF-protected (mandate 8).
func (r *Registry) Register(src Source, cfg Config) error {
	if cfg.WAFProtected {
		// Compile-time guard: WAF-protected sources are never loaded
		return fmt.Errorf("source %s is WAF-protected; evading is impossible by construction (ADR-0004)", cfg.SourceID)
	}

	r.sources[src.Name()] = src
	return nil
}

// Get retrieves a source by name.
func (r *Registry) Get(name string) (Source, bool) {
	src, ok := r.sources[name]
	return src, ok
}

// ListSources returns all registered sources.
func (r *Registry) ListSources() []string {
	var names []string
	for name := range r.sources {
		names = append(names, name)
	}
	return names
}
