package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"metargb/financial-service/internal/models"
	"metargb/financial-service/internal/parsian"
)

// Mock repositories and deps (also used by HandleCallback tests)

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

type mockParsianClient struct {
	requestResponse *parsian.RequestResponse
	verifyResponse  *parsian.VerificationResponse
	requestError    error
	verifyError     error
}

func (m *mockParsianClient) RequestPayment(params parsian.RequestParams) (*parsian.RequestResponse, error) {
	if m.requestError != nil {
		return nil, m.requestError
	}
	return m.requestResponse, nil
}

func (m *mockParsianClient) VerifyPayment(params parsian.VerificationParams) (*parsian.VerificationResponse, error) {
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

type spyWallet struct {
	addCalls []struct {
		userID uint64
		asset  string
		amt    float64
	}
}

func (s *spyWallet) AddBalance(ctx context.Context, userID uint64, asset string, amount float64) error {
	s.addCalls = append(s.addCalls, struct {
		userID uint64
		asset  string
		amt    float64
	}{userID, asset, amount})
	return nil
}

type spyReferral struct {
	calls int
}

func (s *spyReferral) ProcessReferral(ctx context.Context, buyerUserID, orderID uint64, asset string, amount float64) error {
	s.calls++
	return nil
}

type spyNotify struct {
	calls   int
	lastUID uint64
}

func (s *spyNotify) NotifyPurchaseSuccess(ctx context.Context, userID, orderID uint64, asset string, amount float64) error {
	s.calls++
	s.lastUID = userID
	return nil
}

func TestOrderService_CreateOrder(t *testing.T) {
	tests := []struct {
		name          string
		userID        uint64
		amount        int32
		asset         string
		canBuy        bool
		rate          float64
		parsianStatus int32
		parsianToken  int64
		expectError   bool
		errorType     error
	}{
		{
			name:          "successful order creation",
			userID:        1,
			amount:        10,
			asset:         "psc",
			canBuy:        true,
			rate:          1000.0,
			parsianStatus: 0,
			parsianToken:  12345,
			expectError:   false,
		},
		{
			name:        "invalid amount",
			userID:      1,
			amount:      0,
			asset:       "psc",
			canBuy:      true,
			expectError: true,
			errorType:   ErrInvalidAmount,
		},
		{
			name:        "invalid asset",
			userID:      1,
			amount:      10,
			asset:       "invalid",
			canBuy:      true,
			expectError: true,
			errorType:   ErrInvalidAsset,
		},
		{
			name:        "user not eligible",
			userID:      1,
			amount:      10,
			asset:       "psc",
			canBuy:      false,
			expectError: true,
			errorType:   ErrUserNotEligible,
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
			parsianClient := &mockParsianClient{
				requestResponse: &parsian.RequestResponse{
					Status: tt.parsianStatus,
					Token:  tt.parsianToken,
				},
			}
			orderPolicy := &mockOrderPolicy{canBuy: tt.canBuy}
			jalaliConverter := &mockJalaliConverter{}

			config := OrderConfig{
				ParsianMerchantID:            "test_merchant",
				ParsianLoanAccountMerchantID: "test_loan_merchant",
				ParsianCallbackURL:           "http://localhost/callback",
				FrontendURL:                  "http://localhost",
			}

			svc := NewOrderService(
				orderRepo,
				transactionRepo,
				paymentRepo,
				variableRepo,
				firstOrderRepo,
				parsianClient,
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
			}
		})
	}
}

func TestOrderService_HandleCallback(t *testing.T) {
	pt := func(s string) *string { return &s }
	pu := func(u uint64) *uint64 { return &u }

	const pendingStatus int32 = -138

	t.Run("status not zero marks failed", func(t *testing.T) {
		or := &mockOrderRepo{orders: map[uint64]*models.Order{
			1: {ID: 1, UserID: 10, Asset: "psc", Amount: 5, Status: pendingStatus},
		}}
		tr := &mockTransactionRepo{transactions: map[string]*models.Transaction{
			"TR-1": {
				ID: "TR-1", UserID: 10, Asset: "psc", Amount: 5, Action: "deposit",
				Status: pendingStatus, PayableType: pt("App\\Models\\Order"), PayableID: pu(1),
			},
		}}
		svc := NewOrderService(or, tr, &mockPaymentRepo{}, &mockVariableRepo{rates: map[string]float64{"psc": 1000}},
			&mockFirstOrderRepo{}, &mockParsianClient{}, &mockOrderPolicy{}, &mockJalaliConverter{},
			nil, nil, nil, OrderConfig{FrontendURL: "https://example.com"})
		u, err := svc.HandleCallback(context.Background(), 1, -1, 999, nil)
		if err != nil {
			t.Fatal(err)
		}
		if u == "" {
			t.Fatal("expected redirect URL")
		}
		if or.orders[1].Status != -1 {
			t.Fatalf("order status = %d", or.orders[1].Status)
		}
	})

	t.Run("verify error returns redirect without updating order", func(t *testing.T) {
		or := &mockOrderRepo{orders: map[uint64]*models.Order{
			1: {ID: 1, UserID: 10, Asset: "psc", Amount: 5, Status: pendingStatus},
		}}
		tr := &mockTransactionRepo{transactions: map[string]*models.Transaction{
			"TR-1": {
				ID: "TR-1", UserID: 10, Asset: "psc", Amount: 5, Action: "deposit",
				Status: pendingStatus, PayableType: pt("App\\Models\\Order"), PayableID: pu(1),
			},
		}}
		parsianClient := &mockParsianClient{verifyError: errors.New("network")}
		svc := NewOrderService(or, tr, &mockPaymentRepo{}, &mockVariableRepo{rates: map[string]float64{"psc": 1000}},
			&mockFirstOrderRepo{}, parsianClient, &mockOrderPolicy{}, &mockJalaliConverter{},
			nil, nil, nil, OrderConfig{FrontendURL: "https://example.com"})
		_, err := svc.HandleCallback(context.Background(), 1, 0, 999, nil)
		if err != nil {
			t.Fatal(err)
		}
		if or.orders[1].Status != pendingStatus {
			t.Fatalf("expected order untouched, got status %d", or.orders[1].Status)
		}
	})

	t.Run("success regular adds wallet amount only", func(t *testing.T) {
		or := &mockOrderRepo{orders: map[uint64]*models.Order{
			1: {ID: 1, UserID: 10, Asset: "psc", Amount: 5, Status: pendingStatus},
		}}
		tr := &mockTransactionRepo{transactions: map[string]*models.Transaction{
			"TR-1": {
				ID: "TR-1", UserID: 10, Asset: "psc", Amount: 5, Action: "deposit",
				Status: pendingStatus, PayableType: pt("App\\Models\\Order"), PayableID: pu(1),
			},
		}}
		parsianClient := &mockParsianClient{
			verifyResponse: &parsian.VerificationResponse{Status: 0, ReferenceID: 42},
		}
		w := &spyWallet{}
		ref := &spyReferral{}
		n := &spyNotify{}
		svc := NewOrderService(or, tr, &mockPaymentRepo{}, &mockVariableRepo{rates: map[string]float64{"psc": 1000}},
			&mockFirstOrderRepo{count: 1}, parsianClient, &mockOrderPolicy{canGetBonus: false}, &mockJalaliConverter{},
			w, ref, n, OrderConfig{ParsianMerchantID: "m", ParsianLoanAccountMerchantID: "loan", FrontendURL: "https://example.com"})
		_, err := svc.HandleCallback(context.Background(), 1, 0, 999, map[string]string{"CardMaskPan": "1234"})
		if err != nil {
			t.Fatal(err)
		}
		if len(w.addCalls) != 1 || w.addCalls[0].amt != 5 || w.addCalls[0].asset != "psc" {
			t.Fatalf("wallet calls: %+v", w.addCalls)
		}
		if ref.calls != 1 {
			t.Fatalf("expected referral for non-irr, calls=%d", ref.calls)
		}
		if n.calls != 1 || n.lastUID != 10 {
			t.Fatalf("notify calls=%d uid=%d", n.calls, n.lastUID)
		}
	})

	t.Run("success first bonus adds total with bonus", func(t *testing.T) {
		or := &mockOrderRepo{orders: map[uint64]*models.Order{
			1: {ID: 1, UserID: 10, Asset: "psc", Amount: 10, Status: pendingStatus},
		}}
		tr := &mockTransactionRepo{transactions: map[string]*models.Transaction{
			"TR-1": {
				ID: "TR-1", UserID: 10, Asset: "psc", Amount: 10, Action: "deposit",
				Status: pendingStatus, PayableType: pt("App\\Models\\Order"), PayableID: pu(1),
			},
		}}
		parsianClient := &mockParsianClient{
			verifyResponse: &parsian.VerificationResponse{Status: 0, ReferenceID: 99},
		}
		w := &spyWallet{}
		svc := NewOrderService(or, tr, &mockPaymentRepo{}, &mockVariableRepo{rates: map[string]float64{"psc": 1000}},
			&mockFirstOrderRepo{count: 0}, parsianClient, &mockOrderPolicy{canGetBonus: true}, &mockJalaliConverter{},
			w, &spyReferral{}, &spyNotify{}, OrderConfig{ParsianMerchantID: "m", ParsianLoanAccountMerchantID: "loan", FrontendURL: "https://example.com"})
		_, err := svc.HandleCallback(context.Background(), 1, 0, 999, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(w.addCalls) != 1 || w.addCalls[0].amt != 15 {
			t.Fatalf("expected 15 psc total, got %+v", w.addCalls)
		}
	})

	t.Run("irr skips referral", func(t *testing.T) {
		or := &mockOrderRepo{orders: map[uint64]*models.Order{
			1: {ID: 1, UserID: 10, Asset: "irr", Amount: 100, Status: pendingStatus},
		}}
		tr := &mockTransactionRepo{transactions: map[string]*models.Transaction{
			"TR-1": {
				ID: "TR-1", UserID: 10, Asset: "irr", Amount: 100, Action: "deposit",
				Status: pendingStatus, PayableType: pt("App\\Models\\Order"), PayableID: pu(1),
			},
		}}
		parsianClient := &mockParsianClient{
			verifyResponse: &parsian.VerificationResponse{Status: 0, ReferenceID: 1},
		}
		ref := &spyReferral{}
		svc := NewOrderService(or, tr, &mockPaymentRepo{}, &mockVariableRepo{rates: map[string]float64{"irr": 1}},
			&mockFirstOrderRepo{count: 1}, parsianClient, &mockOrderPolicy{canGetBonus: false}, &mockJalaliConverter{},
			&spyWallet{}, ref, nil, OrderConfig{ParsianMerchantID: "m", ParsianLoanAccountMerchantID: "loan", FrontendURL: "https://example.com"})
		_, err := svc.HandleCallback(context.Background(), 1, 0, 999, nil)
		if err != nil {
			t.Fatal(err)
		}
		if ref.calls != 0 {
			t.Fatalf("referral should not run for irr")
		}
	})

	t.Run("order not found", func(t *testing.T) {
		or := &mockOrderRepo{orders: map[uint64]*models.Order{}}
		tr := &mockTransactionRepo{}
		svc := NewOrderService(or, tr, &mockPaymentRepo{}, &mockVariableRepo{}, &mockFirstOrderRepo{},
			&mockParsianClient{}, &mockOrderPolicy{}, &mockJalaliConverter{},
			nil, nil, nil, OrderConfig{FrontendURL: "https://example.com"})
		_, err := svc.HandleCallback(context.Background(), 99, 0, 1, nil)
		if !errors.Is(err, ErrOrderNotFound) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("transaction missing", func(t *testing.T) {
		or := &mockOrderRepo{orders: map[uint64]*models.Order{
			1: {ID: 1, UserID: 10, Asset: "psc", Amount: 5, Status: pendingStatus},
		}}
		tr := &mockTransactionRepo{transactions: map[string]*models.Transaction{}}
		svc := NewOrderService(or, tr, &mockPaymentRepo{}, &mockVariableRepo{rates: map[string]float64{"psc": 1}},
			&mockFirstOrderRepo{}, &mockParsianClient{}, &mockOrderPolicy{}, &mockJalaliConverter{},
			nil, nil, nil, OrderConfig{FrontendURL: "https://example.com"})
		_, err := svc.HandleCallback(context.Background(), 1, 0, 1, nil)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
