package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	storagepb "metarang/shared/pb/storage"
)

var (
	ErrStorageUnavailable  = errors.New("storage service not available")
	ErrStorageUploadFailed = errors.New("storage service upload failed")
)

// FileStorage uploads files via storage-service gRPC.
// Support-service must not write attachment bytes to local disk.
type FileStorage interface {
	UploadChunk(ctx context.Context, uploadID, uploadPath, filename, contentType string, data []byte) (relativePath string, err error)
}

type grpcFileStorage struct {
	client storagepb.FileStorageServiceClient
}

// NewGRPCFileStorage returns a storage-service backed uploader, or nil if client is nil.
func NewGRPCFileStorage(client storagepb.FileStorageServiceClient) FileStorage {
	if client == nil {
		return nil
	}
	return &grpcFileStorage{client: client}
}

func (s *grpcFileStorage) UploadChunk(ctx context.Context, uploadID, uploadPath, filename, contentType string, data []byte) (string, error) {
	if s == nil || s.client == nil {
		return "", ErrStorageUnavailable
	}

	chunkResp, err := s.client.ChunkUpload(ctx, &storagepb.ChunkUploadRequest{
		UploadId:    uploadID,
		ChunkData:   data,
		ChunkIndex:  0,
		TotalChunks: 1,
		Filename:    filename,
		ContentType: contentType,
		TotalSize:   int64(len(data)),
		UploadPath:  uploadPath,
	})
	if err != nil {
		return "", fmt.Errorf("storage upload: %w", err)
	}
	if !chunkResp.Success {
		return "", fmt.Errorf("%w: %s", ErrStorageUploadFailed, chunkResp.Message)
	}
	if !chunkResp.IsFinished {
		return "", fmt.Errorf("%w: upload did not complete", ErrStorageUploadFailed)
	}

	dirPath := chunkResp.FileUrl
	name := chunkResp.FilePath
	if name == "" {
		name = chunkResp.FinalFilename
	}
	if dirPath == "" || name == "" {
		return "", fmt.Errorf("%w: incomplete file path returned", ErrStorageUploadFailed)
	}

	return strings.TrimSuffix(dirPath, "/") + "/" + name, nil
}
