package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration struct.
type Config struct {
	Version    int           `yaml:"version"`
	Runtime    Runtime       `yaml:"runtime"`
	Database   Database      `yaml:"database"`
	Redis      Redis         `yaml:"redis"`
	HTTPClient HTTPClient    `yaml:"http_client"`
	Sources    SourcesConfig `yaml:"sources"`
	Metrics    []Metric      `yaml:"metrics"`
	Cache      CacheConfig   `yaml:"cache"`
	Drift      DriftConfig   `yaml:"drift"`
}

// Runtime configuration.
type Runtime struct {
	LogLevel string `yaml:"log_level"`
	Timezone string `yaml:"timezone"`
}

// Database configuration.
type Database struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	Name         string `yaml:"name"`
	User         string `yaml:"user"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
}

// Redis configuration.
type Redis struct {
	Addr string `yaml:"addr"`
	DB   int    `yaml:"db"`
}

// HTTPClient configuration.
type HTTPClient struct {
	UserAgent            string               `yaml:"user_agent"`
	MaxConcurrentPerHost int                  `yaml:"max_concurrent_per_host"`
	TimeoutMs            int                  `yaml:"timeout_ms"`
	Backoff              BackoffConfig        `yaml:"backoff"`
	CircuitBreaker       CircuitBreakerConfig `yaml:"circuit_breaker"`
}

// BackoffConfig for exponential backoff.
type BackoffConfig struct {
	BaseMs      int     `yaml:"base_ms"`
	Factor      float64 `yaml:"factor"`
	MaxMs       int     `yaml:"max_ms"`
	Jitter      bool    `yaml:"jitter"`
	MaxAttempts int     `yaml:"max_attempts"`
}

// CircuitBreakerConfig for circuit breaker.
type CircuitBreakerConfig struct {
	FailThreshold  int `yaml:"fail_threshold"`
	CooldownMs     int `yaml:"cooldown_ms"`
	HalfOpenProbes int `yaml:"half_open_probes"`
}

// SourcesConfig maps source names to their configs.
type SourcesConfig struct {
	BIST         SourceConfig `yaml:"bist"`
	KAP          SourceConfig `yaml:"kap"`
	MKKVAP       SourceConfig `yaml:"mkk_vap"`
	EVDS         SourceConfig `yaml:"evds"`
	WAFProtected SourceConfig `yaml:"waf_protected"`
}

// SourceConfig is the configuration for a single data source.
type SourceConfig struct {
	Enabled              bool   `yaml:"enabled"`
	Base                 string `yaml:"base"`
	Cadence              string `yaml:"cadence"`
	IntradayVIOP         bool   `yaml:"intraday_viop"`
	ForeignDetailLagDays int    `yaml:"foreign_detail_lag_days"`
	WeeklyFlow           bool   `yaml:"weekly_flow"`
}

// Metric defines a metric configuration.
type Metric struct {
	ID          int    `yaml:"id"`
	Key         string `yaml:"key"`
	Tier        string `yaml:"tier"`
	Color       string `yaml:"color"`
	Priority    bool   `yaml:"priority"`
	Speculative bool   `yaml:"speculative"`
}

// CacheConfig defines cache TTLs.
type CacheConfig struct {
	TTL TTLConfig `yaml:"ttl"`
}

// TTLConfig defines time-to-live values.
type TTLConfig struct {
	IntradayS int `yaml:"intraday_s"`
	DailyS    int `yaml:"daily_s"`
	WeeklyS   int `yaml:"weekly_s"`
}

// DriftConfig gates schema/DOM drift-alert persistence. When Enabled, the
// harvester fingerprints upstream payloads and records BREAKING structural
// changes to the drift_alert table (surfaced on the İzleme Paneli); the
// alerts persist until resolved. Sources is advisory documentation of which
// adapters are instrumented. Omitting the block leaves drift persistence off.
type DriftConfig struct {
	Enabled bool     `yaml:"enabled"`
	Persist bool     `yaml:"persist"`
	Sources []string `yaml:"sources"`
}

// Load reads a YAML config file and overlays environment variables.
func Load(configPath string) (*Config, error) {
	// Read YAML file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate required configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// Validate checks for required fields and valid value ranges.
func (c *Config) Validate() error {
	// Check database config
	if c.Database.Host == "" {
		return fmt.Errorf("database.host is required")
	}
	if c.Database.Port <= 0 {
		return fmt.Errorf("database.port must be > 0")
	}
	if c.Database.Name == "" {
		return fmt.Errorf("database.name is required")
	}
	if c.Database.User == "" {
		return fmt.Errorf("database.user is required")
	}

	// Check Redis config
	if c.Redis.Addr == "" {
		return fmt.Errorf("redis.addr is required")
	}

	// Check HTTP client config
	if c.HTTPClient.MaxConcurrentPerHost <= 0 {
		return fmt.Errorf("http_client.max_concurrent_per_host must be > 0")
	}
	if c.HTTPClient.TimeoutMs <= 0 {
		return fmt.Errorf("http_client.timeout_ms must be > 0")
	}

	// Check sources config
	if c.Sources.BIST.Cadence != "" {
		if !isValidCadence(c.Sources.BIST.Cadence) {
			return fmt.Errorf("invalid cadence: %s", c.Sources.BIST.Cadence)
		}
	}

	// Check cache config
	if c.Cache.TTL.IntradayS <= 0 {
		return fmt.Errorf("cache.ttl.intraday_s must be > 0")
	}
	if c.Cache.TTL.DailyS <= 0 {
		return fmt.Errorf("cache.ttl.daily_s must be > 0")
	}
	if c.Cache.TTL.WeeklyS <= 0 {
		return fmt.Errorf("cache.ttl.weekly_s must be > 0")
	}

	return nil
}

// isValidCadence checks if a cadence string is valid.
func isValidCadence(cadence string) bool {
	validCadences := map[string]bool{
		"event":    true,
		"intraday": true,
		"daily":    true,
		"weekly":   true,
	}
	return validCadences[cadence]
}
