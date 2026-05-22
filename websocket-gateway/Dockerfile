# WebSocket Gateway Dockerfile
FROM docker.arvancloud.ir/node:18-alpine

# Iran-accessible Alpine package mirror (ArvanCloud)
RUN sed -i 's|https://dl-cdn.alpinelinux.org|https://mirror.arvancloud.ir|g' /etc/apk/repositories

WORKDIR /app

COPY ./websocket-gateway/package*.json ./

# Iranian npm mirrors (tried in order). Override one registry:
#   docker compose build --build-arg NPM_REGISTRY=https://mirror-npm.runflare.com websocket-gateway
ARG NPM_REGISTRY=
RUN set -e; \
    if [ -n "${NPM_REGISTRY}" ]; then \
      mirrors="${NPM_REGISTRY}"; \
    else \
      mirrors="https://package-mirror.liara.ir/repository/npm/ https://registry.npm.ir/ https://mirror-npm.runflare.com"; \
    fi; \
    success=0; \
    for reg in $mirrors; do \
      echo ">>> npm install via ${reg}"; \
      npm config set registry "${reg}"; \
      npm config set strict-ssl false; \
      npm config set fetch-retries 3; \
      npm config set fetch-retry-mintimeout 10000; \
      npm config set fetch-retry-maxtimeout 60000; \
      rm -rf node_modules package-lock.json 2>/dev/null || true; \
      if npm install --omit=dev --no-audit --no-fund; then \
        echo ">>> succeeded: ${reg}"; \
        success=1; \
        break; \
      fi; \
      echo ">>> failed: ${reg}"; \
    done; \
    if [ "$success" -ne 1 ]; then \
      echo "ERROR: npm install failed on all registries"; \
      exit 1; \
    fi

COPY ./websocket-gateway/ .
COPY ./shared/proto ./shared/proto

EXPOSE 3002

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD node -e "require('http').get('http://localhost:3002/health', (r) => process.exit(r.statusCode === 200 ? 0 : 1))"

CMD ["node", "server.js"]
