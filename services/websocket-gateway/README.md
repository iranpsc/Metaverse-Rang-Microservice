# WebSocket Gateway (Go)

Real-time event broadcasting gateway for the MetaRGB microservices architecture using Socket.IO and Redis Pub/Sub.

## Features

- Socket.IO server (`github.com/googollee/go-socket.io`)
- Sanctum token validation via auth-service gRPC
- Redis pub/sub channels: `user-status`, `feature-status`, `notifications`
- Health (`/health`) and metrics (`/metrics`) endpoints

## Configuration

See `config.env.sample`.

## Docker

Built from `services/websocket-gateway/Dockerfile` and exposed on port `3002` via docker-compose.

## Client usage

```javascript
import io from 'socket.io-client';

const socket = io('http://localhost:3002', {
  auth: { token: 'your-sanctum-token' },
  transports: ['websocket', 'polling'],
});
```

If your Socket.IO client cannot send `auth.token`, pass the token as a query parameter instead:

```javascript
const socket = io('http://localhost:3002?token=your-sanctum-token');
```
