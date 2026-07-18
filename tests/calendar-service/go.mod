module metarang/calendar-service/tests

go 1.25.12

require (
	github.com/DATA-DOG/go-sqlmock v1.5.2
	google.golang.org/grpc v1.79.3
	metarang/calendar-service v0.0.0
	metarang/shared v0.0.0
)

require (
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.39.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace metarang/shared => ../../shared

replace metarang/calendar-service => ../../services/calendar-service
