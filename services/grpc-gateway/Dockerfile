# Multi-stage build for gRPC Gateway Service
FROM golang:1.24-alpine AS builder

ENV CGO_ENABLED=0

RUN apk add --no-cache git make protoc protobuf-dev

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

WORKDIR /workspace

RUN mkdir -p /workspace/metargb/grpc-gateway /workspace/metargb/shared

COPY ./shared/proto/ /workspace/metargb/shared/proto/

ENV PATH="${PATH}:/root/go/bin"
RUN mkdir -p /workspace/metargb/shared/pb/auth && \
    protoc --go_out=/workspace/metargb/shared/pb/auth --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/auth --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/auth.proto && \
    mkdir -p /workspace/metargb/shared/pb/common && \
    protoc --go_out=/workspace/metargb/shared/pb/common --go_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/common.proto && \
    mkdir -p /workspace/metargb/shared/pb/dynasty && \
    protoc --go_out=/workspace/metargb/shared/pb/dynasty --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/dynasty --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/dynasty.proto && \
    mkdir -p /workspace/metargb/shared/pb/levels && \
    protoc --go_out=/workspace/metargb/shared/pb/levels --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/levels --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/levels.proto && \
    mkdir -p /workspace/metargb/shared/pb/training && \
    protoc --go_out=/workspace/metargb/shared/pb/training --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/training --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/training.proto && \
    mkdir -p /workspace/metargb/shared/pb/features && \
    protoc --go_out=/workspace/metargb/shared/pb/features --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/features --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/features.proto && \
    mkdir -p /workspace/metargb/shared/pb/financial && \
    protoc --go_out=/workspace/metargb/shared/pb/financial --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/financial --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/financial.proto && \
    mkdir -p /workspace/metargb/shared/pb/commercial && \
    protoc --go_out=/workspace/metargb/shared/pb/commercial --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/commercial --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/commercial.proto && \
    mkdir -p /workspace/metargb/shared/pb/social && \
    protoc --go_out=/workspace/metargb/shared/pb/social --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/social --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/social.proto && \
    mkdir -p /workspace/metargb/shared/pb/notifications && \
    protoc --go_out=/workspace/metargb/shared/pb/notifications --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/notifications --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/notifications.proto && \
    mkdir -p /workspace/metargb/shared/pb/support && \
    protoc --go_out=/workspace/metargb/shared/pb/support --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/support --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/support.proto && \
    mkdir -p /workspace/metargb/shared/pb/calendar && \
    protoc --go_out=/workspace/metargb/shared/pb/calendar --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/calendar --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/calendar.proto

COPY ./shared/go.mod /workspace/metargb/shared/
COPY ./shared/go.sum /workspace/metargb/shared/
COPY ./shared/pkg/ /workspace/metargb/shared/pkg/

COPY ./services/grpc-gateway/go.mod ./services/grpc-gateway/go.sum* /workspace/metargb/grpc-gateway/
COPY ./services/grpc-gateway/ /workspace/metargb/grpc-gateway/

WORKDIR /workspace
RUN echo 'go 1.24.0' > go.work && \
    printf '\nuse (\n\t./metargb/grpc-gateway\n\t./metargb/shared\n)\n' >> go.work

WORKDIR /workspace/metargb/grpc-gateway
RUN sed -i 's|replace metargb/shared => ../../shared|replace metargb/shared => /workspace/metargb/shared|g' go.mod && \
    go mod download

ENV GOWORK=/workspace/go.work
RUN go build -trimpath -ldflags="-s -w" -o /app/grpc-gateway ./cmd/server

FROM alpine:3.21

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
