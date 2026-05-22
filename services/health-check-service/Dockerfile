# Health Check Service Dockerfile
FROM docker.arvancloud.ir/golang:1.24-alpine AS builder

# Iran-accessible Alpine package mirror (MobinHost)
RUN sed -i 's|https://dl-cdn.alpinelinux.org|https://mirror.mobinhost.com|g' /etc/apk/repositories
# Iran-accessible Go module proxy (ParsPack Abrha mirror)
ENV GOPROXY=https://mirror.abrha.net/repository/go/,direct
ENV GOSUMDB=off
# Install build dependencies (git needed for GOPROXY=direct)
RUN apk add --no-cache git

WORKDIR /app

# Copy go mod files
COPY services/health-check-service/go.mod services/health-check-service/go.sum* ./

# Download dependencies
# Iran-accessible Go module proxy (ParsPack Abrha mirror)
ENV GOPROXY=https://mirror.abrha.net/repository/go/,direct
ENV GOSUMDB=off
RUN go mod download

# Copy source
COPY services/health-check-service/main.go .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o health-check-service main.go

# Final stage
FROM docker.arvancloud.ir/alpine:3.21

# Iran-accessible Alpine package mirror (MobinHost)
RUN sed -i 's|https://dl-cdn.alpinelinux.org|https://mirror.mobinhost.com|g' /etc/apk/repositories
RUN apk update && apk --no-cache add ca-certificates wget

WORKDIR /app

COPY --from=builder /app/health-check-service .

EXPOSE 8090

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8090/live || exit 1

CMD ["./health-check-service"]

