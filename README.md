# Metarang Microservices

Microservices implementation for the Metarang platform using Golang/gRPC.

## Architecture

### Services

| Service | Port | Description |
|---------|------|-------------|
| auth-service | 50051 | Authentication, User Management, KYC |
| commercial-service | 50052 | Wallet, Transactions, Payments |
| features-service | 50053 | Features (Lands), Marketplace |
| levels-service | 50054 | User Progression, Activities |
| dynasty-service | 50055 | Dynasty, Family Members |
| support-service | 50056 (gRPC), 8070 (HTTP) | Tickets, Reports, Notes |
| training-service | 50057 | Video Tutorials, Comments |
| notifications-service | 50058 | Multi-channel Notifications |
| calendar-service | 50059 (gRPC), 8060 (HTTP) | Events Management |
| storage-service | 50060 (gRPC), 8059 (HTTP) | File Upload & Management |
| financial-service | 50062 | Payment Processing |
| websocket-gateway | 3002 | Real-time Communication |
| Kong API Gateway | 8000 | HTTP/REST edge gateway |
| MySQL | 3306 | Shared Database |
| Redis | 6379 | Caching, Pub/Sub |

### Shared Components

- **shared/proto**: Protocol Buffer definitions
- **shared/pkg**: Shared Go packages (db, auth, logger, metrics, helpers)

## Prerequisites

- **Go** 1.21+ (`go version`)
- **Protocol Buffers** (`protoc --version`)
- **Docker & Docker Compose** (`docker --version`)
- **Make** (`make --version`)

External APIs needed: OAuth server, Kavenegar (SMS), Parsian (payments), FTP (storage).

## Quick Start

### 1. Generate Protocol Buffer Files

```bash
make proto
```

### 2. Configure Services

Each service uses its own `config.env`. Copy from the sample and edit:

```bash
# Example: Auth service
cp services/auth-service/config.env.sample services/auth-service/config.env
# Edit services/auth-service/config.env with your credentials

# Repeat for each service you need:
# services/commercial-service/config.env
# services/notifications-service/config.env
# services/financial-service/config.env
# services/storage-service/config.env
# services/support-service/config.env
# services/websocket-gateway/config.env
# etc.
```

### 3. Start Infrastructure

```bash
docker-compose up -d mysql redis
sleep 10  # Wait for MySQL to be ready
```

### 4. Import Database Schema

```bash
make import-schema
```

### 5. Start All Services

```bash
make dev
```

### 6. Verify

```bash
make ps
curl http://localhost:8000
curl http://localhost:3002/health
```

## Configuration

Each service loads from `config.env` in its directory. Copy `config.env.sample` → `config.env` and set:

- **Database**: `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_DATABASE`
- **OAuth** (auth-service): `OAUTH_SERVER_URL`, `OAUTH_CLIENT_ID`, `OAUTH_CLIENT_SECRET`
- **SMS** (auth, notifications): `KAVENEGAR_API_KEY`
- **Parsian** (commercial, financial): `PARSIAN_MERCHANT_ID`, `PARSIAN_PIN`, etc.
- **FTP** (storage): `FTP_HOST`, `FTP_USER`, `FTP_PASSWORD`, `FTP_BASE_URL`

Docker Compose injects `config.env` via `env_file`; the `environment` section overrides DB_HOST/DB_PORT for container networking.

## Common Commands

```bash
make dev              # Start full dev environment (Compose migrate job)
make up               # Start stack (force-recreate migrate, then apps)
make restart          # migrate → restart
make migrate          # docker compose run --rm migrate
docker compose up -d  # apps wait for migrate service (no Make)
make ps               # Check service status
make logs             # View all logs
make logs-service SERVICE=auth-service   # Service-specific logs
make down             # Stop all services

make build-all        # Build all images
make restart-service SERVICE=auth-service
make clean            # Stop and remove volumes

make kong-validate    # Validate Kong config
make kong-reload      # Reload Kong
```

## Local Development (Without Docker)

1. **Database & Redis**: Start MySQL 8 and Redis locally
2. **Schema**: `mysql -u root -p metarang_db < scripts/schema.sql` then `make migrate-baseline` (or `make migrate` on an older DB)
3. **Migrations**: `make migrate` after pulling new SQL files under `scripts/migrations`
4. **Config**: Copy `config.env.sample` → `config.env` per service
5. **Run services** in separate terminals:

```bash
cd services/auth-service && go run cmd/server/main.go
cd services/commercial-service && go run cmd/server/main.go
cd services/websocket-gateway && go run ./cmd/server
# etc.
```

## Project Structure

```
metarang-microservices/
├── services/
│   ├── auth-service/
│   │   ├── cmd/server/main.go
│   │   ├── internal/handler/    # gRPC handlers
│   │   ├── internal/service/    # Business logic
│   │   ├── internal/repository/ # Data access
│   │   └── config.env.sample
│   └── ...
├── shared/proto/     # .proto files
├── shared/pb/        # Generated Go code
├── kong/             # Kong gateway config
├── scripts/          # Schema, migrations
└── Makefile
```

## Testing

```bash
make test-unit        # Unit tests
make test-services    # Dedicated service test modules
make test-database    # Database tests
make test-all         # Full suite
```

Use `grpcurl` or Postman gRPC for API testing (Kong returns `415` for plain REST on gRPC routes).

## Database Schema

Shared schema in `scripts/schema.sql`. Notes: `transactions.id` is VARCHAR; `feature_properties.id` has prefix/postfix; soft deletes use `deleted_at`; polymorphic relations use `{model}_type` and `{model}_id`.

### Migrations

Schema dumps and existing databases are updated with Laravel-style SQL files in `scripts/migrations`. Applied files are recorded in the `migrations` table (`migration` + `batch`), same as Laravel.

#### Automatic (Docker Compose, no Make required)

A one-shot `migrate` service runs pending SQL before any DB-backed app starts:

```bash
docker compose up -d --force-recreate migrate   # MySQL + migrate job
docker compose up -d                            # apps wait until migrate exits 0
```

Or simply `docker compose up -d` on a fresh stack (migrate runs once). After pulling new migration files, recreate the job:

```bash
docker compose up -d --force-recreate --no-deps migrate
# or
docker compose run --rm migrate
```

Skip: `SKIP_MIGRATE=1 docker compose up -d --force-recreate migrate`  
Production: set `APP_ENV=production` and `MIGRATE_FORCE=1` (or the entrypoint adds `-force` when `APP_ENV=production`).

#### Make wrappers

```bash
make up          # force-recreate migrate → start stack
make dev         # schema import if empty → migrate → start services
make restart     # migrate → restart services
make migrate     # docker compose run --rm migrate (or USE_HOST_MIGRATE=1)
```

Skip auto-migrate: `make up AUTO_MIGRATE=0`. Host CLI: `make migrate USE_HOST_MIGRATE=1`.

```bash
make migrate-make NAME=add_foo_to_bar   # create stub
make migrate                            # run pending files
make migrate-status                     # ran vs pending
make migrate-rollback                   # undo last batch (STEP=1 for one file)
make migrate-baseline                   # mark pending as ran without executing (after schema import)
```

Each file uses `-- migrate:up` / `-- migrate:down`. After `make import-schema`, pending files are baselined so historical ALTERs are not replayed on a dump that already contains those columns. For an older database, skip baseline and run `make migrate`.

DB connection: Compose migrate uses root (`MIGRATE_DB_USER` / `MIGRATE_DB_PASSWORD`, defaulting to `MYSQL_ROOT_PASSWORD`). Host mode uses `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_DATABASE`. Or: `go run ./shared/cmd/migrate help`.

#### Production note

Prefer the same migrate image as a release Job / pipeline step before rolling out services. Compose `depends_on: migrate` is for local/Dokploy Compose stacks; Kubernetes should use a Job (or Helm hook), not migrate-on-boot in every pod.

## API Compatibility

**CRITICAL**: All services MUST maintain 100% API compatibility with the existing platform API (JSON fields, status codes, validation format, Jalali dates, URLs).

## Troubleshooting

| Issue | Command |
|-------|---------|
| Services not starting | `docker-compose logs auth-service` |
| Database connection | `docker compose exec mysql mysql -uroot -proot_password -e "SELECT 1"` |
| Port in use | `lsof -i :50051` (macOS) or `netstat -tulpn \| grep 50051` (Linux) |
| Proto errors | `make clean-proto && make proto` |
| Reset everything | `make clean && make dev` |

## Deployment

```bash
docker build -t metarang/auth-service:latest -f services/auth-service/Dockerfile .
kubectl apply -f k8s/auth-service/
```

See `docs/DEPLOYMENT.md` and `docs/TROUBLESHOOTING.md` for production details.

### Dokploy (shared MySQL with the admin panel)

Create these networks once on the Ubuntu host before the first deploy:

```bash
docker network create dokploy-network   # already exists on a Dokploy server
docker network create metarang-shared
```

Deploy **this** Compose app first. MySQL is reachable from the admin panel as hostname `metarang-mysql` on `metarang-shared`. Set the same secrets in both Dokploy apps:

```env
MYSQL_ROOT_PASSWORD=<strong>
MYSQL_DATABASE=metarang_db
MYSQL_USER=metarang_user
MYSQL_PASSWORD=<strong>
APP_URL=https://api.your-domain
ADMIN_PANEL_URL=https://admin.your-domain
```

Do not enable Isolated Deployment for these two apps. Local `make up` / `make dev` create the networks automatically.

## Development Rules

- **`.cursor/rules/`** – Rules for LLM assistants
- **`docs/`** – Architecture, deployment, troubleshooting

Key principles: 100% API compatibility, layered architecture (handler/service/repository), dependency injection, proper error handling.

## License

Proprietary - metarang Platform
