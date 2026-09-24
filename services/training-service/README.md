# Training Service

The Training Service manages video tutorials, categories, comments, replies, and user interactions (likes, dislikes, views) within the metarang microservices architecture. It replaces the legacy video-tutorials module with 100% API compatibility.

## Overview

This service provides gRPC endpoints for:

- **Video Tutorials**: List, view, search, and look up videos by filename
- **Categories**: Browse categories, subcategories, and category-scoped video lists
- **Comments**: CRUD operations on video comments with like/dislike and report support
- **Replies**: Nested comment replies with interaction tracking
- **Engagement**: View counting and like/dislike interactions on videos and comments

## Architecture

### Technology Stack

- **Language**: Go 1.25+
- **Database**: MySQL (shared instance, utf8mb4)
- **gRPC**: Inter-service communication (port `50057`)
- **HTTP**: Public REST API for Kong (port `8069`)
- **Protocol Buffers**: Service definitions in `shared/proto/training.proto`

### Layered Architecture

```
Handler Layer (internal/handler/)
  ↓
Service Layer (internal/service/)
  ↓
Repository Layer (internal/repository/)
  ↓
Database (MySQL)
```

### Key Components

- **Handlers**: gRPC + HTTP request/response conversion, validation, error mapping, Jalali date formatting
- **Services**: Business logic for videos, categories, comments, and replies
- **Repositories**: Data access with polymorphic queries (legacy `App\Models\*` type strings)
- **Auth Client**: gRPC client for user profile data and HTTP token validation (falls back to direct DB queries if unavailable)
- **Middleware**: Bearer/cookie auth for HTTP routes via auth-service `ValidateToken`

### Service Dependencies

| Service | Purpose | Required |
|---------|---------|----------|
| **Auth Service** | User profile retrieval + HTTP token validation | No for profiles (falls back to DB); yes for authenticated HTTP routes |
| **Kong** | Routes `/api/tutorials`, `/api/video-tutorials`, `/api/comments` to this service | Yes (for public HTTP access) |

## API Endpoints

All HTTP endpoints are served directly by training-service (port `8069`) behind Kong and maintain legacy-compatible JSON responses. Public viewing routes use optional authentication; write actions require a bearer token.

### Video Tutorials

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/tutorials` | Optional | List video tutorials (paginated) |
| `GET` | `/api/tutorials/{slug}` | Optional | Get video by slug (increments view count) |
| `POST` | `/api/video-tutorials` | Optional | Get video by partial filename (v1 modal lookup) |
| `POST` | `/api/tutorials/search` | Optional | Search videos by title |
| `POST` | `/api/tutorials/{video}/interactions` | Bearer | Like or dislike a video |

### Categories

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/tutorials/categories` | Optional | List categories |
| `GET` | `/api/tutorials/categories/{slug}` | Optional | Get category with subcategories |
| `GET` | `/api/tutorials/categories/{slug}/videos` | Optional | List videos in a category |
| `GET` | `/api/tutorials/categories/{category}/{subcategory}` | Optional | Get subcategory with videos |

### Comments

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/tutorials/{video}/comments` | Optional | List top-level comments |
| `POST` | `/api/tutorials/{video}/comments` | Bearer | Add a comment |
| `PUT`/`POST` | `/api/tutorials/{video}/comments/{comment}` | Bearer | Update a comment |
| `DELETE` | `/api/tutorials/{video}/comments/{comment}` | Bearer | Delete a comment |
| `POST` | `/api/tutorials/{video}/comments/{comment}/interactions` | Bearer | Like or dislike a comment |
| `POST` | `/api/tutorials/{video}/comments/{comment}/like` | Bearer | Like a comment |
| `POST` | `/api/tutorials/{video}/comments/{comment}/dislike` | Bearer | Dislike a comment |
| `POST` | `/api/tutorials/{video}/comments/{comment}/report` | Bearer | Report a comment |

### Replies

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/api/comments/{comment}/replies` | Optional | List replies to a comment |
| `POST` | `/api/comments/{comment}/reply` | Bearer | Add a reply |
| `PUT` | `/api/comments/{comment}/replies/{reply}` | Bearer | Update a reply |
| `DELETE` | `/api/comments/{comment}/replies/{reply}` | Bearer | Delete a reply |
| `POST` | `/api/comments/{comment}/replies/{reply}/interactions` | Bearer | Like or dislike a reply |

### gRPC Services

The service exposes four gRPC services defined in `shared/proto/training.proto`:

- `VideoService` — video listing, retrieval, search, views, and interactions
- `CategoryService` — category and subcategory browsing
- `CommentService` — comment CRUD, interactions, and reports
- `ReplyService` — reply CRUD and interactions

## Database Schema

The service uses the shared MySQL database with legacy-compatible polymorphic tables:

| Table | Purpose |
|-------|---------|
| `videos` | Tutorial video records |
| `video_categories` | Top-level categories |
| `video_sub_categories` | Subcategories linked to categories |
| `comments` | Polymorphic comments (`commentable_type = App\Models\Video`) |
| `interactions` | Polymorphic likes/dislikes on videos and comments |
| `views` | Polymorphic view tracking on videos |
| `comment_reports` | User-submitted comment reports |

See `scripts/schema.sql` for the complete schema.

## Business Logic

### Video Views

- View count increments automatically when a video is fetched by slug or filename
- Views are tracked per IP address in the `views` table
- View increment failures are logged but do not fail the request

### Interactions

- Likes and dislikes use the `interactions` table with `liked` boolean (`true` = like, `false` = dislike)
- Authenticated users see their current interaction via `user_interaction.liked` (`true` = like, `false` = dislike) when present
- Video and comment interactions require authentication

### Comments & Replies

- Top-level comments are stored with `parent_id = NULL`
- Replies are stored as comments with a `parent_id` pointing to the parent comment
- Comment content is limited to 2,000 characters
- Only the comment author can update or delete their comment

### URL Building

- Image and video URLs are built from `ADMIN_PANEL_URL` + `/uploads/` + resource path
- Dates are formatted using the Jalali calendar for API compatibility

### Localization

- Validation error messages support multiple locales via `PROJECT_LOCALE` (`EN` or `FA`)
- Locale files: `internal/lang/en.json`, `internal/lang/fa.json`

## Configuration

Copy `config.env.sample` to `config.env` and configure:

```env
# App
APP_URL=http://localhost:8000
PROJECT_LOCALE=EN
ADMIN_PANEL_URL=https://admin.metarang.com

# Database
DB_HOST=localhost
DB_PORT=3306
DB_USER=root
DB_PASSWORD=
DB_DATABASE=metarang_db

# gRPC Server
GRPC_PORT=50057

# HTTP Server (Kong /api/tutorials, /api/video-tutorials, /api/comments)
HTTP_PORT=8069

# Metrics (Prometheus)
METRICS_PORT=9090

# Auth Service (user profiles + HTTP token validation)
AUTH_SERVICE_ADDR=auth-service:50051
```

## Development

### Prerequisites

- Go 1.25+
- MySQL with schema from `scripts/schema.sql`
- Generated proto code (`make proto` or `make gen-training`)

### Building

```bash
cd services/training-service
go build -o bin/training-service ./cmd/server
```

### Running Locally

```bash
cp config.env.sample config.env
# Edit config.env with your local settings

go run ./cmd/server
```

### Testing

```bash
# Unit tests (in-service)
go test ./internal/...

# Dedicated service test module
go test ../../tests/training-service/...

# From repository root
make test-unit
make test-services
```

### Proto Generation

After modifying `shared/proto/training.proto`:

```bash
# From repository root
make gen-training

# Or generate all protos
make proto
```

## Monitoring & Metrics

The service exposes Prometheus metrics on `METRICS_PORT` (default `9090`):

- **Service name**: `training_service`
- **Scrape target**: `training-service:9090` (configured in `monitoring/prometheus/prometheus.yml`)

### Key Metrics

- `training_service_grpc_requests_total` — gRPC request counts by method and status
- `training_service_grpc_request_duration_seconds` — Request latency histograms
- Standard Go runtime and process metrics via Prometheus client

Sentry error tracking is initialized from environment variables via `shared/pkg/sentry`.

## Deployment

### Docker

```bash
# From repository root
docker build -t metarang/training-service:latest -f services/training-service/Dockerfile .
docker run -p 50057:50057 -p 8069:8069 --env-file services/training-service/config.env metarang/training-service:latest
```

### Docker Compose

```bash
# Start training service with dependencies
docker compose up -d training-service

# Or start the full stack
make dev-up
```

Kong routes `/api/tutorials`, `/api/video-tutorials`, and `/api/comments` directly to `training-service:8069`.

## Troubleshooting

### Service won't start

- Verify MySQL is reachable and schema is applied
- Check `config.env` values (especially `DB_*` variables)
- Confirm ports `50057` / `8069` are not in use

### Training HTTP routes unavailable

- Ensure `HTTP_PORT=8069` and Kong `training-api` route are configured
- Verify training-service `/health` on port 8069 is healthy
- Check Kong logs for upstream connection errors

### Auth client warnings

- The service logs a warning and falls back to direct DB user queries if auth-service is unreachable
- This is expected during local development without auth-service running

### Missing user profile photos

- User data is fetched from auth-service when available
- Profile photo paths are resolved from the `images` table with `imageable_type = App\Models\User`

## API Compatibility

This service maintains 100% API compatibility with the existing platform API:

- Exact JSON field names and types
- Exact HTTP status codes and validation error formats
- Jalali date/time formatting
- Legacy polymorphic model type strings (`App\Models\Video`, `App\Models\Comment`)
- URL structures matching the original video-tutorials module

## License

Proprietary — metarang
