module metarang/sitemap-generator-service/tests

go 1.25.13

require metarang/sitemap-generator-service v0.0.0

require (
	github.com/sirupsen/logrus v1.9.3 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	google.golang.org/grpc v1.83.2 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	metarang/shared v0.0.0 // indirect
)

replace metarang/sitemap-generator-service => ../../services/sitemap-generator-service

replace metarang/shared => ../../shared
