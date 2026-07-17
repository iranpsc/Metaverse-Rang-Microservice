# Social Service — Testing

Tests live in `tests/social-service/` (same layout as `tests/calendar-service/`).

HTTP pagination (10/page) for `GET /api/followers` and `GET /api/following` is covered by
gateway tests under `tests/grpc-gateway/internal/handler/social_handler_test.go`.

Profile-limitation authorization on follow is enforced in social-service via an auth-service
gRPC call (`GetProfileLimitations`) and covered by `tests/social-service` service tests.

## Unit tests (no database)

Runs handler and service tests with in-memory gRPC (`internal/testutil/grpc_bufconn.go`) and mocks (`internal/testutil/mocks.go`):

```bash
cd tests/social-service
GOWORK=off go test ./internal/handler/... ./internal/service/... -race -count=1
```

## Coverage gate (≥70%)

Combined **handler + service** statement coverage is enforced by the repo Makefile:

```bash
make test-coverage-social
```

This runs:

```bash
cd tests/social-service && GOWORK=off go test ./internal/handler/... ./internal/service/... -race -coverprofile=coverage.out -covermode=atomic
```

## Repository integration tests (optional)

Repository tests call a real MySQL when `TEST_MYSQL_DSN` is set; otherwise they skip.

Example:

```bash
export TEST_MYSQL_DSN='metarang_user:metarang_password@tcp(127.0.0.1:3306)/metarang_db?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci'
cd tests/social-service
GOWORK=off go test ./internal/repository/... -count=1
```

## Full test module

```bash
cd tests/social-service
GOWORK=off go test ./... -race -count=1
```

Or via Makefile:

```bash
make test-services
```
