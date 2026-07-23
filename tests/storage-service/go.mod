module metarang/storage-service/tests

go 1.25.12

require metarang/storage-service v0.0.0

require (
	github.com/getsentry/sentry-go v0.47.0 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/jlaffaye/ftp v0.2.0 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.39.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260414002931-afd174a4e478 // indirect
	google.golang.org/grpc v1.82.1 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	metarang/shared v0.0.0 // indirect
)

replace metarang/shared => ../../shared

replace metarang/storage-service => ../../services/storage-service
