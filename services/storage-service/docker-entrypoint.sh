#!/bin/sh
set -e

UPLOAD_DIR="${UPLOAD_DIR:-/app/uploads}"
TEMP_DIR="${TEMP_DIR:-/tmp/storage-chunks}"

# Bind mounts (common on Windows) can replace /app/uploads with a root-owned directory.
# Fix ownership when the container starts as root, then drop privileges.
if [ "$(id -u)" = "0" ]; then
	mkdir -p "$UPLOAD_DIR" "$TEMP_DIR"
	if ! chown -R appuser:appuser "$UPLOAD_DIR" "$TEMP_DIR" 2>/dev/null; then
		# chown may be unsupported on some bind mounts; ensure writability instead
		chmod -R 777 "$UPLOAD_DIR" "$TEMP_DIR" 2>/dev/null || true
	fi
	exec su-exec appuser "$@"
fi

mkdir -p "$UPLOAD_DIR" "$TEMP_DIR" 2>/dev/null || true
exec "$@"
