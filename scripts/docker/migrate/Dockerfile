# One-shot Laravel-style SQL migrator (shared/cmd/migrate).
FROM docker.arvancloud.ir/golang:1.25-alpine AS builder

ENV CGO_ENABLED=0
WORKDIR /src

COPY shared/go.mod shared/go.sum ./
RUN go mod download

COPY shared/pkg ./pkg
COPY shared/cmd/migrate ./cmd/migrate

RUN go build -trimpath -ldflags="-s -w" -o /migrate ./cmd/migrate

FROM docker.arvancloud.ir/alpine:3.21

RUN apk add --no-cache ca-certificates tzdata busybox-extras

COPY --from=builder /migrate /usr/local/bin/migrate
COPY scripts/migrations /migrations
COPY scripts/docker/migrate/entrypoint.sh /entrypoint.sh

RUN sed -i 's/\r$//' /entrypoint.sh && chmod +x /entrypoint.sh

WORKDIR /migrations

ENTRYPOINT ["/entrypoint.sh"]
CMD ["up"]
