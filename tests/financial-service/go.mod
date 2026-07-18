module metarang/financial-service/tests

go 1.25.12

require (
	google.golang.org/grpc v1.79.3
	google.golang.org/protobuf v1.36.11
	metarang/financial-service v0.0.0
	metarang/shared v0.0.0
)

require (
	github.com/gabriel-vasile/mimetype v1.4.12 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.16.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/leodido/go-urn v1.2.4 // indirect
	github.com/yaa110/go-persian-calendar v1.2.0 // indirect
	golang.org/x/crypto v0.53.0 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.39.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
)

replace metarang/financial-service => ../../services/financial-service

replace metarang/shared => ../../shared
