package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Env               string
	LogLevel          string
	DatabaseURL       string
	Port              string
	CORSAllowedOrigin string

	EnphaseAPIKey      string
	EnphaseSystemID    string
	EnphaseClientID    string
	EnphaseClientSecret string
	EnphaseAccessToken  string
	EnphaseRefreshToken string

	PollInterval time.Duration

	FakeProvider bool
	FakeSeed     int64
	FakeTimezone string // IANA timezone name for the fake data generator
}

func Load() (*Config, error) {
	fakeSeed, err := strconv.ParseInt(getEnv("FAKE_SEED", "0"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("config: invalid FAKE_SEED: %w", err)
	}

	cfg := &Config{
		Env:               getEnv("GO_ENV", "development"),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
		Port:              getEnv("PORT", "8080"),
		CORSAllowedOrigin: getEnv("CORS_ALLOWED_ORIGIN", "http://localhost:3000"),
		EnphaseAPIKey:       os.Getenv("ENPHASE_API_KEY"),
		EnphaseSystemID:     os.Getenv("ENPHASE_SYSTEM_ID"),
		EnphaseClientID:     os.Getenv("ENPHASE_CLIENT_ID"),
		EnphaseClientSecret: os.Getenv("ENPHASE_CLIENT_SECRET"),
		EnphaseAccessToken:  os.Getenv("ENPHASE_ACCESS_TOKEN"),
		EnphaseRefreshToken: os.Getenv("ENPHASE_REFRESH_TOKEN"),
		FakeProvider:      os.Getenv("FAKE_PROVIDER") == "true",
		FakeSeed:          fakeSeed,
		FakeTimezone:      getEnv("FAKE_TIMEZONE", "Australia/Sydney"),
	}

	cfg.DatabaseURL = requireEnv("DATABASE_URL")

	secs, err := strconv.Atoi(getEnv("POLL_INTERVAL_SECONDS", "300"))
	if err != nil {
		return nil, fmt.Errorf("config: invalid POLL_INTERVAL_SECONDS: %w", err)
	}
	cfg.PollInterval = time.Duration(secs) * time.Second

	return cfg, nil
}

func requireEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		panic(fmt.Sprintf("required environment variable %q is not set — check .env.example", key))
	}
	return v
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
