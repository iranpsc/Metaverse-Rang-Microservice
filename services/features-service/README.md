# Features Service

The Features Service is a Go microservice that handles all feature-related operations in the MetaRGB platform, including marketplace transactions, buy/sell requests, hourly profits, and map data.

## Overview

This service provides gRPC endpoints for:
- **Feature Marketplace**: Buy/sell features, manage buy/sell requests
- **Feature Management**: List, view, and manage user-owned features
- **Hourly Profits**: Withdraw and manage feature hourly profits
- **Maps**: Retrieve map polygons and feature rollups
- **Buildings**: Manage feature buildings and 3D environments

## Architecture

The service follows a layered architecture:

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

- **Handlers**: gRPC request/response conversion, validation, error mapping
- **Services**: Business logic orchestration, cross-service communication
- **Repositories**: Data access layer with direct SQL queries
- **Clients**: gRPC clients for Commercial Service and Notification Service
- **Events**: Redis-based event broadcasting for real-time updates

## Dependencies

### External Services

- **Commercial Service** (gRPC): Wallet operations, transactions
- **Notification Service** (gRPC): Send notifications to users
- **Auth Service** (gRPC): Token validation
- **3D Meta API** (HTTP): Building generation

### Infrastructure

- **MySQL**: Primary database for features, requests, trades
- **Redis**: Event broadcasting (Pub/Sub)

## Configuration

Copy `config.env.sample` to `config.env` and configure:

```env
# Database
DB_HOST=localhost
DB_PORT=3306
DB_USER=root
DB_PASSWORD=
DB_DATABASE=metargb_db

# gRPC Server
GRPC_PORT=50053

# Metrics
METRICS_PORT=9090

# External Services
COMMERCIAL_SERVICE_ADDR=commercial-service:50052
NOTIFICATIONS_SERVICE_ADDR=notifications-service:50058
AUTH_SERVICE_ADDR=auth-service:50051

# Redis (for event broadcasting)
REDIS_ADDR=redis:6379
REDIS_PASSWORD=
BROADCAST_CHANNEL=feature-events

# Commercial Service Configuration
COMMERCIAL_SERVICE_TIMEOUT=3s
COMMERCIAL_SERVICE_RETRIES=3

# 3D Meta API
THREE_D_META_URL=http://3d-meta-api
```

## Running the Service

### Local Development

```bash
# Load environment variables
source config.env

# Run the service
go run cmd/server/main.go
```

### Docker

```bash
# Build and run with Docker Compose (recommended)
make dev-up

# Or build manually
docker build -f Dockerfile -t features-service:latest .
docker run --env-file config.env features-service:latest
```

## API Documentation

### gRPC Services

The service exposes the following gRPC services:

- `FeatureService`: Feature browsing and details
- `FeatureMarketplaceService`: Buy/sell operations, requests
- `FeatureProfitService`: Hourly profit management
- `MapsService`: Map data retrieval
- `BuildingService`: Building management

See `api-docs/features-service/` for detailed REST API documentation (via gRPC Gateway).

### Building Endpoints

The `BuildingService` provides endpoints for managing buildings on features:

#### GetBuildPackage
Fetches available building models from the 3D Meta API for a feature.

**Request:**
```protobuf
message GetBuildPackageRequest {
  uint64 feature_id = 1;
  int32 page = 2; // Optional, defaults to 1
}
```

**Response:**
```protobuf
message BuildPackageResponse {
  repeated BuildingModel models = 1;
  repeated string coordinates = 2; // Feature polygon coordinates
}
```

**Example:**
```bash
grpcurl -plaintext -d '{"feature_id": 123, "page": 1}' \
  localhost:50051 features.BuildingService/GetBuildPackage
```

#### BuildFeature
Starts construction of a building on a feature.

**Request:**
```protobuf
message BuildFeatureRequest {
  uint64 feature_id = 1;
  string building_model_id = 2; // String ID from 3D API (e.g., "model_abc_001")
  string launched_satisfaction = 3; // Must be >= required_satisfaction
  string rotation = 4; // Building rotation angle
  string position = 5; // Format: "x,y" (e.g., "100.5, -50.25")
  BuildingInformation information = 6; // Optional building details
}
```

**Response:**
```protobuf
message BuildFeatureResponse {
  Feature feature = 1; // Returns Feature with building_models array
}
```

**Example:**
```bash
grpcurl -plaintext -d '{
  "feature_id": 123,
  "building_model_id": "model_abc_001",
  "launched_satisfaction": "25.0",
  "rotation": "45.0",
  "position": "100.5, -50.25",
  "information": {
    "activity_line": "Software Development",
    "name": "Tech Solutions Inc",
    "address": "123 Main St",
    "postal_code": "1234567890",
    "website": "https://example.com"
  }
}' localhost:50051 features.BuildingService/BuildFeature
```

**Validation Rules:**
- `building_model_id`: Required, must exist in database
- `launched_satisfaction`: Required, must be >= `required_satisfaction` of the model
- `rotation`: Required, must be a valid number
- `position`: Required, must match regex `^-?\d+(\.\d+)?,\s*-?\d+(\.\d+)?$`
- `information.activity_line`: Max 255 characters
- `information.name`: Max 255 characters
- `information.address`: Max 255 characters
- `information.postal_code`: Must be valid Iranian postal code (10 digits)
- `information.website`: Must be valid HTTP/HTTPS URL, max 255 characters
- `information.description`: Max 5000 characters

#### GetBuildings
Retrieves all buildings attached to a feature.

**Request:**
```protobuf
message GetBuildingsRequest {
  uint64 feature_id = 1;
}
```

**Response:**
```protobuf
message BuildingsResponse {
  repeated Building buildings = 1;
}
```

**Example:**
```bash
grpcurl -plaintext -d '{"feature_id": 123}' \
  localhost:50051 features.BuildingService/GetBuildings
```

#### UpdateBuilding
Updates an existing building's construction details.

**Request:**
```protobuf
message UpdateBuildingRequest {
  uint64 feature_id = 1;
  string building_model_id = 2;
  string launched_satisfaction = 3; // Optional
  string rotation = 4; // Optional
  string position = 5; // Optional
  BuildingInformation information = 6; // Optional
}
```

**Response:**
```protobuf
message BuildingResponse {
  bool success = 1;
  string message = 2;
  Building building = 3;
}
```

**Example:**
```bash
grpcurl -plaintext -d '{
  "feature_id": 123,
  "building_model_id": "model_abc_001",
  "launched_satisfaction": "50.0",
  "rotation": "90.0",
  "position": "120, -60"
}' localhost:50051 features.BuildingService/UpdateBuilding
```

#### DestroyBuilding
Removes a building from a feature and refunds satisfaction.

**Request:**
```protobuf
message DestroyBuildingRequest {
  uint64 feature_id = 1;
  string building_model_id = 2;
}
```

**Response:**
```protobuf
message BuildingResponse {
  bool success = 1;
  string message = 2;
}
```

**Example:**
```bash
grpcurl -plaintext -d '{
  "feature_id": 123,
  "building_model_id": "model_abc_001"
}' localhost:50051 features.BuildingService/DestroyBuilding
```

**Business Logic:**
- Deletes building from database
- Refunds `launched_satisfaction` to user's wallet
- Reactivates hourly profits for the feature
- Deactivates building-related hourly profits

### Building Calculations

#### Required Satisfaction
Calculated when fetching build package:
```
required_satisfaction = area × karbari_coefficient × density × 0.1 ÷ 100
```
Formatted to 4 decimal places (e.g., "12.5000").

#### Construction Duration
Calculated when building:
```
duration_hours = required_satisfaction × 288000 ÷ launched_satisfaction
```
Converted to Jalali calendar dates for `construction_start_date` and `construction_end_date`.

#### Bubble Diameter
Calculated from model attributes:
```
perimeter = 2 × (width + length)
coefficient = 1 + (0.3 × (density - 1))
bubble_diameter = perimeter × coefficient
```

### Error Responses

All endpoints return gRPC status codes:
- `InvalidArgument` (400): Validation errors
- `PermissionDenied` (403): User doesn't own feature
- `FailedPrecondition` (412): Business rule violation (e.g., insufficient wallet, building already exists)
- `Internal` (500): Server errors

Validation errors follow Laravel format:
```json
{
  "message": "The given data was invalid",
  "errors": {
    "launched_satisfaction": [
      "The launched satisfaction must be at least 12.5"
    ],
    "position": [
      "The position format is invalid"
    ]
  }
}
```

## Metrics

Prometheus metrics are exposed on `/metrics` endpoint (port 9090 by default):

- `metargb_features_buy_requests_total`: Buy requests by status (accepted/rejected/cancelled)
- `metargb_features_sell_requests_total`: Total sell requests
- `metargb_features_trades_total`: Trades by type (limited/rgb/user)
- `metargb_features_trade_value_psc`: Trade values in PSC (histogram)
- `metargb_features_trade_value_irr`: Trade values in IRR (histogram)
- `metargb_features_buy_request_locked_assets_psc`: Locked PSC assets (gauge)
- `metargb_features_buy_request_locked_assets_irr`: Locked IRR assets (gauge)

## Event Broadcasting

The service broadcasts `FeatureStatusChanged` events via Redis when:
- A sell request is created
- A sell request is deleted
- A feature is purchased (all three paths: limited, RGB, user-to-user)
- A buy request is accepted

Events are published to the channel specified by `BROADCAST_CHANNEL` (default: `feature-events`).

## Notifications

The service sends notifications for:
- Buy request creation (to buyer and seller)
- Buy request acceptance (to buyer and seller)
- Sell request creation (to seller)
- Feature purchase completion (to buyer)
- Hourly profit withdrawal (to user)

## Error Handling

Errors are mapped to gRPC status codes:
- `InvalidArgument`: Validation errors, bad input
- `NotFound`: Resource not found
- `Unauthenticated`: Missing/invalid token
- `PermissionDenied`: Authorization failure
- `FailedPrecondition`: Business rule violation
- `Internal`: Unexpected server errors

## Testing

See `TESTING.md` for testing guidelines and examples.

## Development

### Project Structure

```
features-service/
├── cmd/server/          # Application entry point
├── internal/
│   ├── handler/         # gRPC handlers
│   ├── service/         # Business logic
│   ├── repository/      # Data access
│   ├── client/          # External service clients
│   ├── events/          # Event broadcasting
│   ├── metrics/         # Prometheus metrics
│   ├── models/          # Domain models
│   └── constants/       # Constants and configuration
├── pkg/                 # Public packages
├── config.env.sample    # Configuration template
└── Dockerfile           # Container definition
```

### Adding New Features

1. Define proto messages in `shared/proto/features/`
2. Run `make proto` to generate Go code
3. Implement repository methods (data access)
4. Implement service methods (business logic)
5. Implement handler methods (gRPC interface)
6. Register handler in `main.go`
7. Add tests

## License

Part of the MetaRGB microservices platform.

