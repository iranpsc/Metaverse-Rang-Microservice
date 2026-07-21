package service_test

import (
	"context"
	"testing"

	"metarang/financial-service/internal/grpcclients"
	"metarang/financial-service/internal/models"
	"metarang/financial-service/internal/sadad"
	"metarang/financial-service/internal/service"
	notificationspb "metarang/shared/pb/notifications"

	"google.golang.org/grpc"
)

type mockSMSServiceClient struct {
	lastRequest *notificationspb.SendSMSRequest
}

func (m *mockSMSServiceClient) SendSMS(_ context.Context, req *notificationspb.SendSMSRequest, _ ...grpc.CallOption) (*notificationspb.SMSResponse, error) {
	m.lastRequest = req
	return &notificationspb.SMSResponse{Sent: true}, nil
}

func (m *mockSMSServiceClient) SendOTP(context.Context, *notificationspb.SendOTPRequest, ...grpc.CallOption) (*notificationspb.SMSResponse, error) {
	panic("unexpected call to SendOTP")
}

var _ notificationspb.SMSServiceClient = (*mockSMSServiceClient)(nil)

func TestOrderService_HandleCallback_sendsTransactionSMSAfterSuccessfulPayment(t *testing.T) {
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
		ID:          "TR-sms-test",
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

	smsClient := &mockSMSServiceClient{}
	svc := service.NewOrderService(
		nil,
		orderRepo,
		transactionRepo,
		&mockPaymentRepo{},
		&mockVariableRepo{rates: map[string]float64{"psc": 1000}},
		&mockFirstOrderRepo{},
		&mockSadadClient{
			verifyResponse: &sadad.VerificationResponse{
				ResCode:          "0",
				RetrivalRefNo:    "99887766",
				CardNumberMasked: "1234****5678",
			},
		},
		&mockOrderPolicy{canBuy: true, canGetBonus: false},
		&mockJalaliConverter{},
		&grpcclients.WalletAdapter{Client: &mockWalletClient{}},
		nil,
		smsClient,
		service.OrderConfig{
			SadadTransactionKey: "dGVzdC10cmFuc2FjdGlvbi1rZXk=",
			SadadCallbackURL:    "http://localhost:8000/api/order/callback",
			FrontendURL:         "http://localhost:5173",
		},
	)

	_, err := svc.HandleCallback(ctx, order.ID, "123456", "0", map[string]string{
		"PrimaryAccNo": "1234****5678",
	})
	if err != nil {
		t.Fatalf("HandleCallback failed: %v", err)
	}

	if smsClient.lastRequest == nil {
		t.Fatal("expected SendSMS to be called after successful payment")
	}
	if smsClient.lastRequest.Phone != "09123456789" {
		t.Fatalf("expected user phone, got %q", smsClient.lastRequest.Phone)
	}
	if smsClient.lastRequest.Template != "transaction" {
		t.Fatalf("expected transaction template, got %q", smsClient.lastRequest.Template)
	}

	tokens := smsClient.lastRequest.Tokens
	if tokens["token10"] != "PSC" {
		t.Fatalf("expected token10 asset name PSC, got %q", tokens["token10"])
	}
	if tokens["token"] != "10" {
		t.Fatalf("expected token asset amount 10, got %q", tokens["token"])
	}
	if tokens["token2"] != "10000" {
		t.Fatalf("expected token2 payment amount 10000, got %q", tokens["token2"])
	}
}
