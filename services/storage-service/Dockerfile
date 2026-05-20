# Multi-stage build for Storage Service
FROM docker.arvancloud.ir/golang:1.24-alpine AS builder

# Iran-accessible Alpine package mirror (MobinHost)
RUN sed -i 's|https://dl-cdn.alpinelinux.org|https://mirror.mobinhost.com|g' /etc/apk/repositories
# Iran-accessible Go module proxy (ParsPack Abrha mirror)
ENV GOPROXY=https://mirror.abrha.net/repository/go/,direct
ENV GOSUMDB=off
# Install build dependencies
RUN apk add --no-cache git make protoc protobuf-dev

WORKDIR /workspace

# Create the proper directory structure
RUN mkdir -p /workspace/metargb/storage-service /workspace/metargb/shared

# Copy shared module first
COPY ./shared/go.mod /workspace/metargb/shared/
COPY ./shared/ /workspace/metargb/shared/

# Copy storage service
COPY ./services/storage-service/go.mod ./services/storage-service/go.sum* /workspace/metargb/storage-service/
COPY ./services/storage-service/ /workspace/metargb/storage-service/

# Setup go workspace
WORKDIR /workspace
RUN echo 'go 1.24.0' > go.work && \
    echo '' >> go.work && \
    echo 'use (' >> go.work && \
    echo '    ./metargb/storage-service' >> go.work && \
    echo '    ./metargb/shared' >> go.work && \
    echo ')' >> go.work

# Download dependencies
WORKDIR /workspace/metargb/storage-service

# Update replace directive for Docker context
RUN sed -i 's|replace metargb/shared => ../../shared|replace metargb/shared => /workspace/metargb/shared|g' go.mod || true

# Ensure metargb/shared is in the go.mod
RUN go get metargb/shared@v0.0.0 || true

# Iran-accessible Go module proxy (ParsPack Abrha mirror)
ENV GOPROXY=https://mirror.abrha.net/repository/go/,direct
ENV GOSUMDB=off
RUN go mod download

# Build the application  
WORKDIR /workspace
ENV GOWORK=/workspace/go.work
RUN cd /workspace/metargb/storage-service && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o /app/storage-service ./cmd/server

# Final stage
FROM docker.arvancloud.ir/alpine:3.21

# Iran-accessible Alpine package mirror (MobinHost)
RUN sed -i 's|https://dl-cdn.alpinelinux.org|https://mirror.mobinhost.com|g' /etc/apk/repositories
RUN apk update && apk --no-cache add ca-certificates tzdata su-exec

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/storage-service .

# Entrypoint fixes bind-mount permissions before dropping to appuser
COPY ./services/storage-service/docker-entrypoint.sh /docker-entrypoint.sh
RUN sed -i 's/\r$//' /docker-entrypoint.sh && chmod +x /docker-entrypoint.sh

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

# Create uploads directory with proper permissions (overridden by bind mounts at runtime)
RUN mkdir -p /app/uploads && \
    chown -R appuser:appuser /app/uploads

# Expose gRPC port
EXPOSE 50060

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD nc -z localhost 50060 || exit 1

# Run as root briefly so entrypoint can chown bind-mounted uploads, then exec as appuser
ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["/app/storage-service"]
