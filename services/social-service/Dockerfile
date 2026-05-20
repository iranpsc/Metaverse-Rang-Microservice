# Multi-stage build for Social Service
FROM docker.arvancloud.ir/golang:1.24-alpine AS builder

# Iran-accessible Alpine package mirror (MobinHost)
RUN sed -i 's|https://dl-cdn.alpinelinux.org|https://mirror.mobinhost.com|g' /etc/apk/repositories
# Iran-accessible Go module proxy (ParsPack Abrha mirror)
ENV GOPROXY=https://mirror.abrha.net/repository/go/,direct
ENV GOSUMDB=off
RUN apk add --no-cache git

WORKDIR /workspace

RUN mkdir -p /workspace/metargb/social-service /workspace/metargb/shared

COPY ./shared/go.mod /workspace/metargb/shared/
COPY ./shared/ /workspace/metargb/shared/

COPY ./services/social-service/go.mod ./services/social-service/go.sum* /workspace/metargb/social-service/
COPY ./services/social-service/ /workspace/metargb/social-service/

WORKDIR /workspace
RUN echo 'go 1.24.0' > go.work && \
    echo '' >> go.work && \
    echo 'use (' >> go.work && \
    echo '    ./metargb/social-service' >> go.work && \
    echo '    ./metargb/shared' >> go.work && \
    echo ')' >> go.work

WORKDIR /workspace/metargb/social-service

RUN sed -i 's|replace metargb/shared => ../../shared|replace metargb/shared => /workspace/metargb/shared|g' go.mod || true

RUN go get metargb/shared@v0.0.0 || true

ENV GOPROXY=https://mirror.abrha.net/repository/go/,direct
ENV GOSUMDB=off
RUN go mod download

WORKDIR /workspace
ENV GOWORK=/workspace/go.work
RUN cd /workspace/metargb/social-service && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o /app/social-service ./cmd/server

FROM docker.arvancloud.ir/alpine:3.21

RUN sed -i 's|https://dl-cdn.alpinelinux.org|https://mirror.mobinhost.com|g' /etc/apk/repositories
RUN apk update && apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/social-service .

RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser && \
    chown -R appuser:appuser /app

USER appuser

EXPOSE 50061

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD nc -z localhost 50061 || exit 1

ENTRYPOINT ["/app/social-service"]
