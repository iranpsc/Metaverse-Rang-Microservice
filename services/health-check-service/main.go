package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9/maintnotifications"
	"github.com/redis/go-redis/v9"
)

// ServiceStatus represents the health status of a service
type ServiceStatus struct {
	Service string `json:"service"`
	Status  string `json:"status"`
	Host    string `json:"host,omitempty"`
	Port    int    `json:"port,omitempty"`
	URL     string `json:"url,omitempty"`
	Error   string `json:"error,omitempty"`
	Latency string `json:"latency,omitempty"`
}

// DependencyHealth represents health of external dependencies
type DependencyHealth struct {
	DatabaseConnections  map[string]DBConnectionStatus `json:"database_connections"` // Map of service name to DB connection status
	CacheMetrics         CacheMetrics                  `json:"cache_metrics"`
	ExternalAPIs         []ExternalAPIStatus           `json:"external_apis"`
	ThirdPartyServices   []ThirdPartyService           `json:"third_party_services"`
	CircuitBreakerStatus map[string]string             `json:"circuit_breaker_status,omitempty"`
}

// DBConnectionStatus represents database connection health
type DBConnectionStatus struct {
	Status    string `json:"status"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Database  string `json:"database"`
	Connected bool   `json:"connected"`
	Latency   string `json:"latency,omitempty"`
	Error     string `json:"error,omitempty"`
	PoolStats struct {
		OpenConnections int `json:"open_connections"`
		InUse           int `json:"in_use"`
		Idle            int `json:"idle"`
	} `json:"pool_stats,omitempty"`
}

// CacheMetrics represents Redis cache performance metrics
type CacheMetrics struct {
	Status      string  `json:"status"`
	HitRate     float64 `json:"hit_rate"`  // Percentage
	MissRate    float64 `json:"miss_rate"` // Percentage
	Hits        int64   `json:"hits"`
	Misses      int64   `json:"misses"`
	MemoryUsage int64   `json:"memory_usage_bytes"`
	Latency     string  `json:"latency,omitempty"`
	Error       string  `json:"error,omitempty"`
}

// ExternalAPIStatus represents external API availability
type ExternalAPIStatus struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Status    string `json:"status"`
	Latency   string `json:"latency,omitempty"`
	Error     string `json:"error,omitempty"`
	LastCheck string `json:"last_check"`
}

// ThirdPartyService represents third-party service health
type ThirdPartyService struct {
	Name         string `json:"name"`
	Status       string `json:"status"`
	ResponseTime string `json:"response_time"`
	Error        string `json:"error,omitempty"`
	LastCheck    string `json:"last_check"`
}

// ServiceUptime tracks uptime and downtime for a service
type ServiceUptime struct {
	ServiceName       string
	FirstSeen         time.Time
	LastSeen          time.Time
	LastStatus        string
	TotalUptime       time.Duration
	TotalDowntime     time.Duration
	DowntimeIncidents []DowntimeIncident
	mu                sync.RWMutex
}

// DowntimeIncident tracks a single downtime event
type DowntimeIncident struct {
	StartTime time.Time     `json:"start_time"`
	EndTime   time.Time     `json:"end_time,omitempty"`
	Duration  time.Duration `json:"duration,omitempty"`
	Resolved  bool          `json:"resolved"`
}

// HealthResponse represents the complete health check response
type HealthResponse struct {
	Status       string           `json:"status"`
	Timestamp    string           `json:"timestamp"`
	Uptime       string           `json:"uptime"`
	Services     []ServiceStatus  `json:"services"`
	Dependencies DependencyHealth `json:"dependencies"`
	Summary      struct {
		Total     int `json:"total"`
		Healthy   int `json:"healthy"`
		Unhealthy int `json:"unhealthy"`
	} `json:"summary"`
	ServiceAvailability map[string]ServiceAvailabilityInfo `json:"service_availability"`
}

// ServiceAvailabilityInfo provides detailed availability metrics
type ServiceAvailabilityInfo struct {
	UptimePercentage  float64           `json:"uptime_percentage"`
	TotalUptime       string            `json:"total_uptime"`
	TotalDowntime     string            `json:"total_downtime"`
	DowntimeIncidents int               `json:"downtime_incidents"`
	CurrentStatus     string            `json:"current_status"`
	LastIncident      *DowntimeIncident `json:"last_incident,omitempty"`
}

var (
	startTime            = time.Now()
	lastHealthCheck      = make(map[string]ServiceStatus)
	serviceUptimes       = make(map[string]*ServiceUptime)
	uptimeMu             sync.RWMutex
	redisClient          *redis.Client
	dbConnection         *sql.DB                    // Legacy connection for backward compatibility
	serviceDBConnections = make(map[string]*sql.DB) // Map of service name to DB connection
	dbConnectionsMu      sync.RWMutex
)

// Map service display names to Prometheus service labels
var serviceNameMap = map[string]string{
	"MySQL":                  "mysql",
	"Redis":                  "redis",
	"Auth Service":           "auth-service",
	"Commercial Service":     "commercial-service",
	"Features Service":       "features-service",
	"Levels Service":         "levels-service",
	"Dynasty Service":        "dynasty-service",
	"Calendar Service":       "calendar-service",
	"Storage Service (gRPC)": "storage-service",
	"Kong API Gateway":       "kong",
	"Kong Admin API":         "kong",
	"WebSocket Gateway":      "websocket-gateway",
	"Storage Service (HTTP)": "storage-service",
	"gRPC Gateway":           "grpc-gateway",
}

// Map service labels to their running ports
var servicePortMap = map[string]string{
	"mysql":                 "3306",
	"redis":                 "6379",
	"auth-service":          "50051",
	"commercial-service":    "50052",
	"features-service":      "50053",
	"levels-service":        "50054",
	"dynasty-service":       "50055",
	"support-service":       "50056",
	"notifications-service": "50058",
	"calendar-service":      "50059",
	"storage-service":       "50060",
	"kong":                  "8000",
	"websocket-gateway":     "3002",
	"grpc-gateway":          "8080",
}

func main() {
	configPaths := []string{
		"config.env",
		"./config.env",
		"../config.env",
		"../../config.env",
		"services/health-check-service/config.env",
	}
	for _, p := range configPaths {
		if err := godotenv.Load(p); err == nil {
			break
		}
	}

	// Initialize Redis client for cache metrics
	initRedisClient()

	// Initialize database connection for DB health checks (legacy)
	initDBConnection()

	// Initialize database connections for each service
	initServiceDBConnections()

	// Start background goroutine to track uptime
	go trackUptime()

	http.HandleFunc("/live", liveHandler)
	http.HandleFunc("/health", healthCheckHandler)
	http.HandleFunc("/api/health", healthCheckHandler)
	http.HandleFunc("/metrics", metricsHandler)

	port := "8090"
	log.Printf("🏥 Health Check Service starting on port %s", port)
	log.Printf("📊 Health check endpoint: http://localhost:%s/health", port)
	log.Printf("📈 Prometheus metrics endpoint: http://localhost:%s/metrics", port)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Failed to start health check service: %v", err)
	}
}

func initRedisClient() {
	redisURL := getEnv("REDIS_URL", "redis://redis:6379")
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Printf("⚠️  Warning: Failed to parse Redis URL: %v", err)
		return
	}
	opts.MaintNotificationsConfig = &maintnotifications.Config{
		Mode: maintnotifications.ModeDisabled,
	}
	redisClient = redis.NewClient(opts)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Printf("⚠️  Warning: Failed to connect to Redis: %v", err)
		redisClient = nil
	}
}

func initDBConnection() {
	dbHost := getEnv("DB_HOST", "mysql")
	dbPort := getEnv("DB_PORT", "3306")
	dbUser := getEnv("DB_USER", "metargb_user")
	dbPassword := getEnv("DB_PASSWORD", "metargb_password")
	dbName := getEnv("DB_DATABASE", "metargb_db")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&timeout=2s",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	var err error
	dbConnection, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Printf("⚠️  Warning: Failed to open database connection: %v", err)
		dbConnection = nil
		return
	}

	dbConnection.SetMaxOpenConns(5)
	dbConnection.SetMaxIdleConns(2)
	dbConnection.SetConnMaxLifetime(5 * time.Minute)
}

// initServiceDBConnections initializes database connections for each service
func initServiceDBConnections() {
	dbHost := getEnv("DB_HOST", "mysql")
	dbPort := getEnv("DB_PORT", "3306")
	dbUser := getEnv("DB_USER", "metargb_user")
	dbPassword := getEnv("DB_PASSWORD", "metargb_password")
	dbName := getEnv("DB_DATABASE", "metargb_db")

	// List of services that use database connections
	services := []string{
		"auth-service",
		"commercial-service",
		"features-service",
		"levels-service",
		"dynasty-service",
		"calendar-service",
		"notifications-service",
		"support-service",
		"storage-service",
	}

	for _, serviceName := range services {
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&timeout=2s&charset=utf8mb4&collation=utf8mb4_unicode_ci",
			dbUser, dbPassword, dbHost, dbPort, dbName)

		db, err := sql.Open("mysql", dsn)
		if err != nil {
			log.Printf("⚠️  Warning: Failed to open database connection for %s: %v", serviceName, err)
			continue
		}

		// Configure connection pool for each service
		db.SetMaxOpenConns(5)
		db.SetMaxIdleConns(2)
		db.SetConnMaxLifetime(5 * time.Minute)

		// Test connection
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := db.PingContext(ctx); err != nil {
			log.Printf("⚠️  Warning: Failed to ping database for %s: %v", serviceName, err)
			cancel()
			db.Close()
			continue
		}
		cancel()

		dbConnectionsMu.Lock()
		serviceDBConnections[serviceName] = db
		dbConnectionsMu.Unlock()

		log.Printf("✅ Database connection initialized for %s", serviceName)
	}
}

func trackUptime() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		uptimeMu.Lock()
		now := time.Now()

		for serviceName, uptime := range serviceUptimes {
			// Check if service status changed
			status, exists := lastHealthCheck[serviceName]
			currentStatus := "unhealthy"
			if exists && status.Status == "healthy" {
				currentStatus = "healthy"
			}

			uptime.mu.Lock()
			// Track status changes
			if uptime.LastStatus != currentStatus {
				if currentStatus == "unhealthy" && uptime.LastStatus == "healthy" {
					// Service went down
					uptime.DowntimeIncidents = append(uptime.DowntimeIncidents, DowntimeIncident{
						StartTime: now,
						Resolved:  false,
					})
				} else if currentStatus == "healthy" && uptime.LastStatus == "unhealthy" {
					// Service came back up
					if len(uptime.DowntimeIncidents) > 0 {
						lastIncident := &uptime.DowntimeIncidents[len(uptime.DowntimeIncidents)-1]
						if !lastIncident.Resolved {
							lastIncident.EndTime = now
							lastIncident.Duration = now.Sub(lastIncident.StartTime)
							lastIncident.Resolved = true
							uptime.TotalDowntime += lastIncident.Duration
						}
					}
				}
				uptime.LastStatus = currentStatus
			}

			// Update uptime/downtime
			if currentStatus == "healthy" {
				if !uptime.LastSeen.IsZero() {
					uptime.TotalUptime += now.Sub(uptime.LastSeen)
				}
				uptime.LastSeen = now
			} else {
				if !uptime.LastSeen.IsZero() {
					uptime.TotalDowntime += now.Sub(uptime.LastSeen)
				}
			}

			uptime.mu.Unlock()
		}
		uptimeMu.Unlock()
	}
}

func getOrCreateUptimeTracker(serviceName string) *ServiceUptime {
	uptimeMu.Lock()
	defer uptimeMu.Unlock()

	if uptime, exists := serviceUptimes[serviceName]; exists {
		return uptime
	}

	uptime := &ServiceUptime{
		ServiceName:       serviceName,
		FirstSeen:         time.Now(),
		LastSeen:          time.Now(),
		LastStatus:        "unknown",
		DowntimeIncidents: make([]DowntimeIncident, 0),
	}
	serviceUptimes[serviceName] = uptime
	return uptime
}

// liveHandler is a fast liveness probe for Docker healthchecks (no dependency checks).
func liveHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	services := []ServiceStatus{}

	// Infrastructure Services
	services = append(services, checkTCP(ctx, "MySQL", "mysql", 3306))
	services = append(services, checkTCP(ctx, "Redis", "redis", 6379))

	// Core Microservices (gRPC)
	services = append(services, checkTCP(ctx, "Auth Service", "auth-service", 50051))
	services = append(services, checkTCP(ctx, "Commercial Service", "commercial-service", 50052))
	services = append(services, checkTCP(ctx, "Features Service", "features-service", 50053))
	services = append(services, checkTCP(ctx, "Levels Service", "levels-service", 50054))
	services = append(services, checkTCP(ctx, "Dynasty Service", "dynasty-service", 50055))
	services = append(services, checkTCP(ctx, "Support Service", "support-service", 50056))
	services = append(services, checkTCP(ctx, "Notifications Service", "notifications-service", 50058))
	services = append(services, checkTCP(ctx, "Calendar Service", "calendar-service", 50059))
	services = append(services, checkTCP(ctx, "Storage Service (gRPC)", "storage-service", 50060))

	// Gateway Services (HTTP)
	services = append(services, checkHTTP(ctx, "Kong API Gateway", "http://kong:8001/status"))
	services = append(services, checkHTTP(ctx, "Kong Admin API", "http://kong:8001/status"))
	services = append(services, checkHTTP(ctx, "WebSocket Gateway", "http://websocket-gateway:3002/health"))
	services = append(services, checkHTTP(ctx, "Storage Service (HTTP)", "http://storage-service:8059/health"))

	// Update uptime trackers
	for _, s := range services {
		uptime := getOrCreateUptimeTracker(s.Service)
		uptime.mu.Lock()
		if s.Status == "healthy" {
			if uptime.LastStatus != "healthy" {
				uptime.LastSeen = time.Now()
			}
		}
		uptime.mu.Unlock()
	}

	// Check dependencies
	dependencies := checkDependencies(ctx)

	// Calculate summary
	healthy := 0
	unhealthy := 0
	for _, s := range services {
		if s.Status == "healthy" {
			healthy++
		} else {
			unhealthy++
		}
	}

	// Determine overall status
	overallStatus := "healthy"
	if unhealthy > 0 {
		overallStatus = "degraded"
	}
	if unhealthy > len(services)/2 {
		overallStatus = "unhealthy"
	}

	uptime := time.Since(startTime)
	response := HealthResponse{
		Status:              overallStatus,
		Timestamp:           time.Now().UTC().Format(time.RFC3339),
		Uptime:              fmt.Sprintf("%.0fs", uptime.Seconds()),
		Services:            services,
		Dependencies:        dependencies,
		ServiceAvailability: getServiceAvailability(),
	}
	response.Summary.Total = len(services)
	response.Summary.Healthy = healthy
	response.Summary.Unhealthy = unhealthy

	var statusCode int
	switch overallStatus {
	case "unhealthy":
		statusCode = http.StatusServiceUnavailable
	case "degraded":
		statusCode = http.StatusOK
	default:
		statusCode = http.StatusOK
	}

	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)

	// Store results for metrics endpoint
	for _, s := range services {
		lastHealthCheck[s.Service] = s
	}
}

func checkDependencies(ctx context.Context) DependencyHealth {
	deps := DependencyHealth{
		DatabaseConnections:  make(map[string]DBConnectionStatus),
		ExternalAPIs:         []ExternalAPIStatus{},
		ThirdPartyServices:   []ThirdPartyService{},
		CircuitBreakerStatus: make(map[string]string),
	}

	// List of all services that should have database connections
	allServices := []string{
		"auth-service",
		"commercial-service",
		"features-service",
		"levels-service",
		"dynasty-service",
		"calendar-service",
		"notifications-service",
		"support-service",
		"storage-service",
	}

	// Check database connections for all services (create connection on-demand if needed)
	for _, serviceName := range allServices {
		// Ensure connection exists, create on-demand if needed
		ensureServiceDBConnection(serviceName)
		deps.DatabaseConnections[serviceName] = checkServiceDatabaseConnection(ctx, serviceName)
	}

	// Also check legacy database connection for backward compatibility
	if dbConnection != nil {
		deps.DatabaseConnections["legacy"] = checkDatabaseConnection(ctx)
	}

	// Check cache metrics
	deps.CacheMetrics = checkCacheMetrics(ctx)

	// Check external APIs (e.g., Parsian payment gateway)
	deps.ExternalAPIs = checkExternalAPIs(ctx)

	// Check third-party services
	deps.ThirdPartyServices = checkThirdPartyServices(ctx)

	// Check circuit breaker status (if Istio is available)
	deps.CircuitBreakerStatus = checkCircuitBreakerStatus(ctx)

	return deps
}

func checkDatabaseConnection(ctx context.Context) DBConnectionStatus {
	status := DBConnectionStatus{
		Host:      getEnv("DB_HOST", "mysql"),
		Port:      3306,
		Database:  getEnv("DB_DATABASE", "metargb_db"),
		Status:    "unhealthy",
		Connected: false,
	}

	if dbConnection == nil {
		status.Error = "Database connection not initialized"
		return status
	}

	start := time.Now()
	err := dbConnection.PingContext(ctx)
	latency := time.Since(start)

	if err != nil {
		status.Error = err.Error()
		status.Latency = latency.String()
		return status
	}

	status.Status = "healthy"
	status.Connected = true
	status.Latency = latency.String()

	// Get connection pool stats
	stats := dbConnection.Stats()
	status.PoolStats.OpenConnections = stats.OpenConnections
	status.PoolStats.InUse = stats.InUse
	status.PoolStats.Idle = stats.Idle

	return status
}

// ensureServiceDBConnection ensures a database connection exists for a service, creating it if needed
func ensureServiceDBConnection(serviceName string) {
	dbConnectionsMu.RLock()
	_, exists := serviceDBConnections[serviceName]
	dbConnectionsMu.RUnlock()

	if exists {
		return // Connection already exists
	}

	// Create connection on-demand
	dbHost := getEnv("DB_HOST", "mysql")
	dbPort := getEnv("DB_PORT", "3306")
	dbUser := getEnv("DB_USER", "metargb_user")
	dbPassword := getEnv("DB_PASSWORD", "metargb_password")
	dbName := getEnv("DB_DATABASE", "metargb_db")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&timeout=2s&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Printf("⚠️  Warning: Failed to open database connection for %s: %v", serviceName, err)
		return
	}

	// Configure connection pool for each service
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Test connection (non-blocking, will be checked later)
	dbConnectionsMu.Lock()
	serviceDBConnections[serviceName] = db
	dbConnectionsMu.Unlock()

	log.Printf("✅ Database connection created for %s", serviceName)
}

// checkServiceDatabaseConnection checks database connection for a specific service
func checkServiceDatabaseConnection(ctx context.Context, serviceName string) DBConnectionStatus {
	status := DBConnectionStatus{
		Host:      getEnv("DB_HOST", "mysql"),
		Port:      3306,
		Database:  getEnv("DB_DATABASE", "metargb_db"),
		Status:    "unhealthy",
		Connected: false,
	}

	dbConnectionsMu.RLock()
	db, exists := serviceDBConnections[serviceName]
	dbConnectionsMu.RUnlock()

	if !exists || db == nil {
		status.Error = fmt.Sprintf("Database connection not initialized for %s", serviceName)
		return status
	}

	start := time.Now()
	err := db.PingContext(ctx)
	latency := time.Since(start)

	if err != nil {
		status.Error = err.Error()
		status.Latency = latency.String()
		return status
	}

	status.Status = "healthy"
	status.Connected = true
	status.Latency = latency.String()

	// Get connection pool stats
	stats := db.Stats()
	status.PoolStats.OpenConnections = stats.OpenConnections
	status.PoolStats.InUse = stats.InUse
	status.PoolStats.Idle = stats.Idle

	return status
}

func checkCacheMetrics(ctx context.Context) CacheMetrics {
	metrics := CacheMetrics{
		Status: "unhealthy",
	}

	if redisClient == nil {
		metrics.Error = "Redis client not initialized"
		return metrics
	}

	start := time.Now()
	info, err := redisClient.Info(ctx, "stats").Result()
	latency := time.Since(start)
	metrics.Latency = latency.String()

	if err != nil {
		metrics.Error = err.Error()
		return metrics
	}

	// Parse Redis INFO stats
	lines := strings.Split(info, "\r\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "keyspace_hits:") {
			fmt.Sscanf(line, "keyspace_hits:%d", &metrics.Hits)
		} else if strings.HasPrefix(line, "keyspace_misses:") {
			fmt.Sscanf(line, "keyspace_misses:%d", &metrics.Misses)
		} else if strings.HasPrefix(line, "used_memory:") {
			var mem int64
			fmt.Sscanf(line, "used_memory:%d", &mem)
			metrics.MemoryUsage = mem
		}
	}

	// Calculate hit/miss rates
	total := metrics.Hits + metrics.Misses
	if total > 0 {
		metrics.HitRate = float64(metrics.Hits) / float64(total) * 100
		metrics.MissRate = float64(metrics.Misses) / float64(total) * 100
	}

	metrics.Status = "healthy"
	return metrics
}

func checkExternalAPIs(ctx context.Context) []ExternalAPIStatus {
	apis := []ExternalAPIStatus{}

	// Check Parsian payment gateway (if configured)
	parsianURL := getEnv("PARSIAN_API_URL", "")
	if parsianURL != "" {
		api := checkExternalAPI(ctx, "Parsian Payment Gateway", parsianURL)
		apis = append(apis, api)
	}

	return apis
}

func checkExternalAPI(ctx context.Context, name, url string) ExternalAPIStatus {
	status := ExternalAPIStatus{
		Name:      name,
		URL:       url,
		Status:    "unhealthy",
		LastCheck: time.Now().UTC().Format(time.RFC3339),
	}

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		status.Error = err.Error()
		return status
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	latency := time.Since(start)
	status.Latency = latency.String()

	if err != nil {
		status.Error = err.Error()
		return status
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		status.Status = "healthy"
	}

	return status
}

func checkThirdPartyServices(ctx context.Context) []ThirdPartyService {
	services := []ThirdPartyService{}

	// Add third-party service checks here
	// Example: Check external notification service, analytics service, etc.

	return services
}

func checkCircuitBreakerStatus(ctx context.Context) map[string]string {
	status := make(map[string]string)

	// Check Istio circuit breaker status if available
	// This would typically query Istio metrics endpoint
	istioMetricsURL := getEnv("ISTIO_METRICS_URL", "")
	if istioMetricsURL != "" {
		// In a real implementation, you would query Istio's metrics endpoint
		// For now, we'll mark it as not available
		status["istio"] = "not_configured"
	}

	return status
}

func getServiceAvailability() map[string]ServiceAvailabilityInfo {
	availability := make(map[string]ServiceAvailabilityInfo)

	uptimeMu.RLock()
	defer uptimeMu.RUnlock()

	now := time.Now()
	for serviceName, uptime := range serviceUptimes {
		uptime.mu.RLock()

		totalTime := now.Sub(uptime.FirstSeen)
		if totalTime == 0 {
			totalTime = 1 * time.Second // Avoid division by zero
		}

		// Calculate current uptime percentage
		currentUptime := uptime.TotalUptime
		if uptime.LastStatus == "healthy" && !uptime.LastSeen.IsZero() {
			currentUptime += now.Sub(uptime.LastSeen)
		}

		uptimePercentage := (float64(currentUptime) / float64(totalTime)) * 100

		info := ServiceAvailabilityInfo{
			UptimePercentage:  uptimePercentage,
			TotalUptime:       uptime.TotalUptime.String(),
			TotalDowntime:     uptime.TotalDowntime.String(),
			DowntimeIncidents: len(uptime.DowntimeIncidents),
			CurrentStatus:     uptime.LastStatus,
		}

		// Get last incident if exists
		if len(uptime.DowntimeIncidents) > 0 {
			lastIncident := uptime.DowntimeIncidents[len(uptime.DowntimeIncidents)-1]
			info.LastIncident = &lastIncident
		}

		availability[serviceName] = info
		uptime.mu.RUnlock()
	}

	return availability
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	// Always run a fresh health check to ensure we have current data
	// This ensures metrics are always up-to-date when Prometheus scrapes
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	services := []ServiceStatus{}
	services = append(services, checkTCP(ctx, "MySQL", "mysql", 3306))
	services = append(services, checkTCP(ctx, "Redis", "redis", 6379))
	services = append(services, checkTCP(ctx, "Auth Service", "auth-service", 50051))
	services = append(services, checkTCP(ctx, "Commercial Service", "commercial-service", 50052))
	services = append(services, checkTCP(ctx, "Features Service", "features-service", 50053))
	services = append(services, checkTCP(ctx, "Levels Service", "levels-service", 50054))
	services = append(services, checkTCP(ctx, "Dynasty Service", "dynasty-service", 50055))
	services = append(services, checkTCP(ctx, "Support Service", "support-service", 50056))
	services = append(services, checkTCP(ctx, "Notifications Service", "notifications-service", 50058))
	services = append(services, checkTCP(ctx, "Calendar Service", "calendar-service", 50059))
	services = append(services, checkTCP(ctx, "Storage Service (gRPC)", "storage-service", 50060))
	services = append(services, checkHTTP(ctx, "Kong API Gateway", "http://kong:8001/status"))
	services = append(services, checkHTTP(ctx, "WebSocket Gateway", "http://websocket-gateway:3002/health"))
	services = append(services, checkHTTP(ctx, "Storage Service (HTTP)", "http://storage-service:8059/health"))
	services = append(services, checkHTTP(ctx, "gRPC Gateway", "http://grpc-gateway:8080/health"))

	// Update lastHealthCheck with fresh data
	for _, s := range services {
		lastHealthCheck[s.Service] = s
	}

	// Log for debugging
	if len(lastHealthCheck) == 0 {
		log.Printf("⚠️  Warning: No services checked in metricsHandler")
	} else {
		log.Printf("✅ Health check completed: %d services checked", len(lastHealthCheck))
	}

	// Ensure we always have data - if health checks failed, still export with unhealthy status
	// This prevents empty metrics which cause Grafana tables to show no data
	if len(lastHealthCheck) == 0 {
		log.Printf("⚠️  No health check data - exporting placeholder metrics")
		// Add placeholder entries for all expected services with unhealthy status
		expectedServices := []struct {
			displayName  string
			serviceLabel string
			port         int
		}{
			{"MySQL", "mysql", 3306},
			{"Redis", "redis", 6379},
			{"Auth Service", "auth-service", 50051},
			{"Commercial Service", "commercial-service", 50052},
			{"Features Service", "features-service", 50053},
			{"Levels Service", "levels-service", 50054},
			{"Dynasty Service", "dynasty-service", 50055},
			{"Support Service", "support-service", 50056},
			{"Notifications Service", "notifications-service", 50058},
			{"Calendar Service", "calendar-service", 50059},
			{"Storage Service (gRPC)", "storage-service", 50060},
			{"Kong API Gateway", "kong", 0},
			{"WebSocket Gateway", "websocket-gateway", 0},
			{"gRPC Gateway", "grpc-gateway", 0},
		}
		for _, svc := range expectedServices {
			lastHealthCheck[svc.displayName] = ServiceStatus{
				Service: svc.displayName,
				Status:  "unhealthy",
				Port:    svc.port,
			}
		}
	}

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	// Export service health status metrics
	exportServiceHealthMetrics(w)

	// Export service availability metrics
	exportServiceAvailabilityMetrics(w)

	// Export dependency health metrics
	exportDependencyHealthMetrics(w)
}

func exportServiceHealthMetrics(w http.ResponseWriter) {
	fmt.Fprintf(w, "# HELP service_health_status Service health status (1=healthy, 0=unhealthy)\n")
	fmt.Fprintf(w, "# TYPE service_health_status gauge\n")

	// Track which services we've exported to avoid duplicates
	exported := make(map[string]bool)
	exportedCount := 0

	// Export metrics for all services in lastHealthCheck
	// This ensures we always have data, even if some services are down
	for displayName, status := range lastHealthCheck {
		serviceLabel := serviceNameMap[displayName]
		if serviceLabel == "" {
			serviceLabel = strings.ToLower(strings.ReplaceAll(displayName, " ", "-"))
		}

		// Use composite key to handle services with same label but different display names
		key := fmt.Sprintf("%s:%s", serviceLabel, displayName)
		if exported[key] {
			continue
		}
		exported[key] = true

		value := 0
		if status.Status == "healthy" {
			value = 1
		}

		port := ""
		if status.Port > 0 {
			port = fmt.Sprintf("%d", status.Port)
		} else {
			port = servicePortMap[serviceLabel]
		}
		if port == "" {
			port = "N/A"
		}

		// Always export metrics - this ensures the Grafana table always shows data
		// Even unhealthy services will show with value 0
		fmt.Fprintf(w, "service_health_status{service=\"%s\",display_name=\"%s\",port=\"%s\"} %d\n",
			serviceLabel, displayName, port, value)
		exportedCount++
	}

	// Log if no services were exported (for debugging)
	if exportedCount == 0 {
		log.Printf("⚠️  ERROR: No service health metrics exported! lastHealthCheck has %d entries", len(lastHealthCheck))
		// Debug: log what's in lastHealthCheck
		for name, status := range lastHealthCheck {
			log.Printf("  Debug - Service: %s, Status: %s, Port: %d", name, status.Status, status.Port)
		}
	} else {
		log.Printf("✅ Exported %d service health metrics", exportedCount)
	}

	// Summary metrics
	healthy := 0
	unhealthy := 0
	for _, status := range lastHealthCheck {
		if status.Status == "healthy" {
			healthy++
		} else {
			unhealthy++
		}
	}

	fmt.Fprintf(w, "\n# HELP service_health_total Total number of services checked\n")
	fmt.Fprintf(w, "# TYPE service_health_total gauge\n")
	fmt.Fprintf(w, "service_health_total %d\n", len(lastHealthCheck))

	fmt.Fprintf(w, "\n# HELP service_health_healthy Number of healthy services\n")
	fmt.Fprintf(w, "# TYPE service_health_healthy gauge\n")
	fmt.Fprintf(w, "service_health_healthy %d\n", healthy)

	fmt.Fprintf(w, "\n# HELP service_health_unhealthy Number of unhealthy services\n")
	fmt.Fprintf(w, "# TYPE service_health_unhealthy gauge\n")
	fmt.Fprintf(w, "service_health_unhealthy %d\n", unhealthy)
}

func exportServiceAvailabilityMetrics(w http.ResponseWriter) {
	fmt.Fprintf(w, "\n# HELP service_uptime_percentage Service uptime percentage (0-100)\n")
	fmt.Fprintf(w, "# TYPE service_uptime_percentage gauge\n")

	fmt.Fprintf(w, "\n# HELP service_uptime_seconds_total Total uptime in seconds\n")
	fmt.Fprintf(w, "# TYPE service_uptime_seconds_total counter\n")

	fmt.Fprintf(w, "\n# HELP service_downtime_seconds_total Total downtime in seconds\n")
	fmt.Fprintf(w, "# TYPE service_downtime_seconds_total counter\n")

	fmt.Fprintf(w, "\n# HELP service_downtime_incidents_total Total number of downtime incidents\n")
	fmt.Fprintf(w, "# TYPE service_downtime_incidents_total counter\n")

	uptimeMu.RLock()
	defer uptimeMu.RUnlock()

	for serviceName, uptime := range serviceUptimes {
		uptime.mu.RLock()

		serviceLabel := serviceNameMap[serviceName]
		if serviceLabel == "" {
			serviceLabel = strings.ToLower(strings.ReplaceAll(serviceName, " ", "-"))
		}

		now := time.Now()
		totalTime := now.Sub(uptime.FirstSeen)
		if totalTime == 0 {
			totalTime = 1 * time.Second
		}

		currentUptime := uptime.TotalUptime
		if uptime.LastStatus == "healthy" && !uptime.LastSeen.IsZero() {
			currentUptime += now.Sub(uptime.LastSeen)
		}

		uptimePercentage := (float64(currentUptime) / float64(totalTime)) * 100

		fmt.Fprintf(w, "service_uptime_percentage{service=\"%s\"} %.2f\n", serviceLabel, uptimePercentage)
		fmt.Fprintf(w, "service_uptime_seconds_total{service=\"%s\"} %.0f\n", serviceLabel, uptime.TotalUptime.Seconds())
		fmt.Fprintf(w, "service_downtime_seconds_total{service=\"%s\"} %.0f\n", serviceLabel, uptime.TotalDowntime.Seconds())
		fmt.Fprintf(w, "service_downtime_incidents_total{service=\"%s\"} %d\n", serviceLabel, len(uptime.DowntimeIncidents))

		uptime.mu.RUnlock()
	}
}

func exportDependencyHealthMetrics(w http.ResponseWriter) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Database connection metrics for each service
	fmt.Fprintf(w, "\n# HELP db_connection_status Database connection status per service (1=connected, 0=disconnected)\n")
	fmt.Fprintf(w, "# TYPE db_connection_status gauge\n")

	fmt.Fprintf(w, "\n# HELP db_connection_latency_seconds Database connection latency per service\n")
	fmt.Fprintf(w, "# TYPE db_connection_latency_seconds gauge\n")

	fmt.Fprintf(w, "\n# HELP db_connection_pool_open Database connection pool open connections per service\n")
	fmt.Fprintf(w, "# TYPE db_connection_pool_open gauge\n")

	fmt.Fprintf(w, "\n# HELP db_connection_pool_in_use Database connection pool in-use connections per service\n")
	fmt.Fprintf(w, "# TYPE db_connection_pool_in_use gauge\n")

	fmt.Fprintf(w, "\n# HELP db_connection_pool_idle Database connection pool idle connections per service\n")
	fmt.Fprintf(w, "# TYPE db_connection_pool_idle gauge\n")

	// List of all services that should have database connections
	allServices := []string{
		"auth-service",
		"commercial-service",
		"features-service",
		"levels-service",
		"dynasty-service",
		"calendar-service",
		"notifications-service",
		"support-service",
		"storage-service",
	}

	// Check database connections for all services (create connection on-demand if needed)
	// IMPORTANT: Always export metrics for ALL services, even if connection fails
	// This ensures Grafana/Prometheus always has data for all services
	log.Printf("📊 Exporting database connection metrics for %d services", len(allServices))

	dbHost := getEnv("DB_HOST", "mysql")
	dbDatabase := getEnv("DB_DATABASE", "metargb_db")

	for _, serviceName := range allServices {
		// Ensure connection exists, create on-demand if needed
		ensureServiceDBConnection(serviceName)

		// Always check the connection status, even if connection doesn't exist
		dbStatus := checkServiceDatabaseConnection(ctx, serviceName)
		dbValue := 0
		if dbStatus.Connected {
			dbValue = 1
		}

		// CRITICAL: Always export status metric for EVERY service
		// Use consistent host/database values to ensure metrics are properly grouped
		// Value: 0 = disconnected, 1 = connected
		fmt.Fprintf(w, "db_connection_status{service=\"%s\",host=\"%s\",database=\"%s\"} %d\n",
			serviceName, dbHost, dbDatabase, dbValue)

		// Export latency only if we have a valid connection and latency measurement
		if dbStatus.Connected && dbStatus.Latency != "" {
			// Parse latency string (e.g., "10ms" or "1.5s")
			latency, err := parseDuration(dbStatus.Latency)
			if err == nil {
				fmt.Fprintf(w, "db_connection_latency_seconds{service=\"%s\",host=\"%s\"} %.4f\n",
					serviceName, dbHost, latency.Seconds())
			}
		}

		// Always export pool stats (will be 0 if connection doesn't exist)
		// Use consistent host value for proper metric grouping
		fmt.Fprintf(w, "db_connection_pool_open{service=\"%s\",host=\"%s\"} %d\n",
			serviceName, dbHost, dbStatus.PoolStats.OpenConnections)

		fmt.Fprintf(w, "db_connection_pool_in_use{service=\"%s\",host=\"%s\"} %d\n",
			serviceName, dbHost, dbStatus.PoolStats.InUse)

		fmt.Fprintf(w, "db_connection_pool_idle{service=\"%s\",host=\"%s\"} %d\n",
			serviceName, dbHost, dbStatus.PoolStats.Idle)
	}
	log.Printf("✅ Finished exporting database connection metrics for %d services", len(allServices))

	// Also export legacy database connection for backward compatibility
	if dbConnection != nil {
		dbStatus := checkDatabaseConnection(ctx)
		dbValue := 0
		if dbStatus.Connected {
			dbValue = 1
		}
		fmt.Fprintf(w, "db_connection_status{service=\"legacy\",host=\"%s\",database=\"%s\"} %d\n",
			dbStatus.Host, dbStatus.Database, dbValue)

		if dbStatus.Latency != "" {
			latency, _ := parseDuration(dbStatus.Latency)
			fmt.Fprintf(w, "db_connection_latency_seconds{service=\"legacy\",host=\"%s\"} %.4f\n",
				dbStatus.Host, latency.Seconds())
		}

		fmt.Fprintf(w, "db_connection_pool_open{service=\"legacy\",host=\"%s\"} %d\n",
			dbStatus.Host, dbStatus.PoolStats.OpenConnections)

		fmt.Fprintf(w, "db_connection_pool_in_use{service=\"legacy\",host=\"%s\"} %d\n",
			dbStatus.Host, dbStatus.PoolStats.InUse)

		fmt.Fprintf(w, "db_connection_pool_idle{service=\"legacy\",host=\"%s\"} %d\n",
			dbStatus.Host, dbStatus.PoolStats.Idle)
	}

	// Cache metrics
	fmt.Fprintf(w, "\n# HELP cache_status Cache status (1=healthy, 0=unhealthy)\n")
	fmt.Fprintf(w, "# TYPE cache_status gauge\n")

	cacheMetrics := checkCacheMetrics(ctx)
	cacheValue := 0
	if cacheMetrics.Status == "healthy" {
		cacheValue = 1
	}
	fmt.Fprintf(w, "cache_status{cache=\"redis\"} %d\n", cacheValue)

	fmt.Fprintf(w, "\n# HELP cache_hit_rate Cache hit rate percentage\n")
	fmt.Fprintf(w, "# TYPE cache_hit_rate gauge\n")
	fmt.Fprintf(w, "cache_hit_rate{cache=\"redis\"} %.2f\n", cacheMetrics.HitRate)

	fmt.Fprintf(w, "\n# HELP cache_miss_rate Cache miss rate percentage\n")
	fmt.Fprintf(w, "# TYPE cache_miss_rate gauge\n")
	fmt.Fprintf(w, "cache_miss_rate{cache=\"redis\"} %.2f\n", cacheMetrics.MissRate)

	fmt.Fprintf(w, "\n# HELP cache_hits_total Total cache hits\n")
	fmt.Fprintf(w, "# TYPE cache_hits_total counter\n")
	fmt.Fprintf(w, "cache_hits_total{cache=\"redis\"} %d\n", cacheMetrics.Hits)

	fmt.Fprintf(w, "\n# HELP cache_misses_total Total cache misses\n")
	fmt.Fprintf(w, "# TYPE cache_misses_total counter\n")
	fmt.Fprintf(w, "cache_misses_total{cache=\"redis\"} %d\n", cacheMetrics.Misses)

	fmt.Fprintf(w, "\n# HELP cache_memory_usage_bytes Cache memory usage in bytes\n")
	fmt.Fprintf(w, "# TYPE cache_memory_usage_bytes gauge\n")
	fmt.Fprintf(w, "cache_memory_usage_bytes{cache=\"redis\"} %d\n", cacheMetrics.MemoryUsage)

	// External API metrics
	fmt.Fprintf(w, "\n# HELP external_api_status External API status (1=healthy, 0=unhealthy)\n")
	fmt.Fprintf(w, "# TYPE external_api_status gauge\n")

	externalAPIs := checkExternalAPIs(ctx)
	for _, api := range externalAPIs {
		value := 0
		if api.Status == "healthy" {
			value = 1
		}
		fmt.Fprintf(w, "external_api_status{name=\"%s\",url=\"%s\"} %d\n", api.Name, api.URL, value)
	}
}

func parseDuration(s string) (time.Duration, error) {
	// Simple parser for duration strings like "10ms", "1.5s", etc.
	if strings.HasSuffix(s, "ms") {
		ms, err := strconv.ParseFloat(strings.TrimSuffix(s, "ms"), 64)
		if err != nil {
			return 0, err
		}
		return time.Duration(ms) * time.Millisecond, nil
	} else if strings.HasSuffix(s, "s") {
		sec, err := strconv.ParseFloat(strings.TrimSuffix(s, "s"), 64)
		if err != nil {
			return 0, err
		}
		return time.Duration(sec) * time.Second, nil
	}
	return time.ParseDuration(s)
}

func checkTCP(ctx context.Context, name, host string, port int) ServiceStatus {
	start := time.Now()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", fmt.Sprintf("%s:%d", host, port))
	latency := time.Since(start)

	if err != nil {
		return ServiceStatus{
			Service: name,
			Status:  "unhealthy",
			Host:    host,
			Port:    port,
			Error:   err.Error(),
			Latency: latency.String(),
		}
	}
	conn.Close()

	return ServiceStatus{
		Service: name,
		Status:  "healthy",
		Host:    host,
		Port:    port,
		Latency: latency.String(),
	}
}

func checkHTTP(ctx context.Context, name, url string) ServiceStatus {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return ServiceStatus{
			Service: name,
			Status:  "unhealthy",
			URL:     url,
			Error:   err.Error(),
		}
	}

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		return ServiceStatus{
			Service: name,
			Status:  "unhealthy",
			URL:     url,
			Error:   err.Error(),
			Latency: latency.String(),
		}
	}
	defer resp.Body.Close()

	status := "healthy"
	if resp.StatusCode >= 400 {
		status = "unhealthy"
	}

	return ServiceStatus{
		Service: name,
		Status:  status,
		URL:     url,
		Latency: latency.String(),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
