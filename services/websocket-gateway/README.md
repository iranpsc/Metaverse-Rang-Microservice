# WebSocket Gateway (Go)

Real-time event broadcasting gateway for the MetaRGB microservices architecture using Socket.IO v4 and Redis Pub/Sub.

## Features

- Socket.IO v4 server (`github.com/zishang520/socket.io/v2`, Engine.IO 4)
- Optional Sanctum token validation via auth-service gRPC (required only for private notifications)
- Public rooms without auth: `feature-status`, `user-status`
- Private room with auth: `user:{id}` (notifications)
- Redis pub/sub channels: `user-status`, `feature-status`, `notifications` (also accepts legacy `user-status-changed` / `feature-events`)
- Health (`/health`) and metrics (`/metrics`) endpoints
- Browser tester at `/` and `/tester`
- CORS via `CORS_ORIGIN`

## Configuration

See `config.env.sample`.

## Docker

Built from `services/websocket-gateway/Dockerfile` and exposed on port `3002` via docker-compose.

## Browser tester

Open `http://localhost:3002/` (also served at `/tester`) to try a connection.

- **WebSocket host** — Socket.IO base URL, for example `http://localhost:3002`
- **Channel** — `feature-status` and `user-status` are public. `notifications` is private and asks for a Sanctum token
- **Event** — Socket.IO event to print, such as `feature-status-changed`, `user-status-changed`, or `notification-received`

The gateway joins public rooms on connect. A valid token also joins `user:{id}` for private notifications. The page shows a success or failure alert and appends matching broadcasts below the form.

## Client usage

Use `socket.io-client@4.x` (Engine.IO 4) to match this gateway:

```javascript
import { io } from 'socket.io-client';

// Public channels only (no token)
const publicSocket = io('http://localhost:3002', {
  path: '/socket.io/',
  transports: ['websocket', 'polling'],
});

// Authenticated: public channels + private notifications
const privateSocket = io('http://localhost:3002', {
  path: '/socket.io/',
  transports: ['websocket', 'polling'],
  auth: { token: 'your-sanctum-token' },
  query: { token: 'your-sanctum-token' },
});

publicSocket.on('connected', (data) => {
  console.log('connected', data.authenticated); // false
});

publicSocket.on('feature-status-changed', (payload) => {
  // { id, rgb, ... }
});

publicSocket.on('user-status-changed', (payload) => {
  // { user_id|id, online, ... }
});

privateSocket.on('notification-received', (payload) => {
  // notification payload (auth required)
});
```

Authentication is optional. Without a token the client joins public rooms only.
With a valid Sanctum token (`auth.token`, `?token=`, or `Authorization: Bearer`) the client also joins `user:{id}` for private notifications.
Invalid tokens are still rejected.
