module metargb/auth-service

go 1.24.0

toolchain go1.24.3

require (
	github.com/go-sql-driver/mysql v1.7.1
	github.com/joho/godotenv v1.5.1
	golang.org/x/crypto v0.43.0
	google.golang.org/grpc v1.76.0
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/gabriel-vasile/mimetype v1.4.12 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.16.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/leodido/go-urn v1.2.4 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	github.com/yaa110/go-persian-calendar v1.2.0 // indirect
	golang.org/x/net v0.46.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/text v0.30.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251029180050-ab9386a59fda // indirect
)

require (
	github.com/redis/go-redis/v9 v9.16.0
	metargb/shared v0.0.0
)

replace metargb/shared => ../../shared
