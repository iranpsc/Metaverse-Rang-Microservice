# Sitemap generator — writes urlsets on a 3-hour schedule.
FROM golang:1.25-alpine AS builder

ENV CGO_ENABLED=0

RUN apk add --no-cache git

WORKDIR /src

COPY services/sitemap-generator-service/go.mod services/sitemap-generator-service/go.sum* ./
RUN go mod download

COPY services/sitemap-generator-service/ ./
RUN go build -trimpath -ldflags="-s -w" -o /out/sitemap-generator-service ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata wget su-exec && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder /out/sitemap-generator-service /app/sitemap-generator-service
COPY services/sitemap-generator-service/templates.json /app/templates.json
COPY services/sitemap-generator-service/docker-entrypoint.sh /entrypoint.sh

RUN chmod +x /entrypoint.sh && \
    mkdir -p /app/sitemaps && \
    chown -R appuser:appuser /app

EXPOSE 8071

HEALTHCHECK --interval=30s --timeout=5s --start-period=15s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://127.0.0.1:8071/live || exit 1

ENTRYPOINT ["/entrypoint.sh"]
