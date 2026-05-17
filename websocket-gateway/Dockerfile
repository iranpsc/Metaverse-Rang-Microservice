# WebSocket Gateway Dockerfile
FROM node:18-alpine

# Iran-accessible Alpine package mirror (MobinHost)
RUN sed -i 's|https://dl-cdn.alpinelinux.org|https://mirror.mobinhost.com|g' /etc/apk/repositories
# Set working directory
WORKDIR /app

# Copy package files
COPY ./websocket-gateway/package*.json ./

# Install dependencies (Iran-accessible npm mirror: Runflare)
RUN npm config set registry https://mirror-npm.runflare.com && \
    npm config set fetch-retries 5 && \
    npm config set fetch-retry-mintimeout 20000 && \
    npm config set fetch-retry-maxtimeout 120000 && \
    npm install --omit=dev

# Copy application files
COPY ./websocket-gateway/ .

# Copy shared proto files for gRPC client
COPY ./shared/proto ./shared/proto

# Expose WebSocket port
EXPOSE 3002

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD node -e "require('http').get('http://localhost:3002/health', (r) => process.exit(r.statusCode === 200 ? 0 : 1))"

# Start server
CMD ["node", "server.js"]

