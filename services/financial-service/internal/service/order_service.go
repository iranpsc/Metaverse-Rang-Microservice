package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"metarang/financial-service/internal/models"
	"metarang/financial-service/internal/repository"
	commercialpb "metarang/shared/pb/commercial"
)

var (
	ErrInvalidAmount   = errors.New("amount must be at least 1")
	ErrInvalidAsset    = errors.New("invalid asset type")
	ErrOrderNotFound   = errors.New("order not found")
	ErrPaymentFailed   = errors.New("payment request failed")
	ErrUserNotEligible = errors.New("user not eligible to buy from store")
)

const (
	orderStatusPending int32 = -138
	statusSuccess      int32 = 0
	statusUnknown      int32 = -1

	transactionActionDeposit = "deposit"
	orderPayableType         = "App\\Models\\Order"
	sadadGateway             = "sadad"

	firstOrderBonusRate = 0.5
	transactionIDBytes  = 4
)

var validOrderAssets = map[string]struct{}{
	"psc":    {},
	"irr":    {},
	"red":    {},
	"blue":   {},
	"yellow": {},
}

type OrderService interface {
	CreateOrder(ctx context.Context, userID uint64, amount int32, asset string) (string, error)
	HandleCallback(ctx context.Context, orderID uint64, token string, resCode string, additionalParams map[string]string) (string, error)
}

// WalletTopUp credits the buyer wallet via commercial-service (optional).
type WalletTopUp interface {
	AddBalance(ctx context.Context, userID uint64, asset string, amount float64) error
}

// ReferralProcessor triggers referral commission via commercial-service (optional).
type ReferralProcessor interface {
	ProcessReferral(ctx context.Context, buyerUserID, orderID uint64, asset string, amount float64) error
}

// PurchaseNotifier sends post-payment notifications via notifications-service (optional).
type PurchaseNotifier interface {
	NotifyPurchaseSuccess(ctx context.Context, userID, orderID uint64, asset string, amount float64) error
}

type orderService struct {
	orderRepo       repository.OrderRepository
	transactionRepo repository.TransactionRepository
	paymentRepo     repository.PaymentRepository
	variableRepo    repository.VariableRepository
	firstOrderRepo  repository.FirstOrderRepository
	sadadClient     SadadClient
	orderPolicy     OrderPolicy
	jalaliConverter JalaliConverter
	walletClient    commercialpb.WalletServiceClient
	sadadConfig     OrderConfig
}

type OrderConfig struct {
	SadadMerchantID             string
	SadadTerminalID             string
	SadadTransactionKey         string
	SadadPaymentIdentityRial    string // Sadad PaymentIdentity for IRR payments
	SadadPaymentIdentityNonRial string // Sadad PaymentIdentity for non-IRR assets
	SadadCallbackURL            string
	FrontendURL                 string
	SadadSandbox                bool // BankTest sandbox omits PaymentIdentity
}

func NewOrderService(
	orderRepo repository.OrderRepository,
	transactionRepo repository.TransactionRepository,
	paymentRepo repository.PaymentRepository,
	variableRepo repository.VariableRepository,
	firstOrderRepo repository.FirstOrderRepository,
	sadadClient SadadClient,
	orderPolicy OrderPolicy,
	jalaliConverter JalaliConverter,
	walletClient commercialpb.WalletServiceClient,
	config OrderConfig,
) OrderService {
	return &orderService{
		orderRepo:       orderRepo,
		transactionRepo: transactionRepo,
		paymentRepo:     paymentRepo,
		variableRepo:    variableRepo,
		firstOrderRepo:  firstOrderRepo,
		sadadClient:     sadadClient,
		orderPolicy:     orderPolicy,
		jalaliConverter: jalaliConverter,
		walletClient:    walletClient,
		sadadConfig:     config,
	}
}

func (s *orderService) CreateOrder(ctx context.Context, userID uint64, amount int32, asset string) (string, error) {
	if err := validateCreateOrderInput(amount, asset); err != nil {
		return "", err
	}

	if err := s.ensureCanBuyFromStore(ctx, userID); err != nil {
		return "", err
	}

	rate, err := s.variableRepo.GetRate(ctx, asset)
	if err != nil {
		return "", fmt.Errorf("failed to get asset rate: %w", err)
	}

	order, err := s.createPendingOrder(ctx, userID, amount, asset)
	if err != nil {
		return "", err
	}

	transaction, err := s.createDepositTransaction(ctx, order, userID, amount, asset)
	if err != nil {
		return "", err
	}

	paymentURL, token, err := s.requestSadadPayment(order.ID, amount, asset, rate)
	if err != nil {
		return "", err
	}

	s.storeTransactionToken(ctx, transaction, token)
	return paymentURL, nil
}

func validateCreateOrderInput(amount int32, asset string) error {
	if amount < 1 {
		return ErrInvalidAmount
	}

	if _, ok := validOrderAssets[asset]; !ok {
		return ErrInvalidAsset
	}

	return nil
}

func (s *orderService) ensureCanBuyFromStore(ctx context.Context, userID uint64) error {
	canBuy, err := s.orderPolicy.CanBuyFromStore(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to check buy permission: %w", err)
	}
	if !canBuy {
		return ErrUserNotEligible
	}

	return nil
}

func (s *orderService) createPendingOrder(ctx context.Context, userID uint64, amount int32, asset string) (*models.Order, error) {
	order := &models.Order{
		UserID: userID,
		Asset:  asset,
		Amount: float64(amount),
		Status: orderStatusPending,
	}

	if err := s.orderRepo.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	return order, nil
}

func (s *orderService) createDepositTransaction(ctx context.Context, order *models.Order, userID uint64, amount int32, asset string) (*models.Transaction, error) {
	transactionID, err := generateTransactionID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate transaction id: %w", err)
	}

	transaction := &models.Transaction{
		ID:          transactionID,
		UserID:      userID,
		Asset:       asset,
		Amount:      float64(amount),
		Action:      transactionActionDeposit,
		Status:      1,
		PayableType: stringPtr(orderPayableType),
		PayableID:   &order.ID,
	}

	if err := s.transactionRepo.Create(ctx, transaction); err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	return transaction, nil
}

func generateTransactionID() (string, error) {
	b := make([]byte, transactionIDBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("TR-%s", hex.EncodeToString(b)), nil
}

func stringPtr(s string) *string {
	return &s
}
