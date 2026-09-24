package service_test

import (
	"context"
	"errors"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	storagepb "metarang/shared/pb/storage"
	"metarang/support-service/internal/service"
)

type stubStorageServer struct {
	storagepb.UnimplementedFileStorageServiceServer
	resp *storagepb.ChunkUploadResponse
	err  error
}

func (s stubStorageServer) ChunkUpload(context.Context, *storagepb.ChunkUploadRequest) (*storagepb.ChunkUploadResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.resp != nil {
		return s.resp, nil
	}
	return &storagepb.ChunkUploadResponse{
		Success:       true,
		IsFinished:    true,
		FileUrl:       "/uploads/reports",
		FilePath:      "hash.png",
		FinalFilename: "hash.png",
	}, nil
}

func TestNewGRPCFileStorage_NilClient(t *testing.T) {
	if service.NewGRPCFileStorage(nil) != nil {
		t.Fatal("expected nil FileStorage for nil client")
	}
}

func TestGRPCFileStorage_UploadChunk(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	storagepb.RegisterFileStorageServiceServer(srv, stubStorageServer{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	fs := service.NewGRPCFileStorage(storagepb.NewFileStorageServiceClient(conn))
	path, err := fs.UploadChunk(context.Background(), "id", "/uploads/reports", "a.png", "image/png", []byte{1, 2, 3})
	if err != nil {
		t.Fatal(err)
	}
	if path != "/uploads/reports/hash.png" {
		t.Fatalf("path=%q", path)
	}
}

func TestGRPCFileStorage_UploadChunkFailed(t *testing.T) {
	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	storagepb.RegisterFileStorageServiceServer(srv, stubStorageServer{
		resp: &storagepb.ChunkUploadResponse{Success: false, Message: "disk full"},
	})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	fs := service.NewGRPCFileStorage(storagepb.NewFileStorageServiceClient(conn))
	_, err = fs.UploadChunk(context.Background(), "id", "/uploads/reports", "a.png", "image/png", []byte{1})
	if err == nil || !errors.Is(err, service.ErrStorageUploadFailed) {
		t.Fatalf("err=%v", err)
	}
}
