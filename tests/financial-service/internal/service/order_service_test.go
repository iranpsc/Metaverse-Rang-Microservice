package service_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"metarang/financial-service/internal/models"
	"metarang/financial-service/internal/sadad"
	"metarang/financial-service/internal/service"
	commercialpb "metarang/shared/pb/commercial"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Mock repositories
type mockOrderRepo struct {
	orders map[uint64]*models.Order
}

func (m *mockOrderRepo) Create(ctx context.Context, order *models.Order) error {
	if m.orders == nil {
		m.orders = make(map[uint64]*models.Order)
	}
	order.ID = uint64(len(m.orders) + 1)
	m.orders[order.ID] = order
	return nil
}

func (m *mockOrderRepo) FindByID(ctx context.Context, id uint64) (*models.Order, error) {
	if order, ok := m.orders[id]; ok {
		return order, nil
	}
	return nil, nil
}

func (m *mockOrderRepo) FindByIDWithUser(ctx context.Context, id uint64) (*models.Order, *models.User, error) {
	order, ok := m.orders[id]
	if !ok {
		return nil, nil, nil
	}
	user := &models.User{
		ID:   order.UserID,
		Name: "Test User",
	}
	return order, user, nil
}

func (m *mockOrderRepo) Update(ctx context.Context, order *models.Order) error {
	if _, ok := m.orders[order.ID]; !ok {
		return sql.ErrNoRows
	}
	m.orders[order.ID] = order
	return nil
}

type mockTransactionRepo struct {
	transactions map[string]*models.Transaction
}

func (m *mockTransactionRepo) Create(ctx context.Context, transaction *models.Transaction) error {
	if m.transactions == nil {
		m.transactions = make(map[string]*models.Transaction)
	}
	m.transactions[transaction.ID] = transaction
	return nil
}

func (m *mockTransactionRepo) Update(ctx context.Context, transaction *models.Transaction) error {
	if _, ok := m.transactions[transaction.ID]; !ok {
		return sql.ErrNoRows
	}
	m.transactions[transaction.ID] = transaction
	return nil
}

func (m *mockTransactionRepo) FindByID(ctx context.Context, id string) (*models.Transaction, error) {
	if t, ok := m.transactions[id]; ok {
		return t, nil
	}
	return nil, nil
}

func (m *mockTransactionRepo) FindByPayable(ctx context.Context, payableType string, payableID uint64) (*models.Transaction, error) {
	for _, t := range m.transactions {
		if t.PayableType != nil && *t.PayableType == payableType &&
			t.PayableID != nil && *t.PayableID == payableID {
			return t, nil
		}
	}
	return nil, nil
}

type mockPaymentRepo struct{}

func (m *mockPaymentRepo) Create(ctx context.Context, payment *models.Payment) error {
	return nil
}

type mockVariableRepo struct {
	rates map[string]float64
}

func (m *mockVariableRepo) GetRate(ctx context.Context, asset string) (float64, error) {
	if rate, ok := m.rates[asset]; ok {
		return rate, nil
	}
	return 0, sql.ErrNoRows
}

type mockFirstOrderRepo struct {
	count int
}

func (m *mockFirstOrderRepo) Create(ctx context.Context, firstOrder *models.FirstOrder) error {
	m.count++
	return nil
}

func (m *mockFirstOrderRepo) Count(ctx context.Context, userID uint64) (int, error) {
	return m.count, nil
}

type mockSadadClient struct {
	requestResponse *sadad.RequestResponse
	verifyResponse  *sadad.VerificationResponse
	requestError    error
	verifyError     error
	lastRequest     sadad.RequestParams
}

func (m *mockSadadClient) RequestPayment(params sadad.RequestParams) (*sadad.RequestResponse, error) {
	m.lastRequest = params
	if m.requestError != nil {
		return nil, m.requestError
	}
	return m.requestResponse, nil
}

func (m *mockSadadClient) VerifyPayment(params sadad.VerificationParams) (*sadad.VerificationResponse, error) {
	if m.verifyError != nil {
		return nil, m.verifyError
	}
	return m.verifyResponse, nil
}

type mockOrderPolicy struct {
	canBuy      bool
	canGetBonus bool
}

func (m *mockOrderPolicy) CanBuyFromStore(ctx context.Context, userID uint64) (bool, error) {
	return m.canBuy, nil
}

func (m *mockOrderPolicy) CanGetBonus(ctx context.Context, userID uint64, asset string) (bool, error) {
	return m.canGetBonus, nil
}

type mockJalaliConverter struct{}

func (m *mockJalaliConverter) NowJalali() string {
	return "1403/01/01"
}

func (m *mockJalaliConverter) FormatJalaliDate(t time.Time) string {
	return "1403/01/01"
}

func TestOrderService_CreateOrder(t *testing.T) {
	tests := []struct {
		name         string
		userID       uint64
		amount       int32
		asset        string
		canBuy       bool
		rate         float64
		sadadResCode string
		sadadToken   string
		expectError  bool
		errorType    error
	}{
		{
			name:         "successful order creation",
			userID:       1,
			amount:       10,
			asset:        "psc",
			canBuy:       true,
			rate:         1000.0,
			sadadResCode: "0",
			sadadToken:   "12345",
			expectError:  false,
		},
		{
			name:        "invalid amount",
			userID:      1,
			amount:      0,
			asset:       "psc",
			canBuy:      true,
			expectError: true,
			errorType:   service.ErrInvalidAmount,
		},
		{
			name:        "invalid asset",
			userID:      1,
			amount:      10,
			asset:       "invalid",
			canBuy:      true,
			expectError: true,
			errorType:   service.ErrInvalidAsset,
		},
		{
			name:        "user not eligible",
			userID:      1,
			amount:      10,
			asset:       "psc",
			canBuy:      false,
			expectError: true,
			errorType:   service.ErrUserNotEligible,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orderRepo := &mockOrderRepo{}
			transactionRepo := &mockTransactionRepo{}
			paymentRepo := &mockPaymentRepo{}
			variableRepo := &mockVariableRepo{
				rates: map[string]float64{"psc": tt.rate},
			}
			firstOrderRepo := &mockFirstOrderRepo{}
			sadadClient := &mockSadadClient{
				requestResponse: &sadad.RequestResponse{
					ResCode: tt.sadadResCode,
					Token:   tt.sadadToken,
				},
			}
			orderPolicy := &mockOrderPolicy{canBuy: tt.canBuy}
			jalaliConverter := &mockJalaliConverter{}

			config := service.OrderConfig{
				SadadMerchantID:             "test_merchant",
				SadadTerminalID:             "test_terminal",
				SadadTransactionKey:         "dGVzdC10cmFuc2FjdGlvbi1rZXk=",
				SadadPaymentIdentityRial:    "1",
				SadadPaymentIdentityNonRial: "2",
				SadadCallbackURL:            "http://localhost/api/order/callback",
				FrontendURL:                 "http://localhost:5173",
			}

			svc := service.NewOrderService(
				orderRepo,
				transactionRepo,
				paymentRepo,
				variableRepo,
				firstOrderRepo,
				sadadClient,
				orderPolicy,
				jalaliConverter,
				nil,
				config,
			)

			ctx := context.Background()
			link, err := svc.CreateOrder(ctx, tt.userID, tt.amount, tt.asset)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				if tt.errorType != nil && !errors.Is(err, tt.errorType) {
					t.Errorf("expected error type %v, got %v", tt.errorType, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if link == "" {
					t.Errorf("expected payment link but got empty")
				}
				if !strings.Contains(sadadClient.lastRequest.ReturnURL, "/api/order/callback") {
					t.Errorf("expected Sadad ReturnURL to use API callback, got %q", sadadClient.lastRequest.ReturnURL)
				}
				if strings.Contains(sadadClient.lastRequest.ReturnURL, "/payment/verify") {
					t.Errorf("Sadad ReturnURL must not point to frontend verify page, got %q", sadadClient.lastRequest.ReturnURL)
				}
			}
		})
	}
}

type mockWalletClient struct {
	addBalanceCalls []*commercialpb.AddBalanceRequest
}

func (m *mockWalletClient) GetWallet(ctx context.Context, in *commercialpb.GetWalletRequest, opts ...grpc.CallOption) (*commercialpb.WalletResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *mockWalletClient) DeductBalance(ctx context.Context, in *commercialpb.DeductBalanceRequest, opts ...grpc.CallOption) (*commercialpb.DeductBalanceResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *mockWalletClient) AddBalance(ctx context.Context, in *commercialpb.AddBalanceRequest, opts ...grpc.CallOption) (*commercialpb.AddBalanceResponse, error) {
	m.addBalanceCalls = append(m.addBalanceCalls, in)
	return &commercialpb.AddBalanceResponse{Success: true}, nil
}

func (m *mockWalletClient) LockBalance(ctx context.Context, in *commercialpb.LockBalanceRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, errors.New("not implemented")
}

func (m *mockWalletClient) UnlockBalance(ctx context.Context, in *commercialpb.UnlockBalanceRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, errors.New("not implemented")
}

func TestOrderService_HandleCallback(t *testing.T) {
	ctx := context.Background()

	orderRepo := &mockOrderRepo{}
	order := &models.Order{
		UserID: 1,
		Asset:  "psc",
		Amount: 10,
		Status: -138,
	}
	if err := orderRepo.Create(ctx, order); err != nil {
		t.Fatalf("failed to seed order: %v", err)
	}

	transactionRepo := &mockTransactionRepo{}
	transaction := &models.Transaction{
		ID:          "TR-test",
		UserID:      1,
		Asset:       "psc",
		Amount:      10,
		Action:      "deposit",
		Status:      1,
		PayableType: stringPtr("App\\Models\\Order"),
		PayableID:   &order.ID,
	}
	if err := transactionRepo.Create(ctx, transaction); err != nil {
		t.Fatalf("failed to seed transaction: %v", err)
	}

	walletClient := &mockWalletClient{}
	sadadClient := &mockSadadClient{
		verifyResponse: &sadad.VerificationResponse{
			ResCode:          "0",
			RetrivalRefNo:    "99887766",
			CardNumberMasked: "1234****5678",
		},
	}

	svc := service.NewOrderService(
		orderRepo,
		transactionRepo,
		&mockPaymentRepo{},
		&mockVariableRepo{rates: map[string]float64{"psc": 1000}},
		&mockFirstOrderRepo{},
		sadadClient,
		&mockOrderPolicy{canBuy: true, canGetBonus: false},
		&mockJalaliConverter{},
		walletClient,
		service.OrderConfig{
			SadadTransactionKey: "dGVzdC10cmFuc2FjdGlvbi1rZXk=",
			SadadCallbackURL:    "http://localhost:8000/api/order/callback",
			FrontendURL:         "http://localhost:5173",
		},
	)

	redirectURL, err := svc.HandleCallback(ctx, order.ID, "123456", "0", map[string]string{
		"PrimaryAccNo": "1234****5678",
		"order_id":     "1",
	})
	if err != nil {
		t.Fatalf("HandleCallback failed: %v", err)
	}

	if !strings.HasPrefix(redirectURL, "http://localhost:5173/payment/verify?") {
		t.Fatalf("expected frontend verify redirect, got %q", redirectURL)
	}
	if strings.Contains(redirectURL, "/api/order/callback") {
		t.Fatalf("frontend redirect must not point to API callback, got %q", redirectURL)
	}
	if strings.Contains(redirectURL, "order_id=") {
		t.Fatalf("redirect must contain only canonical OrderId, got %q", redirectURL)
	}
	if len(walletClient.addBalanceCalls) != 1 {
		t.Fatalf("expected wallet credit after verification, got %d calls", len(walletClient.addBalanceCalls))
	}
	if walletClient.addBalanceCalls[0].Amount != 10 {
		t.Fatalf("expected wallet amount 10, got %v", walletClient.addBalanceCalls[0].Amount)
	}

	updatedOrder, err := orderRepo.FindByID(ctx, order.ID)
	if err != nil {
		t.Fatalf("failed to reload order: %v", err)
	}
	if updatedOrder.Status != 0 {
		t.Fatalf("expected order status 0 after successful payment, got %d", updatedOrder.Status)
	}
}

func TestOrderService_CreateOrder_rejectsFrontendVerifyCallbackURL(t *testing.T) {
	svc := service.NewOrderService(
		&mockOrderRepo{},
		&mockTransactionRepo{},
		&mockPaymentRepo{},
		&mockVariableRepo{rates: map[string]float64{"psc": 1000}},
		&mockFirstOrderRepo{},
		&mockSadadClient{
			requestResponse: &sadad.RequestResponse{ResCode: "0", Token: "12345"},
		},
		&mockOrderPolicy{canBuy: true},
		&mockJalaliConverter{},
		nil,
		service.OrderConfig{
			SadadMerchantID:             "test_merchant",
			SadadTerminalID:             "test_terminal",
			SadadTransactionKey:         "dGVzdC10cmFuc2FjdGlvbi1rZXk=",
			SadadPaymentIdentityNonRial: "2",
			SadadCallbackURL:            "http://localhost:5173/payment/verify",
			FrontendURL:                 "http://localhost:5173",
		},
	)

	_, err := svc.CreateOrder(context.Background(), 1, 10, "psc")
	if err == nil {
		t.Fatal("expected error for frontend verify callback URL")
	}
	if !errors.Is(err, service.ErrPaymentFailed) {
		t.Fatalf("expected ErrPaymentFailed, got %v", err)
	}
}

func stringPtr(s string) *string {
	return &s
}
