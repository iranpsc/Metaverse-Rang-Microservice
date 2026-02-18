package handler

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"metargb/auth-service/internal/lang"
	"metargb/auth-service/internal/models"
	"metargb/auth-service/internal/service"
	pb "metargb/shared/pb/auth"
	storagepb "metargb/shared/pb/storage"
	"metargb/shared/pkg/jalali"
)

func (h *kycHandler) GetKYC(ctx context.Context, req *pb.GetKYCRequest) (*pb.KYCResponse, error) {
	kyc, err := h.kycService.GetKYC(ctx, req.UserId)
	if err != nil {
		return nil, mapKYCServiceError(err, getProjectLocale())
	}

	// If KYC not found, return empty response (matches Laravel behavior)
	if kyc == nil {
		return &pb.KYCResponse{}, nil
	}

	return convertKYCToProto(kyc), nil
}

func (h *kycHandler) UpdateKYC(ctx context.Context, req *pb.UpdateKYCRequest) (*pb.KYCResponse, error) {
	locale := getProjectLocale()
	// Validate melli_card file
	if len(req.MelliCardData) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, lang.T(locale, "melli_card_data is required"))
	}

	if req.MelliCardFilename == "" {
		return nil, status.Errorf(codes.InvalidArgument, lang.T(locale, "melli_card_filename is required"))
	}

	if req.MelliCardContentType == "" {
		return nil, status.Errorf(codes.InvalidArgument, lang.T(locale, "melli_card_content_type is required"))
	}

	// Validate file size (max 5MB = 5 * 1024 * 1024 bytes)
	const maxSize = 5 * 1024 * 1024
	if len(req.MelliCardData) > maxSize {
		return nil, status.Errorf(codes.InvalidArgument, lang.T(locale, "melli_card file size exceeds maximum of 5MB"))
	}

	// Validate content type
	contentType := strings.ToLower(req.MelliCardContentType)
	if contentType != "image/png" && contentType != "image/jpeg" && contentType != "image/jpg" {
		return nil, status.Errorf(codes.InvalidArgument, lang.T(locale, "melli_card must be a PNG or JPEG image"))
	}

	// Validate filename extension
	filenameLower := strings.ToLower(req.MelliCardFilename)
	if !strings.HasSuffix(filenameLower, ".png") && !strings.HasSuffix(filenameLower, ".jpg") && !strings.HasSuffix(filenameLower, ".jpeg") {
		return nil, status.Errorf(codes.InvalidArgument, lang.T(locale, "melli_card filename must have .png, .jpg, or .jpeg extension"))
	}

	// Upload melli_card to storage-service
	var melliCardURL string
	if h.storageClient != nil {
		uploadID := fmt.Sprintf("kyc_melli_card_%d_%d", req.UserId, time.Now().UnixNano())

		chunkReq := &storagepb.ChunkUploadRequest{
			UploadId:    uploadID,
			ChunkData:   req.MelliCardData,
			ChunkIndex:  0,
			TotalChunks: 1,
			Filename:    req.MelliCardFilename,
			ContentType: req.MelliCardContentType,
			TotalSize:   int64(len(req.MelliCardData)),
			UploadPath:  "/uploads/kyc", // Upload path for KYC documents
		}

		chunkResp, err := h.storageClient.ChunkUpload(ctx, chunkReq)
		if err != nil {
			return nil, status.Errorf(codes.Internal, lang.Tf(locale, "failed to upload melli_card to storage service: %v", err))
		}

		if !chunkResp.Success {
			return nil, status.Errorf(codes.Internal, lang.Tf(locale, "storage service upload failed: %s", chunkResp.Message))
		}

		if !chunkResp.IsFinished {
			return nil, status.Errorf(codes.Internal, lang.T(locale, "storage service upload did not complete"))
		}

		// Construct full path from storage service response
		dirPath := chunkResp.FileUrl
		filename := chunkResp.FilePath
		if filename == "" {
			filename = chunkResp.FinalFilename
		}

		if dirPath == "" || filename == "" {
			return nil, status.Errorf(codes.Internal, lang.T(locale, "storage service did not return complete file path"))
		}

		melliCardURL = strings.TrimSuffix(dirPath, "/") + "/" + filename
	} else {
		return nil, status.Errorf(codes.Internal, lang.T(locale, "storage service not available"))
	}

	videoPath := ""
	videoName := ""
	if req.Video != nil {
		videoPath = req.Video.Path
		videoName = req.Video.Name
	}

	kyc, err := h.kycService.UpdateKYC(
		ctx,
		req.UserId,
		req.Fname,
		req.Lname,
		req.MelliCode,
		req.Birthdate,
		req.Province,
		melliCardURL,
		videoPath,
		videoName,
		req.VerifyTextId,
		req.Gender,
	)
	if err != nil {
		return nil, mapKYCServiceError(err, getProjectLocale())
	}

	return convertKYCToProto(kyc), nil
}

// convertKYCToProto converts a KYC model to proto response
func convertKYCToProto(kyc *models.KYC) *pb.KYCResponse {
	birthdate := ""
	if kyc.Birthdate.Valid {
		birthdate = jalali.CarbonToJalali(kyc.Birthdate.Time)
	}

	video := ""
	if kyc.Video.Valid {
		video = kyc.Video.String
	}

	errorStr := ""
	if kyc.Errors.Valid {
		errorStr = kyc.Errors.String
	}

	gender := ""
	if kyc.Gender.Valid {
		gender = kyc.Gender.String
	}

	return &pb.KYCResponse{
		Id:        kyc.ID,
		MelliCard: kyc.MelliCard,
		Fname:     kyc.Fname,
		Lname:     kyc.Lname,
		MelliCode: kyc.MelliCode,
		Birthdate: birthdate,
		Province:  kyc.Province,
		Status:    kyc.Status,
		Video:     video,
		Errors:    errorStr,
		Gender:    gender,
	}
}

// mapKYCServiceError maps KYC service errors to gRPC status codes
func mapKYCServiceError(err error, locale string) error {
	switch {
	case errors.Is(err, service.ErrKYCNotFound):
		return status.Errorf(codes.NotFound, "%s", err.Error())
	case errors.Is(err, service.ErrKYCNotOwned):
		return status.Errorf(codes.PermissionDenied, "%s", err.Error())
	case errors.Is(err, service.ErrKYCNotRejected):
		return status.Errorf(codes.FailedPrecondition, "%s", err.Error())
	case errors.Is(err, service.ErrInvalidFname),
		errors.Is(err, service.ErrInvalidLname),
		errors.Is(err, service.ErrInvalidMelliCode),
		errors.Is(err, service.ErrInvalidBirthdate),
		errors.Is(err, service.ErrInvalidProvince),
		errors.Is(err, service.ErrProvinceRequired),
		errors.Is(err, service.ErrInvalidGender),
		errors.Is(err, service.ErrGenderRequired),
		errors.Is(err, service.ErrVerifyTextIDRequired),
		errors.Is(err, service.ErrVerifyTextIDNotFound),
		errors.Is(err, service.ErrVideoRequired),
		errors.Is(err, service.ErrMelliCardRequired),
		errors.Is(err, service.ErrMelliCodeNotUnique):
		if fields, ok := mapServiceErrorToValidationFields(err, locale); ok {
			return returnValidationError(fields)
		}
		return status.Errorf(codes.InvalidArgument, "%s", err.Error())
	default:
		return status.Errorf(codes.Internal, lang.Tf(locale, "operation failed: %v", err))
	}
}

type kycHandler struct {
	pb.UnimplementedKYCServiceServer
	kycService    service.KYCService
	storageClient storagepb.FileStorageServiceClient
}

func RegisterKYCHandler(grpcServer *grpc.Server, kycService service.KYCService, storageClient storagepb.FileStorageServiceClient) {
	pb.RegisterKYCServiceServer(grpcServer, &kycHandler{
		kycService:    kycService,
		storageClient: storageClient,
	})
}

// mapServiceError maps bank account service errors to gRPC status codes
func mapServiceError(err error, locale string) error {
	switch {
	case errors.Is(err, service.ErrBankAccountNotFound):
		return status.Errorf(codes.NotFound, "%s", err.Error())
	case errors.Is(err, service.ErrBankAccountNotOwned):
		return status.Errorf(codes.PermissionDenied, "%s", err.Error())
	case errors.Is(err, service.ErrBankAccountNotRejected):
		return status.Errorf(codes.FailedPrecondition, "%s", err.Error())
	case errors.Is(err, service.ErrUserNotVerified):
		return status.Errorf(codes.PermissionDenied, "%s", err.Error())
	case errors.Is(err, service.ErrInvalidBankName),
		errors.Is(err, service.ErrInvalidShabaNum),
		errors.Is(err, service.ErrInvalidCardNum),
		errors.Is(err, service.ErrShabaNumNotUnique),
		errors.Is(err, service.ErrCardNumNotUnique):
		if fields, ok := mapServiceErrorToValidationFields(err, locale); ok {
			return returnValidationError(fields)
		}
		return status.Errorf(codes.InvalidArgument, "%s", err.Error())
	default:
		return status.Errorf(codes.Internal, lang.Tf(locale, "operation failed: %v", err))
	}
}

// convertBankAccountToProto converts a BankAccount model to proto response
func convertBankAccountToProto(bankAccount *models.BankAccount) *pb.BankAccountResponse {
	errorStr := ""
	if bankAccount.Errors.Valid {
		errorStr = bankAccount.Errors.String
	}

	return &pb.BankAccountResponse{
		Id:       bankAccount.ID,
		BankName: bankAccount.BankName,
		ShabaNum: bankAccount.ShabaNum,
		CardNum:  bankAccount.CardNum,
		Status:   bankAccount.Status,
		Errors:   errorStr,
	}
}

func (h *kycHandler) ListBankAccounts(ctx context.Context, req *pb.ListBankAccountsRequest) (*pb.ListBankAccountsResponse, error) {
	accounts, err := h.kycService.ListBankAccounts(ctx, req.UserId)
	if err != nil {
		return nil, mapServiceError(err, getProjectLocale())
	}

	var protoAccounts []*pb.BankAccountResponse
	for _, account := range accounts {
		protoAccounts = append(protoAccounts, convertBankAccountToProto(account))
	}

	return &pb.ListBankAccountsResponse{
		Data: protoAccounts,
	}, nil
}

func (h *kycHandler) CreateBankAccount(ctx context.Context, req *pb.CreateBankAccountRequest) (*pb.BankAccountResponse, error) {
	bankAccount, err := h.kycService.CreateBankAccount(ctx, req.UserId, req.BankName, req.ShabaNum, req.CardNum)
	if err != nil {
		return nil, mapServiceError(err, getProjectLocale())
	}

	return convertBankAccountToProto(bankAccount), nil
}

func (h *kycHandler) GetBankAccount(ctx context.Context, req *pb.GetBankAccountRequest) (*pb.BankAccountResponse, error) {
	bankAccount, err := h.kycService.GetBankAccount(ctx, req.UserId, req.BankAccountId)
	if err != nil {
		return nil, mapServiceError(err, getProjectLocale())
	}

	return convertBankAccountToProto(bankAccount), nil
}

func (h *kycHandler) UpdateBankAccount(ctx context.Context, req *pb.UpdateBankAccountRequest) (*pb.BankAccountResponse, error) {
	bankAccount, err := h.kycService.UpdateBankAccount(ctx, req.UserId, req.BankAccountId, req.BankName, req.ShabaNum, req.CardNum)
	if err != nil {
		return nil, mapServiceError(err, getProjectLocale())
	}

	return convertBankAccountToProto(bankAccount), nil
}

func (h *kycHandler) DeleteBankAccount(ctx context.Context, req *pb.DeleteBankAccountRequest) (*emptypb.Empty, error) {
	err := h.kycService.DeleteBankAccount(ctx, req.UserId, req.BankAccountId)
	if err != nil {
		return nil, mapServiceError(err, getProjectLocale())
	}

	return &emptypb.Empty{}, nil
}
