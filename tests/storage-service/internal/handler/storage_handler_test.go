package handler_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	storagepb "metarang/shared/pb/storage"
	"metarang/storage-service/internal/ftp"
	"metarang/storage-service/internal/handler"
	"metarang/storage-service/internal/service"
	"metarang/storage-service/tests/internal/testutil"
)

func newStorageGRPCClient(t *testing.T) storagepb.FileStorageServiceClient {
	t.Helper()
	storageService, _ := testutil.NewTestStorageService(t)
	s := testutil.NewBufGRPCTestServer()
	handler.RegisterStorageHandler(s.Server, storageService)
	s.Start(t)
	return storagepb.NewFileStorageServiceClient(s.GRPCClientConn(t))
}

func TestStorageHandler_UploadFile(t *testing.T) {
	ctx := context.Background()
	client := newStorageGRPCClient(t)

	stream, err := client.UploadFile(ctx)
	if err != nil {
		t.Fatalf("UploadFile stream: %v", err)
	}
	if err := stream.Send(&storagepb.UploadFileRequest{
		Data: &storagepb.UploadFileRequest_Metadata{
			Metadata: &storagepb.FileMetadata{
				Filename:    "doc.txt",
				ContentType: "text/plain",
				UploadPath:  "docs",
			},
		},
	}); err != nil {
		t.Fatalf("Send metadata: %v", err)
	}
	if err := stream.Send(&storagepb.UploadFileRequest{
		Data: &storagepb.UploadFileRequest_ChunkData{ChunkData: []byte("hello")},
	}); err != nil {
		t.Fatalf("Send chunk: %v", err)
	}
	resp, err := stream.CloseAndRecv()
	if err != nil {
		t.Fatalf("CloseAndRecv: %v", err)
	}
	if !resp.Success || resp.FileSize != 5 || resp.Filename != "doc.txt" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestStorageHandler_UploadFile_NoMetadata(t *testing.T) {
	ctx := context.Background()
	client := newStorageGRPCClient(t)

	stream, err := client.UploadFile(ctx)
	if err != nil {
		t.Fatalf("UploadFile stream: %v", err)
	}
	if err := stream.Send(&storagepb.UploadFileRequest{
		Data: &storagepb.UploadFileRequest_ChunkData{ChunkData: []byte("x")},
	}); err != nil {
		t.Fatalf("Send chunk: %v", err)
	}
	_, err = stream.CloseAndRecv()
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestStorageHandler_GetFile(t *testing.T) {
	tempDir := t.TempDir()
	uploadBase := filepath.Join(tempDir, "uploads")
	if err := os.MkdirAll(uploadBase, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(uploadBase, "readme.txt"), []byte("local"), 0644); err != nil {
		t.Fatal(err)
	}

	chunkManager, _ := service.NewChunkManager(filepath.Join(tempDir, "chunks"))
	mockFTP := ftp.NewMockFTPClient(filepath.Join(tempDir, "ftp"), "http://example.com")
	storageService := service.NewStorageService(mockFTP, chunkManager, uploadBase)

	s := testutil.NewBufGRPCTestServer()
	handler.RegisterStorageHandler(s.Server, storageService)
	s.Start(t)
	client := storagepb.NewFileStorageServiceClient(s.GRPCClientConn(t))

	stream, err := client.GetFile(context.Background(), &storagepb.GetFileRequest{FilePath: "readme.txt"})
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	resp, err := stream.Recv()
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	if !bytes.Equal(resp.Data, []byte("local")) {
		t.Fatalf("unexpected data: %q", resp.Data)
	}
}

func TestStorageHandler_GetFile_NotFound(t *testing.T) {
	client := newStorageGRPCClient(t)
	stream, err := client.GetFile(context.Background(), &storagepb.GetFileRequest{FilePath: "missing.bin"})
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	_, err = stream.Recv()
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.NotFound {
		t.Fatalf("expected NotFound, got %v", err)
	}
}

func TestStorageHandler_DeleteFile(t *testing.T) {
	tempDir := t.TempDir()
	ftpDir := filepath.Join(tempDir, "ftp")
	chunkManager, _ := service.NewChunkManager(filepath.Join(tempDir, "chunks"))
	mockFTP := ftp.NewMockFTPClient(ftpDir, "http://example.com")
	storageService := service.NewStorageService(mockFTP, chunkManager, filepath.Join(tempDir, "uploads"))
	if err := mockFTP.UploadFile("drop.txt", bytes.NewReader([]byte("x"))); err != nil {
		t.Fatal(err)
	}

	s := testutil.NewBufGRPCTestServer()
	handler.RegisterStorageHandler(s.Server, storageService)
	s.Start(t)
	client := storagepb.NewFileStorageServiceClient(s.GRPCClientConn(t))

	_, err := client.DeleteFile(context.Background(), &storagepb.DeleteFileRequest{FilePath: "drop.txt"})
	if err != nil {
		t.Fatalf("DeleteFile: %v", err)
	}
}

func TestStorageHandler_DeleteFile_Error(t *testing.T) {
	client := newStorageGRPCClient(t)
	_, err := client.DeleteFile(context.Background(), &storagepb.DeleteFileRequest{FilePath: "missing.txt"})
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.Internal {
		t.Fatalf("expected Internal, got %v", err)
	}
}

func TestStorageHandler_GetFilesByEntity(t *testing.T) {
	client := newStorageGRPCClient(t)
	resp, err := client.GetFilesByEntity(context.Background(), &storagepb.GetFilesByEntityRequest{
		ImageableType: "App\\Models\\User",
		ImageableId:   1,
	})
	if err != nil {
		t.Fatalf("GetFilesByEntity: %v", err)
	}
	if resp == nil || len(resp.Files) != 0 {
		t.Fatalf("expected empty files, got %+v", resp)
	}
}

func TestStorageHandler_ChunkUpload(t *testing.T) {
	client := newStorageGRPCClient(t)
	ctx := context.Background()

	for _, tc := range []struct {
		name string
		req  *storagepb.ChunkUploadRequest
		code codes.Code
	}{
		{"missing upload_id", &storagepb.ChunkUploadRequest{Filename: "a.txt", ChunkIndex: 0, TotalChunks: 1, ChunkData: []byte("x")}, codes.InvalidArgument},
		{"missing filename", &storagepb.ChunkUploadRequest{UploadId: "u1", ChunkIndex: 0, TotalChunks: 1, ChunkData: []byte("x")}, codes.InvalidArgument},
		{"invalid chunk index", &storagepb.ChunkUploadRequest{UploadId: "u1", Filename: "a.txt", ChunkIndex: 2, TotalChunks: 1, ChunkData: []byte("x")}, codes.InvalidArgument},
		{"empty chunk", &storagepb.ChunkUploadRequest{UploadId: "u1", Filename: "a.txt", ChunkIndex: 0, TotalChunks: 1}, codes.InvalidArgument},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.ChunkUpload(ctx, tc.req)
			st, ok := status.FromError(err)
			if !ok || st.Code() != tc.code {
				t.Fatalf("expected %v, got %v", tc.code, err)
			}
		})
	}

	resp, err := client.ChunkUpload(ctx, &storagepb.ChunkUploadRequest{
		UploadId:    "grpc-chunk-1",
		Filename:    "note.txt",
		ContentType: "text/plain",
		ChunkData:   []byte("chunk-bytes"),
		ChunkIndex:  0,
		TotalChunks: 1,
		TotalSize:   11,
	})
	if err != nil {
		t.Fatalf("ChunkUpload success: %v", err)
	}
	if !resp.Success || !resp.IsFinished || resp.PercentageDone != 100.0 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.FinalFilename == "" || resp.FilePath == "" || resp.FileUrl == "" {
		t.Fatalf("expected finished file metadata: %+v", resp)
	}
	if resp.FinalFilename != resp.FilePath {
		t.Fatalf("FinalFilename should be the stored filename, got FinalFilename=%q FilePath=%q", resp.FinalFilename, resp.FilePath)
	}
	if strings.Contains(resp.FinalFilename, "/") {
		t.Fatalf("FinalFilename should not be a MIME type, got %q", resp.FinalFilename)
	}
}

func TestStorageHandler_ChunkUpload_InProgress(t *testing.T) {
	client := newStorageGRPCClient(t)
	resp, err := client.ChunkUpload(context.Background(), &storagepb.ChunkUploadRequest{
		UploadId:    "multi-chunk",
		Filename:    "big.bin",
		ContentType: "application/octet-stream",
		ChunkData:   []byte("part-1"),
		ChunkIndex:  0,
		TotalChunks: 2,
		TotalSize:   12,
	})
	if err != nil {
		t.Fatalf("ChunkUpload: %v", err)
	}
	if resp.IsFinished || resp.PercentageDone != 50.0 {
		t.Fatalf("expected in-progress 50%%, got %+v", resp)
	}
}
