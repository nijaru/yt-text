package config

import (
	"log"
	"os"
	"time"
)

type Config struct {
	DBPath            string
	ServerPort        string
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	TranscribeTimeout time.Duration
}

func LoadConfig() *Config {
	return &Config{
		DBPath:            getEnv("DB_PATH", "./urls.db"),
		ServerPort:        getEnv("SERVER_PORT", "8080"),
		ReadTimeout:       getEnvAsDuration("READ_TIMEOUT", 10*time.Second),
		WriteTimeout:      getEnvAsDuration("WRITE_TIMEOUT", 10*time.Second),
		IdleTimeout:       getEnvAsDuration("IDLE_TIMEOUT", 30*time.Second),
		TranscribeTimeout: getEnvAsDuration("TRANSCRIBE_TIMEOUT", 2*time.Minute),
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
		log.Printf("Invalid duration for %s: %s, using default: %v", key, value, defaultValue)
	}
	return defaultValue
}
