package service

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	storagepb "metarang/shared/pb/storage"
)

// KYCStorageClient moves staged KYC videos into user-specific storage paths.
type KYCStorageClient interface {
	MoveKYCVideo(ctx context.Context, userID uint64, videoPath, videoName string) (string, error)
}

type grpcKYCStorageClient struct {
	client storagepb.FileStorageServiceClient
}

func NewGRPCKYCStorageClient(client storagepb.FileStorageServiceClient) KYCStorageClient {
	return &grpcKYCStorageClient{client: client}
}

func (c *grpcKYCStorageClient) MoveKYCVideo(ctx context.Context, userID uint64, videoPath, videoName string) (string, error) {
	if c.client == nil {
		return "", fmt.Errorf("storage service not available")
	}

	fileData, contentType, err := c.readStagedVideo(ctx, videoPath, videoName)
	if err != nil {
		return "", err
	}

	finalName := filepath.Base(videoName)
	uploadID := fmt.Sprintf("kyc_video_%d_%d", userID, time.Now().UnixNano())
	chunkReq := &storagepb.ChunkUploadRequest{
		UploadId:    uploadID,
		ChunkData:   fileData,
		ChunkIndex:  0,
		TotalChunks: 1,
		Filename:    finalName,
		ContentType: contentType,
		TotalSize:   int64(len(fileData)),
		UploadPath:  fmt.Sprintf("/uploads/kyc/%d", userID),
	}

	chunkResp, err := c.client.ChunkUpload(ctx, chunkReq)
	if err != nil {
		return "", fmt.Errorf("failed to upload kyc video: %w", err)
	}
	if !chunkResp.Success || !chunkResp.IsFinished {
		return "", fmt.Errorf("kyc video upload did not complete: %s", chunkResp.Message)
	}

	dirPath := chunkResp.FileUrl
	filename := chunkResp.FilePath
	if filename == "" {
		filename = chunkResp.FinalFilename
	}
	if dirPath == "" || filename == "" {
		return "", fmt.Errorf("storage service did not return kyc video path")
	}

	return strings.TrimSuffix(dirPath, "/") + "/" + filename, nil
}

func (c *grpcKYCStorageClient) readStagedVideo(ctx context.Context, videoPath, videoName string) ([]byte, string, error) {
	contentType := kycVideoContentType(videoName)
	for _, sourcePath := range kycStagedVideoPaths(videoPath, videoName) {
		stream, err := c.client.GetFile(ctx, &storagepb.GetFileRequest{FilePath: sourcePath})
		if err != nil {
			continue
		}
		var data []byte
		for {
			resp, recvErr := stream.Recv()
			if recvErr == io.EOF {
				break
			}
			if recvErr != nil {
				data = nil
				break
			}
			if resp.ContentType != "" {
				contentType = resp.ContentType
			}
			data = append(data, resp.Data...)
		}
		if len(data) > 0 {
			return data, contentType, nil
		}
	}
	return nil, "", ErrVideoFileNotFound
}

func kycVideoContentType(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".webm":
		return "video/webm"
	case ".mov":
		return "video/quicktime"
	default:
		return "video/mp4"
	}
}

// StagedVideoPaths returns candidate storage paths for a staged KYC video upload.
func StagedVideoPaths(videoPath, videoName string) []string {
	return kycStagedVideoPaths(videoPath, videoName)
}

func kycStagedVideoPaths(videoPath, videoName string) []string {
	dir := strings.Trim(videoPath, "/")
	name := strings.TrimPrefix(videoName, "/")
	seen := make(map[string]struct{})
	var paths []string
	add := func(p string) {
		p = strings.ReplaceAll(p, "\\", "/")
		if _, ok := seen[p]; ok || p == "" {
			return
		}
		seen[p] = struct{}{}
		paths = append(paths, p)
	}
	add(dir + "/" + name)
	if strings.HasPrefix(dir, "upload/") && !strings.HasPrefix(dir, "uploads/") {
		add("uploads/" + strings.TrimPrefix(dir, "upload/") + "/" + name)
	}
	if !strings.HasPrefix(dir, "uploads/") {
		add("uploads/" + dir + "/" + name)
	}
	return paths
}
