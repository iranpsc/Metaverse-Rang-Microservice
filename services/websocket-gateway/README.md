# WebSocket Gateway (Go)

Real-time event broadcasting gateway for the MetaRGB microservices architecture using Socket.IO and Redis Pub/Sub.

## Features

- Socket.IO server (`github.com/googollee/go-socket.io`)
- Sanctum token validation via auth-service gRPC
- Redis pub/sub channels: `user-status`, `feature-status`, `notifications` (also accepts legacy `user-status-changed` / `feature-events`)
- Health (`/health`) and metrics (`/metrics`) endpoints
- CORS via `CORS_ORIGIN`

## Configuration

See `config.env.sample`.

## Docker

Built from `services/websocket-gateway/Dockerfile` and exposed on port `3002` via docker-compose.

## Client usage

Use `socket.io-client@2.x` (Engine.IO 3) to match `go-socket.io` v1.7:

```javascript
import io from 'socket.io-client';

const socket = io('http://localhost:3002', {
  path: '/socket.io/',
  transports: ['websocket', 'polling'],
  query: { token: 'your-sanctum-token' },
});

socket.on('connected', (data) => {
  console.log('connected', data.userId);
});

socket.on('feature-status-changed', (payload) => {
  // { id, rgb, ... }
});

socket.on('user-status-changed', (payload) => {
  // { user_id|id, online, ... }
});

socket.on('notification-received', (payload) => {
  // notification payload
});
```

Authentication requires a Sanctum token via `?token=` query parameter (or `Authorization: Bearer` header for non-browser clients).
On connect the client is joined to `user:{id}` and the public `feature-status` room automatically.
