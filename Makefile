.PHONY: proto clean-proto gen-auth gen-commercial gen-features gen-levels gen-dynasty gen-support gen-training gen-notifications gen-calendar gen-storage gen-financial gen-all help build-all deploy-all test test-unit test-services test-database test-all up down restart logs ps build clean clean-runtime dev dev-up dev-down link-uploads init-storage-uploads init-storage-uploads openapi docs docs-up kong-validate kong-reload migrate migrate-rollback migrate-status migrate-reset migrate-refresh migrate-baseline migrate-make migrate-install ensure-networks wait-mysql start-migrate-job auto-migrate

# Proto generation
PROTO_DIR=shared/proto
PROTO_OUT_DIR=shared/pb

# Docker
DOCKER_REGISTRY=abbasajorloo
VERSION?=latest
MYSQL_ROOT_PASSWORD ?= root_password
MYSQL_DATABASE ?= metarang_db

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
	@echo "  test             - Run all test suites (alias for test-all)"
	@echo "  test-unit        - Run unit tests inside each service module"
	@echo "  test-services    - Run dedicated service test modules (tests/*-service/)"
	@echo "  test-all         - Run all test suites (unit, services, database)"
	@echo "  test-coverage-features  - features-service handler coverage ≥70%"
	@echo "  test-coverage-financial - financial-service internal coverage ≥70%"
	@echo "  test-coverage-social    - social-service internal coverage ≥70%"
	@echo ""
	@echo "Local uploads:"
	@echo "  link-uploads           - Symlink ./uploads -> $(UPLOADS_SRC)"
	@echo "  init-storage-uploads   - Create local storage-service uploads directory"
	@echo ""
	@echo "Docker Compose:"
	@echo "  ensure-networks  - Create dokploy-network and metarang-shared if missing"
	@echo "  wait-mysql       - Block until Compose MySQL is healthy"
	@echo "  dev              - Start complete development environment (migrates if $(MYSQL_DATABASE) exists)"
	@echo "  up               - Start all services (migrates if $(MYSQL_DATABASE) exists, then apps)"
	@echo "  down             - Stop all services"
	@echo "  restart          - Run migrate if $(MYSQL_DATABASE) exists, then restart all services"
	@echo "  ps               - Show service status"
	@echo "  logs             - Follow all service logs"
	@echo "  build            - Build all services"
	@echo "  clean            - Stop services and remove volumes"
	@echo "  clean-runtime    - Full local cleanup (containers, volumes, images, build cache)"
	@echo "  kong-validate    - Validate kong/kong.yml inside the Kong container"
	@echo "  kong-reload      - Restart Kong (required after kong/kong.yml changes in DB-less mode)"
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
	@echo "  import-schema         - Import database schema only (schema.sql), then baseline migrations"
	@echo "  import-database       - Import database with data (metarang_db.sql)"
	@echo "  migrate               - Run pending SQL migrations (Compose migrate service)"
	@echo "  migrate-rollback      - Roll back the last migration batch (STEP=N optional)"
	@echo "  migrate-status        - Show ran / pending migrations"
	@echo "  migrate-reset         - Roll back all migrations"
	@echo "  migrate-refresh       - Reset, then migrate"
	@echo "  migrate-baseline      - Mark pending files as ran without executing SQL"
	@echo "  migrate-make          - Create a migration stub (NAME=add_foo_to_bar)"
	@echo "  AUTO_MIGRATE=0        - Skip migrate on up/dev/dev-up/restart (sets SKIP_MIGRATE=1)"
	@echo "  Auto-migrate also skips when database $(MYSQL_DATABASE) does not exist"
	@echo "  USE_HOST_MIGRATE=1    - Use go run ./shared/cmd/migrate instead of Compose"
	@echo ""
	@echo "Service-specific (set SERVICE=service-name):"
	@echo "  build-service    - Build specific service"
	@echo "  start-service    - Start specific service"
	@echo "  stop-service     - Stop specific service"
	@echo "  logs-service     - View service logs"



# =============================================================================
# OpenAPI Documentation
# =============================================================================

# Generate openapi/openapi.yaml from openapi/routes.yaml (+ handler discovery).
# Uses GOWORK=off so scripts/gen-openapi can run outside the root go.work (Git Bash + Unix).
openapi:
	@echo "📄 Generating OpenAPI specification..."
	cd scripts/gen-openapi && GOWORK=off go run .
	@echo "✅ openapi/openapi.yaml generated"

docs: openapi docs-up
	@echo ""
	@echo "📚 API documentation available at:"
	@echo "  Swagger UI (direct):  http://localhost:8081"
	@echo "  Swagger UI (Kong):    http://localhost:8000/"
	@echo "  OpenAPI spec:         http://localhost:8081/openapi/openapi.yaml"

docs-up: ensure-networks
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

# Dedicated service test modules under tests/ (excludes database)
SERVICE_TEST_MODULES=auth-service calendar-service commercial-service dynasty-service features-service financial-service notifications-service social-service storage-service support-service

test-services:
	@echo "🧪 Running dedicated service test modules..."
ifeq ($(OS),Windows_NT)
	@powershell -NoProfile -Command "$$ErrorActionPreference='Stop'; \
		@('auth-service','calendar-service','commercial-service','dynasty-service','features-service','financial-service','notifications-service','social-service','storage-service','support-service') | ForEach-Object { \
			Write-Host ('Testing ' + $$_ + '...'); \
			Set-Location ('tests/' + $$_); \
			$$env:GOWORK='off'; \
			go test ./... -v -coverprofile=coverage.out; \
			if ($$LASTEXITCODE -ne 0) { exit $$LASTEXITCODE }; \
			Set-Location ../.. \
		}"
else
	@for module in $(SERVICE_TEST_MODULES); do \
		echo "Testing $$module..."; \
		cd tests/$$module && GOWORK=off go test ./... -v -race -coverprofile=coverage.out || exit 1; \
		cd ../..; \
	done
endif
	@echo "✅ All service test modules passed"

COVERAGE_MIN ?= 70

# Features-service handler coverage gate (≥70%, no MySQL required; uses GOWORK=off for local replace)
test-coverage-features:
	@echo "🧪 features-service handler coverage (min $(COVERAGE_MIN)%)..."
ifeq ($(OS),Windows_NT)
	@powershell -NoProfile -Command "$$ErrorActionPreference='Stop'; \
		Set-Location 'tests/features-service'; \
		$$env:GOWORK='off'; \
		go test ./internal/handler/... -coverprofile=coverage.out -covermode=atomic; \
		if ($$LASTEXITCODE -ne 0) { exit $$LASTEXITCODE }; \
		$$line = go tool cover -func coverage.out | Select-String 'total:'; \
		if ($$line -notmatch '(\d+\.\d+)%') { Write-Error 'could not parse coverage total'; exit 1 }; \
		$$pct = [double]$$matches[1]; \
		Write-Host ('handler statements coverage: ' + $$pct + '%'); \
		if ($$pct -lt $(COVERAGE_MIN)) { exit 1 }"
else
	cd tests/features-service && GOWORK=off go test ./internal/handler/... -race -coverprofile=coverage.out -covermode=atomic
	@pct=$$(cd tests/features-service && GOWORK=off go tool cover -func=coverage.out | tail -1 | grep -oE '[0-9]+\.[0-9]+' | tail -1); \
	echo "handler statements coverage: $${pct}%"; \
	awk -v p="$$pct" 'BEGIN{if (p+0 < $(COVERAGE_MIN).0) exit 1}'
endif
	@echo "✅ features-service handler coverage OK"

# Financial-service handler coverage gate (≥70%)
test-coverage-financial:
	@echo "🧪 financial-service internal coverage (min $(COVERAGE_MIN)%)..."
ifeq ($(OS),Windows_NT)
	@powershell -NoProfile -Command "$$ErrorActionPreference='Stop'; \
		Set-Location 'tests/financial-service'; \
		$$env:GOWORK='off'; \
		go test ./internal/... -coverprofile=coverage.out -covermode=atomic -coverpkg='metarang/financial-service/internal/handler,metarang/financial-service/internal/middleware,metarang/financial-service/internal/repository,metarang/financial-service/internal/service,metarang/financial-service/internal/grpcclients,metarang/financial-service/internal/config,metarang/financial-service/internal/sadad,metarang/financial-service/cmd/server'; \
		if ($$LASTEXITCODE -ne 0) { exit $$LASTEXITCODE }; \
		$$line = go tool cover -func coverage.out | Select-String 'total:'; \
		if ($$line -notmatch '(\d+\.\d+)%') { Write-Error 'could not parse coverage total'; exit 1 }; \
		$$pct = [double]$$matches[1]; \
		Write-Host ('financial-service internal statements coverage: ' + $$pct + '%'); \
		if ($$pct -lt $(COVERAGE_MIN)) { exit 1 }"
else
	cd tests/financial-service && GOWORK=off go test ./internal/... -race -coverprofile=coverage.out -covermode=atomic -coverpkg=metarang/financial-service/internal/handler,metarang/financial-service/internal/middleware,metarang/financial-service/internal/repository,metarang/financial-service/internal/service,metarang/financial-service/internal/grpcclients,metarang/financial-service/internal/config,metarang/financial-service/internal/sadad,metarang/financial-service/cmd/server
	@pct=$$(cd tests/financial-service && GOWORK=off go tool cover -func=coverage.out | tail -1 | grep -oE '[0-9]+\.[0-9]+' | tail -1); \
	echo "financial-service internal statements coverage: $${pct}%"; \
	awk -v p="$$pct" 'BEGIN{if (p+0 < $(COVERAGE_MIN).0) exit 1}'
endif
	@echo "✅ financial-service coverage OK"

# Social-service internal coverage gate (≥70%: handler, middleware, repository, service, lang)
test-coverage-social:
	@echo "🧪 social-service internal coverage (min $(COVERAGE_MIN)%)..."
ifeq ($(OS),Windows_NT)
	@powershell -NoProfile -Command "$$ErrorActionPreference='Stop'; \
		Set-Location 'tests/social-service'; \
		$$env:GOWORK='off'; \
		go test ./... -coverprofile=coverage.out -covermode=atomic -coverpkg='metarang/social-service/internal/handler,metarang/social-service/internal/middleware,metarang/social-service/internal/repository,metarang/social-service/internal/service,metarang/social-service/internal/lang'; \
		if ($$LASTEXITCODE -ne 0) { exit $$LASTEXITCODE }; \
		$$line = go tool cover -func coverage.out | Select-String 'total:'; \
		if ($$line -notmatch '(\d+\.\d+)%') { Write-Error 'could not parse coverage total'; exit 1 }; \
		$$pct = [double]$$matches[1]; \
		Write-Host ('social-service internal statements coverage: ' + $$pct + '%'); \
		if ($$pct -lt $(COVERAGE_MIN)) { exit 1 }"
else
	cd tests/social-service && GOWORK=off go test ./... -race -coverprofile=coverage.out -covermode=atomic -coverpkg=metarang/social-service/internal/handler,metarang/social-service/internal/middleware,metarang/social-service/internal/repository,metarang/social-service/internal/service,metarang/social-service/internal/lang
	@pct=$$(cd tests/social-service && GOWORK=off go tool cover -func=coverage.out | tail -1 | grep -oE '[0-9]+\.[0-9]+' | tail -1); \
	echo "social-service internal statements coverage: $${pct}%"; \
	awk -v p="$$pct" 'BEGIN{if (p+0 < $(COVERAGE_MIN).0) exit 1}'
endif
	@echo "✅ social-service coverage OK"

# Notifications-service internal coverage gate (≥70%)
test-coverage-notifications:
	@echo "🧪 notifications-service internal coverage (min $(COVERAGE_MIN)%)..."
ifeq ($(OS),Windows_NT)
	@powershell -NoProfile -Command "$$ErrorActionPreference='Stop'; \
		Set-Location 'tests/notifications-service'; \
		$$env:GOWORK='off'; \
		go test ./... -coverprofile=coverage.out -covermode=atomic -coverpkg='metarang/notifications-service/internal/handler,metarang/notifications-service/internal/middleware,metarang/notifications-service/internal/repository,metarang/notifications-service/internal/service'; \
		if ($$LASTEXITCODE -ne 0) { exit $$LASTEXITCODE }; \
		$$line = go tool cover -func coverage.out | Select-String 'total:'; \
		if ($$line -notmatch '(\d+\.\d+)%') { Write-Error 'could not parse coverage total'; exit 1 }; \
		$$pct = [double]$$matches[1]; \
		Write-Host ('notifications-service internal statements coverage: ' + $$pct + '%'); \
		if ($$pct -lt $(COVERAGE_MIN)) { exit 1 }"
else
	cd tests/notifications-service && GOWORK=off go test ./... -race -coverprofile=coverage.out -covermode=atomic -coverpkg=metarang/notifications-service/internal/handler,metarang/notifications-service/internal/middleware,metarang/notifications-service/internal/repository,metarang/notifications-service/internal/service
	@pct=$$(cd tests/notifications-service && GOWORK=off go tool cover -func=coverage.out | tail -1 | grep -oE '[0-9]+\.[0-9]+' | tail -1); \
	echo "notifications-service internal statements coverage: $${pct}%"; \
	awk -v p="$$pct" 'BEGIN{if (p+0 < $(COVERAGE_MIN).0) exit 1}'
endif
	@echo "✅ notifications-service coverage OK"

# Database tests
test-database:
	@echo "🧪 Running database schema and concurrency tests..."
	cd tests/database && go test -v ./...

# Run all tests
test-all: test-unit test-services test-database
	@echo "✅ All test suites passed"

test: test-all

# =============================================================================
# Local uploads symlink
# =============================================================================

.PHONY: link-uploads

# Default values
UPLOADS_PATH ?= $(CURDIR)/services/storage-service/uploads
UPLOADS_LINK ?= $(CURDIR)/uploads

link-uploads:
	@echo "Creating symlink: $(UPLOADS_LINK) -> $(UPLOADS_PATH)"
ifeq ($(OS),Windows_NT)
	@powershell -NoProfile -Command "$$ErrorActionPreference='Stop'; \
		if (-not (Test-Path -LiteralPath '$(UPLOADS_PATH)')) { New-Item -ItemType Directory -Force -Path '$(UPLOADS_PATH)' | Out-Null }; \
		$$target = (Resolve-Path -LiteralPath '$(UPLOADS_PATH)').Path; \
		if (Test-Path -LiteralPath '$(UPLOADS_LINK)') { \
			$$item = Get-Item -LiteralPath '$(UPLOADS_LINK)' -Force; \
			if ($$item.Attributes -band [IO.FileAttributes]::ReparsePoint) { \
				Remove-Item -LiteralPath '$(UPLOADS_LINK)' -Force; \
			} else { \
				Write-Error '$(UPLOADS_LINK) already exists and is not a symlink/junction'; exit 1 \
			}; \
		}; \
		try { \
			New-Item -ItemType SymbolicLink -Path '$(UPLOADS_LINK)' -Target $$target | Out-Null; \
			Write-Host 'Created symlink: $(UPLOADS_LINK) -> $(UPLOADS_PATH)' \
		} catch { \
			Write-Host 'Symbolic link unavailable (enable Developer Mode or run as admin); creating directory junction...'; \
			$$null = cmd /c mklink /J \"$(UPLOADS_LINK)\" \"$$target\"; \
			if ($$LASTEXITCODE -ne 0) { exit $$LASTEXITCODE }; \
			Write-Host 'Created junction: $(UPLOADS_LINK) -> $(UPLOADS_PATH)' \
		}"
else
	@mkdir -p "$(UPLOADS_PATH)"
	@ln -sfn "$(UPLOADS_PATH)" "$(UPLOADS_LINK)"
	@echo "Created symlink: $(UPLOADS_LINK) -> $(UPLOADS_PATH)"
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

.PHONY: up down restart logs ps build clean import-schema import-database dev-up dev-down dev-build dev-logs dev-restart dev-ps kong-validate kong-reload migrate migrate-rollback migrate-status migrate-reset migrate-refresh migrate-baseline migrate-make migrate-install wait-mysql start-migrate-job auto-migrate

# When 1 (default), make up/dev/dev-up/restart run migrations if $(MYSQL_DATABASE) exists.
AUTO_MIGRATE ?= 1
# When 1, run migrations via go run on the host instead of the Compose migrate service.
USE_HOST_MIGRATE ?= 0

kong-validate:
	@echo "🔍 Validating Kong declarative config..."
	$(DOCKER_COMPOSE) exec -T kong kong config parse /kong/kong.yml

# DB-less Kong loads kong.yml only at startup — restart after route changes.
kong-reload:
	@echo "🔄 Restarting Kong to apply kong/kong.yml..."
	$(DOCKER_COMPOSE) restart kong

# External networks used by Dokploy (Traefik) and the Laravel admin panel (shared MySQL).
ensure-networks:
ifeq ($(OS),Windows_NT)
	-@docker network create dokploy-network
	-@docker network create metarang-shared
else
	@docker network inspect dokploy-network >/dev/null 2>&1 || docker network create dokploy-network
	@docker network inspect metarang-shared >/dev/null 2>&1 || docker network create metarang-shared
endif

up: init-storage-uploads ensure-networks
	@echo "🚀 Starting all microservices..."
	@echo "Starting MySQL, Redis, and migrate job..."
	$(DOCKER_COMPOSE) up -d mysql
	@$(MAKE) start-migrate-job
	$(DOCKER_COMPOSE) up -d
	@echo "✅ All services started!"
	@echo ""
	@echo "Services available at:"
	@echo "  Kong API Gateway: http://localhost:8000"
	@echo "  API Docs:         http://localhost:8000/"
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
	@$(MAKE) auto-migrate
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
build-clean:
	@echo "🔄 Cleaning up and rebuilding all services..."
	$(DOCKER_COMPOSE) down -v
	docker system prune -f
	$(DOCKER_COMPOSE) build --no-cache
	@echo "✅ Clean build complete"

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

# Laravel-style SQL migrations (scripts/migrations + `migrations` table).
DB_HOST ?= 127.0.0.1
DB_PORT ?= 3306
DB_USER ?= root
DB_PASSWORD ?= root_password
DB_DATABASE ?= metarang_db
MIGRATIONS_PATH ?= scripts/migrations
STEP ?=
PRETEND ?=
MYSQL_WAIT_SECONDS ?= 90

# Host go-run migrator (USE_HOST_MIGRATE=1).
ifeq ($(OS),Windows_NT)
run_host_migrate = powershell -NoProfile -Command "$$ErrorActionPreference='Stop'; $$env:DB_HOST='$(DB_HOST)'; $$env:DB_PORT='$(DB_PORT)'; $$env:DB_USER='$(DB_USER)'; $$env:DB_PASSWORD='$(DB_PASSWORD)'; $$env:DB_DATABASE='$(DB_DATABASE)'; go run ./shared/cmd/migrate $(1) -path=$(MIGRATIONS_PATH) $(if $(STEP),-step=$(STEP),) $(if $(PRETEND),-pretend,)"
else
run_host_migrate = DB_HOST=$(DB_HOST) DB_PORT=$(DB_PORT) DB_USER=$(DB_USER) DB_PASSWORD=$(DB_PASSWORD) DB_DATABASE=$(DB_DATABASE) go run ./shared/cmd/migrate $(1) -path=$(MIGRATIONS_PATH) $(if $(STEP),-step=$(STEP),) $(if $(PRETEND),-pretend,)
endif

# Compose one-shot migrator (default). Ensures MySQL is up, then run --rm migrate.
define run_compose_migrate
	$(DOCKER_COMPOSE) up -d mysql
	$(DOCKER_COMPOSE) run --rm migrate $(1) $(if $(STEP),-step=$(STEP),) $(if $(PRETEND),-pretend,)
endef

ifeq ($(USE_HOST_MIGRATE),1)
run_migrate = $(run_host_migrate)
else
run_migrate = $(run_compose_migrate)
endif

# Block until the Compose MySQL service accepts connections (required before migrate).
wait-mysql:
	@echo "⏳ Waiting for MySQL to be ready..."
ifeq ($(OS),Windows_NT)
	@powershell -NoProfile -Command "\
		$$deadline = (Get-Date).AddSeconds($(MYSQL_WAIT_SECONDS)); \
		while ($$true) { \
			cmd /c 'docker compose exec -T mysql mysqladmin ping -h localhost -uroot -p$(MYSQL_ROOT_PASSWORD) --silent >nul 2>&1'; \
			if ($$LASTEXITCODE -eq 0) { Write-Host '✅ MySQL is ready'; exit 0 }; \
			if ((Get-Date) -gt $$deadline) { Write-Error 'MySQL did not become ready within $(MYSQL_WAIT_SECONDS)s'; exit 1 }; \
			Start-Sleep -Seconds 2 \
		}"
else
	@deadline=$$(( $$(date +%s) + $(MYSQL_WAIT_SECONDS) )); \
	while true; do \
		if $(DOCKER_COMPOSE) exec -T mysql mysqladmin ping -h localhost -uroot -p$(MYSQL_ROOT_PASSWORD) --silent 2>/dev/null; then \
			echo "✅ MySQL is ready"; \
			exit 0; \
		fi; \
		if [ "$$(date +%s)" -ge "$$deadline" ]; then \
			echo "❌ MySQL did not become ready within $(MYSQL_WAIT_SECONDS)s"; \
			exit 1; \
		fi; \
		sleep 2; \
	done
endif

# Recreate the Compose migrate job. Skips SQL when AUTO_MIGRATE=0 or MYSQL_DATABASE is missing.
# The container still runs (no-op) so DB-backed services can wait on service_completed_successfully.
start-migrate-job:
	@$(MAKE) wait-mysql
ifeq ($(AUTO_MIGRATE),1)
ifeq ($(OS),Windows_NT)
	@powershell -NoProfile -Command "\
		$$ErrorActionPreference='Continue'; \
		$$count = (docker compose exec -T mysql mysql -uroot -p$(MYSQL_ROOT_PASSWORD) -Nse \"SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME='$(MYSQL_DATABASE)';\" 2>$$null | Select-Object -Last 1).Trim(); \
		if ($$count -ne '1') { \
			Write-Host '⏭️  Skipping migrations: database $(MYSQL_DATABASE) does not exist'; \
			$$env:SKIP_MIGRATE='1'; \
		} else { \
			Write-Host '📦 Database $(MYSQL_DATABASE) exists; running migrations...'; \
			$$env:SKIP_MIGRATE='0'; \
		}; \
		$(DOCKER_COMPOSE) up -d --force-recreate migrate"
else
	@DB_COUNT=$$($(DOCKER_COMPOSE) exec -T mysql mysql -uroot -p$(MYSQL_ROOT_PASSWORD) -Nse "SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME='$(MYSQL_DATABASE)';" 2>/dev/null | tr -d '\r'); \
	if [ "$$DB_COUNT" != "1" ]; then \
		echo "⏭️  Skipping migrations: database $(MYSQL_DATABASE) does not exist"; \
		SKIP_MIGRATE=1 $(DOCKER_COMPOSE) up -d --force-recreate migrate; \
	else \
		echo "📦 Database $(MYSQL_DATABASE) exists; running migrations..."; \
		SKIP_MIGRATE=0 $(DOCKER_COMPOSE) up -d --force-recreate migrate; \
	fi
endif
else
	@echo "⏭️  Skipping migrations (AUTO_MIGRATE=$(AUTO_MIGRATE))"
ifeq ($(OS),Windows_NT)
	@powershell -NoProfile -Command "$$env:SKIP_MIGRATE='1'; $(DOCKER_COMPOSE) up -d --force-recreate migrate"
else
	SKIP_MIGRATE=1 $(DOCKER_COMPOSE) up -d --force-recreate migrate
endif
endif

# Explicit migrate (restart/dev-restart). Skips when AUTO_MIGRATE=0 or MYSQL_DATABASE is missing.
auto-migrate:
ifeq ($(AUTO_MIGRATE),1)
	$(DOCKER_COMPOSE) up -d mysql
	@$(MAKE) wait-mysql
ifeq ($(OS),Windows_NT)
	@powershell -NoProfile -Command "$$ErrorActionPreference='Continue'; \
		$$count = (docker compose exec -T mysql mysql -uroot -p$(MYSQL_ROOT_PASSWORD) -Nse \"SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME='$(MYSQL_DATABASE)';\" 2>$$null | Select-Object -Last 1).Trim(); \
		if ($$count -ne '1') { \
			Write-Host '⏭️  Skipping migrations: database $(MYSQL_DATABASE) does not exist'; \
			exit 0; \
		}; \
		Write-Host '📦 Database $(MYSQL_DATABASE) exists; running migrations...'; \
		$$ErrorActionPreference='Stop'; \
		& make migrate; \
		if ($$LASTEXITCODE -ne 0) { exit $$LASTEXITCODE }"
else
	@DB_COUNT=$$($(DOCKER_COMPOSE) exec -T mysql mysql -uroot -p$(MYSQL_ROOT_PASSWORD) -Nse "SELECT COUNT(*) FROM information_schema.SCHEMATA WHERE SCHEMA_NAME='$(MYSQL_DATABASE)';" 2>/dev/null | tr -d '\r'); \
	if [ "$$DB_COUNT" != "1" ]; then \
		echo "⏭️  Skipping migrations: database $(MYSQL_DATABASE) does not exist"; \
	else \
		echo "📦 Database $(MYSQL_DATABASE) exists; running migrations..."; \
		$(MAKE) migrate; \
	fi
endif
else
	@echo "⏭️  Skipping migrations (AUTO_MIGRATE=$(AUTO_MIGRATE))"
endif

import-schema:
	@echo "📥 Importing database schema..."
	@if [ ! -f scripts/schema.sql ]; then \
		echo "❌ scripts/schema.sql not found!"; \
		exit 1; \
	fi
	$(DOCKER_COMPOSE) exec -T mysql mysql -uroot -p$(MYSQL_ROOT_PASSWORD) $(MYSQL_DATABASE) < scripts/schema.sql
	@echo "✅ Schema imported successfully"
	@echo "Recording existing SQL migrations as applied (schema dump already includes them)..."
	@$(call run_migrate,baseline)
	@echo ""
	@echo "Verifying tables..."
ifeq ($(OS),Windows_NT)
	@$(DOCKER_COMPOSE) exec -T mysql mysql -uroot -p$(MYSQL_ROOT_PASSWORD) $(MYSQL_DATABASE) -e "SELECT COUNT(*) as table_count FROM information_schema.tables WHERE table_schema='$(MYSQL_DATABASE)';" 2>nul | findstr /v table_count || echo "Could not verify"
else
	@$(DOCKER_COMPOSE) exec -T mysql mysql -uroot -p$(MYSQL_ROOT_PASSWORD) $(MYSQL_DATABASE) -e "SELECT COUNT(*) as table_count FROM information_schema.tables WHERE table_schema='$(MYSQL_DATABASE)';" 2>/dev/null | grep -v table_count || echo "Could not verify"
endif

import-database:
ifeq ($(wildcard scripts/metarang_db.sql),)
ifeq ($(wildcard scripts/metarang_db.sql),)
	@echo "❌ Database dump not found. Place metarang_db.sql or metarang_db.sql in scripts/"
	@exit 1
else
	$(eval DB_DUMP_FILE := scripts/metarang_db.sql)
endif
else
	$(eval DB_DUMP_FILE := scripts/metarang_db.sql)
endif
	@echo "Importing database (schema + data) from $(DB_DUMP_FILE)..."
	@echo "Dropping and recreating database..."
ifeq ($(OS),Windows_NT)
	@$(DOCKER_COMPOSE) exec -T mysql mysql -uroot -p$(MYSQL_ROOT_PASSWORD) -e "DROP DATABASE IF EXISTS $(MYSQL_DATABASE); CREATE DATABASE $(MYSQL_DATABASE) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
	@echo "Importing data..."
	@powershell -NoProfile -Command "Get-Content -Raw '$(DB_DUMP_FILE)' | docker compose exec -T mysql mysql -uroot -p$(MYSQL_ROOT_PASSWORD) $(MYSQL_DATABASE)"
else
	@$(DOCKER_COMPOSE) exec -T mysql mysql -uroot -p$(MYSQL_ROOT_PASSWORD) -e "DROP DATABASE IF EXISTS $(MYSQL_DATABASE); CREATE DATABASE $(MYSQL_DATABASE) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;" 2>/dev/null || true
	@echo "Importing data..."
	@$(DOCKER_COMPOSE) exec -T mysql mysql -uroot -p$(MYSQL_ROOT_PASSWORD) $(MYSQL_DATABASE) < $(DB_DUMP_FILE)
endif
	@echo "Database imported successfully"
	@echo ""
	@echo "Verifying import..."
ifeq ($(OS),Windows_NT)
	@$(DOCKER_COMPOSE) exec -T mysql mysql -uroot -p$(MYSQL_ROOT_PASSWORD) $(MYSQL_DATABASE) -e "SELECT COUNT(*) as table_count FROM information_schema.tables WHERE table_schema='$(MYSQL_DATABASE)';" 2>nul | findstr /v table_count || echo "Could not verify table count"
	@$(DOCKER_COMPOSE) exec -T mysql mysql -uroot -p$(MYSQL_ROOT_PASSWORD) $(MYSQL_DATABASE) -e "SELECT COUNT(*) as row_count FROM account_securities;" 2>nul | findstr /v row_count || echo "Could not verify data"
else
	@$(DOCKER_COMPOSE) exec -T mysql mysql -uroot -p$(MYSQL_ROOT_PASSWORD) $(MYSQL_DATABASE) -e "SELECT COUNT(*) as table_count FROM information_schema.tables WHERE table_schema='$(MYSQL_DATABASE)';" 2>/dev/null | grep -v table_count || echo "Could not verify table count"
	@$(DOCKER_COMPOSE) exec -T mysql mysql -uroot -p$(MYSQL_ROOT_PASSWORD) $(MYSQL_DATABASE) -e "SELECT COUNT(*) as row_count FROM account_securities;" 2>/dev/null | grep -v row_count || echo "Could not verify data"
endif

migrate:
	@echo "Running pending migrations..."
	@$(call run_migrate,up)

migrate-rollback:
	@echo "Rolling back last migration batch..."
	@$(call run_migrate,rollback)

migrate-status:
	@$(call run_migrate,status)

migrate-reset:
	@echo "Rolling back all migrations..."
	@$(call run_migrate,reset)

migrate-refresh:
	@echo "Refreshing migrations..."
	@$(call run_migrate,refresh)

migrate-baseline:
	@echo "Baselining pending migrations (record only)..."
	@$(call run_migrate,baseline)

migrate-install:
	@$(call run_migrate,install)

migrate-make:
	@if [ -z "$(NAME)" ]; then \
		echo "❌ Please specify NAME=snake_case_description"; \
		echo "Example: make migrate-make NAME=add_wallet_login_to_personal_access_tokens"; \
		exit 1; \
	fi
	go run ./shared/cmd/migrate make -path=$(MIGRATIONS_PATH) $(NAME)

dev: ensure-networks
	@echo "🚀 Starting development environment..."
	@echo "ℹ️  Each service uses its own config.env (copy from config.env.sample)"
	@echo "Starting MySQL and Redis..."
	$(DOCKER_COMPOSE) up -d mysql redis
	@$(MAKE) wait-mysql
	@echo "Checking if schema needs to be imported..."
ifeq ($(OS),Windows_NT)
	@powershell -NoProfile -Command "$$ErrorActionPreference='Continue'; $$tableCount = (docker compose exec -T mysql mysql -uroot -p$(MYSQL_ROOT_PASSWORD) $(MYSQL_DATABASE) -e \"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='$(MYSQL_DATABASE)';\" 2>$$null | Select-Object -Last 1).Trim(); if ($$tableCount -eq '0') { Write-Host 'Importing schema...'; make import-schema } else { Write-Host (\"✅ Database already initialized ({0} tables)\" -f $$tableCount) }"
else
	@TABLE_COUNT=$$($(DOCKER_COMPOSE) exec -T mysql mysql -uroot -p$(MYSQL_ROOT_PASSWORD) $(MYSQL_DATABASE) -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='$(MYSQL_DATABASE)';" 2>/dev/null | tail -1); \
	if [ "$$TABLE_COUNT" = "0" ]; then \
		echo "Importing schema..."; \
		make import-schema; \
	else \
		echo "✅ Database already initialized ($$TABLE_COUNT tables)"; \
	fi
endif
	@echo "Starting migrate job and all services..."
	@$(MAKE) start-migrate-job
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

# =============================================================================
# Development with Hot Reloading
# =============================================================================

dev-up: init-storage-uploads ensure-networks
	@echo "🚀 Starting development environment with Docker Compose Watch..."
	@echo "ℹ️  File changes will automatically trigger rebuilds (Go services)"
	@echo ""
	@echo "Starting MySQL, Redis, and migrate job..."
	$(DOCKER_COMPOSE) up -d mysql
	@$(MAKE) start-migrate-job
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
	@$(MAKE) auto-migrate
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
