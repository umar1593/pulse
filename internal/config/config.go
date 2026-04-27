// Package config loads typed configuration from environment variables.
// Kept dependency-free on purpose; we'll graduate to a richer loader
// (caarlos0/env or viper) only if it pulls its weight.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTP HTTPConfig
	DB   DBConfig
}

type HTTPConfig struct {
	Addr            string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

type DBConfig struct {
	DSN               string
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	HealthCheckPeriod time.Duration
}

func Load() (Config, error) {
	dsn := os.Getenv("PULSE_DB_DSN")
	if dsn == "" {
		return Config{}, errors.New("PULSE_DB_DSN is required")
	}

	maxConns, err := envInt32("PULSE_DB_MAX_CONNS", 20)
	if err != nil {
		return Config{}, err
	}
	minConns, err := envInt32("PULSE_DB_MIN_CONNS", 2)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		HTTP: HTTPConfig{
			Addr:            envString("PULSE_HTTP_ADDR", ":8080"),
			ReadTimeout:     envDuration("PULSE_HTTP_READ_TIMEOUT", 5*time.Second),
			WriteTimeout:    envDuration("PULSE_HTTP_WRITE_TIMEOUT", 10*time.Second),
			ShutdownTimeout: envDuration("PULSE_SHUTDOWN_TIMEOUT", 15*time.Second),
		},
		DB: DBConfig{
			DSN:               dsn,
			MaxConns:          maxConns,
			MinConns:          minConns,
			MaxConnLifetime:   envDuration("PULSE_DB_MAX_CONN_LIFETIME", 30*time.Minute),
			HealthCheckPeriod: envDuration("PULSE_DB_HEALTH_CHECK_PERIOD", 30*time.Second),
		},
	}
	return cfg, nil
}

func envString(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt32(key string, def int32) (int32, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("env %s: %w", key, err)
	}
	return int32(n), nil
}

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
