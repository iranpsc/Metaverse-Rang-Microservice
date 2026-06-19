# Multi-stage build for Storage Service
FROM golang:1.24-alpine AS builder

ENV CGO_ENABLED=0

RUN apk add --no-cache git

WORKDIR /workspace

RUN mkdir -p /workspace/metargb/storage-service /workspace/metargb/shared

COPY ./shared/go.mod /workspace/metargb/shared/
COPY ./shared/ /workspace/metargb/shared/

COPY ./services/storage-service/go.mod ./services/storage-service/go.sum* /workspace/metargb/storage-service/
COPY ./services/storage-service/ /workspace/metargb/storage-service/

WORKDIR /workspace
RUN echo 'go 1.24.0' > go.work && \
    printf '\nuse (\n\t./metargb/storage-service\n\t./metargb/shared\n)\n' >> go.work

WORKDIR /workspace/metargb/storage-service
RUN sed -i 's|replace metargb/shared => ../../shared|replace metargb/shared => /workspace/metargb/shared|g' go.mod || true && \
    go get metargb/shared@v0.0.0 || true && \
    go mod download

ENV GOWORK=/workspace/go.work
RUN go build -trimpath -ldflags="-s -w" -o /app/storage-service ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata su-exec netcat-openbsd && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder --chown=appuser:appuser /app/storage-service .
COPY ./services/storage-service/docker-entrypoint.sh /docker-entrypoint.sh
RUN sed -i 's/\r$//' /docker-entrypoint.sh && chmod +x /docker-entrypoint.sh

RUN mkdir -p /app/uploads && chown -R appuser:appuser /app/uploads

EXPOSE 50060

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD nc -z localhost 50060 || exit 1

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["/app/storage-service"]
