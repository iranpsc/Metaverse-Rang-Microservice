# Multi-stage build for Calendar Service
FROM golang:1.25-alpine AS builder

ENV CGO_ENABLED=0

RUN apk add --no-cache git

WORKDIR /workspace

RUN mkdir -p /workspace/metargb/calendar-service /workspace/metargb/shared

COPY ./shared/go.mod /workspace/metargb/shared/
COPY ./services/calendar-service/go.mod ./services/calendar-service/go.sum* /workspace/metargb/calendar-service/

WORKDIR /workspace
RUN echo 'go 1.24.0' > go.work && \
    printf '\nuse (\n\t./metargb/calendar-service\n\t./metargb/shared\n)\n' >> go.work

WORKDIR /workspace/metargb/calendar-service
RUN sed -i 's|replace metargb/shared => ../../shared|replace metargb/shared => /workspace/metargb/shared|g' go.mod && \
    go mod download

COPY ./shared/ /workspace/metargb/shared/
COPY ./services/calendar-service/ /workspace/metargb/calendar-service/

WORKDIR /workspace/metargb/calendar-service
RUN sed -i 's|replace metargb/shared => ../../shared|replace metargb/shared => /workspace/metargb/shared|g' go.mod

ENV GOWORK=/workspace/go.work
RUN go build -trimpath -ldflags="-s -w" -o /app/calendar-service ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata netcat-openbsd && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder --chown=appuser:appuser /app/calendar-service .

USER appuser

EXPOSE 50059

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD nc -z localhost 50059 || exit 1

ENTRYPOINT ["/app/calendar-service"]
