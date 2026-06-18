package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port          string
	ImportWorkers int
	CORSOrigin    string
}

func Load() Config {
	return Config{
		Port:          getEnv("PORT", "8080"),
		ImportWorkers: getEnvInt("IMPORT_WORKERS", 20),
		CORSOrigin:    getEnv("CORS_ORIGIN", "*"),
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getEnvInt(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
