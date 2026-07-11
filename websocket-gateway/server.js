// Load config.env for local development (Docker injects env via env_file)
require('dotenv').config({ path: 'config.env' });

const express = require('express');
const http = require('http');
const socketIo = require('socket.io');
const Redis = require('ioredis');
const grpc = require('@grpc/grpc-js');
const protoLoader = require('@grpc/proto-loader');
const { Sentry, initSentry, captureException, flush } = require('./sentry');

const app = express();
initSentry('websocket-gateway', app);
const server = http.createServer(app);

// Socket.IO configuration with CORS
const io = socketIo(server, {
  cors: {
    origin: process.env.CORS_ORIGIN || '*',
    methods: ['GET', 'POST'],
    credentials: true
  },
  transports: ['websocket', 'polling']
});

// Redis clients
const redis = new Redis(process.env.REDIS_URL || 'redis://localhost:6379');
const subscriber = redis.duplicate();

// Auth service client for token validation
const packageDefinition = protoLoader.loadSync(
  __dirname + '/shared/proto/auth.proto',
  {
    keepCase: true,
    longs: String,
    enums: String,
    defaults: true,
    oneofs: true
  }
);

const authProto = grpc.loadPackageDefinition(packageDefinition).auth;
const authClient = new authProto.AuthService(
  process.env.AUTH_SERVICE_ADDR || 'localhost:50051',
  grpc.credentials.createInsecure()
);

// User socket mapping
const userSockets = new Map(); // userId -> Set of socket IDs

// Authentication middleware
io.use(async (socket, next) => {
  const token = socket.handshake.auth.token;
  
  if (!token) {
    return next(new Error('Authentication error: No token provided'));
  }
  
  // Validate token via Auth service
  authClient.ValidateToken({ token }, (err, response) => {
    if (err || !response || !response.valid) {
      console.error('Token validation failed:', err || 'Invalid token');
      return next(new Error('Authentication error: Invalid token'));
    }
    
    socket.userId = response.user_id;
    socket.username = response.username || 'Unknown';
    console.log(`Token validated for user ${socket.userId}`);
    next();
  });
});

// Connection handling
io.on('connection', (socket) => {
  console.log(`User ${socket.userId} (${socket.username}) connected: ${socket.id}`);
  
  // Add to user socket mapping
  if (!userSockets.has(socket.userId)) {
    userSockets.set(socket.userId, new Set());
  }
  userSockets.get(socket.userId).add(socket.id);
  
  // Join user-specific room
  socket.join(`user:${socket.userId}`);
  
  // Send welcome message
  socket.emit('connected', {
    message: 'Connected to MetaRGB WebSocket Gateway',
    userId: socket.userId,
    timestamp: new Date().toISOString()
  });
  
  // Handle custom events from client
  socket.on('ping', () => {
    socket.emit('pong', { timestamp: Date.now() });
  });
  
  // Handle disconnection
  socket.on('disconnect', () => {
    console.log(`User ${socket.userId} disconnected: ${socket.id}`);
    const sockets = userSockets.get(socket.userId);
    if (sockets) {
      sockets.delete(socket.id);
      if (sockets.size === 0) {
        userSockets.delete(socket.userId);
      }
    }
  });
  
  // Handle errors
  socket.on('error', (error) => {
    console.error(`Socket error for user ${socket.userId}:`, error);
    captureException(error);
  });
});

// Redis pub/sub subscriptions
subscriber.subscribe('user-status', 'feature-status', 'notifications', (err, count) => {
  if (err) {
    console.error('Failed to subscribe to Redis channels:', err);
    captureException(err);
  } else {
    console.log(`Subscribed to ${count} Redis channels`);
  }
});

subscriber.on('message', (channel, message) => {
  try {
    const data = JSON.parse(message);
    
    switch (channel) {
      case 'user-status':
        // Broadcast user status change to the specific user
        if (data.user_id) {
          console.log(`Broadcasting user-status to user:${data.user_id}`);
          io.to(`user:${data.user_id}`).emit('user-status-changed', data);
        }
        break;
        
      case 'feature-status':
        // Broadcast feature ownership change to involved users
        console.log('Broadcasting feature-status change');
        if (data.old_owner_id) {
          io.to(`user:${data.old_owner_id}`).emit('feature-status-changed', {
            ...data,
            userType: 'old_owner'
          });
        }
        if (data.new_owner_id) {
          io.to(`user:${data.new_owner_id}`).emit('feature-status-changed', {
            ...data,
            userType: 'new_owner'
          });
        }
        break;
        
      case 'notifications':
        // Send notification to specific user
        if (data.user_id) {
          console.log(`Sending notification to user:${data.user_id}`);
          io.to(`user:${data.user_id}`).emit('notification-received', {
            id: data.id,
            type: data.type,
            title: data.title,
            message: data.message,
            data: data.data || {},
            created_at: data.created_at,
            timestamp: new Date().toISOString()
          });
        }
        break;
        
      default:
        console.log(`Unknown channel: ${channel}`);
    }
  } catch (error) {
    console.error(`Error processing message from ${channel}:`, error);
    captureException(error);
  }
});

subscriber.on('error', (error) => {
  console.error('Redis subscriber error:', error);
  captureException(error);
});

redis.on('error', (error) => {
  console.error('Redis client error:', error);
  captureException(error);
});

// Health check endpoint
app.get('/health', (req, res) => {
  res.json({
    status: 'healthy',
    connections: io.engine.clientsCount,
    users: userSockets.size,
    timestamp: new Date().toISOString()
  });
});

// Metrics endpoint
app.get('/metrics', (req, res) => {
  const metrics = {
    totalConnections: io.engine.clientsCount,
    totalUsers: userSockets.size,
    usersList: Array.from(userSockets.keys()),
    uptime: process.uptime(),
    memory: process.memoryUsage(),
    timestamp: new Date().toISOString()
  };
  res.json(metrics);
});

Sentry.setupExpressErrorHandler(app);

// Start server
const PORT = process.env.PORT || 3000;
server.listen(PORT, () => {
  console.log(`WebSocket gateway listening on port ${PORT}`);
  console.log(`Redis URL: ${process.env.REDIS_URL || 'redis://localhost:6379'}`);
  console.log(`Auth Service: ${process.env.AUTH_SERVICE_ADDR || 'localhost:50051'}`);
  console.log(`CORS Origin: ${process.env.CORS_ORIGIN || '*'}`);
});

async function shutdown(signal) {
  console.log(`${signal} signal received: closing HTTP server`);
  server.close(async () => {
    console.log('HTTP server closed');
    redis.quit();
    subscriber.quit();
    await flush(2000);
    process.exit(0);
  });
}

// Graceful shutdown
process.on('SIGTERM', () => {
  shutdown('SIGTERM');
});

process.on('SIGINT', () => {
  shutdown('SIGINT');
});
