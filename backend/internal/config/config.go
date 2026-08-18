package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Addr             string
	DatabaseURL      string
	JWTSecret        string
	NetworkName      string
	IdleTimeoutMin   int
	BatchConcurrency int
	SecureCookies    bool
}

func Load() Config {
	return Config{
		Addr:             getEnv("ADDR", ":8080"),
		DatabaseURL:      getEnv("DATABASE_URL", "postgres://opencode:opencode@localhost:5432/opencode?sslmode=disable"),
		JWTSecret:        getEnv("JWT_SECRET", "dev-secret-change-me"),
		NetworkName:      getEnv("NETWORK_NAME", "devcapsule_user-net"),
		IdleTimeoutMin:   getEnvInt("IDLE_TIMEOUT_MIN", 30),
		BatchConcurrency: getEnvInt("BATCH_CONCURRENCY", 5),
		// Set COOKIE_SECURE=1 when the platform is served over HTTPS (e.g.
		// behind a TLS-terminating proxy). Kept off by default so the local
		// plain-HTTP dev setup keeps working.
		SecureCookies: getEnvBool("COOKIE_SECURE", false),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "on":
			return true
		case "0", "false", "no", "off":
			return false
		}
	}
	return def
}
