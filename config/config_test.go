package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	os.Setenv("DB_PATH", "/tmp/test.db")
	os.Setenv("SERVER_PORT", "9090")
	os.Setenv("READ_TIMEOUT", "10s")
	os.Setenv("WRITE_TIMEOUT", "20s")
	os.Setenv("IDLE_TIMEOUT", "30s")
	os.Setenv("TRANSCRIBE_TIMEOUT", "5m")
	os.Setenv("RATE_LIMIT", "10")
	os.Setenv("RATE_LIMIT_INTERVAL", "2s")
	os.Setenv("MODEL_NAME", "large.en")

	cfg := LoadConfig()

	if cfg.DBPath != "/tmp/test.db" {
		t.Errorf("expected /tmp/test.db, got %s", cfg.DBPath)
	}
	if cfg.ServerPort != "9090" {
		t.Errorf("expected 9090, got %s", cfg.ServerPort)
	}
	if cfg.ReadTimeout != 10*time.Second {
		t.Errorf("expected 10s, got %s", cfg.ReadTimeout)
	}
	if cfg.WriteTimeout != 20*time.Second {
		t.Errorf("expected 20s, got %s", cfg.WriteTimeout)
	}
	if cfg.IdleTimeout != 30*time.Second {
		t.Errorf("expected 30s, got %s", cfg.IdleTimeout)
	}
	if cfg.TranscribeTimeout != 5*time.Minute {
		t.Errorf("expected 5m, got %s", cfg.TranscribeTimeout)
	}
	if cfg.RateLimit != 10 {
		t.Errorf("expected 10, got %d", cfg.RateLimit)
	}
	if cfg.RateLimitInterval != 2*time.Second {
		t.Errorf("expected 2s, got %s", cfg.RateLimitInterval)
	}
	if cfg.ModelName != "large.en" {
		t.Errorf("expected large.en, got %s", cfg.ModelName)
	}
}
