# Multi-stage build for Commercial Service
FROM docker.arvancloud.ir/golang:1.24-alpine AS builder

# Iran-accessible Alpine package mirror (MobinHost)
RUN sed -i 's|https://dl-cdn.alpinelinux.org|https://mirror.mobinhost.com|g' /etc/apk/repositories
# Iran-accessible Go module proxy (ParsPack Abrha mirror)
ENV GOPROXY=https://mirror.abrha.net/repository/go/,direct
ENV GOSUMDB=off
# Install build dependencies
RUN apk add --no-cache git make protoc protobuf-dev

# Install protoc-gen-go and protoc-gen-go-grpc plugins
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.6 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

WORKDIR /workspace

# Create the proper directory structure
RUN mkdir -p /workspace/metargb/commercial-service /workspace/metargb/shared

# Copy proto files first
COPY ./shared/proto/ /workspace/metargb/shared/proto/

# Generate proto files before copying shared module
ENV PATH="${PATH}:/root/go/bin"
RUN mkdir -p /workspace/metargb/shared/pb/commercial && \
    protoc --go_out=/workspace/metargb/shared/pb/commercial --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/commercial --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/commercial.proto

# Generate auth proto files (needed by shared/pkg/auth)
RUN mkdir -p /workspace/metargb/shared/pb/auth && \
    protoc --go_out=/workspace/metargb/shared/pb/auth --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/auth --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/auth.proto

# Copy shared module first
COPY ./shared/go.mod /workspace/metargb/shared/
COPY ./shared/ /workspace/metargb/shared/

# Copy commercial service
COPY ./services/commercial-service/go.mod ./services/commercial-service/go.sum* /workspace/metargb/commercial-service/
COPY ./services/commercial-service/ /workspace/metargb/commercial-service/

# Setup go workspace
WORKDIR /workspace
RUN echo 'go 1.24.0' > go.work && \
    echo '' >> go.work && \
    echo 'use (' >> go.work && \
    echo '    ./metargb/commercial-service' >> go.work && \
    echo '    ./metargb/shared' >> go.work && \
    echo ')' >> go.work

# Download dependencies
WORKDIR /workspace/metargb/commercial-service

# Update replace directive for Docker context
RUN sed -i 's|replace metargb/shared => ../../shared|replace metargb/shared => /workspace/metargb/shared|g' go.mod

# Iran-accessible Go module proxy (ParsPack Abrha mirror)
ENV GOPROXY=https://mirror.abrha.net/repository/go/,direct
ENV GOSUMDB=off
RUN go mod download

# Build the application  
WORKDIR /workspace
ENV GOWORK=/workspace/go.work
RUN cd /workspace/metargb/commercial-service && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o /app/commercial-service ./cmd/server

# Final stage
FROM docker.arvancloud.ir/alpine:3.21

# Iran-accessible Alpine package mirror (MobinHost)
RUN sed -i 's|https://dl-cdn.alpinelinux.org|https://mirror.mobinhost.com|g' /etc/apk/repositories
RUN apk update && apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/commercial-service .

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

USER appuser

# Expose gRPC port
EXPOSE 50052

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD nc -z localhost 50052 || exit 1

# Run the application
ENTRYPOINT ["/app/commercial-service"]

