#!/bin/sh
# Regenerates openapi/openapi.yaml from routes.yaml, patches servers with APP_URL,
# then starts Swagger UI. Runs on every container start.
set -eu

APP_URL="${APP_URL:-http://localhost:8000}"
APP_URL="${APP_URL%/}"

REPO_ROOT="${REPO_ROOT:-/workspace}"
HTML_DIR="/usr/share/nginx/html"
SRC_DIR="${REPO_ROOT}/openapi"

if [ ! -f "${SRC_DIR}/routes.yaml" ]; then
	echo "openapi: missing ${SRC_DIR}/routes.yaml" >&2
	exit 1
fi

echo "openapi: regenerating OpenAPI specification..."
(
	cd "${REPO_ROOT}"
	APP_URL="${APP_URL}" gen-openapi
)

mkdir -p "${HTML_DIR}/openapi"

if [ "$APP_URL" != "http://localhost:8000" ]; then
	awk -v app_url="$APP_URL" '
		/^servers:/ {
			print
			print "    - description: API Gateway"
			print "      url: " app_url
			in_servers = 1
			next
		}
		in_servers && /^[^ \t]/ {
			in_servers = 0
		}
		in_servers {
			next
		}
		{ print }
	' "${SRC_DIR}/openapi.yaml" > "${HTML_DIR}/openapi/openapi.yaml"
else
	cp "${SRC_DIR}/openapi.yaml" "${HTML_DIR}/openapi/openapi.yaml"
fi

# Stock Swagger UI initializer + Docker configurator (do not overwrite initializer.js).
export URL="${URL:-/openapi/openapi.yaml}"

echo "openapi: Swagger UI ready (APP_URL=${APP_URL})"
cd /
exec /docker-entrypoint.sh nginx -g "daemon off;"
