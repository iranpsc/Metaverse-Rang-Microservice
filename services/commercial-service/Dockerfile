# Multi-stage build for Commercial Service
FROM golang:1.25-alpine AS builder

ENV CGO_ENABLED=0

RUN apk add --no-cache git make protoc protobuf-dev

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

WORKDIR /workspace

RUN mkdir -p /workspace/metargb/commercial-service /workspace/metargb/shared

COPY ./shared/proto/ /workspace/metargb/shared/proto/

ENV PATH="${PATH}:/root/go/bin"
RUN mkdir -p /workspace/metargb/shared/pb/commercial && \
    protoc --go_out=/workspace/metargb/shared/pb/commercial --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/commercial --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/commercial.proto && \
    mkdir -p /workspace/metargb/shared/pb/auth && \
    protoc --go_out=/workspace/metargb/shared/pb/auth --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/auth --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/auth.proto

COPY ./shared/go.mod /workspace/metargb/shared/
COPY ./shared/pkg/ /workspace/metargb/shared/pkg/

COPY ./services/commercial-service/go.mod ./services/commercial-service/go.sum* /workspace/metargb/commercial-service/
COPY ./services/commercial-service/ /workspace/metargb/commercial-service/

WORKDIR /workspace
RUN echo 'go 1.24.0' > go.work && \
    printf '\nuse (\n\t./metargb/commercial-service\n\t./metargb/shared\n)\n' >> go.work

WORKDIR /workspace/metargb/commercial-service
RUN sed -i 's|replace metargb/shared => ../../shared|replace metargb/shared => /workspace/metargb/shared|g' go.mod && \
    go mod download

ENV GOWORK=/workspace/go.work
RUN go build -trimpath -ldflags="-s -w" -o /app/commercial-service ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata netcat-openbsd && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder --chown=appuser:appuser /app/commercial-service .
COPY --chown=appuser:appuser ./services/commercial-service/config.env.sample ./config.env

USER appuser

EXPOSE 50052

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD nc -z localhost 50052 || exit 1

ENTRYPOINT ["/app/commercial-service"]
