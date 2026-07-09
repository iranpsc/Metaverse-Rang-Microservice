package auth

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ClientDialOptions returns default insecure dial options with internal service credentials.
func ClientDialOptions() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(UnaryClientInterceptor()),
	}
}

// DialAuthService connects to auth-service and returns a token validator.
func DialAuthService(addr string) (*grpc.ClientConn, TokenValidator, error) {
	conn, err := grpc.NewClient(addr, ClientDialOptions()...)
	if err != nil {
		return nil, nil, fmt.Errorf("connect auth service: %w", err)
	}
	return conn, NewAuthServiceTokenValidator(conn), nil
}

// ServerInterceptors builds the standard unary interceptor chain for gRPC servers.
func ServerInterceptors(metricsInterceptor grpc.UnaryServerInterceptor, validator TokenValidator) []grpc.UnaryServerInterceptor {
	chain := []grpc.UnaryServerInterceptor{metricsInterceptor}
	if validator != nil {
		chain = append(chain, UnaryServerInterceptor(validator))
	}
	return chain
}
