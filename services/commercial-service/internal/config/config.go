// Package config loads configuration for the commercial service.
package config

import (
	"os"
)

// Config holds all configuration for the commercial service
type Config struct {
	Database DatabaseConfig
	Server   ServerConfig
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

// ServerConfig holds server configuration
type ServerConfig struct {
	GRPCPort string
	HTTPPort string
	Locale   string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() *Config {
	return &Config{
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "3306"),
			User:     getEnv("DB_USER", "root"),
			Password: getEnv("DB_PASSWORD", ""),
			Database: getEnv("DB_DATABASE", "metarang"),
		},
		Server: ServerConfig{
			GRPCPort: getEnv("GRPC_PORT", "50052"),
			HTTPPort: getEnv("HTTP_PORT", "8080"),
			Locale:   getEnv("LOCALE", "en"),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
