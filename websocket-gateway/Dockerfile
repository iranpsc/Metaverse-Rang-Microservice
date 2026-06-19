# WebSocket Gateway - multi-stage build
FROM node:18-alpine AS deps

WORKDIR /app

COPY websocket-gateway/package*.json ./
RUN npm install --omit=dev --no-audit --no-fund

FROM node:18-alpine

WORKDIR /app

COPY --from=deps --chown=node:node /app/node_modules ./node_modules
COPY --chown=node:node websocket-gateway/ .
COPY --chown=node:node shared/proto ./shared/proto

USER node

EXPOSE 3002

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD node -e "require('http').get('http://localhost:3002/health', (r) => process.exit(r.statusCode === 200 ? 0 : 1))"

CMD ["node", "server.js"]
