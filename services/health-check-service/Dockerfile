# Health Check Service Dockerfile
FROM golang:1.24-alpine AS builder

ENV CGO_ENABLED=0

RUN apk add --no-cache git

WORKDIR /app

COPY services/health-check-service/go.mod services/health-check-service/go.sum* ./
RUN go mod download

COPY services/health-check-service/main.go .
RUN go build -trimpath -ldflags="-s -w" -o health-check-service main.go

FROM alpine:3.21

RUN apk add --no-cache ca-certificates wget && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder --chown=appuser:appuser /app/health-check-service .

USER appuser

EXPOSE 8090

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8090/live || exit 1

ENTRYPOINT ["./health-check-service"]
