# Metarang Microservices

Microservices implementation for the Metarang platform migration from Laravel monolith to Golang/gRPC.

## Architecture

### Services

| Service | Port | Description |
|---------|------|-------------|
| auth-service | 50051 | Authentication, User Management, KYC |
| commercial-service | 50052 | Wallet, Transactions, Payments |
| features-service | 50053 | Features (Lands), Marketplace |
| levels-service | 50054 | User Progression, Activities |
| dynasty-service | 50055 | Dynasty, Family Members |
| support-service | 50056 | Tickets, Reports |
| training-service | 50057 | Video Tutorials, Comments |
| notifications-service | 50058 | Multi-channel Notifications |
| calendar-service | 50059 | Events Management |
| storage-service | 50060 (gRPC), 8059 (HTTP) | File Upload & Management |
| financial-service | 50062 | Payment Processing |
| grpc-gateway | 8080 | REST to gRPC Translation |
| websocket-gateway | 3002 | Real-time Communication |
| Kong API Gateway | 8000 | HTTP/REST → gRPC |
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
# services/grpc-gateway/config.env
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
make dev              # Start full dev environment
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
2. **Schema**: `mysql -u root -p metarang_db < scripts/schema.sql`
3. **Config**: Copy `config.env.sample` → `config.env` per service
4. **Run services** in separate terminals:

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
make test-integration # Integration tests
make test-golden      # Golden JSON (Laravel compatibility)
make test-database    # Database tests
make test-all         # Full suite
```

Use `grpcurl` or Postman gRPC for API testing (Kong returns `415` for plain REST on gRPC routes).

## Database Schema

Shared schema in `scripts/schema.sql`. Notes: `transactions.id` is VARCHAR; `feature_properties.id` has prefix/postfix; soft deletes use `deleted_at`; polymorphic relations use `{model}_type` and `{model}_id`.

## API Compatibility

**CRITICAL**: All services MUST maintain 100% API compatibility with the Laravel monolith (JSON fields, status codes, validation format, Jalali dates, URLs). Golden tests enforce this.

## Troubleshooting

| Issue | Command |
|-------|---------|
| Services not starting | `docker-compose logs auth-service` |
| Database connection | `docker exec metarang-mysql mysql -uroot -proot_password -e "SELECT 1"` |
| Port in use | `lsof -i :50051` (macOS) or `netstat -tulpn \| grep 50051` (Linux) |
| Proto errors | `make clean-proto && make proto` |
| Reset everything | `make clean && make dev` |

## Deployment

```bash
docker build -t metarang/auth-service:latest -f services/auth-service/Dockerfile .
kubectl apply -f k8s/auth-service/
```

See `docs/DEPLOYMENT.md` and `docs/TROUBLESHOOTING.md` for production details.

## Development Rules

- **`.cursor/rules/`** – Rules for LLM assistants
- **`docs/`** – Architecture, deployment, troubleshooting

Key principles: 100% Laravel API compatibility, layered architecture (handler/service/repository), dependency injection, proper error handling.

## License

Proprietary - metarang Platform
