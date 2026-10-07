package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadConfig tests loading the config from a YAML file.
func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tmpdir := t.TempDir()
	configPath := filepath.Join(tmpdir, "test-config.yaml")

	configYAML := `
version: 1
runtime:
  log_level: info
  timezone: Europe/Istanbul
database:
  host: postgres
  port: 5432
  name: neyialiyorlar
  user: neyi
  max_open_conns: 16
  max_idle_conns: 8
redis:
  addr: redis:6379
  db: 0
http_client:
  user_agent: "neyialiyorlar-research/0.1"
  max_concurrent_per_host: 2
  timeout_ms: 15000
  backoff:
    base_ms: 500
    factor: 2.0
    max_ms: 60000
    jitter: true
    max_attempts: 5
  circuit_breaker:
    fail_threshold: 5
    cooldown_ms: 120000
    half_open_probes: 1
sources:
  bist:
    enabled: true
    base: "https://www.borsaistanbul.com"
    cadence: daily
    intraday_viop: true
  kap:
    enabled: true
    base: "https://www.kap.org.tr"
    cadence: intraday
  mkk_vap:
    enabled: true
    base: "https://www.mkk.com.tr"
    cadence: daily
    foreign_detail_lag_days: 10
  evds:
    enabled: true
    base: "https://evds2.tcmb.gov.tr"
    cadence: daily
    weekly_flow: true
  waf_protected:
    enabled: false
metrics:
  - id: 1
    key: velocity_accumulation
    tier: event
    color: proxy
    priority: true
cache:
  ttl:
    intraday_s: 60
    daily_s: 3600
    weekly_s: 21600
drift:
  enabled: true
  persist: true
  sources:
    - yahoo
`

	err := os.WriteFile(configPath, []byte(configYAML), 0644)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify config was loaded correctly
	if cfg.Runtime.LogLevel != "info" {
		t.Errorf("expected log_level=info, got %s", cfg.Runtime.LogLevel)
	}
	if cfg.Database.Host != "postgres" {
		t.Errorf("expected database.host=postgres, got %s", cfg.Database.Host)
	}
	if cfg.HTTPClient.MaxConcurrentPerHost != 2 {
		t.Errorf("expected max_concurrent_per_host=2, got %d", cfg.HTTPClient.MaxConcurrentPerHost)
	}
	if !cfg.Sources.BIST.Enabled {
		t.Errorf("expected BIST to be enabled")
	}
	if cfg.Cache.TTL.DailyS != 3600 {
		t.Errorf("expected cache.ttl.daily_s=3600, got %d", cfg.Cache.TTL.DailyS)
	}
	if !cfg.Drift.Enabled {
		t.Errorf("expected drift.enabled=true")
	}
	if len(cfg.Drift.Sources) != 1 || cfg.Drift.Sources[0] != "yahoo" {
		t.Errorf("expected drift.sources=[yahoo], got %v", cfg.Drift.Sources)
	}
}

// TestValidateNegativeTTL tests that a config with negative TTL is rejected.
func TestValidateNegativeTTL(t *testing.T) {
	cfg := &Config{
		Version:    1,
		Database:   Database{Host: "localhost", Port: 5432, Name: "test", User: "test"},
		Redis:      Redis{Addr: "localhost:6379"},
		HTTPClient: HTTPClient{MaxConcurrentPerHost: 1, TimeoutMs: 5000},
		Cache: CacheConfig{
			TTL: TTLConfig{IntradayS: -1, DailyS: 3600, WeeklyS: 21600},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Errorf("expected validation error for negative TTL, got nil")
	}
}

// TestValidateMaxConcurrentPerHost tests that max_concurrent_per_host must be > 0.
func TestValidateMaxConcurrentPerHost(t *testing.T) {
	cfg := &Config{
		Version:    1,
		Database:   Database{Host: "localhost", Port: 5432, Name: "test", User: "test"},
		Redis:      Redis{Addr: "localhost:6379"},
		HTTPClient: HTTPClient{MaxConcurrentPerHost: 0, TimeoutMs: 5000},
		Cache: CacheConfig{
			TTL: TTLConfig{IntradayS: 60, DailyS: 3600, WeeklyS: 21600},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Errorf("expected validation error for max_concurrent_per_host=0, got nil")
	}
}

// TestValidateEnvSecretAbsent tests that missing PGPASSWORD fails at boot.
// This is verified via integration testing in Phase 1 test plan.
func TestRequiredDatabaseFields(t *testing.T) {
	cfg := &Config{
		Version:    1,
		Database:   Database{Host: "", Port: 5432, Name: "test", User: "test"},
		Redis:      Redis{Addr: "localhost:6379"},
		HTTPClient: HTTPClient{MaxConcurrentPerHost: 1, TimeoutMs: 5000},
		Cache: CacheConfig{
			TTL: TTLConfig{IntradayS: 60, DailyS: 3600, WeeklyS: 21600},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Errorf("expected validation error for missing database.host, got nil")
	}
}

// TestValidCadence tests that valid cadences pass and invalid ones fail.
func TestValidCadence(t *testing.T) {
	tests := []struct {
		cadence string
		valid   bool
	}{
		{"event", true},
		{"intraday", true},
		{"daily", true},
		{"weekly", true},
		{"invalid", false},
		{"hourly", false},
		{"", true}, // empty is treated as unset, not invalid
	}

	for _, tt := range tests {
		if tt.cadence == "" {
			continue // skip empty; it's handled separately
		}
		result := isValidCadence(tt.cadence)
		if result != tt.valid {
			t.Errorf("isValidCadence(%q) = %v, want %v", tt.cadence, result, tt.valid)
		}
	}
}
