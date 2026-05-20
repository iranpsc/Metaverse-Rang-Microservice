# Multi-stage build for Training Service
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
RUN mkdir -p /workspace/metargb/training-service /workspace/metargb/shared

# Copy shared module first
COPY ./shared/go.mod /workspace/metargb/shared/
COPY ./shared/ /workspace/metargb/shared/

# Copy training service
COPY ./services/training-service/go.mod ./services/training-service/go.sum* /workspace/metargb/training-service/
COPY ./services/training-service/ /workspace/metargb/training-service/

# Verify cmd/server directory exists (debug step)
RUN ls -la /workspace/metargb/training-service/ && \
    ls -la /workspace/metargb/training-service/cmd/ || echo "cmd directory not found" && \
    ls -la /workspace/metargb/training-service/cmd/server/ || echo "cmd/server directory not found"

# Setup go workspace
WORKDIR /workspace
RUN echo 'go 1.24.0' > go.work && \
    echo '' >> go.work && \
    echo 'use (' >> go.work && \
    echo '    ./metargb/training-service' >> go.work && \
    echo '    ./metargb/shared' >> go.work && \
    echo ')' >> go.work

# Download dependencies
WORKDIR /workspace/metargb/training-service

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
RUN cd /workspace/metargb/training-service && \
    ls -la cmd/ && \
    CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o /app/training-service ./cmd/server

# Final stage
FROM docker.arvancloud.ir/alpine:3.21

# Iran-accessible Alpine package mirror (MobinHost)
RUN sed -i 's|https://dl-cdn.alpinelinux.org|https://mirror.mobinhost.com|g' /etc/apk/repositories
RUN apk update && apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/training-service .

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

USER appuser

# Expose gRPC port
EXPOSE 50057

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD nc -z localhost 50057 || exit 1

# Run the application
ENTRYPOINT ["/app/training-service"]
