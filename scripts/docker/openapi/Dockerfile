# syntax=docker/dockerfile:1

# Builds gen-openapi and serves Swagger UI, regenerating the spec on every start.
FROM golang:1.25-alpine AS builder

ENV CGO_ENABLED=0
WORKDIR /src

COPY scripts/gen-openapi/go.mod scripts/gen-openapi/go.sum ./
RUN --mount=type=cache,id=go-mod,target=/go/pkg/mod,sharing=locked \
    go mod download

COPY scripts/gen-openapi/ ./
RUN --mount=type=cache,id=go-mod,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,id=go-build,target=/root/.cache/go-build,sharing=locked \
    go build -trimpath -buildvcs=false -ldflags="-s -w" -o /gen-openapi .

FROM swaggerapi/swagger-ui:v5.18.2

COPY --link --from=builder /gen-openapi /usr/local/bin/gen-openapi
COPY scripts/docker/openapi/entrypoint.sh /entrypoint.sh

RUN sed -i 's/\r$//' /entrypoint.sh && chmod +x /entrypoint.sh /usr/local/bin/gen-openapi

ENTRYPOINT ["/bin/sh", "/entrypoint.sh"]
