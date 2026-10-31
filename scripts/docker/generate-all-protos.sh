#!/bin/sh
set -eu

SHARED_DIR="${1:-/workspace/metarang/shared}"
PROTO_DIR="${SHARED_DIR}/proto"
PB_DIR="${SHARED_DIR}/pb"

mkdir -p "${PB_DIR}"

generate_go() {
	name="$1"
	mkdir -p "${PB_DIR}/${name}"
	protoc --go_out="${PB_DIR}/${name}" --go_opt=paths=source_relative \
		-I="${PROTO_DIR}" "${PROTO_DIR}/${name}.proto"
}

generate_grpc() {
	name="$1"
	mkdir -p "${PB_DIR}/${name}"
	protoc --go_out="${PB_DIR}/${name}" --go_opt=paths=source_relative \
		--go-grpc_out="${PB_DIR}/${name}" --go-grpc_opt=paths=source_relative \
		-I="${PROTO_DIR}" "${PROTO_DIR}/${name}.proto"
}

generate_go common

for name in auth calendar commercial dynasty features financial levels notifications social storage support training; do
	generate_grpc "${name}"
done

protoc --go_out="${PB_DIR}/commercial" --go_opt=paths=source_relative \
 --go-grpc_out="${PB_DIR}/commercial" --go-grpc_opt=paths=source_relative \
 -I="${PROTO_DIR}" "${PROTO_DIR}/commercial_rates.proto"
