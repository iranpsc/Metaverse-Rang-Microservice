# Multi-stage build for Support Service
FROM golang:1.25-alpine AS builder

ENV CGO_ENABLED=0

RUN apk add --no-cache git

WORKDIR /workspace

RUN mkdir -p /workspace/metargb/support-service /workspace/metargb/shared

COPY ./shared/go.mod /workspace/metargb/shared/
COPY ./shared/ /workspace/metargb/shared/

COPY ./services/support-service/go.mod ./services/support-service/go.sum* /workspace/metargb/support-service/
COPY ./services/support-service/ /workspace/metargb/support-service/

WORKDIR /workspace
RUN echo 'go 1.24.0' > go.work && \
    printf '\nuse (\n\t./metargb/support-service\n\t./metargb/shared\n)\n' >> go.work

WORKDIR /workspace/metargb/support-service
RUN sed -i 's|replace metargb/shared => ../../shared|replace metargb/shared => /workspace/metargb/shared|g' go.mod || true && \
    go get metargb/shared@v0.0.0 || true && \
    go mod download

ENV GOWORK=/workspace/go.work
RUN go build -trimpath -ldflags="-s -w" -o /app/support-service ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata netcat-openbsd && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder --chown=appuser:appuser /app/support-service .

USER appuser

EXPOSE 50056

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD nc -z localhost 50056 || exit 1

ENTRYPOINT ["/app/support-service"]
