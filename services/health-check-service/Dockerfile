# syntax=docker.arvancloud.ir/docker/dockerfile:1

# Health Check Service Dockerfile
FROM docker.arvancloud.ir/golang:1.26.10-alpine AS builder

ENV CGO_ENABLED=0
ENV GOPROXY=https://mirror.kargadan.ir/nexus/repository/go-group/,https://goproxy.cn,direct
ENV GOSUMDB=off
WORKDIR /src

COPY services/health-check-service/go.mod services/health-check-service/go.sum* ./
RUN --mount=type=cache,id=go-mod,target=/go/pkg/mod,sharing=locked \
    go mod download

COPY services/health-check-service/main.go ./
RUN --mount=type=cache,id=go-mod,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,id=go-build,target=/root/.cache/go-build,sharing=locked \
    go build -trimpath -buildvcs=false -ldflags="-s -w" -o /app/health-check-service main.go

FROM docker.arvancloud.ir/alpine:3.21

# Alpine already includes BusyBox wget, which accepts the healthcheck flags.
RUN sed -i 's#https://dl-cdn.alpinelinux.org/alpine#https://mirror.arvancloud.ir/alpine#g' /etc/apk/repositories \
 && apk add --no-cache --upgrade ca-certificates tzdata libcrypto3 libssl3 \
    && addgroup -g 1000 appuser \
    && adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --link --from=builder --chown=1000:1000 /app/health-check-service /app/health-check-service

USER appuser

EXPOSE 8090

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8090/live || exit 1

ENTRYPOINT ["./health-check-service"]
