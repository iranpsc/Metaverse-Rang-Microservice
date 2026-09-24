module metarang/notifications-service/tests

go 1.25.13

require (
	github.com/DATA-DOG/go-sqlmock v1.5.2
	github.com/stretchr/testify v1.11.1
	google.golang.org/grpc v1.83.2
	google.golang.org/protobuf v1.36.11
	metarang/notifications-service v0.0.0
	metarang/shared v0.0.0
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/gabriel-vasile/mimetype v1.4.12 // indirect
	github.com/getsentry/sentry-go v0.47.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.16.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/kavenegar/kavenegar-go v0.0.0-20240205151018-77039f51467d // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/leodido/go-urn v1.2.4 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/yaa110/go-persian-calendar v1.2.0 // indirect
	golang.org/x/crypto v0.55.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace metarang/notifications-service => ../../services/notifications-service

replace metarang/shared => ../../shared
