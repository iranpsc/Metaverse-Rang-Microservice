# Multi-stage build for Social Service
FROM golang:1.25-alpine AS builder

ENV CGO_ENABLED=0

RUN apk add --no-cache git

WORKDIR /workspace

RUN mkdir -p /workspace/metargb/social-service /workspace/metargb/shared

COPY ./shared/go.mod /workspace/metargb/shared/
COPY ./shared/ /workspace/metargb/shared/

COPY ./services/social-service/go.mod ./services/social-service/go.sum* /workspace/metargb/social-service/
COPY ./services/social-service/ /workspace/metargb/social-service/

WORKDIR /workspace
RUN echo 'go 1.24.0' > go.work && \
    printf '\nuse (\n\t./metargb/social-service\n\t./metargb/shared\n)\n' >> go.work

WORKDIR /workspace/metargb/social-service
RUN sed -i 's|replace metargb/shared => ../../shared|replace metargb/shared => /workspace/metargb/shared|g' go.mod || true && \
    go get metargb/shared@v0.0.0 || true && \
    go mod download

ENV GOWORK=/workspace/go.work
RUN go build -trimpath -ldflags="-s -w" -o /app/social-service ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata netcat-openbsd && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder --chown=appuser:appuser /app/social-service .

USER appuser

EXPOSE 50061

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD nc -z localhost 50061 || exit 1

ENTRYPOINT ["/app/social-service"]
