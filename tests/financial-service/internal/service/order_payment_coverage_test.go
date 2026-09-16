package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	"metarang/financial-service/internal/constants"
	"metarang/financial-service/internal/grpcclients"
	"metarang/financial-service/internal/models"
	"metarang/financial-service/internal/sadad"
	"metarang/financial-service/internal/service"
	commercialpb "metarang/shared/pb/commercial"
)

type mockReferralProcessor struct {
	err    error
	called bool
}

func (m *mockReferralProcessor) ProcessReferral(ctx context.Context, buyerUserID, orderID uint64, asset string, amount float64) error {
	m.called = true
	return m.err
}

func defaultOrderConfig() service.OrderConfig {
	return service.OrderConfig{
		SadadMerchantID:             "m",
		SadadTerminalID:             "t",
		SadadTransactionKey:         "dGVzdC10cmFuc2FjdGlvbi1rZXk=",
		SadadPaymentIdentityRial:    "IRRIAL",
		SadadPaymentIdentityNonRial: "IRNON",
		SadadCallbackURL:            "http://localhost/api/order/callback",
		FrontendURL:                 "http://localhost:5173",
	}
}

func seedCallbackOrder(t *testing.T, asset string, amount float64, token int64) (*mockOrderRepo, *mockTransactionRepo, *models.Order) {
	t.Helper()
	ctx := context.Background()
	orderRepo := &mockOrderRepo{}
	order := &models.Order{UserID: 1, Asset: asset, Amount: amount, Status: constants.OrderStatusPending}
	if err := orderRepo.Create(ctx, order); err != nil {
		t.Fatal(err)
	}
	transactionRepo := &mockTransactionRepo{}
	payable := constants.OrderPayableType
	tx := &models.Transaction{
		ID:          "TR-cov",
		UserID:      1,
		Asset:       asset,
		Amount:      amount,
		Action:      "deposit",
		Status:      1,
		Token:       &token,
		PayableType: &payable,
		PayableID:   &order.ID,
	}
	if err := transactionRepo.Create(ctx, tx); err != nil {
		t.Fatal(err)
	}
	return orderRepo, transactionRepo, order
}

func TestCreateOrder_additionalBranches(t *testing.T) {
	ctx := context.Background()

	t.Run("sandbox omits multiplexing", func(t *testing.T) {
		sadadClient := &mockSadadClient{requestResponse: &sadad.RequestResponse{ResCode: "0", Token: "99"}}
		cfg := defaultOrderConfig()
		cfg.SadadSandbox = true
		svc := service.NewOrderService(nil, &mockOrderRepo{}, &mockTransactionRepo{}, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"psc": 1000}}, &mockFirstOrderRepo{},
			sadadClient, &mockOrderPolicy{canBuy: true}, &mockJalaliConverter{}, nil, nil, nil, cfg)
		if _, err := svc.CreateOrder(ctx, 1, 5, "psc"); err != nil {
			t.Fatal(err)
		}
		if sadadClient.lastRequest.MultiplexingData != nil {
			t.Fatal("sandbox should omit multiplexing")
		}
	})

	t.Run("irr multiplexing amount to rial iban", func(t *testing.T) {
		sadadClient := &mockSadadClient{requestResponse: &sadad.RequestResponse{ResCode: "0", Token: "99"}}
		svc := service.NewOrderService(nil, &mockOrderRepo{}, &mockTransactionRepo{}, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"irr": 1}}, &mockFirstOrderRepo{},
			sadadClient, &mockOrderPolicy{canBuy: true}, &mockJalaliConverter{}, nil, nil, nil, defaultOrderConfig())
		if _, err := svc.CreateOrder(ctx, 1, 5, "irr"); err != nil {
			t.Fatal(err)
		}
		mux := sadadClient.lastRequest.MultiplexingData
		if mux == nil || mux.Type != "Amount" || len(mux.MultiplexingRows) != 1 {
			t.Fatalf("expected Amount multiplexing with 1 row, got %+v", mux)
		}
		if mux.MultiplexingRows[0].IbanNumber != "IRRIAL" || mux.MultiplexingRows[0].Value != 5 {
			t.Fatalf("unexpected irr multiplexing row %+v", mux.MultiplexingRows[0])
		}
	})

	t.Run("non-irr multiplexing amount to non-rial iban", func(t *testing.T) {
		sadadClient := &mockSadadClient{requestResponse: &sadad.RequestResponse{ResCode: "0", Token: "99"}}
		svc := service.NewOrderService(nil, &mockOrderRepo{}, &mockTransactionRepo{}, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"psc": 1000}}, &mockFirstOrderRepo{},
			sadadClient, &mockOrderPolicy{canBuy: true}, &mockJalaliConverter{}, nil, nil, nil, defaultOrderConfig())
		if _, err := svc.CreateOrder(ctx, 1, 5, "psc"); err != nil {
			t.Fatal(err)
		}
		mux := sadadClient.lastRequest.MultiplexingData
		if mux == nil || mux.Type != "Amount" || len(mux.MultiplexingRows) != 1 {
			t.Fatalf("expected Amount multiplexing with 1 row, got %+v", mux)
		}
		if mux.MultiplexingRows[0].IbanNumber != "IRNON" || mux.MultiplexingRows[0].Value != 5000 {
			t.Fatalf("unexpected non-irr multiplexing row %+v", mux.MultiplexingRows[0])
		}
	})

	t.Run("missing IBANs", func(t *testing.T) {
		cfg := defaultOrderConfig()
		cfg.SadadPaymentIdentityRial = ""
		svc := service.NewOrderService(nil, &mockOrderRepo{}, &mockTransactionRepo{}, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"psc": 1000}}, &mockFirstOrderRepo{},
			&mockSadadClient{requestResponse: &sadad.RequestResponse{ResCode: "0", Token: "1"}},
			&mockOrderPolicy{canBuy: true}, &mockJalaliConverter{}, nil, nil, nil, cfg)
		_, err := svc.CreateOrder(ctx, 1, 5, "psc")
		if !errors.Is(err, service.ErrPaymentFailed) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("empty callback URL", func(t *testing.T) {
		cfg := defaultOrderConfig()
		cfg.SadadCallbackURL = ""
		svc := service.NewOrderService(nil, &mockOrderRepo{}, &mockTransactionRepo{}, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"psc": 1000}}, &mockFirstOrderRepo{},
			&mockSadadClient{requestResponse: &sadad.RequestResponse{ResCode: "0", Token: "1"}},
			&mockOrderPolicy{canBuy: true}, &mockJalaliConverter{}, nil, nil, nil, cfg)
		_, err := svc.CreateOrder(ctx, 1, 5, "psc")
		if !errors.Is(err, service.ErrPaymentFailed) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("sadad failure uses description then error message", func(t *testing.T) {
		svc := service.NewOrderService(nil, &mockOrderRepo{}, &mockTransactionRepo{}, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"psc": 1000}}, &mockFirstOrderRepo{},
			&mockSadadClient{requestResponse: &sadad.RequestResponse{ResCode: "101", Description: "bad merchant"}},
			&mockOrderPolicy{canBuy: true}, &mockJalaliConverter{}, nil, nil, nil, defaultOrderConfig())
		_, err := svc.CreateOrder(ctx, 1, 5, "psc")
		if !errors.Is(err, service.ErrPaymentFailed) {
			t.Fatalf("err=%v", err)
		}
		if !strings.Contains(err.Error(), "bad merchant") {
			t.Fatalf("expected description in error, got %v", err)
		}

		svc = service.NewOrderService(nil, &mockOrderRepo{}, &mockTransactionRepo{}, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"psc": 1000}}, &mockFirstOrderRepo{},
			&mockSadadClient{requestResponse: &sadad.RequestResponse{ResCode: "102"}},
			&mockOrderPolicy{canBuy: true}, &mockJalaliConverter{}, nil, nil, nil, defaultOrderConfig())
		_, err = svc.CreateOrder(ctx, 1, 5, "psc")
		if !errors.Is(err, service.ErrPaymentFailed) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("non numeric token is ignored", func(t *testing.T) {
		svc := service.NewOrderService(nil, &mockOrderRepo{}, &mockTransactionRepo{}, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"psc": 1000}}, &mockFirstOrderRepo{},
			&mockSadadClient{requestResponse: &sadad.RequestResponse{ResCode: "0", Token: "not-an-int"}},
			&mockOrderPolicy{canBuy: true}, &mockJalaliConverter{}, nil, nil, nil, defaultOrderConfig())
		if _, err := svc.CreateOrder(ctx, 1, 5, "psc"); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("token update failure is non fatal", func(t *testing.T) {
		txRepo := &mockTransactionRepo{updateErr: errors.New("token write failed")}
		svc := service.NewOrderService(nil, &mockOrderRepo{}, txRepo, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"psc": 1000}}, &mockFirstOrderRepo{},
			&mockSadadClient{requestResponse: &sadad.RequestResponse{ResCode: "0", Token: "555"}},
			&mockOrderPolicy{canBuy: true}, &mockJalaliConverter{}, nil, nil, nil, defaultOrderConfig())
		if _, err := svc.CreateOrder(ctx, 1, 5, "psc"); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("policy error", func(t *testing.T) {
		svc := service.NewOrderService(nil, &mockOrderRepo{}, &mockTransactionRepo{}, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"psc": 1000}}, &mockFirstOrderRepo{},
			&mockSadadClient{}, &mockOrderPolicy{canBuyErr: errors.New("policy down")}, &mockJalaliConverter{},
			nil, nil, nil, defaultOrderConfig())
		if _, err := svc.CreateOrder(ctx, 1, 5, "psc"); err == nil {
			t.Fatal("expected policy error")
		}
	})

	t.Run("rate missing", func(t *testing.T) {
		svc := service.NewOrderService(nil, &mockOrderRepo{}, &mockTransactionRepo{}, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{}}, &mockFirstOrderRepo{},
			&mockSadadClient{}, &mockOrderPolicy{canBuy: true}, &mockJalaliConverter{},
			nil, nil, nil, defaultOrderConfig())
		if _, err := svc.CreateOrder(ctx, 1, 5, "psc"); err == nil {
			t.Fatal("expected rate error")
		}
	})

	t.Run("order create error", func(t *testing.T) {
		svc := service.NewOrderService(nil, &mockOrderRepo{createErr: errors.New("insert failed")}, &mockTransactionRepo{},
			&mockPaymentRepo{}, &mockVariableRepo{rates: map[string]float64{"psc": 1000}}, &mockFirstOrderRepo{},
			&mockSadadClient{}, &mockOrderPolicy{canBuy: true}, &mockJalaliConverter{},
			nil, nil, nil, defaultOrderConfig())
		if _, err := svc.CreateOrder(ctx, 1, 5, "psc"); err == nil {
			t.Fatal("expected create error")
		}
	})

	t.Run("transaction create error", func(t *testing.T) {
		svc := service.NewOrderService(nil, &mockOrderRepo{}, &mockTransactionRepo{createErr: errors.New("tx insert failed")},
			&mockPaymentRepo{}, &mockVariableRepo{rates: map[string]float64{"psc": 1000}}, &mockFirstOrderRepo{},
			&mockSadadClient{}, &mockOrderPolicy{canBuy: true}, &mockJalaliConverter{},
			nil, nil, nil, defaultOrderConfig())
		if _, err := svc.CreateOrder(ctx, 1, 5, "psc"); err == nil {
			t.Fatal("expected transaction create error")
		}
	})

	t.Run("cleanup logs delete errors", func(t *testing.T) {
		svc := service.NewOrderService(nil,
			&mockOrderRepo{deleteErr: errors.New("order delete failed")},
			&mockTransactionRepo{deleteErr: errors.New("tx delete failed")},
			&mockPaymentRepo{}, &mockVariableRepo{rates: map[string]float64{"psc": 1000}}, &mockFirstOrderRepo{},
			&mockSadadClient{requestError: errors.New("gateway down")},
			&mockOrderPolicy{canBuy: true}, &mockJalaliConverter{},
			nil, nil, nil, defaultOrderConfig())
		if _, err := svc.CreateOrder(ctx, 1, 5, "psc"); err == nil {
			t.Fatal("expected payment error")
		}
	})
}

func TestHandleCallback_additionalBranches(t *testing.T) {
	ctx := context.Background()
	verifyOK := &sadad.VerificationResponse{ResCode: "0", RetrivalRefNo: "111", CardNumberMasked: "****1111"}

	t.Run("order lookup error", func(t *testing.T) {
		svc := service.NewOrderService(nil, &mockOrderRepo{findWithUserErr: errors.New("db")}, &mockTransactionRepo{},
			&mockPaymentRepo{}, &mockVariableRepo{}, &mockFirstOrderRepo{}, &mockSadadClient{},
			&mockOrderPolicy{}, &mockJalaliConverter{}, nil, nil, nil, defaultOrderConfig())
		if _, err := svc.HandleCallback(ctx, 9, "t", "0", nil); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("order not found", func(t *testing.T) {
		svc := service.NewOrderService(nil, &mockOrderRepo{}, &mockTransactionRepo{},
			&mockPaymentRepo{}, &mockVariableRepo{}, &mockFirstOrderRepo{}, &mockSadadClient{},
			&mockOrderPolicy{}, &mockJalaliConverter{}, nil, nil, nil, defaultOrderConfig())
		_, err := svc.HandleCallback(ctx, 99, "t", "0", nil)
		if !errors.Is(err, service.ErrOrderNotFound) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("transaction lookup error", func(t *testing.T) {
		orderRepo, txRepo, order := seedCallbackOrder(t, "psc", 10, 123)
		txRepo.findByPayableErr = errors.New("tx query failed")
		svc := service.NewOrderService(nil, orderRepo, txRepo, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"psc": 1}}, &mockFirstOrderRepo{},
			&mockSadadClient{}, &mockOrderPolicy{}, &mockJalaliConverter{}, nil, nil, nil, defaultOrderConfig())
		if _, err := svc.HandleCallback(ctx, order.ID, "t", "0", nil); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("transaction missing", func(t *testing.T) {
		orderRepo := &mockOrderRepo{}
		order := &models.Order{UserID: 1, Asset: "psc", Amount: 10, Status: -138}
		_ = orderRepo.Create(ctx, order)
		svc := service.NewOrderService(nil, orderRepo, &mockTransactionRepo{}, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"psc": 1}}, &mockFirstOrderRepo{},
			&mockSadadClient{}, &mockOrderPolicy{}, &mockJalaliConverter{}, nil, nil, nil, defaultOrderConfig())
		if _, err := svc.HandleCallback(ctx, order.ID, "t", "0", nil); err == nil {
			t.Fatal("expected missing transaction")
		}
	})

	t.Run("failed resCode marks order failed", func(t *testing.T) {
		orderRepo, txRepo, order := seedCallbackOrder(t, "psc", 10, 123)
		svc := service.NewOrderService(nil, orderRepo, txRepo, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"psc": 1}}, &mockFirstOrderRepo{},
			&mockSadadClient{}, &mockOrderPolicy{}, &mockJalaliConverter{}, nil, nil, nil, defaultOrderConfig())
		url, err := svc.HandleCallback(ctx, order.ID, "t", "101", nil)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(url, "ResCode=101") {
			t.Fatalf("url=%s", url)
		}
	})

	t.Run("failed resCode order update error", func(t *testing.T) {
		orderRepo, txRepo, order := seedCallbackOrder(t, "psc", 10, 123)
		orderRepo.updateErr = errors.New("order update failed")
		svc := service.NewOrderService(nil, orderRepo, txRepo, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"psc": 1}}, &mockFirstOrderRepo{},
			&mockSadadClient{}, &mockOrderPolicy{}, &mockJalaliConverter{}, nil, nil, nil, defaultOrderConfig())
		if _, err := svc.HandleCallback(ctx, order.ID, "t", "7", nil); err == nil {
			t.Fatal("expected update error")
		}
	})

	t.Run("failed resCode transaction update error", func(t *testing.T) {
		orderRepo, txRepo, order := seedCallbackOrder(t, "psc", 10, 123)
		txRepo.updateErr = errors.New("tx update failed")
		svc := service.NewOrderService(nil, orderRepo, txRepo, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"psc": 1}}, &mockFirstOrderRepo{},
			&mockSadadClient{}, &mockOrderPolicy{}, &mockJalaliConverter{}, nil, nil, nil, defaultOrderConfig())
		if _, err := svc.HandleCallback(ctx, order.ID, "t", "7", nil); err == nil {
			t.Fatal("expected tx update error")
		}
	})

	t.Run("verify uses stored token when request token empty", func(t *testing.T) {
		orderRepo, txRepo, order := seedCallbackOrder(t, "psc", 10, 777)
		sadadClient := &mockSadadClient{verifyResponse: verifyOK}
		svc := service.NewOrderService(nil, orderRepo, txRepo, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"psc": 1000}}, &mockFirstOrderRepo{},
			sadadClient, &mockOrderPolicy{canGetBonus: false}, &mockJalaliConverter{},
			&grpcclients.WalletAdapter{Client: &mockWalletClient{}}, nil, nil, defaultOrderConfig())
		if _, err := svc.HandleCallback(ctx, order.ID, "", "0", nil); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("card pan from CardMaskPan then card_pan then masked then default", func(t *testing.T) {
		cases := []struct {
			name   string
			params map[string]string
			masked string
		}{
			{"CardMaskPan", map[string]string{"CardMaskPan": "mask-a"}, ""},
			{"card_pan", map[string]string{"card_pan": "mask-b"}, ""},
			{"verify masked", map[string]string{}, "mask-c"},
			{"default", map[string]string{}, ""},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				orderRepo, txRepo, order := seedCallbackOrder(t, "psc", 10, 1)
				svc := service.NewOrderService(nil, orderRepo, txRepo, &mockPaymentRepo{},
					&mockVariableRepo{rates: map[string]float64{"psc": 1}}, &mockFirstOrderRepo{},
					&mockSadadClient{verifyResponse: &sadad.VerificationResponse{ResCode: "0", RetrivalRefNo: "9", CardNumberMasked: tc.masked}},
					&mockOrderPolicy{}, &mockJalaliConverter{},
					&grpcclients.WalletAdapter{Client: &mockWalletClient{}}, nil, nil, defaultOrderConfig())
				if _, err := svc.HandleCallback(ctx, order.ID, "tok", "0", tc.params); err != nil {
					t.Fatal(err)
				}
			})
		}
	})

	t.Run("bonus credits extra wallet amount and creates first order", func(t *testing.T) {
		orderRepo, txRepo, order := seedCallbackOrder(t, "yellow", 10, 1)
		wallet := &mockWalletClient{}
		first := &mockFirstOrderRepo{}
		svc := service.NewOrderService(nil, orderRepo, txRepo, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"yellow": 2}}, first,
			&mockSadadClient{verifyResponse: verifyOK},
			&mockOrderPolicy{canGetBonus: true}, &mockJalaliConverter{},
			&grpcclients.WalletAdapter{Client: wallet}, nil, nil, defaultOrderConfig())
		if _, err := svc.HandleCallback(ctx, order.ID, "tok", "0", nil); err != nil {
			t.Fatal(err)
		}
		if first.count != 1 {
			t.Fatalf("expected first order record, count=%d", first.count)
		}
		want := 10 + 10*constants.FirstOrderBonusRate
		if wallet.addBalanceCalls[0].Amount != want {
			t.Fatalf("wallet amount=%v want=%v", wallet.addBalanceCalls[0].Amount, want)
		}
	})

	t.Run("bonus eligibility error", func(t *testing.T) {
		orderRepo, txRepo, order := seedCallbackOrder(t, "psc", 10, 1)
		svc := service.NewOrderService(nil, orderRepo, txRepo, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"psc": 1}}, &mockFirstOrderRepo{},
			&mockSadadClient{verifyResponse: verifyOK},
			&mockOrderPolicy{canGetBonusErr: errors.New("bonus check failed")}, &mockJalaliConverter{},
			&grpcclients.WalletAdapter{Client: &mockWalletClient{}}, nil, nil, defaultOrderConfig())
		if _, err := svc.HandleCallback(ctx, order.ID, "tok", "0", nil); err == nil {
			t.Fatal("expected bonus error")
		}
	})

	t.Run("payment create error", func(t *testing.T) {
		orderRepo, txRepo, order := seedCallbackOrder(t, "psc", 10, 1)
		svc := service.NewOrderService(nil, orderRepo, txRepo, &mockPaymentRepo{createErr: errors.New("pay insert")},
			&mockVariableRepo{rates: map[string]float64{"psc": 1}}, &mockFirstOrderRepo{},
			&mockSadadClient{verifyResponse: verifyOK},
			&mockOrderPolicy{}, &mockJalaliConverter{},
			&grpcclients.WalletAdapter{Client: &mockWalletClient{}}, nil, nil, defaultOrderConfig())
		if _, err := svc.HandleCallback(ctx, order.ID, "tok", "0", nil); err == nil {
			t.Fatal("expected payment create error")
		}
	})

	t.Run("first order create error", func(t *testing.T) {
		orderRepo, txRepo, order := seedCallbackOrder(t, "psc", 10, 1)
		svc := service.NewOrderService(nil, orderRepo, txRepo, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"psc": 1}}, &mockFirstOrderRepo{createErr: errors.New("fo insert")},
			&mockSadadClient{verifyResponse: verifyOK},
			&mockOrderPolicy{canGetBonus: true}, &mockJalaliConverter{},
			&grpcclients.WalletAdapter{Client: &mockWalletClient{}}, nil, nil, defaultOrderConfig())
		if _, err := svc.HandleCallback(ctx, order.ID, "tok", "0", nil); err == nil {
			t.Fatal("expected first order create error")
		}
	})

	t.Run("wallet not configured", func(t *testing.T) {
		orderRepo, txRepo, order := seedCallbackOrder(t, "psc", 10, 1)
		svc := service.NewOrderService(nil, orderRepo, txRepo, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"psc": 1}}, &mockFirstOrderRepo{},
			&mockSadadClient{verifyResponse: verifyOK},
			&mockOrderPolicy{}, &mockJalaliConverter{},
			nil, nil, nil, defaultOrderConfig())
		if _, err := svc.HandleCallback(ctx, order.ID, "tok", "0", nil); err == nil {
			t.Fatal("expected wallet client error")
		}
	})

	t.Run("wallet add fails", func(t *testing.T) {
		orderRepo, txRepo, order := seedCallbackOrder(t, "psc", 10, 1)
		svc := service.NewOrderService(nil, orderRepo, txRepo, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"psc": 1}}, &mockFirstOrderRepo{},
			&mockSadadClient{verifyResponse: verifyOK},
			&mockOrderPolicy{}, &mockJalaliConverter{},
			&grpcclients.WalletAdapter{Client: &mockWalletClient{addErr: errors.New("wallet down")}},
			nil, nil, defaultOrderConfig())
		if _, err := svc.HandleCallback(ctx, order.ID, "tok", "0", nil); err == nil {
			t.Fatal("expected wallet error")
		}
	})

	t.Run("referral success and warning", func(t *testing.T) {
		orderRepo, txRepo, order := seedCallbackOrder(t, "psc", 10, 1)
		ref := &mockReferralProcessor{}
		svc := service.NewOrderService(nil, orderRepo, txRepo, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"psc": 1}}, &mockFirstOrderRepo{},
			&mockSadadClient{verifyResponse: verifyOK},
			&mockOrderPolicy{}, &mockJalaliConverter{},
			&grpcclients.WalletAdapter{Client: &mockWalletClient{}}, ref, nil, defaultOrderConfig())
		if _, err := svc.HandleCallback(ctx, order.ID, "tok", "0", nil); err != nil {
			t.Fatal(err)
		}
		if !ref.called {
			t.Fatal("expected referral")
		}

		orderRepo, txRepo, order = seedCallbackOrder(t, "psc", 10, 1)
		ref = &mockReferralProcessor{err: errors.New("ref failed")}
		svc = service.NewOrderService(nil, orderRepo, txRepo, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"psc": 1}}, &mockFirstOrderRepo{},
			&mockSadadClient{verifyResponse: verifyOK},
			&mockOrderPolicy{}, &mockJalaliConverter{},
			&grpcclients.WalletAdapter{Client: &mockWalletClient{}}, ref, nil, defaultOrderConfig())
		if _, err := svc.HandleCallback(ctx, order.ID, "tok", "0", nil); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("sms skipped for empty phone and logs send errors", func(t *testing.T) {
		empty := ""
		orderRepo, txRepo, order := seedCallbackOrder(t, "red", 10.5, 1)
		orderRepo.userPhone = &empty
		sms := &mockSMSServiceClient{}
		svc := service.NewOrderService(nil, orderRepo, txRepo, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"red": 1.5}}, &mockFirstOrderRepo{},
			&mockSadadClient{verifyResponse: verifyOK},
			&mockOrderPolicy{}, &mockJalaliConverter{},
			&grpcclients.WalletAdapter{Client: &mockWalletClient{}}, nil, sms, defaultOrderConfig())
		if _, err := svc.HandleCallback(ctx, order.ID, "tok", "0", nil); err != nil {
			t.Fatal(err)
		}
		if sms.lastRequest != nil {
			t.Fatal("expected SMS skip for empty phone")
		}

		orderRepo, txRepo, order = seedCallbackOrder(t, "blue", 3, 1)
		sms = &mockSMSServiceClient{sendErr: errors.New("sms down")}
		svc = service.NewOrderService(nil, orderRepo, txRepo, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"blue": 2}}, &mockFirstOrderRepo{},
			&mockSadadClient{verifyResponse: verifyOK},
			&mockOrderPolicy{}, &mockJalaliConverter{},
			&grpcclients.WalletAdapter{Client: &mockWalletClient{}}, nil, sms, defaultOrderConfig())
		if _, err := svc.HandleCallback(ctx, order.ID, "tok", "0", nil); err != nil {
			t.Fatal(err)
		}

		orderRepo, txRepo, order = seedCallbackOrder(t, "irr", 4, 1)
		sms = &mockSMSServiceClient{}
		svc = service.NewOrderService(nil, orderRepo, txRepo, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"irr": 1}}, &mockFirstOrderRepo{},
			&mockSadadClient{verifyResponse: verifyOK},
			&mockOrderPolicy{}, &mockJalaliConverter{},
			&grpcclients.WalletAdapter{Client: &mockWalletClient{}}, nil, sms, defaultOrderConfig())
		if _, err := svc.HandleCallback(ctx, order.ID, "tok", "0", nil); err != nil {
			t.Fatal(err)
		}
		if sms.lastRequest.Tokens["token10"] != "ریال" {
			t.Fatalf("token10=%q", sms.lastRequest.Tokens["token10"])
		}

		orderRepo, txRepo, order = seedCallbackOrder(t, "custom-asset", 4, 1)
		sms = &mockSMSServiceClient{}
		svc = service.NewOrderService(nil, orderRepo, txRepo, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"custom-asset": 1}}, &mockFirstOrderRepo{},
			&mockSadadClient{verifyResponse: verifyOK},
			&mockOrderPolicy{}, &mockJalaliConverter{},
			&grpcclients.WalletAdapter{Client: &mockWalletClient{}}, nil, sms, defaultOrderConfig())
		if _, err := svc.HandleCallback(ctx, order.ID, "tok", "0", nil); err != nil {
			t.Fatal(err)
		}
		if sms.lastRequest.Tokens["token10"] != "custom-asset" {
			t.Fatalf("token10=%q", sms.lastRequest.Tokens["token10"])
		}
	})

	t.Run("empty frontend URL", func(t *testing.T) {
		orderRepo, txRepo, order := seedCallbackOrder(t, "psc", 10, 1)
		cfg := defaultOrderConfig()
		cfg.FrontendURL = ""
		svc := service.NewOrderService(nil, orderRepo, txRepo, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"psc": 1}}, &mockFirstOrderRepo{},
			&mockSadadClient{verifyResponse: verifyOK},
			&mockOrderPolicy{}, &mockJalaliConverter{},
			&grpcclients.WalletAdapter{Client: &mockWalletClient{}}, nil, nil, cfg)
		if _, err := svc.HandleCallback(ctx, order.ID, "tok", "0", nil); err == nil {
			t.Fatal("expected frontend URL error")
		}
	})

	t.Run("verify failure also fails to mark order", func(t *testing.T) {
		orderRepo, txRepo, order := seedCallbackOrder(t, "psc", 10, 1)
		orderRepo.updateErr = errors.New("cannot mark failed")
		svc := service.NewOrderService(nil, orderRepo, txRepo, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"psc": 1}}, &mockFirstOrderRepo{},
			&mockSadadClient{verifyError: errors.New("verify timeout")},
			&mockOrderPolicy{}, &mockJalaliConverter{}, nil, nil, nil, defaultOrderConfig())
		if _, err := svc.HandleCallback(ctx, order.ID, "tok", "0", nil); err == nil {
			t.Fatal("expected combined verify/mark error")
		}
	})

	t.Run("verify decline empty rescode uses -1 and mark error", func(t *testing.T) {
		orderRepo, txRepo, order := seedCallbackOrder(t, "psc", 10, 1)
		orderRepo.updateErr = errors.New("cannot mark failed")
		svc := service.NewOrderService(nil, orderRepo, txRepo, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"psc": 1}}, &mockFirstOrderRepo{},
			&mockSadadClient{verifyResponse: &sadad.VerificationResponse{ResCode: ""}},
			&mockOrderPolicy{}, &mockJalaliConverter{}, nil, nil, nil, defaultOrderConfig())
		if _, err := svc.HandleCallback(ctx, order.ID, "tok", "0", nil); err == nil {
			t.Fatal("expected mark error after decline")
		}
	})

	t.Run("rate error on successful callback", func(t *testing.T) {
		orderRepo, txRepo, order := seedCallbackOrder(t, "psc", 10, 1)
		svc := service.NewOrderService(nil, orderRepo, txRepo, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{}}, &mockFirstOrderRepo{},
			&mockSadadClient{verifyResponse: verifyOK},
			&mockOrderPolicy{}, &mockJalaliConverter{}, nil, nil, nil, defaultOrderConfig())
		if _, err := svc.HandleCallback(ctx, order.ID, "tok", "0", nil); err == nil {
			t.Fatal("expected rate error")
		}
	})

	t.Run("db tx path commits payment", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		mock.ExpectBegin()
		mock.ExpectCommit()

		orderRepo, txRepo, order := seedCallbackOrder(t, "psc", 10, 1)
		first := &mockFirstOrderRepo{}
		svc := service.NewOrderService(db, orderRepo, txRepo, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"psc": 1}}, first,
			&mockSadadClient{verifyResponse: verifyOK},
			&mockOrderPolicy{canGetBonus: true}, &mockJalaliConverter{},
			&grpcclients.WalletAdapter{Client: &mockWalletClient{}}, nil, nil, defaultOrderConfig())
		if _, err := svc.HandleCallback(ctx, order.ID, "tok", "0", nil); err != nil {
			t.Fatal(err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("db begin error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close()
		mock.ExpectBegin().WillReturnError(errors.New("begin failed"))

		orderRepo, txRepo, order := seedCallbackOrder(t, "psc", 10, 1)
		svc := service.NewOrderService(db, orderRepo, txRepo, &mockPaymentRepo{},
			&mockVariableRepo{rates: map[string]float64{"psc": 1}}, &mockFirstOrderRepo{},
			&mockSadadClient{verifyResponse: verifyOK},
			&mockOrderPolicy{}, &mockJalaliConverter{},
			&grpcclients.WalletAdapter{Client: &mockWalletClient{}}, nil, nil, defaultOrderConfig())
		if _, err := svc.HandleCallback(ctx, order.ID, "tok", "0", nil); err == nil {
			t.Fatal("expected begin error")
		}
	})
}

var _ commercialpb.WalletServiceClient = (*mockWalletClient)(nil)
