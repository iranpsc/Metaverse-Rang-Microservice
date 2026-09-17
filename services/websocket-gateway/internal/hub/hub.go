// Package hub manages WebSocket connections and event broadcasting.
package hub

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zishang520/engine.io/v2/types"
	"github.com/zishang520/socket.io/v2/socket"
)

const (
	publicFeatureRoom = "feature-status"
	publicUserRoom    = "user-status"
)

// TokenValidator validates Sanctum tokens for incoming Socket.IO connections.
type TokenValidator interface {
	ValidateToken(ctx context.Context, token string) (uint64, error)
}

// connectionIdentity is stored on each socket after the auth middleware runs.
// Anonymous public clients have Authenticated=false and UserID=0.
type connectionIdentity struct {
	UserID        uint64
	Authenticated bool
}

// Hub manages Socket.IO connections and Redis-driven broadcasts.
type Hub struct {
	server  *socket.Server
	handler http.Handler
	users   map[uint64]map[string]struct{}
	mu      sync.RWMutex
	auth    TokenValidator
}

// New creates a Socket.IO v4 hub.
// Public rooms (feature-status, user-status) are open without a token.
// Private notifications require a valid Sanctum token (user:{id} room).
func New(validator TokenValidator, corsOrigins []string) *Hub {
	h := &Hub{
		users: make(map[uint64]map[string]struct{}),
		auth:  validator,
	}

	opts := socket.DefaultServerOptions()
	opts.SetCors(&types.Cors{
		Origin:  corsOriginOption(corsOrigins),
		Methods: []string{http.MethodGet, http.MethodPost},
		AllowedHeaders: []string{
			"Authorization",
			"Content-Type",
			"Accept",
			"Origin",
			"X-Requested-With",
		},
		Credentials:          true,
		OptionsSuccessStatus: http.StatusNoContent,
	})

	server := socket.NewServer(nil, opts)

	server.Use(func(client *socket.Socket, next func(*socket.ExtendedError)) {
		token := extractToken(client)
		if token == "" {
			// Anonymous clients may listen to public channels only.
			client.SetData(connectionIdentity{})
			next(nil)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		userID, err := h.auth.ValidateToken(ctx, token)
		if err != nil {
			log.Printf("token validation failed: %v", err)
			next(socket.NewExtendedError("unauthorized", map[string]any{"message": "unauthorized"}))
			return
		}

		client.SetData(connectionIdentity{UserID: userID, Authenticated: true})
		next(nil)
	})

	if err := server.On("connection", func(clients ...any) {
		client, ok := clients[0].(*socket.Socket)
		if !ok {
			return
		}

		identity, ok := client.Data().(connectionIdentity)
		if !ok {
			client.Disconnect(true)
			return
		}

		socketID := string(client.Id())
		rooms := []socket.Room{socket.Room(publicFeatureRoom), socket.Room(publicUserRoom)}
		if identity.Authenticated {
			h.track(identity.UserID, socketID)
			rooms = append(rooms, socket.Room(userRoom(identity.UserID)))
		}
		client.Join(rooms...)

		payload := map[string]any{
			"message":       "Connected to metarang WebSocket Gateway",
			"authenticated": identity.Authenticated,
			"timestamp":     time.Now().UTC().Format(time.RFC3339),
		}
		if identity.Authenticated {
			payload["userId"] = identity.UserID
		}
		_ = client.Emit("connected", payload)

		if err := client.On("client-ping", func(...any) {
			_ = client.Emit("client-pong", map[string]any{"timestamp": time.Now().UnixMilli()})
		}); err != nil {
			log.Printf("failed to register client-ping handler: %v", err)
		}
		// Keep legacy alias for older clients.
		if err := client.On("ping", func(...any) {
			_ = client.Emit("pong", map[string]any{"timestamp": time.Now().UnixMilli()})
		}); err != nil {
			log.Printf("failed to register ping handler: %v", err)
		}
		if identity.Authenticated {
			userID := identity.UserID
			if err := client.On("disconnect", func(...any) {
				h.untrack(userID, socketID)
			}); err != nil {
				log.Printf("failed to register disconnect handler: %v", err)
			}
		}
	}); err != nil {
		log.Printf("failed to register connection handler: %v", err)
	}

	h.server = server
	h.handler = server.ServeHandler(opts)
	return h
}

// Close shuts down the Socket.IO server.
func (h *Hub) Close() error {
	if h.server == nil {
		return nil
	}
	h.server.Close(nil)
	return nil
}

// ServeHTTP handles Socket.IO traffic.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.handler.ServeHTTP(w, r)
}

func extractToken(s *socket.Socket) string {
	if s == nil || s.Handshake() == nil {
		return ""
	}
	hs := s.Handshake()
	return tokenFromHandshake(hs.Query, hs.Auth, hs.Headers)
}

func tokenFromHandshake(query map[string][]string, auth any, headers map[string][]string) string {
	if token := firstValue(query, "token"); token != "" {
		return token
	}
	if token := tokenFromAuth(auth); token != "" {
		return token
	}
	if authHeader := firstHeader(headers, "Authorization"); authHeader != "" {
		return strings.TrimPrefix(strings.TrimSpace(authHeader), "Bearer ")
	}
	return ""
}

func tokenFromAuth(auth any) string {
	switch v := auth.(type) {
	case map[string]any:
		if token, ok := v["token"].(string); ok {
			return token
		}
	case map[string]string:
		return v["token"]
	}
	return ""
}

func firstValue(values map[string][]string, key string) string {
	if values == nil {
		return ""
	}
	if vals := values[key]; len(vals) > 0 {
		return vals[0]
	}
	return ""
}

func firstHeader(headers map[string][]string, key string) string {
	if headers == nil {
		return ""
	}
	for _, candidate := range []string{key, strings.ToLower(key), http.CanonicalHeaderKey(key)} {
		if vals := headers[candidate]; len(vals) > 0 && vals[0] != "" {
			return vals[0]
		}
	}
	return ""
}

func corsOriginOption(corsOrigins []string) any {
	origins := make([]any, 0, len(corsOrigins))
	allowAll := false
	for _, origin := range corsOrigins {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		if origin == "*" {
			allowAll = true
			continue
		}
		origins = append(origins, origin)
	}
	if allowAll || len(origins) == 0 {
		return true
	}
	if len(origins) == 1 {
		return origins[0]
	}
	return origins
}

func (h *Hub) track(userID uint64, socketID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.users[userID] == nil {
		h.users[userID] = make(map[string]struct{})
	}
	h.users[userID][socketID] = struct{}{}
}

func (h *Hub) untrack(userID uint64, socketID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	sockets, ok := h.users[userID]
	if !ok {
		return
	}
	delete(sockets, socketID)
	if len(sockets) == 0 {
		delete(h.users, userID)
	}
}

// Stats returns connection metrics for health endpoints.
func (h *Hub) Stats() (connections int, users int) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, sockets := range h.users {
		connections += len(sockets)
	}
	return connections, len(h.users)
}

// BroadcastUserStatus sends a user-status event to the public user-status room.
func (h *Hub) BroadcastUserStatus(data map[string]any) {
	if _, ok := numericID(data["user_id"]); !ok {
		if _, ok = numericID(data["id"]); !ok {
			return
		}
	}
	_ = h.server.To(socket.Room(publicUserRoom)).Emit("user-status-changed", data)
}

// BroadcastFeatureStatus sends feature status updates to map clients and involved owners.
func (h *Hub) BroadcastFeatureStatus(data map[string]any) {
	// Public map updates (id + rgb) go to every connected client.
	if _, hasID := numericID(data["id"]); hasID {
		_ = h.server.To(socket.Room(publicFeatureRoom)).Emit("feature-status-changed", data)
	}

	if oldOwner, ok := numericID(data["old_owner_id"]); ok {
		_ = h.server.To(socket.Room(userRoom(oldOwner))).Emit("feature-status-changed", merge(data, map[string]any{"userType": "old_owner"}))
	}
	if newOwner, ok := numericID(data["new_owner_id"]); ok {
		_ = h.server.To(socket.Room(userRoom(newOwner))).Emit("feature-status-changed", merge(data, map[string]any{"userType": "new_owner"}))
	}
}

// BroadcastNotification sends a notification to a user room.
func (h *Hub) BroadcastNotification(data map[string]any) {
	userID, ok := numericID(data["user_id"])
	if !ok {
		return
	}
	payload := map[string]any{
		"id":         data["id"],
		"type":       data["type"],
		"title":      data["title"],
		"message":    data["message"],
		"data":       data["data"],
		"created_at": data["created_at"],
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}
	_ = h.server.To(socket.Room(userRoom(userID))).Emit("notification-received", payload)
}

func userRoom(userID uint64) string {
	return "user:" + formatUint(userID)
}

func merge(base map[string]any, extra map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(extra))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func numericID(value any) (uint64, bool) {
	switch v := value.(type) {
	case float64:
		return uint64(v), true
	case json.Number:
		n, err := v.Int64()
		return uint64(n), err == nil
	case int:
		return uint64(v), true
	case int64:
		return uint64(v), true
	case uint64:
		return v, true
	default:
		return 0, false
	}
}

func formatUint(v uint64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
