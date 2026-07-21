// Package service implements business logic for the financial service.
package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"

	"metarang/financial-service/internal/constants"
	"metarang/financial-service/internal/models"
	"metarang/financial-service/internal/repository"
	notificationspb "metarang/shared/pb/notifications"
)

var (
	ErrInvalidAmount   = errors.New("amount must be at least 1")
	ErrInvalidAsset    = errors.New("invalid asset type")
	ErrOrderNotFound   = errors.New("order not found")
	ErrPaymentFailed   = errors.New("payment request failed")
	ErrUserNotEligible = errors.New("user not eligible to buy from store")
)

const transactionIDBytes = 4

var validOrderAssets = sliceToSet(constants.ValidOrderAssets)

func sliceToSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

type OrderService interface {
	CreateOrder(ctx context.Context, userID uint64, amount int32, asset string) (string, error)
	HandleCallback(ctx context.Context, orderID uint64, token string, resCode string, additionalParams map[string]string) (string, error)
}

// WalletTopUp credits the buyer wallet via commercial-service.
type WalletTopUp interface {
	AddBalance(ctx context.Context, userID uint64, asset string, amount float64) error
}

// ReferralProcessor triggers referral commission via commercial-service (optional).
type ReferralProcessor interface {
	ProcessReferral(ctx context.Context, buyerUserID, orderID uint64, asset string, amount float64) error
}

type orderService struct {
	db                *sql.DB
	orderRepo         repository.OrderRepository
	transactionRepo   repository.TransactionRepository
	paymentRepo       repository.PaymentRepository
	variableRepo      repository.VariableRepository
	firstOrderRepo    repository.FirstOrderRepository
	sadadClient       SadadClient
	orderPolicy       OrderPolicy
	jalaliConverter   JalaliConverter
	walletTopUp       WalletTopUp
	referralProcessor ReferralProcessor
	smsClient         notificationspb.SMSServiceClient
	sadadConfig       OrderConfig
}

type OrderConfig struct {
	SadadMerchantID             string
	SadadTerminalID             string
	SadadTransactionKey         string
	SadadPaymentIdentityRial    string // IBAN / settlement identity for IRR payments
	SadadPaymentIdentityNonRial string // IBAN / settlement identity for non-IRR assets
	SadadCallbackURL            string
	FrontendURL                 string
	SadadSandbox                bool // BankTest sandbox omits MultiplexingData
}

func NewOrderService(
	db *sql.DB,
	orderRepo repository.OrderRepository,
	transactionRepo repository.TransactionRepository,
	paymentRepo repository.PaymentRepository,
	variableRepo repository.VariableRepository,
	firstOrderRepo repository.FirstOrderRepository,
	sadadClient SadadClient,
	orderPolicy OrderPolicy,
	jalaliConverter JalaliConverter,
	walletTopUp WalletTopUp,
	referralProcessor ReferralProcessor,
	smsClient notificationspb.SMSServiceClient,
	config OrderConfig,
) OrderService {
	return &orderService{
		db:                db,
		orderRepo:         orderRepo,
		transactionRepo:   transactionRepo,
		paymentRepo:       paymentRepo,
		variableRepo:      variableRepo,
		firstOrderRepo:    firstOrderRepo,
		sadadClient:       sadadClient,
		orderPolicy:       orderPolicy,
		jalaliConverter:   jalaliConverter,
		walletTopUp:       walletTopUp,
		referralProcessor: referralProcessor,
		smsClient:         smsClient,
		sadadConfig:       config,
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
		s.cleanupPendingOrder(ctx, order.ID, transaction.ID)
		return "", err
	}

	s.storeTransactionToken(ctx, transaction, token)
	return paymentURL, nil
}

func (s *orderService) cleanupPendingOrder(ctx context.Context, orderID uint64, transactionID string) {
	if err := s.transactionRepo.Delete(ctx, transactionID); err != nil {
		logPaymentWarning("failed to delete pending transaction %s: %v", transactionID, err)
	}
	if err := s.orderRepo.Delete(ctx, orderID); err != nil {
		logPaymentWarning("failed to delete pending order %d: %v", orderID, err)
	}
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
		Status: constants.OrderStatusPending,
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

	payableType := constants.OrderPayableType
	transaction := &models.Transaction{
		ID:          transactionID,
		UserID:      userID,
		Asset:       asset,
		Amount:      float64(amount),
		Action:      constants.TransactionActionDeposit,
		Status:      1,
		PayableType: &payableType,
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
