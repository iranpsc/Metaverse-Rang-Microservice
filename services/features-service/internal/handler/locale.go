package handler

import (
	"strings"
)

// projectLocale holds the PROJECT_LOCALE from config, set at startup
var projectLocale string

// SetProjectLocale sets the project locale for all features-service handlers
func SetProjectLocale(locale string) {
	locale = strings.ToLower(strings.TrimSpace(locale))
	if locale != "fa" && locale != "en" {
		locale = "en"
	}
	projectLocale = locale
}

// GetProjectLocale returns the project locale, defaulting to "en"
func GetProjectLocale() string {
	if projectLocale == "" {
		return "en"
	}
	return projectLocale
}
