# Multi-stage build for gRPC Gateway Service
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make protoc protobuf-dev

# Install protoc-gen-go and protoc-gen-go-grpc plugins
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

WORKDIR /workspace

# Create the proper directory structure
RUN mkdir -p /workspace/metargb/grpc-gateway /workspace/metargb/shared

# Copy proto files first
COPY ./shared/proto/ /workspace/metargb/shared/proto/

# Generate proto files before copying shared module
ENV PATH="${PATH}:/root/go/bin"
RUN mkdir -p /workspace/metargb/shared/pb/auth && \
    protoc --go_out=/workspace/metargb/shared/pb/auth --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/auth --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/auth.proto && \
    mkdir -p /workspace/metargb/shared/pb/common && \
    protoc --go_out=/workspace/metargb/shared/pb/common --go_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/common.proto && \
    mkdir -p /workspace/metargb/shared/pb/dynasty && \
    protoc --go_out=/workspace/metargb/shared/pb/dynasty --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/dynasty --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/dynasty.proto && \
    mkdir -p /workspace/metargb/shared/pb/levels && \
    protoc --go_out=/workspace/metargb/shared/pb/levels --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/levels --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/levels.proto && \
    mkdir -p /workspace/metargb/shared/pb/training && \
    protoc --go_out=/workspace/metargb/shared/pb/training --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/training --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/training.proto && \
    mkdir -p /workspace/metargb/shared/pb/features && \
    protoc --go_out=/workspace/metargb/shared/pb/features --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/features --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/features.proto && \
    mkdir -p /workspace/metargb/shared/pb/financial && \
    protoc --go_out=/workspace/metargb/shared/pb/financial --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/financial --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/financial.proto && \
    mkdir -p /workspace/metargb/shared/pb/social && \
    protoc --go_out=/workspace/metargb/shared/pb/social --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/social --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/social.proto && \
    mkdir -p /workspace/metargb/shared/pb/notifications && \
    protoc --go_out=/workspace/metargb/shared/pb/notifications --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/notifications --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/notifications.proto && \
    mkdir -p /workspace/metargb/shared/pb/support && \
    protoc --go_out=/workspace/metargb/shared/pb/support --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/support --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/support.proto && \
    mkdir -p /workspace/metargb/shared/pb/calendar && \
    protoc --go_out=/workspace/metargb/shared/pb/calendar --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/calendar --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/calendar.proto

# Copy shared module first
COPY ./shared/go.mod /workspace/metargb/shared/
COPY ./shared/ /workspace/metargb/shared/

# Copy gateway service
COPY ./services/grpc-gateway/go.mod /workspace/metargb/grpc-gateway/
COPY ./services/grpc-gateway/ /workspace/metargb/grpc-gateway/

# Setup go workspace
WORKDIR /workspace
RUN echo 'go 1.24.0' > go.work && \
    echo '' >> go.work && \
    echo 'use (' >> go.work && \
    echo '    ./metargb/grpc-gateway' >> go.work && \
    echo '    ./metargb/shared' >> go.work && \
    echo ')' >> go.work

# Download dependencies
WORKDIR /workspace/metargb/grpc-gateway

# Update replace directive for Docker context
RUN sed -i 's|replace metargb/shared => ../../shared|replace metargb/shared => /workspace/metargb/shared|g' go.mod

ENV GOPROXY=https://goproxy.io,direct
RUN go mod download

# Build the application
WORKDIR /workspace
ENV GOWORK=/workspace/go.work
RUN cd /workspace/metargb/grpc-gateway && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o /app/grpc-gateway ./cmd/server

# Final stage
FROM alpine:latest

RUN apk update && apk --no-cache add ca-certificates tzdata wget

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/grpc-gateway .

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

USER appuser

# Expose HTTP port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
ENTRYPOINT ["/app/grpc-gateway"]

