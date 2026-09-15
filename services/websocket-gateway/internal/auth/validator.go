// Package auth provides token validation for WebSocket connections.
package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	pb "metarang/shared/pb/auth"
	grpcutil "metarang/shared/pkg/grpc"
)

// Validator validates Sanctum tokens via auth-service.
type Validator struct {
	client     pb.AuthServiceClient
	httpClient *http.Client
	httpBase   string
}

// NewValidator dials auth-service and returns a token validator.
// grpcAddr is host:port for gRPC (e.g. auth-service:50051).
// Optional AUTH_SERVICE_HTTP_URL enables HTTP fallback (default derived from gRPC host:8066).
func NewValidator(ctx context.Context, grpcAddr string) (*Validator, error) {
	_ = ctx
	conn, err := grpcutil.DialContextWithTimeout(grpcAddr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("dial auth service: %w", err)
	}

	httpBase := strings.TrimRight(envOr("AUTH_SERVICE_HTTP_URL", defaultHTTPBase(grpcAddr)), "/")

	return &Validator{
		client: pb.NewAuthServiceClient(conn),
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		httpBase: httpBase,
	}, nil
}

// ValidateToken checks whether the token is valid and returns the user ID.
func (v *Validator) ValidateToken(ctx context.Context, token string) (uint64, error) {
	grpcCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	resp, err := v.client.ValidateToken(grpcCtx, &pb.ValidateTokenRequest{Token: token})
	if err == nil {
		if resp == nil || !resp.Valid {
			return 0, fmt.Errorf("invalid token")
		}
		return resp.UserId, nil
	}

	userID, httpErr := v.validateTokenHTTP(ctx, token)
	if httpErr != nil {
		return 0, fmt.Errorf("grpc: %v; http: %w", err, httpErr)
	}
	return userID, nil
}

func (v *Validator) validateTokenHTTP(ctx context.Context, token string) (uint64, error) {
	if v.httpBase == "" {
		return 0, fmt.Errorf("http fallback not configured")
	}

	body, err := json.Marshal(map[string]string{"token": token})
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.httpBase+"/api/auth/validate", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := v.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()

	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return 0, err
	}
	if res.StatusCode >= 300 {
		return 0, fmt.Errorf("status %d: %s", res.StatusCode, string(raw))
	}

	var parsed struct {
		Data struct {
			Valid  bool   `json:"valid"`
			UserID uint64 `json:"user_id"`
		} `json:"data"`
		Valid  bool   `json:"valid"`
		UserID uint64 `json:"user_id"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return 0, err
	}

	valid := parsed.Data.Valid || parsed.Valid
	userID := parsed.Data.UserID
	if userID == 0 {
		userID = parsed.UserID
	}
	if !valid || userID == 0 {
		return 0, fmt.Errorf("invalid token")
	}
	return userID, nil
}

func defaultHTTPBase(grpcAddr string) string {
	host := grpcAddr
	if i := strings.LastIndex(grpcAddr, ":"); i > 0 {
		host = grpcAddr[:i]
	}
	if host == "" {
		return "http://auth-service:8066"
	}
	return "http://" + host + ":8066"
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
