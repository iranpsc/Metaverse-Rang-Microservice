package service_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"metarang/financial-service/internal/grpcclients"
	"metarang/financial-service/internal/models"
	"metarang/financial-service/internal/sadad"
	"metarang/financial-service/internal/service"
	commercialpb "metarang/shared/pb/commercial"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Mock repositories
type mockOrderRepo struct {
	orders          map[uint64]*models.Order
	createErr       error
	updateErr       error
	deleteErr       error
	findWithUserErr error
	userPhone       *string
}

func (m *mockOrderRepo) Create(ctx context.Context, order *models.Order) error {
	if m.createErr != nil {
		return m.createErr
	}
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
	if m.findWithUserErr != nil {
		return nil, nil, m.findWithUserErr
	}
	order, ok := m.orders[id]
	if !ok {
		return nil, nil, nil
	}
	phone := "09123456789"
	if m.userPhone != nil {
		phone = *m.userPhone
	}
	user := &models.User{
		ID:    order.UserID,
		Name:  "Test User",
		Phone: phone,
	}
	return order, user, nil
}

func (m *mockOrderRepo) Update(ctx context.Context, order *models.Order) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if _, ok := m.orders[order.ID]; !ok {
		return sql.ErrNoRows
	}
	m.orders[order.ID] = order
	return nil
}

func (m *mockOrderRepo) UpdateWithTx(ctx context.Context, tx *sql.Tx, order *models.Order) error {
	return m.Update(ctx, order)
}

func (m *mockOrderRepo) Delete(ctx context.Context, id uint64) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.orders, id)
	return nil
}

type mockTransactionRepo struct {
	transactions     map[string]*models.Transaction
	createErr        error
	updateErr        error
	deleteErr        error
	findByPayableErr error
}

func (m *mockTransactionRepo) Create(ctx context.Context, transaction *models.Transaction) error {
	if m.createErr != nil {
		return m.createErr
	}
	if m.transactions == nil {
		m.transactions = make(map[string]*models.Transaction)
	}
	m.transactions[transaction.ID] = transaction
	return nil
}

func (m *mockTransactionRepo) Update(ctx context.Context, transaction *models.Transaction) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	if _, ok := m.transactions[transaction.ID]; !ok {
		return sql.ErrNoRows
	}
	m.transactions[transaction.ID] = transaction
	return nil
}

func (m *mockTransactionRepo) UpdateWithTx(ctx context.Context, tx *sql.Tx, transaction *models.Transaction) error {
	return m.Update(ctx, transaction)
}

func (m *mockTransactionRepo) Delete(ctx context.Context, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.transactions, id)
	return nil
}

func (m *mockTransactionRepo) FindByID(ctx context.Context, id string) (*models.Transaction, error) {
	if t, ok := m.transactions[id]; ok {
		return t, nil
	}
	return nil, nil
}

func (m *mockTransactionRepo) FindByPayable(ctx context.Context, payableType string, payableID uint64) (*models.Transaction, error) {
	if m.findByPayableErr != nil {
		return nil, m.findByPayableErr
	}
	for _, t := range m.transactions {
		if t.PayableType != nil && *t.PayableType == payableType &&
			t.PayableID != nil && *t.PayableID == payableID {
			return t, nil
		}
	}
	return nil, nil
}

type mockPaymentRepo struct {
	createErr error
}

func (m *mockPaymentRepo) Create(ctx context.Context, payment *models.Payment) error {
	if m.createErr != nil {
		return m.createErr
	}
	return nil
}

func (m *mockPaymentRepo) CreateWithTx(ctx context.Context, tx *sql.Tx, payment *models.Payment) error {
	return m.Create(ctx, payment)
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
	count     int
	createErr error
	countErr  error
}

func (m *mockFirstOrderRepo) Create(ctx context.Context, firstOrder *models.FirstOrder) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.count++
	return nil
}

func (m *mockFirstOrderRepo) CreateWithTx(ctx context.Context, tx *sql.Tx, firstOrder *models.FirstOrder) error {
	return m.Create(ctx, firstOrder)
}

func (m *mockFirstOrderRepo) Count(ctx context.Context, userID uint64) (int, error) {
	if m.countErr != nil {
		return 0, m.countErr
	}
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
	canBuy         bool
	canGetBonus    bool
	canBuyErr      error
	canGetBonusErr error
}

func (m *mockOrderPolicy) CanBuyFromStore(ctx context.Context, userID uint64) (bool, error) {
	if m.canBuyErr != nil {
		return false, m.canBuyErr
	}
	return m.canBuy, nil
}

func (m *mockOrderPolicy) CanGetBonus(ctx context.Context, userID uint64, asset string) (bool, error) {
	if m.canGetBonusErr != nil {
		return false, m.canGetBonusErr
	}
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
				nil,
				orderRepo,
				transactionRepo,
				paymentRepo,
				variableRepo,
				firstOrderRepo,
				sadadClient,
				orderPolicy,
				jalaliConverter,
				nil,
				nil,
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
				mux := sadadClient.lastRequest.MultiplexingData
				if mux == nil || mux.Type != "Amount" || len(mux.MultiplexingRows) != 1 {
					t.Fatalf("expected Amount MultiplexingData with 1 row, got %+v", mux)
				}
				if mux.MultiplexingRows[0].IbanNumber != "2" {
					t.Errorf("expected non-IRR IBAN for non-IRR asset, got %+v", mux.MultiplexingRows[0])
				}
				if mux.MultiplexingRows[0].Value <= 0 {
					t.Errorf("expected positive Amount multiplexing value, got %+v", mux.MultiplexingRows[0])
				}
			}
		})
	}
}

type mockWalletClient struct {
	addBalanceCalls []*commercialpb.AddBalanceRequest
	addErr          error
}

func (m *mockWalletClient) GetWallet(ctx context.Context, in *commercialpb.GetWalletRequest, opts ...grpc.CallOption) (*commercialpb.WalletResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *mockWalletClient) CreateWallet(ctx context.Context, in *commercialpb.CreateWalletRequest, opts ...grpc.CallOption) (*commercialpb.WalletResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *mockWalletClient) DeductBalance(ctx context.Context, in *commercialpb.DeductBalanceRequest, opts ...grpc.CallOption) (*commercialpb.DeductBalanceResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *mockWalletClient) AddBalance(ctx context.Context, in *commercialpb.AddBalanceRequest, opts ...grpc.CallOption) (*commercialpb.AddBalanceResponse, error) {
	if m.addErr != nil {
		return nil, m.addErr
	}
	m.addBalanceCalls = append(m.addBalanceCalls, in)
	return &commercialpb.AddBalanceResponse{Success: true}, nil
}

func (m *mockWalletClient) LockBalance(ctx context.Context, in *commercialpb.LockBalanceRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, errors.New("not implemented")
}

func (m *mockWalletClient) UnlockBalance(ctx context.Context, in *commercialpb.UnlockBalanceRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	return nil, errors.New("not implemented")
}

var _ commercialpb.WalletServiceClient = (*mockWalletClient)(nil)

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
			ResCode:       "0",
			RetrivalRefNo: "99887766",
		},
	}

	svc := service.NewOrderService(
		nil,
		orderRepo,
		transactionRepo,
		&mockPaymentRepo{},
		&mockVariableRepo{rates: map[string]float64{"psc": 1000}},
		&mockFirstOrderRepo{},
		sadadClient,
		&mockOrderPolicy{canBuy: true, canGetBonus: false},
		&mockJalaliConverter{},
		&grpcclients.WalletAdapter{Client: walletClient},
		nil,
		nil,
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

func TestOrderService_HandleCallback_verifyErrorMarksFailedAndRedirects(t *testing.T) {
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
		ID:          "TR-verify-fail",
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

	svc := service.NewOrderService(
		nil,
		orderRepo,
		transactionRepo,
		&mockPaymentRepo{},
		&mockVariableRepo{rates: map[string]float64{"psc": 1000}},
		&mockFirstOrderRepo{},
		&mockSadadClient{verifyError: errors.New("gateway timeout")},
		&mockOrderPolicy{canBuy: true},
		&mockJalaliConverter{},
		nil,
		nil,
		nil,
		service.OrderConfig{
			SadadTransactionKey: "dGVzdC10cmFuc2FjdGlvbi1rZXk=",
			SadadCallbackURL:    "http://localhost:8000/api/order/callback",
			FrontendURL:         "http://localhost:5173",
		},
	)

	redirectURL, err := svc.HandleCallback(ctx, order.ID, "123456", "0", nil)
	if err != nil {
		t.Fatalf("HandleCallback should redirect after verify failure, got error: %v", err)
	}
	if !strings.Contains(redirectURL, "ResCode=-1") {
		t.Fatalf("expected redirect with ResCode=-1, got %q", redirectURL)
	}

	updatedOrder, _ := orderRepo.FindByID(ctx, order.ID)
	if updatedOrder.Status != -1 {
		t.Fatalf("expected order status -1 after verify error, got %d", updatedOrder.Status)
	}
}

func TestOrderService_HandleCallback_verifyDeclinedMarksFailedAndRedirects(t *testing.T) {
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
		ID:          "TR-verify-decline",
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

	svc := service.NewOrderService(
		nil,
		orderRepo,
		transactionRepo,
		&mockPaymentRepo{},
		&mockVariableRepo{rates: map[string]float64{"psc": 1000}},
		&mockFirstOrderRepo{},
		&mockSadadClient{
			verifyResponse: &sadad.VerificationResponse{ResCode: "101"},
		},
		&mockOrderPolicy{canBuy: true},
		&mockJalaliConverter{},
		nil,
		nil,
		nil,
		service.OrderConfig{
			SadadTransactionKey: "dGVzdC10cmFuc2FjdGlvbi1rZXk=",
			SadadCallbackURL:    "http://localhost:8000/api/order/callback",
			FrontendURL:         "http://localhost:5173",
		},
	)

	redirectURL, err := svc.HandleCallback(ctx, order.ID, "123456", "0", nil)
	if err != nil {
		t.Fatalf("HandleCallback should redirect after verify decline, got error: %v", err)
	}
	if !strings.Contains(redirectURL, "ResCode=101") {
		t.Fatalf("expected redirect with ResCode=101, got %q", redirectURL)
	}

	updatedOrder, _ := orderRepo.FindByID(ctx, order.ID)
	if updatedOrder.Status != 101 {
		t.Fatalf("expected order status 101 after verify decline, got %d", updatedOrder.Status)
	}
}

func TestOrderService_CreateOrder_rejectsFrontendVerifyCallbackURL(t *testing.T) {
	svc := service.NewOrderService(
		nil,
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
		nil,
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

func TestOrderService_CreateOrder_cleansUpPendingRecordsOnSadadFailure(t *testing.T) {
	orderRepo := &mockOrderRepo{}
	transactionRepo := &mockTransactionRepo{}

	svc := service.NewOrderService(
		nil,
		orderRepo,
		transactionRepo,
		&mockPaymentRepo{},
		&mockVariableRepo{rates: map[string]float64{"psc": 1000}},
		&mockFirstOrderRepo{},
		&mockSadadClient{requestError: errors.New("gateway unavailable")},
		&mockOrderPolicy{canBuy: true},
		&mockJalaliConverter{},
		nil,
		nil,
		nil,
		service.OrderConfig{
			SadadMerchantID:             "test_merchant",
			SadadTerminalID:             "test_terminal",
			SadadTransactionKey:         "dGVzdC10cmFuc2FjdGlvbi1rZXk=",
			SadadPaymentIdentityRial:    "1",
			SadadPaymentIdentityNonRial: "2",
			SadadCallbackURL:            "http://localhost/api/order/callback",
			FrontendURL:                 "http://localhost:5173",
		},
	)

	_, err := svc.CreateOrder(context.Background(), 1, 10, "psc")
	if err == nil {
		t.Fatal("expected Sadad failure error")
	}

	if len(orderRepo.orders) != 0 {
		t.Fatalf("expected pending order cleanup, still have %d orders", len(orderRepo.orders))
	}
	if len(transactionRepo.transactions) != 0 {
		t.Fatalf("expected pending transaction cleanup, still have %d transactions", len(transactionRepo.transactions))
	}
}

func stringPtr(s string) *string {
	return &s
}
