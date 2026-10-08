# Support Service

A Go microservice that provides comprehensive support ticket, reporting, and user event management functionality, fully compatible with the existing platform API.

## Features

### 1. Ticket System
- Create and manage support tickets
- Department-based or user-to-user tickets
- Response system for ticket conversations
- Status management (New, Answered, Resolved, Unresolved, Tracking, Closed)
- Attachment support for up to 5 files (pdf, docx, jpg, jpeg, png, doc)
- Notification integration
- Authorization policies

### 2. Report System
- User reports with subject, title, and content
- Image attachments (polymorphic)
- Status tracking
- Pagination support

### 3. User Event System
- Track user events with IP and device
- Report suspicious events
- Response system for event reports
- Status and closure tracking

### 4. Notes
- Personal notes for the authenticated user
- Add and delete attachments, at most 5 files (pdf, docx, jpg, jpeg, png)

## Technology Stack

- **Language**: Go 1.24
- **Protocol**: gRPC
- **Database**: MySQL (shared instance)
- **Dependencies**:
  - `google.golang.org/grpc` - gRPC framework
  - `github.com/go-sql-driver/mysql` - MySQL driver
  - `github.com/joho/godotenv` - Environment configuration

## Project Structure

```
support-service/
├── cmd/
│   └── server/
│       └── main.go                 # Application entry point
├── internal/
│   ├── handler/                    # gRPC handlers
│   │   ├── ticket_handler.go
│   │   ├── report_handler.go
│   │   └── user_event_handler.go
│   ├── service/                    # Business logic
│   │   ├── ticket_service.go
│   │   ├── report_service.go
│   │   └── user_event_service.go
│   ├── repository/                 # Database operations
│   │   ├── ticket_repository.go
│   │   ├── report_repository.go
│   │   └── user_event_repository.go
│   ├── models/                     # Data models
│   │   ├── ticket.go
│   │   ├── report.go
│   │   └── user_event.go
│   └── utils/                      # Utilities
│       └── jalali.go               # Jalali date conversion
├── config.env.sample               # Configuration template
├── Dockerfile                      # Docker configuration
├── go.mod                          # Go dependencies
└── README.md                       # This file
```

## Installation

### Prerequisites
- Go 1.24 or higher
- MySQL database (shared instance)
- Access to shared protobuf definitions

### Setup

1. **Clone and navigate to service:**
```bash
cd metarang-microservices/services/support-service
```

2. **Install dependencies:**
```bash
go mod download
```

3. **Configure environment:**
```bash
cp config.env.sample .env
# Edit .env with your configuration
```

4. **Run the service:**
```bash
go run cmd/server/main.go
```

## Configuration

Create a `.env` file with the following variables:

```env
# Database Configuration
DB_HOST=localhost
DB_PORT=3306
DB_DATABASE=metarang_db
DB_USER=root
DB_PASSWORD=

# gRPC Configuration
GRPC_PORT=50054

# Service Dependencies
NOTIFICATION_SERVICE_ADDR=localhost:50055
```

## Database Schema

The service uses the existing shared database tables:

### Tickets
- `tickets` - Main ticket table
- `ticket_responses` - Ticket responses

### Reports
- `reports` - User reports
- `images` - Polymorphic image attachments

### User Events
- `user_events` - User event tracking
- `user_event_reports` - Event reports
- `user_event_report_responses` - Report responses

## API Reference

### TicketService

#### CreateTicket
Create a new support ticket.

**Request:**
```protobuf
message CreateTicketRequest {
  uint64 user_id = 1;
  string title = 2;
  string content = 3;
  string attachment = 4;
  uint64 receiver_id = 5;
  string department = 6;
}
```

**Response:** `TicketResponse`

#### GetTickets
List user's tickets with pagination.

**Request:**
```protobuf
message GetTicketsRequest {
  uint64 user_id = 1;
  PaginationRequest pagination = 2;
  int32 status_filter = 3;
}
```

**Response:** `TicketsResponse`

#### GetTicket
Get a single ticket with all details.

**Request:**
```protobuf
message GetTicketRequest {
  uint64 ticket_id = 1;
  uint64 user_id = 2;
}
```

**Response:** `TicketResponse`

#### UpdateTicket
Update an existing ticket.

**Request:**
```protobuf
message UpdateTicketRequest {
  uint64 ticket_id = 1;
  uint64 user_id = 2;
  string title = 3;
  string content = 4;
  string attachment = 5;
}
```

**Response:** `TicketResponse`

#### AddResponse
Add a response to a ticket.

**Request:**
```protobuf
message AddResponseRequest {
  uint64 ticket_id = 1;
  uint64 user_id = 2;
  string response = 3;
  string attachment = 4;
}
```

**Response:** `TicketResponse`

#### CloseTicket
Close a ticket.

**Request:**
```protobuf
message CloseTicketRequest {
  uint64 ticket_id = 1;
  uint64 user_id = 2;
}
```

**Response:** `TicketResponse`

### ReportService

#### CreateReport
Create a new report.

**Request:**
```protobuf
message CreateReportRequest {
  uint64 user_id = 1;
  string reportable_type = 2;
  uint64 reportable_id = 3;
  string reason = 4;
  string description = 5;
}
```

**Response:** `ReportResponse`

#### GetReports
List user's reports.

**Request:**
```protobuf
message GetReportsRequest {
  uint64 user_id = 1;
  PaginationRequest pagination = 2;
}
```

**Response:** `ReportsResponse`

#### GetReport
Get a single report.

**Request:**
```protobuf
message GetReportRequest {
  uint64 report_id = 1;
}
```

**Response:** `ReportResponse`

### UserEventReportService

#### CreateUserEvent
Create a new user event.

**Request:**
```protobuf
message CreateUserEventRequest {
  uint64 user_id = 1;
  string title = 2;
  string description = 3;
  string event_date = 4;
}
```

**Response:** `UserEventResponse`

#### GetUserEvents
List user's events.

**Request:**
```protobuf
message GetUserEventsRequest {
  uint64 user_id = 1;
  PaginationRequest pagination = 2;
}
```

**Response:** `UserEventsResponse`

#### GetUserEvent
Get a single user event.

**Request:**
```protobuf
message GetUserEventRequest {
  uint64 event_id = 1;
}
```

**Response:** `UserEventResponse`

#### ReportUserEvent
Create a report for a user event.

**Request:**
```protobuf
message ReportUserEventRequest {
  uint64 event_id = 1;
  uint64 reporter_id = 2;
  string suspicious_citizen = 3;
  string event_description = 4;
}
```

**Response:** `UserEventReportResponse`

#### SendEventReportResponse
Send a response to an event report.

**Request:**
```protobuf
message SendEventReportResponseRequest {
  uint64 report_id = 1;
  uint64 responder_id = 2;
  string response = 3;
}
```

**Response:** `Empty`

## Features

### Ticket Status Codes
- `0` - NEW: Newly created ticket
- `1` - ANSWERED: Response has been added
- `2` - RESOLVED: Issue resolved
- `3` - UNRESOLVED: Issue not resolved
- `4` - TRACKING: Under investigation
- `5` - CLOSED: Ticket closed

### Supported Departments
- `technical_support` - Technical Support (پشتیبانی فنی)
- `citizens_safety` - Citizens Safety (امنیت شهروندان)
- `investment` - Investment (سرمایه گذاری)
- `inspection` - Inspection (بازرسی)
- `protection` - Protection (حراست)
- `ztb` - ZTB Management (مدیریت کل ز ت ب)

### Authorization Rules
- **View Ticket**: Sender or receiver only
- **Update Ticket**: Sender only
- **Add Response**: Sender or receiver (if ticket is open)
- **Close Ticket**: Sender only (if ticket is open)

### Jalali Date Support
All dates are formatted in Jalali (Persian) calendar:
- Date format: `YYYY/MM/DD`
- Time format: `HH:MM:SS`
- DateTime format: `YYYY/MM/DD HH:MM:SS`

## Development

### Running Tests
```bash
go test ./...
```

### Building
```bash
go build -o server cmd/server/main.go
```

### Docker
```bash
docker build -t support-service .
docker run -p 50054:50054 support-service
```

## Integration

### Database compatibility
The service uses the shared MySQL database and maintains full compatibility:
- Same table structure
- Same field names (including legacy typos)
- Same status codes
- Same business logic

### With Other Services
- **Notification Service**: Sends notifications for ticket events
- **User Service**: Should query for user information (currently uses placeholders)
- **API Gateway**: Exposes HTTP endpoints that call this gRPC service

## Monitoring

The service logs important events:
- Database connection status
- gRPC server startup
- Request processing errors
- Notification sending failures

## Error Handling

gRPC error codes used:
- `InvalidArgument` - Missing or invalid required fields
- `NotFound` - Requested resource not found
- `Internal` - Database or system errors
- `PermissionDenied` - Authorization failures

## Performance

- Database connection pooling (max 25 connections)
- Indexed queries for fast lookups
- Pagination to limit result sets
- Efficient joins for related data

## Compatibility

This service is **100% compatible** with the existing platform API:
- All features replicated
- All business logic matched
- All database schema compatible
- All status codes identical
- All field names preserved (including typos)

See `IMPLEMENTATION_COMPARISON.md` for detailed comparison.

## Contributing

1. Follow Go best practices
2. Maintain API compatibility
3. Add tests for new features
4. Update documentation
5. Use conventional commit messages

## License

Proprietary - metarang Platform

