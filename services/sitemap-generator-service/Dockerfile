# syntax=docker/dockerfile:1

# Multi-stage build for Sitemap Generator Service
FROM golang:1.25-alpine AS builder

ENV CGO_ENABLED=0

WORKDIR /src

COPY shared/go.mod shared/go.sum shared/
COPY services/sitemap-generator-service/go.mod services/sitemap-generator-service/go.sum* services/sitemap-generator-service/
RUN printf 'go 1.25.13\n\nuse (\n\t./services/sitemap-generator-service\n\t./shared\n)\n' > go.work

ENV GOWORK=/src/go.work
WORKDIR /src/services/sitemap-generator-service
RUN --mount=type=cache,id=go-mod,target=/go/pkg/mod,sharing=locked \
    go mod download

COPY shared/pkg /src/shared/pkg
COPY services/sitemap-generator-service/ /src/services/sitemap-generator-service/

WORKDIR /src/services/sitemap-generator-service
RUN --mount=type=cache,id=go-mod,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,id=go-build,target=/root/.cache/go-build,sharing=locked \
    go build -trimpath -buildvcs=false -ldflags="-s -w" -o /app/sitemap-generator-service ./cmd/server

FROM alpine:3.21

# Alpine already includes BusyBox wget, which accepts the healthcheck flags.
RUN apk add --no-cache --upgrade ca-certificates tzdata libcrypto3 libssl3 \
    && addgroup -g 1000 appuser \
    && adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --link --from=builder --chown=1000:1000 /app/sitemap-generator-service /app/sitemap-generator-service
COPY --chown=1000:1000 services/sitemap-generator-service/templates ./templates

RUN mkdir -p /opt/metarang/sitemaps && chown -R appuser:appuser /opt/metarang/sitemaps

USER appuser

EXPOSE 8071

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://127.0.0.1:8071/health || exit 1

ENTRYPOINT ["/app/sitemap-generator-service"]
