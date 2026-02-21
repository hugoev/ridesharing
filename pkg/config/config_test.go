package config

import (
	"os"
	"testing"
	"time"
)

func TestLoad_Defaults(t *testing.T) {
	cfg := Load()

	if cfg.ServerPort != "8080" {
		t.Errorf("ServerPort = %q, want 8080", cfg.ServerPort)
	}
	if cfg.RedisAddr != "localhost:6379" {
		t.Errorf("RedisAddr = %q, want localhost:6379", cfg.RedisAddr)
	}
	if cfg.RedisDB != 0 {
		t.Errorf("RedisDB = %d, want 0", cfg.RedisDB)
	}
	if cfg.RateLimit != 100 {
		t.Errorf("RateLimit = %v, want 100", cfg.RateLimit)
	}
	if cfg.RateBurst != 200 {
		t.Errorf("RateBurst = %d, want 200", cfg.RateBurst)
	}
	if cfg.JWTExpiration != 24*time.Hour {
		t.Errorf("JWTExpiration = %v, want 24h", cfg.JWTExpiration)
	}
	if len(cfg.KafkaBrokers) != 1 || cfg.KafkaBrokers[0] != "localhost:9092" {
		t.Errorf("KafkaBrokers = %v, want [localhost:9092]", cfg.KafkaBrokers)
	}
}

func TestLoad_OverrideFromEnv(t *testing.T) {
	os.Setenv("SERVER_PORT", "9090")
	os.Setenv("REDIS_DB", "3")
	os.Setenv("RATE_LIMIT", "50.5")
	os.Setenv("JWT_EXPIRATION", "2h")
	defer func() {
		os.Unsetenv("SERVER_PORT")
		os.Unsetenv("REDIS_DB")
		os.Unsetenv("RATE_LIMIT")
		os.Unsetenv("JWT_EXPIRATION")
	}()

	cfg := Load()

	if cfg.ServerPort != "9090" {
		t.Errorf("ServerPort = %q, want 9090", cfg.ServerPort)
	}
	if cfg.RedisDB != 3 {
		t.Errorf("RedisDB = %d, want 3", cfg.RedisDB)
	}
	if cfg.RateLimit != 50.5 {
		t.Errorf("RateLimit = %v, want 50.5", cfg.RateLimit)
	}
	if cfg.JWTExpiration != 2*time.Hour {
		t.Errorf("JWTExpiration = %v, want 2h", cfg.JWTExpiration)
	}
}

func TestLoad_InvalidEnvFallsBackToDefault(t *testing.T) {
	os.Setenv("REDIS_DB", "not-a-number")
	os.Setenv("RATE_LIMIT", "also-not-a-number")
	os.Setenv("JWT_EXPIRATION", "invalid-duration")
	defer func() {
		os.Unsetenv("REDIS_DB")
		os.Unsetenv("RATE_LIMIT")
		os.Unsetenv("JWT_EXPIRATION")
	}()

	cfg := Load()

	if cfg.RedisDB != 0 {
		t.Errorf("RedisDB = %d, want 0 (fallback)", cfg.RedisDB)
	}
	if cfg.RateLimit != 100 {
		t.Errorf("RateLimit = %v, want 100 (fallback)", cfg.RateLimit)
	}
	if cfg.JWTExpiration != 24*time.Hour {
		t.Errorf("JWTExpiration = %v, want 24h (fallback)", cfg.JWTExpiration)
	}
}
