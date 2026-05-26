package app

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr       string
	PostgresDSN    string
	RedisAddr      string
	RedisPassword  string
	RedisDB        int
	BaseURL        string
	SessionKey     string
	RedirectPrefix string
	CacheTTL       time.Duration
}

func LoadConfig() (Config, error) {
	cfg := Config{
		HTTPAddr:       env("HTTP_ADDR", ":8080"),
		PostgresDSN:    env("POSTGRES_DSN", "postgres://postgres:postgres@localhost:5432/shorte?sslmode=disable"),
		RedisAddr:      env("REDIS_ADDR", "localhost:6379"),
		RedisPassword:  env("REDIS_PASSWORD", ""),
		RedisDB:        envInt("REDIS_DB", 0),
		BaseURL:        env("BASE_URL", "http://localhost:8080"),
		SessionKey:     env("SESSION_KEY", "change-me-32-bytes-minimum"),
		RedirectPrefix: env("REDIRECT_PREFIX", "/r/"),
		CacheTTL:       envDuration("CACHE_TTL", 24*time.Hour),
	}
	if len(cfg.SessionKey) < 16 {
		return Config{}, fmt.Errorf("SESSION_KEY must be at least 16 chars")
	}
	return cfg, nil
}

func env(k, d string) string {
	v := os.Getenv(k)
	if v == "" {
		return d
	}
	return v
}

func envInt(k string, d int) int {
	v := os.Getenv(k)
	if v == "" {
		return d
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return d
	}
	return n
}

func envDuration(k string, d time.Duration) time.Duration {
	v := os.Getenv(k)
	if v == "" {
		return d
	}
	dd, err := time.ParseDuration(v)
	if err != nil {
		return d
	}
	return dd
}
