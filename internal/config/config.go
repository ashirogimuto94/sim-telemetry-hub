package config

import (
	"os"
	"strconv"
)

type Config struct {
	ServerPort     string
	DBHost         string
	DBPort         string
	DBUser         string
	DBPassword     string
	DBName         string
	DBSSLMode      string
	WorkerPoolSize int
	JobQueueBuffer int
}

func Load() *Config {
	return &Config{
		ServerPort:     getEnv("SERVER_PORT", "8080"),
		DBHost:         getEnv("DB_HOST", "localhost"),
		DBPort:         getEnv("DB_PORT", "5432"),
		DBUser:         getEnv("DB_USER", "simuser"),
		DBPassword:     getEnv("DB_PASSWORD", "simsecret"),
		DBName:         getEnv("DB_NAME", "simtelemetry"),
		DBSSLMode:      getEnv("DB_SSLMODE", "disable"),
		WorkerPoolSize: getEnvAsInt("WORKER_POOL_SIZE", 10),
		JobQueueBuffer: getEnvAsInt("JOB_QUEUE_BUFFER", 1000),
	}
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	valStr := getEnv(key, "")
	if val, err := strconv.Atoi(valStr); err == nil {
		return val
	}
	return fallback
}
