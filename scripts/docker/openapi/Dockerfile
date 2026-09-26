# Builds gen-openapi and serves Swagger UI, regenerating the spec on every start.
FROM golang:1.25-alpine AS builder

ENV CGO_ENABLED=0
WORKDIR /src

COPY scripts/gen-openapi/go.mod scripts/gen-openapi/go.sum ./
RUN go mod download

COPY scripts/gen-openapi/ ./
RUN go build -trimpath -ldflags="-s -w" -o /gen-openapi .

FROM swaggerapi/swagger-ui:v5.18.2

COPY --from=builder /gen-openapi /usr/local/bin/gen-openapi
COPY scripts/docker/openapi/entrypoint.sh /entrypoint.sh

RUN sed -i 's/\r$//' /entrypoint.sh && chmod +x /entrypoint.sh /usr/local/bin/gen-openapi

ENTRYPOINT ["/bin/sh", "/entrypoint.sh"]
