package config

import (
	"os"
	"strings"
)

type Config struct {
	HTTPPort                string
	AuthServiceAddr         string
	CalendarServiceAddr     string
	DynastyServiceAddr      string
	FeaturesServiceAddr     string
	FinancialServiceAddr    string
	SocialServiceAddr       string
	LevelsServiceAddr       string
	TrainingServiceAddr     string
	SupportServiceAddr      string
	NotificationServiceAddr string
	StorageServiceAddr      string
	Locale                  string
	AppURL                  string
}

func Load() *Config {
	locale := getEnv("PROJECT_LOCALE", getEnv("LOCALE", "en"))
	locale = strings.ToLower(locale)
	if locale != "fa" && locale != "en" {
		locale = "en"
	}
	return &Config{
		HTTPPort:                getEnv("HTTP_PORT", "8080"),
		AuthServiceAddr:         getEnv("AUTH_SERVICE_ADDR", "auth-service:50051"),
		CalendarServiceAddr:     getEnv("CALENDAR_SERVICE_ADDR", "calendar-service:50059"),
		DynastyServiceAddr:      getEnv("DYNASTY_SERVICE_ADDR", "dynasty-service:50055"),
		FeaturesServiceAddr:     getEnv("FEATURES_SERVICE_ADDR", "features-service:50053"),
		FinancialServiceAddr:    getEnv("FINANCIAL_SERVICE_ADDR", "financial-service:50062"),
		SocialServiceAddr:       getEnv("SOCIAL_SERVICE_ADDR", "social-service:50061"),
		LevelsServiceAddr:       getEnv("LEVELS_SERVICE_ADDR", "levels-service:50054"),
		TrainingServiceAddr:     getEnv("TRAINING_SERVICE_ADDR", "training-service:50057"),
		SupportServiceAddr:      getEnv("SUPPORT_SERVICE_ADDR", "support-service:50056"),
		NotificationServiceAddr: getEnv("NOTIFICATION_SERVICE_ADDR", "notifications-service:50058"),
		StorageServiceAddr:      getEnv("STORAGE_SERVICE_ADDR", "storage-service:8059"),
		Locale:                  locale,
		AppURL:                  getEnv("APP_URL", ""),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
