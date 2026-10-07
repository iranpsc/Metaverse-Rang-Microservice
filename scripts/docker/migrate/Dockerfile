# syntax=docker/dockerfile:1

# One-shot Laravel-style SQL migrator (shared/cmd/migrate).
FROM golang:1.25-alpine AS builder

ENV CGO_ENABLED=0
WORKDIR /src

COPY shared/go.mod shared/go.sum shared/
WORKDIR /src/shared
RUN --mount=type=cache,id=go-mod,target=/go/pkg/mod,sharing=locked \
    go mod download

COPY shared/pkg /src/shared/pkg
COPY shared/cmd/migrate /src/shared/cmd/migrate

WORKDIR /src/shared
RUN --mount=type=cache,id=go-mod,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,id=go-build,target=/root/.cache/go-build,sharing=locked \
    go build -trimpath -buildvcs=false -ldflags="-s -w" -o /migrate ./cmd/migrate

FROM alpine:3.21

# BusyBox nc is already in Alpine and supports the entrypoint probe.
RUN apk add --no-cache --upgrade ca-certificates tzdata libcrypto3 libssl3

COPY --link --from=builder /migrate /usr/local/bin/migrate
COPY scripts/migrations /migrations
COPY scripts/docker/migrate/entrypoint.sh /entrypoint.sh

RUN sed -i 's/\r$//' /entrypoint.sh && chmod +x /entrypoint.sh

WORKDIR /migrations

ENTRYPOINT ["/entrypoint.sh"]
CMD ["up"]
