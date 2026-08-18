package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	keys := []string{"ADDR", "DATABASE_URL", "JWT_SECRET", "NETWORK_NAME", "IDLE_TIMEOUT_MIN", "BATCH_CONCURRENCY", "COOKIE_SECURE"}
	for _, k := range keys {
		os.Unsetenv(k)
	}
	cfg := Load()
	if cfg.Addr != ":8080" {
		t.Fatalf("default Addr = %q", cfg.Addr)
	}
	if cfg.NetworkName != "devcapsule_user-net" {
		t.Fatalf("default NetworkName = %q", cfg.NetworkName)
	}
	if cfg.IdleTimeoutMin != 30 {
		t.Fatalf("default IdleTimeoutMin = %d", cfg.IdleTimeoutMin)
	}
	if cfg.BatchConcurrency != 5 {
		t.Fatalf("default BatchConcurrency = %d", cfg.BatchConcurrency)
	}
	if cfg.SecureCookies {
		t.Fatal("SecureCookies should default to false")
	}
}

func TestLoadFromEnv(t *testing.T) {
	os.Setenv("ADDR", ":9090")
	os.Setenv("NETWORK_NAME", "custom-net")
	os.Setenv("IDLE_TIMEOUT_MIN", "60")
	os.Setenv("BATCH_CONCURRENCY", "10")
	defer os.Unsetenv("ADDR")
	defer os.Unsetenv("NETWORK_NAME")
	defer os.Unsetenv("IDLE_TIMEOUT_MIN")
	defer os.Unsetenv("BATCH_CONCURRENCY")

	cfg := Load()
	if cfg.Addr != ":9090" {
		t.Fatalf("Addr = %q", cfg.Addr)
	}
	if cfg.NetworkName != "custom-net" {
		t.Fatalf("NetworkName = %q", cfg.NetworkName)
	}
	if cfg.IdleTimeoutMin != 60 {
		t.Fatalf("IdleTimeoutMin = %d", cfg.IdleTimeoutMin)
	}
	if cfg.BatchConcurrency != 10 {
		t.Fatalf("BatchConcurrency = %d", cfg.BatchConcurrency)
	}
}

func TestLoadCookieSecureEnv(t *testing.T) {
	os.Setenv("COOKIE_SECURE", "1")
	defer os.Unsetenv("COOKIE_SECURE")
	if cfg := Load(); !cfg.SecureCookies {
		t.Fatal("COOKIE_SECURE=1 should enable SecureCookies")
	}
	os.Setenv("COOKIE_SECURE", "0")
	if cfg := Load(); cfg.SecureCookies {
		t.Fatal("COOKIE_SECURE=0 should disable SecureCookies")
	}
	os.Setenv("COOKIE_SECURE", "bogus")
	if cfg := Load(); cfg.SecureCookies {
		t.Fatal("invalid COOKIE_SECURE should fall back to default false")
	}
}

func TestLoadInvalidIntFallsBack(t *testing.T) {
	os.Setenv("IDLE_TIMEOUT_MIN", "not-a-number")
	defer os.Unsetenv("IDLE_TIMEOUT_MIN")
	cfg := Load()
	if cfg.IdleTimeoutMin != 30 {
		t.Fatalf("expected default 30, got %d", cfg.IdleTimeoutMin)
	}
}
