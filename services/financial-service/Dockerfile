# Multi-stage build for Financial Service
FROM golang:1.24-alpine AS builder

ENV CGO_ENABLED=0

RUN apk add --no-cache git make protoc protobuf-dev

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

WORKDIR /workspace

RUN mkdir -p /workspace/metargb/financial-service /workspace/metargb/shared

COPY ./shared/proto/ /workspace/metargb/shared/proto/

ENV PATH="${PATH}:/root/go/bin"
RUN mkdir -p /workspace/metargb/shared/pb/financial && \
    protoc --go_out=/workspace/metargb/shared/pb/financial --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/financial --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/financial.proto && \
    mkdir -p /workspace/metargb/shared/pb/auth && \
    protoc --go_out=/workspace/metargb/shared/pb/auth --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/auth --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/auth.proto

COPY ./shared/go.mod /workspace/metargb/shared/
COPY ./services/financial-service/go.mod ./services/financial-service/go.sum* /workspace/metargb/financial-service/

WORKDIR /workspace
RUN echo 'go 1.24.0' > go.work && \
    printf '\nuse (\n\t./metargb/financial-service\n\t./metargb/shared\n)\n' >> go.work

WORKDIR /workspace/metargb/financial-service
RUN sed -i 's|replace metargb/shared => ../../shared|replace metargb/shared => /workspace/metargb/shared|g' go.mod || true && \
    go mod download

COPY ./shared/ /workspace/metargb/shared/
COPY ./services/financial-service/ /workspace/metargb/financial-service/

WORKDIR /workspace/metargb/financial-service
RUN sed -i 's|replace metargb/shared => ../../shared|replace metargb/shared => /workspace/metargb/shared|g' go.mod || true

ENV GOWORK=/workspace/go.work
RUN go build -trimpath -ldflags="-s -w" -o /app/financial-service ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata netcat-openbsd && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder --chown=appuser:appuser /app/financial-service .

USER appuser

EXPOSE 50062

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD nc -z localhost 50062 || exit 1

ENTRYPOINT ["/app/financial-service"]
