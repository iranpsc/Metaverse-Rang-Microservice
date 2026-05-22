package config

import (
	"os"
)

// Config holds all configuration for the commercial service
type Config struct {
	Database DatabaseConfig
	Parsian  ParsianConfig
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

// ParsianConfig holds Parsian payment gateway configuration
// Matches Laravel's config/parsian.php
type ParsianConfig struct {
	MerchantID            string // Regular merchant ID
	PIN                   string // PIN for regular merchant
	CallbackURL           string // Callback URL for payment gateway
	LoanAccountMerchantID string // Loan account merchant ID (for IRR)
	LoanAccountPIN        string // PIN for loan account
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
			Database: getEnv("DB_DATABASE", "metargb"),
		},
		Parsian: ParsianConfig{
			MerchantID:            getEnv("PARSIAN_MERCHANT_ID", ""),
			PIN:                   getEnv("PARSIAN_PIN", ""),
			CallbackURL:           getEnv("PARSIAN_CALLBACK_URL", ""),
			LoanAccountMerchantID: getEnv("PARSIAN_LOAN_ACCOUNT_MERCHANT_ID", ""),
			LoanAccountPIN:        getEnv("PARSIAN_LOAN_ACCOUNT_PIN", ""),
		},
		Server: ServerConfig{
			GRPCPort: getEnv("GRPC_PORT", "50052"),
			HTTPPort: getEnv("HTTP_PORT", "8080"),
			Locale:   getEnv("LOCALE", "en"),
		},
	}
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
