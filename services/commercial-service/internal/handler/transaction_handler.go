package handler

import (
	"context"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"metarang/commercial-service/internal/models"
	"metarang/commercial-service/internal/service"
	pb "metarang/shared/pb/commercial"
)

type TransactionHandler struct {
	pb.UnimplementedTransactionServiceServer
	transactionService service.TransactionService
}

func NewTransactionHandler(transactionService service.TransactionService) *TransactionHandler {
	return &TransactionHandler{
		transactionService: transactionService,
	}
}

func RegisterTransactionHandler(grpcServer *grpc.Server, transactionService service.TransactionService) {
	handler := NewTransactionHandler(transactionService)
	pb.RegisterTransactionServiceServer(grpcServer, handler)
}

func (h *TransactionHandler) ListTransactions(ctx context.Context, req *pb.ListTransactionsRequest) (*pb.ListTransactionsResponse, error) {
	filters := make(map[string]interface{})

	if req.Search != "" {
		filters["search"] = req.Search
	}
	if req.StartDateTime != "" {
		filters["start_date_time"] = req.StartDateTime
	}
	if req.EndDateTime != "" {
		filters["end_date_time"] = req.EndDateTime
	}
	if req.Action != "" {
		filters["action"] = req.Action
	}
	if req.Asset != "" {
		filters["asset"] = req.Asset
	}
	if req.Type != "" {
		filters["type"] = req.Type
	}
	if len(req.Status) > 0 {
		filters["status"] = req.Status
	}

	perPage := int(req.PerPage)
	if perPage <= 0 {
		perPage = 15
	}
	filters["per_page"] = perPage

	page := int(req.Page)
	if page <= 0 {
		page = 1
	}
	filters["page"] = page

	transactions, err := h.transactionService.ListTransactions(ctx, req.UserId, filters)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to list transactions: %v", err)
	}

	hasMore := len(transactions) > perPage
	if hasMore {
		transactions = transactions[:perPage]
	}

	var resources []*pb.TransactionResource
	for _, t := range transactions {
		resources = append(resources, toTransactionResource(t))
	}

	currentPage := int32(page)
	return &pb.ListTransactionsResponse{
		Transactions: resources,
		CurrentPage:  currentPage,
		HasMorePages: hasMore,
	}, nil
}

func (h *TransactionHandler) GetLatestTransaction(ctx context.Context, req *pb.GetLatestTransactionRequest) (*pb.LatestTransactionResponse, error) {
	transaction, err := h.transactionService.GetLatestTransaction(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get latest transaction: %v", err)
	}

	response := &pb.LatestTransactionResponse{}

	if transaction != nil {
		response.LatestTransaction = &pb.Transaction{
			Id:        transaction.ID,
			UserId:    transaction.UserID,
			Asset:     transaction.Asset,
			Amount:    transaction.Amount,
			Action:    transaction.Action,
			Status:    transaction.Status,
			CreatedAt: timestamppb.New(transaction.CreatedAt),
			UpdatedAt: timestamppb.New(transaction.UpdatedAt),
		}

		if transaction.Token != nil {
			response.LatestTransaction.Token = *transaction.Token
		}
		if transaction.RefID != nil {
			response.LatestTransaction.RefId = *transaction.RefID
		}
		if transaction.PayableType != nil {
			response.LatestTransaction.PayableType = *transaction.PayableType
		}
		if transaction.PayableID != nil {
			response.LatestTransaction.PayableId = *transaction.PayableID
		}
	}

	return response, nil
}

func (h *TransactionHandler) CreateTransaction(ctx context.Context, req *pb.CreateTransactionRequest) (*pb.Transaction, error) {
	transaction := &models.Transaction{
		UserID: req.UserId,
		Asset:  req.Asset,
		Amount: req.Amount,
		Action: req.Action,
		Status: req.Status,
	}

	if req.PayableType != "" {
		transaction.PayableType = &req.PayableType
	}
	if req.PayableId > 0 {
		transaction.PayableID = &req.PayableId
	}

	err := h.transactionService.CreateTransaction(ctx, transaction)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create transaction: %v", err)
	}

	return &pb.Transaction{
		Id:        transaction.ID,
		UserId:    transaction.UserID,
		Asset:     transaction.Asset,
		Amount:    transaction.Amount,
		Action:    transaction.Action,
		Status:    transaction.Status,
		CreatedAt: timestamppb.New(transaction.CreatedAt),
		UpdatedAt: timestamppb.New(transaction.UpdatedAt),
	}, nil
}

// toTransactionResource maps a transaction DTO to the API resource.
// Core fields are always included regardless of transaction status.
func toTransactionResource(t *models.TransactionDTO) *pb.TransactionResource {
	amount, _ := strconv.ParseFloat(t.Amount, 64)

	return &pb.TransactionResource{
		Id:     t.ID,
		Asset:  t.Asset,
		Amount: amount,
		Status: t.Status,
		Date:   t.Date,
		Time:   t.Time,
		Type:   t.Type,
		Action: t.Action,
	}
}
