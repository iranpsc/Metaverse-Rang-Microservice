# Multi-stage build for Sitemap Generator Service
FROM golang:1.25-alpine AS builder

ENV CGO_ENABLED=0

RUN apk add --no-cache git make

WORKDIR /workspace

RUN mkdir -p /workspace/metarang/sitemap-generator-service /workspace/metarang/shared

COPY ./shared/go.mod ./shared/go.sum /workspace/metarang/shared/
COPY ./shared/pkg/ /workspace/metarang/shared/pkg/

COPY ./services/sitemap-generator-service/go.mod ./services/sitemap-generator-service/go.sum* /workspace/metarang/sitemap-generator-service/
COPY ./services/sitemap-generator-service/ /workspace/metarang/sitemap-generator-service/

WORKDIR /workspace
RUN echo 'go 1.25.13' > go.work && \
    printf '\nuse (\n\t./metarang/sitemap-generator-service\n\t./metarang/shared\n)\n' >> go.work

WORKDIR /workspace/metarang/sitemap-generator-service
RUN sed -i 's|replace metarang/shared => ../../shared|replace metarang/shared => /workspace/metarang/shared|g' go.mod && \
    go mod download

ENV GOWORK=/workspace/go.work
RUN go build -trimpath -ldflags="-s -w" -o /app/sitemap-generator-service ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata wget && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder --chown=appuser:appuser /app/sitemap-generator-service .
COPY --from=builder --chown=appuser:appuser /workspace/metarang/sitemap-generator-service/templates ./templates

RUN mkdir -p /opt/metarang/sitemaps && chown -R appuser:appuser /opt/metarang/sitemaps

USER appuser

EXPOSE 8071

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://127.0.0.1:8071/health || exit 1

ENTRYPOINT ["/app/sitemap-generator-service"]
