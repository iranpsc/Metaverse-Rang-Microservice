# Metarang Microservices: Agent Guide

This repository is the Go microservices implementation of the Metarang platform. Treat the existing public API as a compatibility contract: preserve request and response JSON shapes, status codes, validation messages and formats, Jalali date behavior, and URL paths unless a change explicitly authorizes a breaking API change.

## Repository map

| Path | Purpose |
| --- | --- |
| `services/<service>/` | Independently versioned Go modules and deployable services. Each normally contains `cmd/server`, `internal`, a `Dockerfile`, and `config.env.sample`. |
| `shared/` | Shared Go module: source protobuf contracts in `proto/`, generated Go stubs in `pb/`, reusable packages in `pkg/`, and shared command helpers in `cmd/`. |
| `tests/<service>/` | Separate Go modules for broader handler, service, repository, and integration-style tests. |
| `tests/database/` | Schema and concurrency tests that require MySQL. |
| `scripts/` | Canonical `schema.sql`, ordered SQL migrations, and operational helper scripts. |
| `kong/` | Kong declarative gateway configuration. |
| `openapi/` | Generated/served API documentation assets. |
| `k8s/` | Kubernetes manifests, including Istio, Kong, monitoring, service, and config resources. |
| `monitoring/` | Prometheus and Grafana configuration. |
| `docs/` | Architecture, system behavior, and error-catalog documentation. |
| `.github/workflows/` | Changed-service CI, shared-package checks, and deployment workflows. |
| `docker-compose.yml` / `docker-compose.dev.yml` | Local full-stack runtime; the dev overlay includes Mailpit. |

## Services and boundaries

The Go workspace (`go.work`) includes every service plus `shared`. The current service set is:

- `auth-service` — identity, authentication, user data, KYC, and account security.
- `buildings-service` — buildings and building-entry flows.
- `calendar-service` — calendar events and interactions.
- `commercial-service` — wallets, payments, orders, referrals, and transactions.
- `dynasty-service` — dynasties and family relationships.
- `features-service` — features/lands and marketplace operations.
- `financial-service` — payment processing.
- `health-check-service` — health endpoints.
- `levels-service` — progression, activities, and challenges.
- `notifications-service` — in-app, SMS, and email notifications.
- `sitemap-generator-service` — sitemap generation.
- `social-service` — social and challenge features.
- `storage-service` — file and image storage with FTP integration.
- `support-service` — tickets, reports, notes, and user events.
- `training-service` — tutorials, categories, comments, and interactions.
- `websocket-gateway` — real-time Socket.IO/WebSocket communication and Redis pub/sub.

Most services follow this dependency direction:

```text
transport handler (gRPC and/or HTTP) -> service (business rules) -> repository (persistence/external data)
```

Keep business decisions out of handlers and direct SQL/external persistence work out of services. Common supporting packages include `internal/models`, `middleware`, `client`, `auth`, `lang`, `config`, and `pubsub`; use the conventions already present in the service you change. Some services expose gRPC only, while support, calendar, storage, sitemap, and the websocket gateway also expose HTTP-facing behavior. Kong is the public edge gateway; service-to-service communication is primarily gRPC.

## Technology and runtime

- Go **1.26** multi-module workspace; each service and test suite has its own `go.mod`.
- gRPC and Protocol Buffers for internal contracts; protobuf sources are in `shared/proto` and generated code is in `shared/pb`.
- HTTP/JSON endpoints where required, with Kong as the API gateway.
- MySQL 8 is the shared database; Redis supports caching and pub/sub.
- Docker Compose supports local development; Kubernetes/Istio configuration supports deployment.
- Prometheus and Grafana provide observability.
- External integrations include OAuth, Kavenegar SMS, SMTP, Parsian payment services, FTP storage, and a 3D environment API.

## Development workflow

1. Work in the smallest affected service module. Run Go commands from that module when appropriate; the workspace links local modules during ordinary development.
2. For a service contract change, update the `.proto` source first and regenerate stubs with `make proto`. Do not hand-edit generated files in `shared/pb`.
3. For persisted-data changes, add an ordered migration under `scripts/migrations`; do not silently edit the schema dump as the only migration path. Use `make migrate-make NAME=<descriptive_name>` to create a stub when suitable.
4. Copy `config.env.sample` to ignored `config.env` locally. Never commit credentials, tokens, private keys, or real provider settings.
5. Use `make dev` for the local full stack, or start individual services with `go run ./cmd/server` from the relevant service. `make up`, `make down`, `make ps`, `make logs`, and `make logs-service SERVICE=<name>` manage the Compose environment.
6. Use `make openapi` to generate documentation and `make docs`/`make docs-up` to serve it. Validate gateway changes with `make kong-validate` before reloading Kong.

Compose runs the migration job before database-backed applications. Preserve this startup expectation when changing service dependencies or database configuration.

## Testing and quality checks

Tests are deliberately split across two locations:

- Source-adjacent unit tests belong next to the code in `services/<service>/...` as `*_test.go` files.
- Richer service suites belong in `tests/<service>/internal/...`. Reuse that service's `internal/testutil` helpers, mocks, SQL mocks, or gRPC bufconn utilities when available.
- Database schema/concurrency coverage belongs under `tests/database/` and requires a MySQL test database loaded from `scripts/schema.sql`.

Use the narrowest relevant command first, then broaden before handoff:

```bash
cd services/<service> && go test ./... -race
cd tests/<service> && GOWORK=off go test ./... -race
make test-unit
make test-services
make test-database
make test-all
make lint
```

`make test-unit` runs service-module tests, `make test-services` runs the separate test modules, and `make test-all` includes database tests. CI detects affected services; a change in `shared/` rebuilds and tests all services. Keep tests deterministic: use fixed timestamps/data where possible and avoid races in parallel tests. The project target is greater than 80% unit coverage per service; dedicated coverage targets also exist for selected services.

Formatting and static analysis are enforced by `.golangci.yml`: `gofmt`, `goimports` (with `metarang` local imports), `errcheck`, `govet`, `ineffassign`, `staticcheck`, and `unused`. Use `make lint`; use `make lint-fix` only when its formatting changes are intended and reviewed.

## Code and API conventions

- Follow the existing service-local patterns before introducing a new abstraction or package.
- Keep exported names idiomatic Go; format with `gofmt`/`goimports` and handle returned errors rather than discarding them.
- Validate and authorize at the transport boundary with the service's established middleware/helpers. Preserve existing response envelopes, localized messages, pagination behavior, and error mappings.
- Maintain Persian (`fa`) and English (`en`) localization resources when introducing user-visible text in services that use `internal/lang`.
- Treat currency/balance changes as transactional operations. Preserve decimal precision and locking behavior in commercial/financial paths.
- Respect soft-delete and polymorphic-relation conventions in the schema; inspect existing model/repository code before changing related tables.
- Use service clients/protobufs for cross-service calls instead of importing another service's `internal` code or reaching directly into its tables.
- Keep HTTP route, gRPC method, Kong route, OpenAPI, and protobuf changes synchronized when the public behavior changes.

## Security and configuration

- Never hardcode or log secrets, access tokens, payment credentials, OAuth client secrets, FTP passwords, or personally sensitive values.
- Use `config.env` for local secret values and retain safe placeholders/documentation in `config.env.sample`.
- Authenticate and authorize before exposing user-specific data or mutating state; reuse established auth/account-security middleware.
- Treat uploads, payment callbacks, KYC material, and websocket authorization as security-sensitive paths. Add focused negative-path tests when changing them.
- Read `SECURITY.md` for the vulnerability reporting policy; do not place security disclosures in public issues.

## Git and review conventions

Use focused conventional commits with one of `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, or `chore`, plus a scope such as `auth`, `commercial`, `storage`, `gateway`, `shared`, `proto`, `kong`, `scripts`, or `docs`:

```text
feat(auth): add SMS-based OTP login
fix(commercial): handle payment timeout
```

Before submitting a change, check the affected service's tests, the dedicated `tests/<service>` module when it exists, linting, migration behavior, and API compatibility. Avoid unrelated refactors and do not alter generated artifacts, schema history, deployment configuration, or shared contracts without the corresponding source change and verification.

## Useful references

- `README.md` — setup, service ports, local runtime, migrations, and common commands.
- `docs/ARCHITECTURE.md` — system boundaries and service responsibilities.
- `CONTRIBUTING.md` — contribution principles and compatibility requirements.
- `tests/README.md` — test-suite layout and database-test setup.
- `Makefile` — canonical commands; prefer these to ad-hoc Compose sequences when a target exists.
