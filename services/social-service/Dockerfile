# Multi-stage build for Social Service
FROM docker.arvancloud.ir/golang:1.25-alpine AS builder

ENV CGO_ENABLED=0

RUN apk add --no-cache git make protoc protobuf-dev

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

WORKDIR /workspace

RUN mkdir -p /workspace/metarang/social-service /workspace/metarang/shared

COPY ./shared/proto/ /workspace/metarang/shared/proto/
COPY ./scripts/docker/generate-all-protos.sh /tmp/generate-all-protos.sh

ENV PATH="${PATH}:/root/go/bin"
RUN sed -i 's/\r$//' /tmp/generate-all-protos.sh && sh /tmp/generate-all-protos.sh /workspace/metarang/shared

COPY ./shared/go.mod ./shared/go.sum /workspace/metarang/shared/
COPY ./shared/pkg/ /workspace/metarang/shared/pkg/

COPY ./services/social-service/go.mod ./services/social-service/go.sum* /workspace/metarang/social-service/
COPY ./services/social-service/ /workspace/metarang/social-service/

WORKDIR /workspace
RUN echo 'go 1.25.12' > go.work && \
    printf '\nuse (\n\t./metarang/social-service\n\t./metarang/shared\n)\n' >> go.work

WORKDIR /workspace/metarang/social-service
RUN sed -i 's|replace metarang/shared => ../../shared|replace metarang/shared => /workspace/metarang/shared|g' go.mod || true && \
    go mod download

ENV GOWORK=/workspace/go.work
RUN go build -trimpath -ldflags="-s -w" -o /app/social-service ./cmd/server

FROM docker.arvancloud.ir/alpine:3.21

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
