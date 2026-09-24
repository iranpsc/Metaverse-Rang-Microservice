package handler

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	commonpb "metarang/shared/pb/common"
	storagepb "metarang/shared/pb/storage"
	"metarang/storage-service/internal/service"
)

type StorageHandler struct {
	storagepb.UnimplementedFileStorageServiceServer
	service *service.StorageService
}

func RegisterStorageHandler(grpcServer *grpc.Server, svc *service.StorageService) {
	handler := &StorageHandler{service: svc}
	storagepb.RegisterFileStorageServiceServer(grpcServer, handler)
}

// UploadFile handles streaming file uploads
func (h *StorageHandler) UploadFile(stream storagepb.FileStorageService_UploadFileServer) error {
	var metadata *storagepb.FileMetadata
	var fileData bytes.Buffer

	// Receive file chunks
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "failed to receive chunk: %v", err)
		}

		switch data := req.Data.(type) {
		case *storagepb.UploadFileRequest_Metadata:
			metadata = data.Metadata

		case *storagepb.UploadFileRequest_ChunkData:
			if _, err := fileData.Write(data.ChunkData); err != nil {
				return status.Errorf(codes.Internal, "failed to write chunk: %v", err)
			}
		}
	}

	if metadata == nil {
		return status.Errorf(codes.InvalidArgument, "no metadata provided")
	}

	// Upload file to FTP
	url, err := h.service.UploadFile(
		metadata.Filename,
		metadata.ContentType,
		fileData.Bytes(),
		metadata.UploadPath,
	)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to upload file: %v", err)
	}

	// Send response
	response := &storagepb.UploadFileResponse{
		FileUrl:  url,
		Filename: metadata.Filename,
		FileSize: int64(fileData.Len()),
		Success:  true,
		Message:  "File uploaded successfully",
	}

	return stream.SendAndClose(response)
}

// GetFile retrieves a file from storage
func (h *StorageHandler) GetFile(req *storagepb.GetFileRequest, stream storagepb.FileStorageService_GetFileServer) error {
	data, contentType, err := h.service.GetFile(req.FilePath)
	if err != nil {
		return status.Errorf(codes.NotFound, "failed to get file: %v", err)
	}

	// Send file data
	response := &storagepb.GetFileResponse{
		Data:        data,
		ContentType: contentType,
		FileSize:    int64(len(data)),
	}

	if err := stream.Send(response); err != nil {
		return status.Errorf(codes.Internal, "failed to send file: %v", err)
	}

	return nil
}

// DeleteFile deletes a file from storage
func (h *StorageHandler) DeleteFile(ctx context.Context, req *storagepb.DeleteFileRequest) (*commonpb.Empty, error) {
	if err := h.service.DeleteFile(req.FilePath); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete file: %v", err)
	}

	return &commonpb.Empty{}, nil
}

// GetFilesByEntity retrieves files by entity type and ID
func (h *StorageHandler) GetFilesByEntity(ctx context.Context, req *storagepb.GetFilesByEntityRequest) (*storagepb.FilesResponse, error) {
	// This method would query the images table
	// For now, return empty response
	return &storagepb.FilesResponse{Files: []*storagepb.FileInfo{}}, nil
}

// ChunkUpload handles chunk-based file uploads with progress tracking
func (h *StorageHandler) ChunkUpload(ctx context.Context, req *storagepb.ChunkUploadRequest) (*storagepb.ChunkUploadResponse, error) {
	// Validate request
	if req.UploadId == "" {
		return nil, status.Errorf(codes.InvalidArgument, "upload_id is required")
	}
	if req.Filename == "" {
		return nil, status.Errorf(codes.InvalidArgument, "filename is required")
	}
	if req.ChunkIndex < 0 || req.ChunkIndex >= req.TotalChunks {
		return nil, status.Errorf(codes.InvalidArgument, "invalid chunk_index")
	}
	if len(req.ChunkData) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "chunk_data is empty")
	}

	// HandleChunkUpload returns (isFinished, progress, publicDir, storedName, mimeType, err).
	// Callers treat FileUrl as the public directory and FilePath as the stored filename.
	isFinished, progress, publicDir, storedName, _, err := h.service.HandleChunkUpload(
		req.UploadId,
		req.Filename,
		req.ContentType,
		req.ChunkData,
		req.ChunkIndex,
		req.TotalChunks,
		req.TotalSize,
		req.UploadPath,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to handle chunk upload: %v", err)
	}

	// Build response
	response := &storagepb.ChunkUploadResponse{
		Success:        true,
		PercentageDone: progress,
		IsFinished:     isFinished,
	}

	if isFinished {
		response.Message = "File uploaded successfully"
		response.FileUrl = publicDir
		response.FilePath = storedName
		response.FinalFilename = storedName
	} else {
		response.Message = fmt.Sprintf("Chunk %d/%d uploaded", req.ChunkIndex+1, req.TotalChunks)
	}

	return response, nil
}
