package main

import (
	"context"
	"database/sql"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"unicode"

	"github.com/neyialiyorlar/services/api/internal/handler"
	"github.com/neyialiyorlar/services/api/internal/middleware"
	"github.com/neyialiyorlar/services/api/internal/model"
	"github.com/neyialiyorlar/services/api/internal/synthetic"
	sharedlog "github.com/neyialiyorlar/services/shared/logger"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"gopkg.in/yaml.v3"
)

type Config struct {
	API struct {
		Port              int           `yaml:"port"`
		ReadHeaderTimeout time.Duration `yaml:"read_header_timeout"`
		ReadTimeout       time.Duration `yaml:"read_timeout"`
		WriteTimeout      time.Duration `yaml:"write_timeout"`
		IdleTimeout       time.Duration `yaml:"idle_timeout"`
		MaxHeaderBytes    int           `yaml:"max_header_bytes"`
		MaxBodySize       int64         `yaml:"max_body_size"`
		MaxQueries        int           `yaml:"max_query_metrics"`
		RequestTimeout    time.Duration `yaml:"request_timeout"`
		WSDrainTimeout    time.Duration `yaml:"ws_drain_timeout_s"`
		CORSOrigins       []string      `yaml:"cors_origins"`
		WSOriginAllowlist []string      `yaml:"ws_origin_allowlist"`
		WSMaxMessageSize  int64         `yaml:"ws_max_message_size"`
		WSPingInterval    time.Duration `yaml:"ws_ping_interval"`
	} `yaml:"api"`
	Database struct {
		URL string `yaml:"url"`
	} `yaml:"database"`
	Redis struct {
		Addr     string `yaml:"addr"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
	} `yaml:"redis"`
	Metrics []struct {
		Key  string `yaml:"key"`
		Tier string `yaml:"tier"`
	} `yaml:"metrics"`
	Synthetic struct {
		Enabled       bool `yaml:"enabled"`
		TickIntervalS int  `yaml:"tick_interval_s"`
		HistoryPoints int  `yaml:"history_points"`
		Entities      []struct {
			ID            string `yaml:"id"`
			Type          string `yaml:"type"`
			DisplayTicker string `yaml:"display_ticker"`
			ISIN          string `yaml:"isin"`
			Basket        string `yaml:"basket"`
		} `yaml:"entities"`
		Sources []string `yaml:"sources"`
	} `yaml:"synthetic"`
}

func main() {
	configPath := flag.String("config", "/etc/neyialiyorlar/neyialiyorlar.yaml", "Path to config file")
	healthcheck := flag.Bool("healthcheck", false, "run healthcheck and exit")
	flag.Parse()

	// Load config
	configData, err := os.ReadFile(*configPath)
	if err != nil {
		slog.Error("failed to read config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	var cfg Config
	if err := yaml.Unmarshal(configData, &cfg); err != nil {
		slog.Error("failed to parse config", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Init pretty logger early — all subsequent slog calls use this.
	sharedlog.New("api", "info")

	// Connect to database
	db, err := sql.Open("postgres", cfg.Database.URL)
	if err != nil {
		slog.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	// Test database connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := db.PingContext(ctx); err != nil {
		slog.Error("failed to ping database", slog.String("error", err.Error()))
		cancel()
		os.Exit(1)
	}
	cancel()

	// Connect to Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		slog.Error("failed to ping redis", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// If healthcheck flag is set, just verify config and exit
	if *healthcheck {
		slog.Info("healthcheck passed")
		os.Exit(0)
	}

	// Build the metric catalogue from config (key, display name, tier).
	catalog := buildCatalog(cfg)

	// Build the entity catalogue (securities + baskets) from config so the
	// dashboard can list every monitored stock and its fund membership.
	entities := buildEntities(cfg)

	// Postgres-backed store shared by the synthetic engine and the status endpoint.
	store := synthetic.NewPostgresStore(db)

	// Create API server
	apiServer := handler.NewServer(db, redisClient,
		handler.WithMaxBodySize(cfg.API.MaxBodySize),
		handler.WithMaxQueries(cfg.API.MaxQueries),
		handler.WithCatalog(catalog),
		handler.WithEntities(entities),
		handler.WithCounter(store),
	)

	// Root context cancelled on shutdown; drives the background engine goroutine.
	rootCtx, rootCancel := context.WithCancel(context.Background())
	defer rootCancel()

	// Synthetic dev data engine: backfills history and streams clearly-labelled
	// (approx) synthetic values so the full DB -> API -> WS -> dashboard path is
	// usable before the real harvester/analytic pipeline lands. Honest by design:
	// every value carries the `approx` confidence flag and the UI shows a banner.
	if cfg.Synthetic.Enabled {
		cache := synthetic.NewRedisCache(func(ctx context.Context, key, value string, ttl time.Duration) error {
			return redisClient.Set(ctx, key, value, ttl).Err()
		}, time.Hour)
		engine := synthetic.NewEngine(buildSyntheticConfig(cfg, catalog), store, apiServer.Hub(), cache, slog.Default())
		apiServer.SetTicker(engine)

		go func() {
			if err := engine.Backfill(rootCtx); err != nil {
				slog.Error("synthetic backfill failed", slog.String("error", err.Error()))
				return
			}
			engine.Run(rootCtx)
		}()
		slog.Info("synthetic data engine enabled", slog.Int("metrics", len(catalog)), slog.Int("entities", len(cfg.Synthetic.Entities)))
	}

	// Setup routes with middleware
	mux := http.NewServeMux()

	// Route handlers with versioned API path
	mux.HandleFunc("GET /api/v1/metrics", apiServer.ListMetrics)
	mux.HandleFunc("GET /api/v1/entities", apiServer.ListEntities)
	mux.HandleFunc("GET /api/v1/entities/{id}/signal", apiServer.GetEntitySignal)
	mux.HandleFunc("GET /api/v1/entities/{id}/metrics", apiServer.GetEntityMetrics)
	mux.HandleFunc("GET /api/v1/entities/{id}/funds", apiServer.GetEntityFunds)
	mux.HandleFunc("GET /api/v1/entities/{id}/presence", apiServer.GetEntityPresence)
	mux.HandleFunc("GET /api/v1/metrics/{key}/latest", apiServer.GetMetricLatest)
	mux.HandleFunc("GET /api/v1/metrics/{key}/top", apiServer.GetMetricTop)
	mux.HandleFunc("GET /api/v1/metrics/{key}/series", apiServer.GetMetricSeries)
	mux.HandleFunc("GET /api/v1/sources/health", apiServer.GetSourcesHealth)
	mux.HandleFunc("POST /api/v1/query", apiServer.QueryMetrics)
	mux.HandleFunc("GET /api/v1/admin/status", apiServer.AdminStatus)
	mux.HandleFunc("GET /api/v1/admin/scraper", apiServer.GetScraperStatus)
	mux.HandleFunc("GET /api/v1/admin/events", apiServer.GetAdminEvents)
	mux.HandleFunc("GET /api/v1/admin/drift", apiServer.GetAdminDrift)
	mux.HandleFunc("POST /api/v1/admin/drift/{id}/resolve", apiServer.ResolveAdminDrift)
	mux.HandleFunc("POST /api/v1/admin/tick", apiServer.AdminTick)
	mux.HandleFunc("/api/v1/ws", apiServer.ServeWS)

	// Middleware chain (applied in order: outermost first)
	var timeoutMw func(http.Handler) http.Handler
	if cfg.API.RequestTimeout > 0 {
		timeoutMw = middleware.Timeout(cfg.API.RequestTimeout)
	} else {
		timeoutMw = middleware.Timeout(30 * time.Second) // Default 30s
	}

	handler := middleware.Chain(
		mux,
		middleware.StructuredLogging,  // innermost: logs after responses
		timeoutMw,                     // timeout enforcement
		middleware.RequestIDInjection, // request ID (used in logs)
		middleware.PanicRecovery,      // outermost: catches panics
	)

	// Apply global middleware for CORS and security headers
	handler = globalMiddleware(handler, cfg)

	// Create HTTP server with timeouts
	srv := &http.Server{
		Addr:              ":8080",
		Handler:           handler,
		ReadHeaderTimeout: cfg.API.ReadHeaderTimeout,
		ReadTimeout:       cfg.API.ReadTimeout,
		WriteTimeout:      cfg.API.WriteTimeout,
		IdleTimeout:       cfg.API.IdleTimeout,
		MaxHeaderBytes:    cfg.API.MaxHeaderBytes,
	}

	// Graceful shutdown setup
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-sigChan
		slog.Info("received SIGTERM, shutting down gracefully")

		// Stop background engine goroutine first.
		rootCancel()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Close WS connections before server shutdown
		apiServer.CloseWS(time.Duration(cfg.API.WSDrainTimeout) * time.Second)

		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("error during server shutdown", slog.String("error", err.Error()))
		}
	}()

	slog.Info("starting API server", slog.String("addr", srv.Addr))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

// globalMiddleware applies CORS and security headers
func globalMiddleware(next http.Handler, cfg Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS (dev-only, permissive)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Prevent MIME sniffing on API JSON responses
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// NOTE: Content-Security-Policy belongs on the HTML document served by
		// nginx, NOT on JSON API responses. Setting CSP here caused Firefox to
		// reject same-origin XHR calls made by the Flutter app.

		// Handle OPTIONS preflight
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// buildCatalog turns the config metric list into the API metric catalogue.
// Display names are derived from the key (snake_case -> Title Case) and every
// entry defaults to the `approx` confidence flag until real data overrides it.
func buildCatalog(cfg Config) []model.MetricInfo {
	out := make([]model.MetricInfo, 0, len(cfg.Metrics))
	for _, m := range cfg.Metrics {
		t := model.CadenceTier(m.Tier)
		if t == "" {
			t = model.CadenceTierDaily // safe default if config omits tier
		}
		out = append(out, model.MetricInfo{
			Key:  m.Key,
			Name: displayName(m.Key),
			Flag: model.ConfidenceApprox,
			Tier: t,
		})
	}
	return out
}

// buildEntities turns the config entity list into the API entity catalogue
// (securities + baskets). It is the single source of truth for the monitored
// stock list and each security's fund/basket membership.
func buildEntities(cfg Config) []model.EntityInfo {
	out := make([]model.EntityInfo, 0, len(cfg.Synthetic.Entities))
	for _, e := range cfg.Synthetic.Entities {
		out = append(out, model.EntityInfo{
			ID:            e.ID,
			Type:          e.Type,
			DisplayTicker: e.DisplayTicker,
			ISIN:          e.ISIN,
			Basket:        e.Basket,
		})
	}
	return out
}

// buildSyntheticConfig maps the parsed config into the synthetic engine config.
func buildSyntheticConfig(cfg Config, catalog []model.MetricInfo) synthetic.Config {
	cat := make([]synthetic.Metric, 0, len(catalog))
	for _, m := range catalog {
		cat = append(cat, synthetic.Metric{Key: m.Key, Name: m.Name, Tier: m.Tier})
	}
	ents := make([]synthetic.Entity, 0, len(cfg.Synthetic.Entities))
	for _, e := range cfg.Synthetic.Entities {
		ents = append(ents, synthetic.Entity{
			ID:            e.ID,
			Type:          e.Type,
			DisplayTicker: e.DisplayTicker,
			ISIN:          e.ISIN,
		})
	}
	return synthetic.Config{
		Enabled:       cfg.Synthetic.Enabled,
		TickInterval:  time.Duration(cfg.Synthetic.TickIntervalS) * time.Second,
		HistoryPoints: cfg.Synthetic.HistoryPoints,
		Entities:      ents,
		Sources:       cfg.Synthetic.Sources,
		Catalog:       cat,
	}
}

// displayName converts a snake_case metric key into a Title Case label.
func displayName(key string) string {
	parts := strings.Split(key, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		r := []rune(p)
		r[0] = unicode.ToUpper(r[0])
		parts[i] = string(r)
	}
	return strings.Join(parts, " ")
}
