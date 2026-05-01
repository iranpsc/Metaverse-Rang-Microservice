.PHONY: proto clean-proto gen-auth gen-commercial gen-features gen-levels gen-dynasty gen-support gen-training gen-notifications gen-calendar gen-storage gen-financial gen-all help build-all deploy-all test up down restart logs ps build clean dev dev-up dev-down

# Proto generation
PROTO_DIR=shared/proto
PROTO_OUT_DIR=shared/pb

# Docker
DOCKER_REGISTRY=metargb
VERSION?=latest

# Docker Compose compatibility - auto-detect docker-compose or docker compose plugin
# Windows PowerShell doesn't support 'command -v', so default to 'docker compose' (modern Docker Desktop)
ifeq ($(OS),Windows_NT)
DOCKER_COMPOSE := docker compose
else
DOCKER_COMPOSE := $(shell command -v docker-compose 2> /dev/null || echo "docker compose")
endif

help:
	@echo "Available targets:"
	@echo ""
	@echo "Proto Generation:"
	@echo "  proto            - Generate all proto files"
	@echo "  gen-auth         - Generate auth service proto"
	@echo "  gen-commercial   - Generate commercial service proto"
	@echo "  gen-features     - Generate features service proto"
	@echo "  gen-levels       - Generate levels service proto"
	@echo "  clean-proto      - Clean generated proto files"
	@echo ""
	@echo "Build:"
	@echo "  build-all        - Build all service Docker images"
	@echo "  build-features   - Build features service Docker image"
	@echo "  build-levels     - Build levels service Docker image"
	@echo ""
	@echo "Deploy:"
	@echo "  deploy-all       - Deploy all services to Kubernetes"
	@echo ""
	@echo "Test:"
	@echo "  test             - Run integration tests"
	@echo "  test-all         - Run all test suites"
	@echo "  test-coverage-features - features-service handler coverage ≥70% (GOWORK=off)"
	@echo ""
	@echo "Database:"
	@echo "  import-schema    - Import database schema only (schema.sql)"
	@echo "  import-database  - Import database with data (metargb_db.sql)"
	@echo ""
	@echo "Docker:"
	@echo "  up, down, build, logs, ps - Compose lifecycle"
	@echo "  dev-up, dev-down - Development with watch mode"



# =============================================================================
# Testing
# =============================================================================

# Unit tests
test-unit:
	@echo "🧪 Running unit tests for all services..."
	@for service in services/*/; do \
		if [ -f "$$service/go.mod" ]; then \
			echo "Testing $$(basename $$service)..."; \
			cd $$service && go test ./internal/... -v -race -coverprofile=coverage.out || exit 1; \
			cd ../..; \
		fi \
	done
	@echo "✅ All unit tests passed"

# Features-service handler coverage gate (≥70%, no MySQL required; uses GOWORK=off for local replace)
test-coverage-features:
	@echo "🧪 features-service handler coverage (min 70%)..."
	cd services/features-service && GOWORK=off go test ./internal/handler/... -race -coverprofile=coverage.out -covermode=atomic
	@pct=$$(cd services/features-service && GOWORK=off go tool cover -func=coverage.out | tail -1 | grep -oE '[0-9]+\.[0-9]+' | tail -1); \
	echo "handler statements coverage: $${pct}%"; \
	awk -v p="$$pct" 'BEGIN{if (p+0 < 70.0) exit 1}'
	@echo "✅ features-service handler coverage OK"

# Integration tests
test-integration:
	@echo "🧪 Running integration tests..."
	cd tests/integration && go test -v ./...

# Golden JSON tests
test-golden:
	@echo "🧪 Running golden JSON comparison tests..."
	cd tests/golden && go test -v ./...

# Database tests
test-database:
	@echo "🧪 Running database schema and concurrency tests..."
	cd tests/database && go test -v ./...

# Run all tests
test-all: test-unit test-integration test-golden test-database
	@echo "✅ All test suites passed"

# Legacy test target (kept for backward compatibility)
test: test-integration

# =============================================================================
# Docker Compose Management
# =============================================================================

.PHONY: up down restart logs ps build clean import-schema import-database help-docker dev-up dev-down dev-build dev-logs dev-restart dev-ps

up:
	@echo "🚀 Starting all microservices..."
	$(DOCKER_COMPOSE) up -d
	@echo "✅ All services started!"
	@echo ""
	@echo "Services available at:"
	@echo "  Kong API Gateway: http://localhost:8000"
	@echo "  Kong Admin:       http://localhost:8001"
	@echo "  WebSocket:        http://localhost:3000"
	@echo ""
	@echo "Run 'make ps' to check service status"
	@echo "Run 'make logs' to view logs"

down:
	@echo "🛑 Stopping all microservices..."
	$(DOCKER_COMPOSE) down
	@echo "✅ All services stopped"

restart:
	@echo "🔄 Restarting all microservices..."
	$(DOCKER_COMPOSE) restart
	@echo "✅ All services restarted"

logs:
	$(DOCKER_COMPOSE) logs -f

ps:
	@echo "📊 Service Status:"
	@echo ""
	$(DOCKER_COMPOSE) ps
	@echo ""
	@echo "Healthy services:"
	@docker ps --filter "health=healthy" --format "  ✅ {{.Names}}"
	@echo ""
	@echo "Unhealthy services:"
	@docker ps --filter "health=unhealthy" --format "  ❌ {{.Names}}"

build:
	@echo "🔨 Building all services..."
	$(DOCKER_COMPOSE) build
	@echo "✅ Build complete"

build-service:
	@if [ -z "$(SERVICE)" ]; then \
		echo "❌ Please specify SERVICE=service-name"; \
		echo "Example: make build-service SERVICE=auth-service"; \
		exit 1; \
	fi
	@echo "🔨 Building $(SERVICE)..."
	$(DOCKER_COMPOSE) build $(SERVICE)
	@echo "✅ $(SERVICE) built successfully"

clean:
	@echo "🧹 Cleaning up Docker resources..."
	$(DOCKER_COMPOSE) down -v
	docker system prune -f
	@echo "✅ Cleanup complete"

import-schema:
	@echo "📥 Importing database schema..."
	@if [ ! -f scripts/schema.sql ]; then \
		echo "❌ scripts/schema.sql not found!"; \
		exit 1; \
	fi
	docker exec -i metargb-mysql mysql -uroot -proot_password metargb_db < scripts/schema.sql
	@echo "✅ Schema imported successfully"
	@echo ""
	@echo "Verifying tables..."
	@docker exec metargb-mysql mysql -uroot -proot_password metargb_db -e "SELECT COUNT(*) as table_count FROM information_schema.tables WHERE table_schema='metargb_db';" 2>/dev/null | grep -v table_count || echo "Could not verify"

import-database:
	@echo "Importing database (schema + data) from metargb_db.sql..."
	@echo "Dropping and recreating database..."
ifeq ($(OS),Windows_NT)
	@docker exec -i metargb-mysql mysql -uroot -proot_password -e "DROP DATABASE IF EXISTS metargb_db; CREATE DATABASE metargb_db CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;" 2>nul
	@echo "Importing data..."
	@powershell -Command "Get-Content scripts\metargb_db.sql | docker exec -i metargb-mysql mysql -uroot -proot_password metargb_db"
else
	@docker exec -i metargb-mysql mysql -uroot -proot_password -e "DROP DATABASE IF EXISTS metargb_db; CREATE DATABASE metargb_db CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;" 2>/dev/null || true
	@echo "Importing data..."
	@docker exec -i metargb-mysql mysql -uroot -proot_password metargb_db < scripts/metargb_db.sql
endif
	@echo "Database imported successfully"
	@echo ""
	@echo "Verifying import..."
ifeq ($(OS),Windows_NT)
	@docker exec metargb-mysql mysql -uroot -proot_password metargb_db -e "SELECT COUNT(*) as table_count FROM information_schema.tables WHERE table_schema='metargb_db';" 2>nul | findstr /v table_count || echo "Could not verify table count"
	@docker exec metargb-mysql mysql -uroot -proot_password metargb_db -e "SELECT COUNT(*) as row_count FROM account_securities;" 2>nul | findstr /v row_count || echo "Could not verify data"
else
	@docker exec metargb-mysql mysql -uroot -proot_password metargb_db -e "SELECT COUNT(*) as table_count FROM information_schema.tables WHERE table_schema='metargb_db';" 2>/dev/null | grep -v table_count || echo "Could not verify table count"
	@docker exec metargb-mysql mysql -uroot -proot_password metargb_db -e "SELECT COUNT(*) as row_count FROM account_securities;" 2>/dev/null | grep -v row_count || echo "Could not verify data"
endif

dev:
	@echo "🚀 Starting development environment..."
	@echo "ℹ️  Each service uses its own config.env (copy from config.env.sample)"
	@echo "Starting MySQL and Redis..."
	$(DOCKER_COMPOSE) up -d mysql redis
	@echo "Waiting for database to be ready..."
	@sleep 10
	@echo "Checking if schema needs to be imported..."
	@TABLE_COUNT=$$(docker exec metargb-mysql mysql -uroot -proot_password metargb_db -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='metargb_db';" 2>/dev/null | tail -1); \
	if [ "$$TABLE_COUNT" = "0" ]; then \
		echo "Importing schema..."; \
		make import-schema; \
	else \
		echo "✅ Database already initialized ($$TABLE_COUNT tables)"; \
	fi
	@echo ""
	@echo "Starting all services..."
	$(DOCKER_COMPOSE) up -d
	@echo ""
	@echo "✅ Development environment ready!"
	@make ps

stop-service:
	@if [ -z "$(SERVICE)" ]; then \
		echo "❌ Please specify SERVICE=service-name"; \
		exit 1; \
	fi
	$(DOCKER_COMPOSE) stop $(SERVICE)

start-service:
	@if [ -z "$(SERVICE)" ]; then \
		echo "❌ Please specify SERVICE=service-name"; \
		exit 1; \
	fif
	$(DOCKER_COMPOSE) start $(SERVICE)

logs-service:
	@if [ -z "$(SERVICE)" ]; then \
		echo "❌ Please specify SERVICE=service-name"; \
		exit 1; \
	fi
	$(DOCKER_COMPOSE) logs -f $(SERVICE)

# =============================================================================
# Development with Hot Reloading
# =============================================================================

dev-up:
	@echo "🚀 Starting development environment with Docker Compose Watch..."
	@echo "ℹ️  File changes will automatically trigger rebuilds (Go) or restarts (Node.js)"
	@echo ""
	$(DOCKER_COMPOSE) up --watch
	@echo "✅ Development services started with watch mode!"

dev-down:
	@echo "🛑 Stopping development services..."
	$(DOCKER_COMPOSE) down
	@echo "✅ Development services stopped"

dev-build:
	@echo "🔨 Building development images..."
	$(DOCKER_COMPOSE) build
	@echo "✅ Development images built successfully"

dev-logs:
	@echo "📝 Following development service logs (Ctrl+C to stop)..."
	$(DOCKER_COMPOSE) logs -f

dev-restart:
	@echo "🔄 Restarting development services..."
	$(DOCKER_COMPOSE) restart
	@echo "✅ Development services restarted"

dev-ps:
	@echo "📊 Development Service Status:"
	@echo ""
	$(DOCKER_COMPOSE) ps
	@echo ""
	@echo "Healthy services:"
	@docker ps --filter "health=healthy" --format "  ✅ {{.Names}}"
	@echo ""
	@echo "Unhealthy services:"
	@docker ps --filter "health=unhealthy" --format "  ❌ {{.Names}}"

help-docker:
	@echo "Docker Compose Commands:"
	@echo ""
	@echo "  make dev              - Start complete development environment"
	@echo "  make up               - Start all services"
	@echo "  make down             - Stop all services"
	@echo "  make restart          - Restart all services"
	@echo "  make ps               - Show service status"
	@echo "  make logs             - Follow all service logs"
	@echo "  make build            - Build all services"
	@echo "  make clean            - Stop services and remove volumes"
	@echo "  make import-schema    - Import database schema only"
	@echo "  make import-database  - Import database with data (metargb_db.sql)"
	@echo ""
	@echo "Development (Docker Compose Watch):"
	@echo "  make dev-up           - Start services with watch mode (auto-rebuild/restart)"
	@echo "  make dev-down         - Stop development services"
	@echo "  make dev-build        - Build development images"
	@echo "  make dev-logs         - View logs from dev services"
	@echo ""
	@echo "Service-specific commands:"
	@echo "  make build-service SERVICE=auth-service   - Build specific service"
	@echo "  make start-service SERVICE=auth-service   - Start specific service"
	@echo "  make stop-service SERVICE=auth-service    - Stop specific service"
	@echo "  make logs-service SERVICE=auth-service    - View service logs"
	@echo ""
	@echo "Examples:"
	@echo "  make dev                                  - Complete setup"
	@echo "  make dev-up                               - Start with watch mode (auto-rebuild/restart)"
	@echo "  make logs-service SERVICE=auth-service    - View auth logs"
	@echo "  make restart                              - Restart everything"
