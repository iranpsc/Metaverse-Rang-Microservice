# Social Service

gRPC service for **challenge** (quiz) APIs and **follow** relationships, aligned with the Laravel monolith routes documented under `api-docs/social-service/`.

## Responsibilities

- **ChallengeService**: timings (`system_variables`), random unanswered question, answer submission with PSC prize via **commercial-service**, and advertisements.
- **FollowService**: followers/following lists and follow/unfollow/remove using the shared MySQL `follows` and `users` tables.

## Configuration

Copy `config.env.sample` to `config.env` and set:

| Variable | Description |
|----------|-------------|
| `DB_*` | Shared MySQL database (same schema as Laravel). |
| `GRPC_PORT` | gRPC listen port (default **50061**). |
| `COMMERCIAL_SERVICE_ADDR` | `commercial-service:50052` for wallet PSC credits on correct answers. |
| `PROJECT_LOCALE` | Advertisement localization (`EN` or `FA`). |
| `PROJECT_URL` | Base URL used for advertisement media links. |

## Running locally

```bash
cd services/social-service
cp config.env.sample config.env
# edit config.env
go run ./cmd/server
```

## Docker

Built by [`Dockerfile`](Dockerfile) from repo root:

```bash
docker compose build social-service
```

## Architecture

```
cmd/server/main.go → repository → service → handler (gRPC)
```

HTTP clients use **grpc-gateway** (`SOCIAL_SERVICE_ADDR`) which maps `/api/challenge/*` and `/api/follow*` to this service.

See also [TESTING.md](TESTING.md).
