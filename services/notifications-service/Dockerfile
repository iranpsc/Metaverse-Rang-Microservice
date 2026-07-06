# Multi-stage build for Notifications Service
FROM golang:1.25-alpine AS builder

ENV CGO_ENABLED=0

RUN apk add --no-cache git

WORKDIR /workspace

RUN mkdir -p /workspace/metargb/notifications-service /workspace/metargb/shared

COPY ./shared/go.mod /workspace/metargb/shared/
COPY ./shared/ /workspace/metargb/shared/

COPY ./services/notifications-service/go.mod ./services/notifications-service/go.sum* /workspace/metargb/notifications-service/
COPY ./services/notifications-service/ /workspace/metargb/notifications-service/

WORKDIR /workspace
RUN echo 'go 1.25.0' > go.work && \
    printf '\nuse (\n\t./metargb/notifications-service\n\t./metargb/shared\n)\n' >> go.work

WORKDIR /workspace/metargb/notifications-service
RUN sed -i 's|replace metargb/shared => ../../shared|replace metargb/shared => /workspace/metargb/shared|g' go.mod && \
    go mod download

ENV GOWORK=/workspace/go.work
RUN go build -trimpath -ldflags="-s -w" -o /app/notifications-service ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata netcat-openbsd && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder --chown=appuser:appuser /app/notifications-service .

USER appuser

EXPOSE 50058

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD nc -z localhost 50058 || exit 1

ENTRYPOINT ["/app/notifications-service"]
