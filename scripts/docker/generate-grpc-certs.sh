#!/bin/sh
# Populate the grpc_certs Docker volume for inter-service gRPC TLS/mTLS.
# Idempotent: skips when GRPC_TLS_ENABLED is not true or certificates already exist.
set -eu

OUT_DIR="/certs/grpc"
HOST_DIR="/host-certs/grpc"

fix_cert_permissions() {
  dir="$1"
  chmod 644 "$dir"/*.crt
  chmod 640 "$dir"/*.key
  chown -R 1000:1000 "$dir"
}

if [ "${GRPC_TLS_ENABLED:-false}" != "true" ]; then
  echo "gRPC TLS disabled, skipping certificate setup"
  exit 0
fi

if [ -f "$OUT_DIR/server.crt" ] && [ -f "$OUT_DIR/server.key" ] && [ -f "$OUT_DIR/ca.crt" ]; then
  echo "gRPC TLS certificates already present in $OUT_DIR"
  fix_cert_permissions "$OUT_DIR"
  exit 0
fi

if [ -f "$HOST_DIR/server.crt" ] && [ -f "$HOST_DIR/server.key" ] && [ -f "$HOST_DIR/ca.crt" ]; then
  mkdir -p "$OUT_DIR"
  cp "$HOST_DIR/ca.crt" "$HOST_DIR/ca.key" "$HOST_DIR/server.crt" "$HOST_DIR/server.key" \
    "$HOST_DIR/client.crt" "$HOST_DIR/client.key" "$OUT_DIR/"
  fix_cert_permissions "$OUT_DIR"
  echo "Copied gRPC TLS certificates from $HOST_DIR to $OUT_DIR"
  exit 0
fi

if ! command -v openssl >/dev/null 2>&1; then
  sed -i 's#https://dl-cdn.alpinelinux.org/alpine#https://mirror.arvancloud.ir/alpine#g' /etc/apk/repositories
  apk add --no-cache openssl
fi

mkdir -p "$OUT_DIR"

CA_KEY="$OUT_DIR/ca.key"
CA_CERT="$OUT_DIR/ca.crt"
SERVER_KEY="$OUT_DIR/server.key"
SERVER_CERT="$OUT_DIR/server.crt"
CLIENT_KEY="$OUT_DIR/client.key"
CLIENT_CERT="$OUT_DIR/client.crt"
SAN_FILE="$OUT_DIR/server-san.ext"

openssl genrsa -out "$CA_KEY" 4096
openssl req -x509 -new -nodes -key "$CA_KEY" -sha256 -days 3650 \
  -subj "/CN=metarang-grpc-ca" -out "$CA_CERT"

openssl genrsa -out "$SERVER_KEY" 4096
openssl req -new -key "$SERVER_KEY" -subj "/CN=metarang-grpc-server" -out "$OUT_DIR/server.csr"
printf '%s\n' \
  'subjectAltName=DNS:localhost,DNS:*.metarang-network,DNS:auth-service,DNS:commercial-service,DNS:features-service,DNS:levels-service,DNS:dynasty-service,DNS:financial-service,DNS:notifications-service,DNS:calendar-service,DNS:support-service,DNS:training-service,DNS:social-service,DNS:storage-service,DNS:grpc-gateway' \
  > "$SAN_FILE"
openssl x509 -req -in "$OUT_DIR/server.csr" -CA "$CA_CERT" -CAkey "$CA_KEY" -CAcreateserial \
  -out "$SERVER_CERT" -days 825 -sha256 -extfile "$SAN_FILE"

openssl genrsa -out "$CLIENT_KEY" 4096
openssl req -new -key "$CLIENT_KEY" -subj "/CN=metarang-grpc-client" -out "$OUT_DIR/client.csr"
openssl x509 -req -in "$OUT_DIR/client.csr" -CA "$CA_CERT" -CAkey "$CA_KEY" -CAcreateserial \
  -out "$CLIENT_CERT" -days 825 -sha256

rm -f "$OUT_DIR/server.csr" "$OUT_DIR/client.csr" "$OUT_DIR/ca.srl" "$SAN_FILE"

fix_cert_permissions "$OUT_DIR"
echo "Generated gRPC TLS certificates in $OUT_DIR"
