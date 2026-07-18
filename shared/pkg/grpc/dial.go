// Package grpcutil provides shared gRPC client and server dial helpers.
package grpcutil

import (
	"context"
	"fmt"
	"time"

	"metarang/shared/pkg/auth"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// ClientDialOptions returns standard client dial options: TLS (when enabled) and service-auth propagation.
func ClientDialOptions(extra ...grpc.DialOption) ([]grpc.DialOption, error) {
	return clientDialOptions(nil, extra)
}

// ClientDialOptionsWithInterceptors returns dial options with additional client interceptors chained before service auth.
func ClientDialOptionsWithInterceptors(extraInterceptors ...grpc.UnaryClientInterceptor) ([]grpc.DialOption, error) {
	return clientDialOptions(extraInterceptors, nil)
}

func clientDialOptions(extraInterceptors []grpc.UnaryClientInterceptor, extra []grpc.DialOption) ([]grpc.DialOption, error) {
	creds, err := ClientCredentials()
	if err != nil {
		return nil, err
	}

	interceptors := append(extraInterceptors, serviceAuthClientInterceptor())
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		grpc.WithChainUnaryInterceptor(interceptors...),
	}
	opts = append(opts, extra...)
	return opts, nil
}

// NewClient creates a gRPC client connection with shared TLS and service-auth settings.
func NewClient(target string, extra ...grpc.DialOption) (*grpc.ClientConn, error) {
	opts, err := ClientDialOptions(extra...)
	if err != nil {
		return nil, err
	}
	return grpc.NewClient(target, opts...)
}

// DialContext dials a gRPC server with TLS and service-auth propagation.
func DialContext(ctx context.Context, target string, extra ...grpc.DialOption) (*grpc.ClientConn, error) {
	_ = ctx
	opts, err := ClientDialOptions(extra...)
	if err != nil {
		return nil, err
	}
	return grpc.NewClient(target, opts...)
}

// DialContextWithTimeout dials with the given timeout (blocking until connected or deadline).
func DialContextWithTimeout(target string, timeout time.Duration, extra ...grpc.DialOption) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return DialContext(ctx, target, extra...)
}

// ServerOptions returns gRPC server options including TLS credentials when enabled.
func ServerOptions(extra ...grpc.ServerOption) ([]grpc.ServerOption, error) {
	creds, err := ServerCredentials()
	if err != nil {
		return nil, err
	}

	opts := make([]grpc.ServerOption, 0, len(extra)+1)
	if creds != nil {
		opts = append(opts, grpc.Creds(creds))
	}
	opts = append(opts, extra...)
	return opts, nil
}

// NewServer creates a gRPC server with shared TLS settings.
func NewServer(extra ...grpc.ServerOption) (*grpc.Server, error) {
	opts, err := ServerOptions(extra...)
	if err != nil {
		return nil, err
	}
	return grpc.NewServer(opts...), nil
}

func serviceAuthClientInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		if auth.RequiresServiceAuth(method) {
			secret := auth.ServiceSecretFromEnv()
			if secret == "" {
				return fmt.Errorf("INTERNAL_SERVICE_SECRET is required for %s", method)
			}
			ctx = metadata.AppendToOutgoingContext(ctx, auth.ServiceTokenMetadataKey, secret)
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
