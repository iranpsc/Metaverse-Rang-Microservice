module metarang/calendar-service/tests

go 1.25.12

require (
	github.com/DATA-DOG/go-sqlmock v1.5.2
	google.golang.org/grpc v1.82.1
	metarang/calendar-service v0.0.0
	metarang/shared v0.0.0
)

require (
	github.com/getsentry/sentry-go v0.47.0 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.39.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260414002931-afd174a4e478 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace metarang/shared => ../../shared

replace metarang/calendar-service => ../../services/calendar-service
