# Multi-stage build for Training Service
FROM golang:1.24-alpine AS builder

ENV CGO_ENABLED=0

RUN apk add --no-cache git make protoc protobuf-dev

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.11 && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1

WORKDIR /workspace

RUN mkdir -p /workspace/metargb/training-service /workspace/metargb/shared

COPY ./shared/go.mod /workspace/metargb/shared/
COPY ./shared/go.sum /workspace/metargb/shared/
COPY ./shared/proto/ /workspace/metargb/shared/proto/
COPY ./shared/pkg/ /workspace/metargb/shared/pkg/
COPY ./shared/pb/ /workspace/metargb/shared/pb/

ENV PATH="${PATH}:/root/go/bin"
RUN protoc --go_out=/workspace/metargb/shared/pb/training --go_opt=paths=source_relative \
        --go-grpc_out=/workspace/metargb/shared/pb/training --go-grpc_opt=paths=source_relative \
        -I=/workspace/metargb/shared/proto /workspace/metargb/shared/proto/training.proto

COPY ./services/training-service/go.mod ./services/training-service/go.sum* /workspace/metargb/training-service/
COPY ./services/training-service/ /workspace/metargb/training-service/

WORKDIR /workspace
RUN echo 'go 1.24.0' > go.work && \
    printf '\nuse (\n\t./metargb/training-service\n\t./metargb/shared\n)\n' >> go.work

WORKDIR /workspace/metargb/training-service
RUN sed -i 's|replace metargb/shared => ../../shared|replace metargb/shared => /workspace/metargb/shared|g' go.mod || true && \
    go get metargb/shared@v0.0.0 || true && \
    GOSUMDB=off go mod download

ENV GOWORK=/workspace/go.work
RUN go build -trimpath -ldflags="-s -w" -o /app/training-service ./cmd/server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata netcat-openbsd && \
    addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

COPY --from=builder --chown=appuser:appuser /app/training-service .

USER appuser

EXPOSE 50057

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD nc -z localhost 50057 || exit 1

ENTRYPOINT ["/app/training-service"]
