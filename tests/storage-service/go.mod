module metarang/storage-service/tests

go 1.25.13

require (
	github.com/DATA-DOG/go-sqlmock v1.5.2
	google.golang.org/grpc v1.83.2
	metarang/shared v0.0.0
	metarang/storage-service v0.0.0
)

require (
	github.com/getsentry/sentry-go v0.47.0 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/jlaffaye/ftp v0.2.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace metarang/shared => ../../shared

replace metarang/storage-service => ../../services/storage-service
