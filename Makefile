.PHONY: proto clean-proto gen-auth gen-commercial gen-features gen-levels gen-dynasty gen-support gen-training gen-notifications gen-calendar gen-storage gen-financial gen-all help build-all deploy-all test up down restart logs ps build clean clean-runtime dev dev-up dev-down link-uploads init-storage-uploads init-storage-uploads openapi docs docs-up

# Proto generation
PROTO_DIR=shared/proto
PROTO_OUT_DIR=shared/pb

# Docker
DOCKER_REGISTRY=metargb
VERSION?=latest

# Local uploads (storage-service writes here; link-uploads exposes it at project root)
UPLOADS_SRC=services/storage-service/uploads
UPLOADS_LINK=uploads

# Docker Compose compatibility - auto-detect docker-compose or docker compose plugin
# Windows PowerShell doesn't support 'command -v', so default to 'docker compose' (modern Docker Desktop)
ifeq ($(OS),Windows_NT)
DOCKER_COMPOSE := docker compose
else
DOCKER_COMPOSE := $(shell command -v docker-compose 2> /dev/null || echo "docker compose")
endif

help:
	@echo "Metarang microservices — available targets"
	@echo ""
	@echo "Proto Generation:"
	@echo "  proto            - Generate all proto files"
	@echo "  gen-auth         - Generate auth service proto"
	@echo "  gen-commercial   - Generate commercial service proto"
	@echo "  gen-features     - Generate features service proto"
	@echo "  gen-levels       - Generate levels service proto"
	@echo "  clean-proto      - Clean generated proto files"
	@echo ""
	@echo "API Documentation:"
	@echo "  openapi          - Generate openapi/openapi.yaml from openapi/routes.yaml"
	@echo "  docs             - Generate spec and start Swagger UI"
	@echo "  docs-up          - Start Swagger UI only (spec must exist)"
	@echo ""
	@echo "Docker Images:"
	@echo "  build-all        - Build all service Docker images"
	@echo "  build-features   - Build features service Docker image"
	@echo "  build-levels     - Build levels service Docker image"
	@echo "  build-service    - Build one service (SERVICE=auth-service)"
	@echo ""
	@echo "Deploy:"
	@echo "  deploy-all       - Deploy all services to Kubernetes"
	@echo ""
	@echo "Testing:"
	@echo "  test             - Run integration tests"
	@echo "  test-all         - Run all test suites (unit, integration, golden, database)"
	@echo "  test-coverage-features  - features-service handler coverage ≥70%"
	@echo "  test-coverage-financial - financial-service handler coverage ≥70%"
	@echo "  test-coverage-social    - social-service handler+service coverage ≥70%"
	@echo ""
	@echo "Local uploads:"
	@echo "  link-uploads           - Symlink ./uploads -> $(UPLOADS_SRC)"
	@echo "  init-storage-uploads   - Create local storage-service uploads directory"
	@echo ""
	@echo "Docker Compose:"
	@echo "  dev              - Start complete development environment"
	@echo "  up               - Start all services"
	@echo "  down             - Stop all services"
	@echo "  restart          - Restart all services"
	@echo "  ps               - Show service status"
	@echo "  logs             - Follow all service logs"
	@echo "  build            - Build all services"
	@echo "  clean            - Stop services and remove volumes"
	@echo "  clean-runtime    - Full local cleanup (containers, volumes, images, build cache)"
	@echo ""
	@echo "Docker Compose Watch:"
	@echo "  dev-up           - Start with watch mode (auto-rebuild/restart)"
	@echo "  dev-down         - Stop development services"
	@echo "  dev-build        - Build development images"
	@echo "  dev-logs         - View logs from dev services"
	@echo "  dev-restart      - Restart development services"
	@echo "  dev-ps           - Show development service status"
	@echo ""
	@echo "Database:"
	@echo "  import-schema    - Import database schema only (schema.sql)"
	@echo "  import-database  - Import database with data (metargb_db.sql)"
	@echo ""
	@echo "Service-specific (set SERVICE=service-name):"
	@echo "  build-service    - Build specific service"
	@echo "  start-service    - Start specific service"
	@echo "  stop-service     - Stop specific service"
	@echo "  logs-service     - View service logs"



# =============================================================================
# OpenAPI Documentation
# =============================================================================

openapi:
	@echo "📄 Generating OpenAPI specification..."
ifeq ($(OS),Windows_NT)
	@powershell -NoProfile -Command "cd scripts/gen-openapi; $$env:GOWORK='off'; go run ."
else
	cd scripts/gen-openapi && GOWORK=off go run .
endif
	@echo "✅ openapi/openapi.yaml generated"

docs: openapi docs-up
	@echo ""
	@echo "📚 API documentation available at:"
	@echo "  Swagger UI (direct):  http://localhost:8081"
	@echo "  Swagger UI (Kong):    http://localhost:8000/docs"
	@echo "  OpenAPI spec:         http://localhost:8081/openapi/openapi.yaml"

docs-up:
	@echo "🚀 Starting Swagger UI..."
	$(DOCKER_COMPOSE) up -d swagger-ui
	@echo "✅ Swagger UI started"

# =============================================================================
# Testing
# =============================================================================

# Unit tests
test-unit:
	@echo "🧪 Running unit tests for all services..."
	@for service in services/*/; do \
		if [ -f "$$service/go.mod" ]; then \
			echo "Testing $$(basename $$service)..."; \
			cd $$service && \
			if [ -d internal ]; then \
				go test ./internal/... -v -race -coverprofile=coverage.out || exit 1; \
			else \
				go test ./... -v -race -coverprofile=coverage.out || exit 1; \
			fi; \
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

# Financial-service handler coverage gate (≥70%)
test-coverage-financial:
	@echo "🧪 financial-service handler coverage (min 70%)..."
	cd services/financial-service && go test ./internal/handler/... -race -coverprofile=coverage.out -covermode=atomic
	@pct=$$(cd services/financial-service && go tool cover -func=coverage.out | tail -1 | grep -oE '[0-9]+\.[0-9]+' | tail -1); \
	echo "financial-service handler statements coverage: $${pct}%"; \
	awk -v p="$$pct" 'BEGIN{if (p+0 < 70.0) exit 1}'
	@echo "✅ financial-service handler coverage OK"

# Social-service handler + service coverage gate (≥70%, combined packages)
test-coverage-social:
	@echo "🧪 social-service handler+service coverage (min 70%)..."
	cd services/social-service && go test ./internal/handler/... ./internal/service/... -race -coverprofile=coverage.out -covermode=atomic
	@pct=$$(cd services/social-service && go tool cover -func=coverage.out | tail -1 | grep -oE '[0-9]+\.[0-9]+' | tail -1); \
	echo "social-service handler+service statements coverage: $${pct}%"; \
	awk -v p="$$pct" 'BEGIN{if (p+0 < 70.0) exit 1}'
	@echo "✅ social-service coverage OK"

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
# Local uploads symlink
# =============================================================================

.PHONY: link-uploads

link-uploads:
	@echo "Creating symlink: $(UPLOADS_LINK) -> $(UPLOADS_SRC)"
ifeq ($(OS),Windows_NT)
	@powershell -NoProfile -Command "$$ErrorActionPreference='Stop'; \
		if (-not (Test-Path -LiteralPath '$(UPLOADS_SRC)')) { New-Item -ItemType Directory -Force -Path '$(UPLOADS_SRC)' | Out-Null }; \
		$$target = (Resolve-Path -LiteralPath '$(UPLOADS_SRC)').Path; \
		if (Test-Path -LiteralPath '$(UPLOADS_LINK)') { \
			$$item = Get-Item -LiteralPath '$(UPLOADS_LINK)' -Force; \
			if ($$item.Attributes -band [IO.FileAttributes]::ReparsePoint) { \
				Write-Host 'Link already exists: $(UPLOADS_LINK) -> $(UPLOADS_SRC)'; exit 0 \
			}; \
			Write-Error '$(UPLOADS_LINK) already exists and is not a link to $(UPLOADS_SRC)'; exit 1 \
		}; \
		try { \
			New-Item -ItemType SymbolicLink -Path '$(UPLOADS_LINK)' -Target $$target | Out-Null; \
			Write-Host 'Created symlink: $(UPLOADS_LINK) -> $(UPLOADS_SRC)' \
		} catch { \
			Write-Host 'Symbolic link unavailable (enable Developer Mode or run as admin); creating directory junction...'; \
			$$null = cmd /c mklink /J \"$(UPLOADS_LINK)\" \"$$target\"; \
			if ($$LASTEXITCODE -ne 0) { exit $$LASTEXITCODE }; \
			Write-Host 'Created junction: $(UPLOADS_LINK) -> $(UPLOADS_SRC)' \
		}"
else
	@mkdir -p $(UPLOADS_SRC)
	@ln -sfn $(UPLOADS_SRC) $(UPLOADS_LINK)
	@echo "Created symlink: $(UPLOADS_LINK) -> $(UPLOADS_SRC)"
endif

.PHONY: init-storage-uploads

init-storage-uploads:
	@echo "Ensuring storage upload directory exists: $(UPLOADS_SRC)"
ifeq ($(OS),Windows_NT)
	@powershell -NoProfile -Command "New-Item -ItemType Directory -Force -Path '$(UPLOADS_SRC)' | Out-Null"
else
	@mkdir -p $(UPLOADS_SRC)
endif

# =============================================================================
# Docker Compose Management
# =============================================================================

.PHONY: up down restart logs ps build clean import-schema import-database dev-up dev-down dev-build dev-logs dev-restart dev-ps

up: init-storage-uploads
	@echo "🚀 Starting all microservices..."
	$(DOCKER_COMPOSE) up -d
	@echo "✅ All services started!"
	@echo ""
	@echo "Services available at:"
	@echo "  Kong API Gateway: http://localhost:8000"
	@echo "  API Docs:         http://localhost:8000/docs"
	@echo "  Swagger UI:       http://localhost:8081"
	@echo "  Kong Admin:       http://localhost:8001"
	@echo "  WebSocket:        http://localhost:3002"
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

clean-runtime:
	@echo "💥 Removing all local runtime/build artifacts for this project..."
	$(DOCKER_COMPOSE) down --volumes --remove-orphans --rmi all
	@echo "🧱 Pruning Docker build cache..."
	docker builder prune -af
	@echo "✅ Runtime cleanup complete"

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
ifeq ($(OS),Windows_NT)
	@docker exec metargb-mysql mysql -uroot -proot_password metargb_db -e "SELECT COUNT(*) as table_count FROM information_schema.tables WHERE table_schema='metargb_db';" 2>nul | findstr /v table_count || echo "Could not verify"
else
	@docker exec metargb-mysql mysql -uroot -proot_password metargb_db -e "SELECT COUNT(*) as table_count FROM information_schema.tables WHERE table_schema='metargb_db';" 2>/dev/null | grep -v table_count || echo "Could not verify"
endif

import-database:
ifeq ($(wildcard scripts/metargb_db.sql),)
ifeq ($(wildcard scripts/metarang_db.sql),)
	@echo "❌ Database dump not found. Place metargb_db.sql or metarang_db.sql in scripts/"
	@exit 1
else
	$(eval DB_DUMP_FILE := scripts/metarang_db.sql)
endif
else
	$(eval DB_DUMP_FILE := scripts/metargb_db.sql)
endif
	@echo "Importing database (schema + data) from $(DB_DUMP_FILE)..."
	@echo "Dropping and recreating database..."
ifeq ($(OS),Windows_NT)
	@docker exec -i metargb-mysql mysql -uroot -proot_password -e "DROP DATABASE IF EXISTS metargb_db; CREATE DATABASE metargb_db CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
	@echo "Importing data..."
	@powershell -NoProfile -Command "Get-Content -Raw '$(DB_DUMP_FILE)' | docker exec -i metargb-mysql mysql -uroot -proot_password metargb_db"
else
	@docker exec -i metargb-mysql mysql -uroot -proot_password -e "DROP DATABASE IF EXISTS metargb_db; CREATE DATABASE metargb_db CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;" 2>/dev/null || true
	@echo "Importing data..."
	@docker exec -i metargb-mysql mysql -uroot -proot_password metargb_db < $(DB_DUMP_FILE)
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
ifeq ($(OS),Windows_NT)
	@powershell -NoProfile -Command "Start-Sleep -Seconds 10"
	@echo "Checking if schema needs to be imported..."
	@powershell -NoProfile -Command "$$ErrorActionPreference='Continue'; $$tableCount = (docker exec metargb-mysql mysql -uroot -proot_password metargb_db -e \"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='metargb_db';\" 2>$$null | Select-Object -Last 1).Trim(); if ($$tableCount -eq '0') { Write-Host 'Importing schema...'; make import-schema } else { Write-Host (\"✅ Database already initialized ({0} tables)\" -f $$tableCount) }"
else
	@sleep 10
	@echo "Checking if schema needs to be imported..."
	@TABLE_COUNT=$$(docker exec metargb-mysql mysql -uroot -proot_password metargb_db -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='metargb_db';" 2>/dev/null | tail -1); \
	if [ "$$TABLE_COUNT" = "0" ]; then \
		echo "Importing schema..."; \
		make import-schema; \
	else \
		echo "✅ Database already initialized ($$TABLE_COUNT tables)"; \
	fi
endif
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
	fi
	$(DOCKER_COMPOSE) start $(SERVICE)

logs-service:
	@if [ -z "$(SERVICE)" ]; then \
		echo "❌ Please specify SERVICE=service-name"; \
		exit 1; \
	fi
	$(DOCKER_COMPOSE) logs -f $(SERVICE)

# Ensure storage-service upload bind mount exists before Docker creates a root-owned dir
.PHONY: init-storage-uploads
init-storage-uploads:
ifeq ($(OS),Windows_NT)
	@powershell -NoProfile -Command "if (-not (Test-Path -LiteralPath '$(UPLOADS_SRC)')) { New-Item -ItemType Directory -Force -Path '$(UPLOADS_SRC)' | Out-Null }"
else
	@mkdir -p $(UPLOADS_SRC)
endif

# =============================================================================
# Development with Hot Reloading
# =============================================================================

dev-up: init-storage-uploads
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
