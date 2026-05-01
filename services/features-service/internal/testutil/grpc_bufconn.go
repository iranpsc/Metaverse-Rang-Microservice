package testutil

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// BufGRPCTestServer hosts a gRPC service on an in-memory listener for tests.
type BufGRPCTestServer struct {
	Lis    *bufconn.Listener
	Server *grpc.Server
}

// NewBufGRPCTestServer creates a server; register services on s.Server, then call Start.
func NewBufGRPCTestServer() *BufGRPCTestServer {
	lis := bufconn.Listen(bufSize)
	return &BufGRPCTestServer{
		Lis:    lis,
		Server: grpc.NewServer(),
	}
}

func (s *BufGRPCTestServer) Start(t *testing.T) {
	t.Helper()
	go func() {
		if err := s.Server.Serve(s.Lis); err != nil {
			t.Logf("bufconn serve: %v", err)
		}
	}()
	t.Cleanup(func() { s.Server.Stop() })
}

// BufDialContext returns a dialer for grpc.DialContext(..., WithContextDialer(dialer)).
func (s *BufGRPCTestServer) BufDialContext(context.Context, string) (net.Conn, error) {
	return s.Lis.Dial()
}

// GRPCClientConn dials the in-memory server (insecure).
func (s *BufGRPCTestServer) GRPCClientConn(t *testing.T) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(s.BufDialContext),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}
