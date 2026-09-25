package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// Config is the sitemap generator process configuration.
type Config struct {
	DBHost        string
	DBPort        string
	DBUser        string
	DBPassword    string
	DBDatabase    string
	TemplatesPath string
	OutputDir     string
	ExportPath    string
	AdminPanelURL string
	HTTPPort      string
	RunOnce       bool
}

// Load reads config.env (when present) and environment variables.
func Load() Config {
	loadEnvFile()
	root := serviceRoot()
	output := getenv("SITEMAPS_OUTPUT_DIR", "")
	if output == "" {
		output = filepath.Join(root, "sitemaps")
	}
	templates := getenv("TEMPLATES_PATH", "")
	if templates == "" {
		templates = filepath.Join(root, "templates.json")
	}
	return Config{
		DBHost:        getenv("DB_HOST", "localhost"),
		DBPort:        getenv("DB_PORT", "3306"),
		DBUser:        getenv("DB_USER", "root"),
		DBPassword:    os.Getenv("DB_PASSWORD"),
		DBDatabase:    getenv("DB_DATABASE", "metarang_db"),
		TemplatesPath: templates,
		OutputDir:     output,
		ExportPath:    getenv("SITEMAPS_EXPORT_PATH", "/opt/metarang/sitemaps"),
		AdminPanelURL: getenv("ADMIN_PANEL_URL", "https://admin.rgb.irpsc.com"),
		HTTPPort:      getenv("HTTP_PORT", "8071"),
		RunOnce:       os.Getenv("SITEMAP_RUN_ONCE") == "true" || os.Getenv("SITEMAP_RUN_ONCE") == "1",
	}
}

// DSN is the MySQL connection string for the shared schema.
func (c Config) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci&loc=Local",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBDatabase)
}

func loadEnvFile() {
	candidates := []string{
		"services/sitemap-generator-service/config.env",
		"config.env",
		"./config.env",
	}
	for _, path := range candidates {
		if err := godotenv.Load(path); err == nil {
			return
		}
	}
}

func serviceRoot() string {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if _, err := os.Stat(filepath.Join(dir, "templates.json")); err == nil {
			return dir
		}
	}
	if _, err := os.Stat("templates.json"); err == nil {
		return "."
	}
	return "."
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
