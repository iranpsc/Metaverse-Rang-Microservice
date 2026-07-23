module metarang/auth-service/tests

go 1.25.12

require (
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.3.0
	github.com/go-sql-driver/mysql v1.7.1
	github.com/redis/go-redis/v9 v9.17.2
	github.com/stretchr/testify v1.11.1
	golang.org/x/crypto v0.53.0
	google.golang.org/grpc v1.82.1
	metarang/auth-service v0.0.0
	metarang/shared v0.0.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/gabriel-vasile/mimetype v1.4.12 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.16.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/leodido/go-urn v1.2.4 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/yaa110/go-persian-calendar v1.2.0 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.39.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260414002931-afd174a4e478 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace metarang/auth-service => ../../services/auth-service

replace metarang/shared => ../../shared
