# Multi-stage build for gRPC Gateway Service
FROM docker.arvancloud.ir/golang:1.25-alpine AS builder

ENV CGO_ENABLED=0

RUN apk add --no-cache git make protoc protobuf-dev

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

WORKDIR /workspace

RUN mkdir -p /workspace/metarang/grpc-gateway /workspace/metarang/shared

COPY ./shared/proto/ /workspace/metarang/shared/proto/
COPY ./scripts/docker/generate-all-protos.sh /tmp/generate-all-protos.sh

ENV PATH="${PATH}:/root/go/bin"
RUN sed -i 's/\r$//' /tmp/generate-all-protos.sh && sh /tmp/generate-all-protos.sh /workspace/metarang/shared

COPY ./shared/go.mod ./shared/go.sum /workspace/metarang/shared/
COPY ./shared/pkg/ /workspace/metarang/shared/pkg/

COPY ./services/grpc-gateway/go.mod ./services/grpc-gateway/go.sum* /workspace/metarang/grpc-gateway/
COPY ./services/grpc-gateway/ /workspace/metarang/grpc-gateway/

WORKDIR /workspace
RUN echo 'go 1.25.12' > go.work && \
    printf '\nuse (\n\t./metarang/grpc-gateway\n\t./metarang/shared\n)\n' >> go.work

WORKDIR /workspace/metarang/grpc-gateway
RUN sed -i 's|replace metarang/shared => ../../shared|replace metarang/shared => /workspace/metarang/shared|g' go.mod && \
    go mod download

ENV GOWORK=/workspace/go.work
RUN go build -trimpath -ldflags="-s -w" -o /app/grpc-gateway ./cmd/server

FROM docker.arvancloud.ir/alpine:3.21

RUN apk add --no-cache ca-certificates tzdata wget && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder --chown=appuser:appuser /app/grpc-gateway .

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/grpc-gateway"]
