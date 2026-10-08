# syntax=docker/dockerfile:1

# Multi-stage build for Features Service
FROM golang:1.25-alpine AS builder

ENV CGO_ENABLED=0

# protobuf-dev provides the well-known google/protobuf includes that protoc imports.
RUN apk add --no-cache git protoc protobuf-dev

ENV PATH="/go/bin:${PATH}"
RUN --mount=type=cache,id=go-mod,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,id=go-build,target=/root/.cache/go-build,sharing=locked \
    go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11 \
 && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

WORKDIR /src

# Module files only, so a source edit does not download modules again.
COPY shared/go.mod shared/go.sum shared/
COPY services/features-service/go.mod services/features-service/go.sum* services/features-service/
RUN printf 'go 1.25.13\n\nuse (\n\t./services/features-service\n\t./shared\n)\n' > go.work

ENV GOWORK=/src/go.work
WORKDIR /src/services/features-service
RUN --mount=type=cache,id=go-mod,target=/go/pkg/mod,sharing=locked \
    go mod download

# Proto generation depends on .proto files, not on service source.
COPY shared/proto /src/shared/proto
COPY scripts/docker/generate-all-protos.sh /tmp/generate-all-protos.sh
RUN sed -i 's/\r$//' /tmp/generate-all-protos.sh \
 && sh /tmp/generate-all-protos.sh /src/shared

COPY shared/pkg /src/shared/pkg
COPY services/features-service/ /src/services/features-service/

WORKDIR /src/services/features-service
RUN --mount=type=cache,id=go-mod,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,id=go-build,target=/root/.cache/go-build,sharing=locked \
    go build -trimpath -buildvcs=false -ldflags="-s -w" -o /app/features-service ./cmd/server \
 && go build -trimpath -buildvcs=false -ldflags="-s -w" -o /app/calculate-hourly-profits ./cmd/calculate-profits

FROM alpine:3.21

# Alpine already includes BusyBox wget, which accepts the healthcheck flags.
RUN apk add --no-cache --upgrade ca-certificates tzdata libcrypto3 libssl3 \
    && addgroup -g 1000 appuser \
    && adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --link --from=builder --chown=1000:1000 /app/features-service /app/calculate-hourly-profits /app/

USER appuser

EXPOSE 50053 8062

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://127.0.0.1:8062/health || exit 1

ENTRYPOINT ["/app/features-service"]
