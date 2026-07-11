module metargb/calendar-service/tests

go 1.25.0

require (
	google.golang.org/grpc v1.79.3
	metargb/calendar-service v0.0.0
	metargb/shared v0.0.0
)

require (
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.39.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace metargb/shared => ../../shared

replace metargb/calendar-service => ../../services/calendar-service
