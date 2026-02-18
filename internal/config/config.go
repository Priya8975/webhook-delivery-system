package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the application.
type Config struct {
	Port        string
	DatabaseURL string
	RedisURL    string
	NumWorkers  int
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	port := getEnv("PORT", "8080")
	dbURL := getEnv("DATABASE_URL", "")
	redisURL := getEnv("REDIS_URL", "")
	numWorkers := getEnvInt("NUM_WORKERS", 50)

	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if redisURL == "" {
		return nil, fmt.Errorf("REDIS_URL is required")
	}

	return &Config{
		Port:        port,
		DatabaseURL: dbURL,
		RedisURL:    redisURL,
		NumWorkers:  numWorkers,
	}, nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		n, err := strconv.Atoi(val)
		if err == nil {
			return n
		}
	}
	return fallback
}
