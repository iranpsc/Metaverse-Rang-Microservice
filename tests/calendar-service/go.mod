module metarang/calendar-service/tests

go 1.26.0

require (
	github.com/DATA-DOG/go-sqlmock v1.5.2
	google.golang.org/grpc v1.83.2
	google.golang.org/protobuf v1.36.11
	metarang/calendar-service v0.0.0
	metarang/shared v0.0.0
)

require (
	github.com/getsentry/sentry-go v0.47.0 // indirect
	golang.org/x/net v0.60.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
)

replace metarang/shared => ../../shared

replace metarang/calendar-service => ../../services/calendar-service
