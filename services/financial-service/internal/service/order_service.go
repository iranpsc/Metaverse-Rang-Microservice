package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"metargb/financial-service/internal/models"
	"metargb/financial-service/internal/sadad"
	"metargb/financial-service/internal/repository"
	commercialpb "metargb/shared/pb/commercial"
	"strings"
)

var (
	ErrInvalidAmount   = errors.New("amount must be at least 1")
	ErrInvalidAsset    = errors.New("invalid asset type")
	ErrOrderNotFound   = errors.New("order not found")
	ErrPaymentFailed   = errors.New("payment request failed")
	ErrUserNotEligible = errors.New("user not eligible to buy from store")
)

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
	SadadPaymentIdentityRial    string // multiplexing identity for IRR payments
	SadadPaymentIdentityNonRial string // multiplexing identity for non-IRR assets
	SadadCallbackURL            string
	FrontendURL                 string
	SadadSandbox                bool // BankTest sandbox skips multiplexing payment identities
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
	if amount < 1 {
		return "", ErrInvalidAmount
	}

	validAssets := map[string]bool{"psc": true, "irr": true, "red": true, "blue": true, "yellow": true}
	if !validAssets[asset] {
		return "", ErrInvalidAsset
	}

	canBuy, err := s.orderPolicy.CanBuyFromStore(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to check buy permission: %w", err)
	}
	if !canBuy {
		return "", ErrUserNotEligible
	}

	rate, err := s.variableRepo.GetRate(ctx, asset)
	if err != nil {
		return "", fmt.Errorf("failed to get asset rate: %w", err)
	}

	order := &models.Order{
		UserID: userID,
		Asset:  asset,
		Amount: float64(amount),
		Status: -138,
	}

	err = s.orderRepo.Create(ctx, order)
	if err != nil {
		return "", fmt.Errorf("failed to create order: %w", err)
	}

	transactionID, err := generateTransactionID()
	if err != nil {
		return "", fmt.Errorf("failed to generate transaction id: %w", err)
	}

	transaction := &models.Transaction{
		ID:          transactionID,
		UserID:      userID,
		Asset:       asset,
		Amount:      float64(amount),
		Action:      "deposit",
		Status:      1,
		PayableType: stringPtr("App\\Models\\Order"),
		PayableID:   &order.ID,
	}

	err = s.transactionRepo.Create(ctx, transaction)
	if err != nil {
		return "", fmt.Errorf("failed to create transaction: %w", err)
	}

	paymentIdentity := s.getPaymentIdentity(asset)
	if paymentIdentity == "" && !s.sadadConfig.SadadSandbox {
		return "", fmt.Errorf("%w: payment identity not configured for asset %s", ErrPaymentFailed, asset)
	}

	amountInRials := int64(float64(amount) * rate)
	returnURL, err := s.buildSadadReturnURL(order.ID)
	if err != nil {
		return "", err
	}

	params := sadad.RequestParams{
		MerchantID:      s.sadadConfig.SadadMerchantID,
		TerminalID:      s.sadadConfig.SadadTerminalID,
		TransactionKey:  s.sadadConfig.SadadTransactionKey,
		OrderID:         fmt.Sprintf("%d", order.ID),
		Amount:          amountInRials,
		ReturnURL:       returnURL,
		PaymentIdentity: paymentIdentity,
	}

	response, err := s.sadadClient.RequestPayment(params)
	if err != nil {
		return "", fmt.Errorf("failed to request payment: %w", err)
	}

	if !response.Success() {
		msg := response.Description
		if msg == "" {
			msg = response.Error().Message()
		}
		return "", fmt.Errorf("%w: %s", ErrPaymentFailed, msg)
	}

	if tokenInt, err := strconv.ParseInt(response.Token, 10, 64); err == nil {
		transaction.Token = &tokenInt
		err = s.transactionRepo.Update(ctx, transaction)
		if err != nil {
			fmt.Printf("Warning: failed to update transaction with token: %v\n", err)
		}
	}

	return response.URL(), nil
}

func (s *orderService) getPaymentIdentity(asset string) string {
	if asset == "irr" {
		return s.sadadConfig.SadadPaymentIdentityRial
	}
	return s.sadadConfig.SadadPaymentIdentityNonRial
}

func (s *orderService) HandleCallback(ctx context.Context, orderID uint64, token string, resCode string, additionalParams map[string]string) (string, error) {
	order, _, err := s.orderRepo.FindByIDWithUser(ctx, orderID)
	if err != nil {
		return "", fmt.Errorf("failed to find order: %w", err)
	}
	if order == nil {
		return "", ErrOrderNotFound
	}

	transaction, err := s.transactionRepo.FindByPayable(ctx, "App\\Models\\Order", orderID)
	if err != nil {
		return "", fmt.Errorf("failed to find transaction: %w", err)
	}
	if transaction == nil {
		return "", fmt.Errorf("transaction not found for order")
	}

	if resCode == "0" {
		rate, err := s.variableRepo.GetRate(ctx, order.Asset)
		if err != nil {
			return "", fmt.Errorf("failed to get rate: %w", err)
		}

		verifyToken := token
		if verifyToken == "" && transaction.Token != nil {
			verifyToken = strconv.FormatInt(*transaction.Token, 10)
		}

		verifyResponse, err := s.sadadClient.VerifyPayment(sadad.VerificationParams{
			TransactionKey: s.sadadConfig.SadadTransactionKey,
			Token:          verifyToken,
		})
		if err == nil && verifyResponse.Success() {
			order.Status = 0
			if err := s.orderRepo.Update(ctx, order); err != nil {
				return "", fmt.Errorf("failed to update order: %w", err)
			}

			transaction.Status = 0
			refID, parseErr := strconv.ParseInt(verifyResponse.RetrivalRefNo, 10, 64)
			if parseErr != nil {
				refID = 0
			}
			transaction.RefID = &refID
			if err := s.transactionRepo.Update(ctx, transaction); err != nil {
				return "", fmt.Errorf("failed to update transaction: %w", err)
			}

			canGetBonus, err := s.orderPolicy.CanGetBonus(ctx, order.UserID, order.Asset)
			if err != nil {
				return "", fmt.Errorf("failed to check bonus eligibility: %w", err)
			}

			amount := order.Amount * rate

			if canGetBonus {
				bonus := order.Amount * 0.5
				totalAmount := order.Amount + bonus

				if err := s.addWalletBalance(ctx, order.UserID, order.Asset, totalAmount); err != nil {
					return "", fmt.Errorf("failed to add wallet balance with bonus: %w", err)
				}

				jalaliDate := s.jalaliConverter.NowJalali()
				firstOrder := &models.FirstOrder{
					UserID: order.UserID,
					Type:   order.Asset,
					Amount: order.Amount,
					Date:   jalaliDate,
					Bonus:  bonus,
				}
				if err := s.firstOrderRepo.Create(ctx, firstOrder); err != nil {
					return "", fmt.Errorf("failed to create first order record: %w", err)
				}
			} else if err := s.addWalletBalance(ctx, order.UserID, order.Asset, order.Amount); err != nil {
				return "", fmt.Errorf("failed to add wallet balance: %w", err)
			}

			cardPan := additionalParams["PrimaryAccNo"]
			if cardPan == "" {
				cardPan = additionalParams["CardMaskPan"]
			}
			if cardPan == "" {
				cardPan = additionalParams["card_pan"]
			}
			if cardPan == "" {
				cardPan = verifyResponse.CardNumberMasked
			}
			if cardPan == "" {
				cardPan = "card-hash"
			}

			payment := &models.Payment{
				UserID:  order.UserID,
				RefID:   refID,
				CardPan: cardPan,
				Gateway: "sadad",
				Amount:  amount,
				Product: order.Asset,
			}
			if err := s.paymentRepo.Create(ctx, payment); err != nil {
				return "", fmt.Errorf("failed to create payment record: %w", err)
			}
		} else if err == nil {
			statusCode, parseErr := strconv.ParseInt(verifyResponse.ResCode, 10, 32)
			if parseErr != nil {
				statusCode = -1
			}
			order.Status = int32(statusCode)
			s.orderRepo.Update(ctx, order)
		}
	} else {
		statusCode, parseErr := strconv.ParseInt(resCode, 10, 32)
		if parseErr != nil {
			statusCode = -1
		}
		order.Status = int32(statusCode)
		s.orderRepo.Update(ctx, order)
		transaction.Status = int32(statusCode)
		s.transactionRepo.Update(ctx, transaction)
	}

	return s.buildPaymentVerifyRedirectURL(orderID, token, resCode, additionalParams)
}

func (s *orderService) buildSadadReturnURL(orderID uint64) (string, error) {
	callbackURL := strings.TrimSpace(s.sadadConfig.SadadCallbackURL)
	if callbackURL == "" {
		return "", fmt.Errorf("%w: payment callback URL is not configured", ErrPaymentFailed)
	}
	if strings.Contains(callbackURL, "/payment/verify") {
		return "", fmt.Errorf("%w: Sadad ReturnUrl must be the API callback /api/payment/callback, not the frontend verify page", ErrPaymentFailed)
	}
	if !strings.Contains(callbackURL, "/api/payment/callback") {
		return "", fmt.Errorf("%w: Sadad ReturnUrl must include /api/payment/callback", ErrPaymentFailed)
	}
	return fmt.Sprintf("%s?order_id=%d", strings.TrimSuffix(callbackURL, "/"), orderID), nil
}

func (s *orderService) buildPaymentVerifyRedirectURL(orderID uint64, token, resCode string, additionalParams map[string]string) (string, error) {
	redirectURL, err := s.paymentVerifyRedirectURL()
	if err != nil {
		return "", err
	}

	u, err := url.Parse(redirectURL)
	if err != nil {
		return "", fmt.Errorf("invalid frontend URL: %w", err)
	}

	q := u.Query()
	q.Set("OrderId", fmt.Sprintf("%d", orderID))
	q.Set("ResCode", resCode)
	q.Set("Token", token)
	for k, v := range additionalParams {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	return u.String(), nil
}

func (s *orderService) paymentVerifyRedirectURL() (string, error) {
	frontendURL := strings.TrimSpace(s.sadadConfig.FrontendURL)
	if frontendURL == "" {
		return "", fmt.Errorf("FRONTEND_URL is not configured")
	}

	base, err := url.Parse(strings.TrimSuffix(frontendURL, "/"))
	if err != nil {
		return "", fmt.Errorf("invalid FRONTEND_URL: %w", err)
	}

	verifyURL := base.ResolveReference(&url.URL{Path: "/payment/verify"})
	return verifyURL.String(), nil
}

func (s *orderService) addWalletBalance(ctx context.Context, userID uint64, asset string, amount float64) error {
	if s.walletClient == nil {
		return fmt.Errorf("wallet client not configured")
	}

	resp, err := s.walletClient.AddBalance(ctx, &commercialpb.AddBalanceRequest{
		UserId: userID,
		Asset:  asset,
		Amount: amount,
	})
	if err != nil {
		return fmt.Errorf("wallet AddBalance gRPC failed: %w", err)
	}
	if resp != nil && !resp.Success {
		msg := "unknown error"
		if resp.Message != "" {
			msg = resp.Message
		}
		return fmt.Errorf("wallet AddBalance rejected: %s", msg)
	}

	return nil
}

func generateTransactionID() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("TR-%s", hex.EncodeToString(b)), nil
}

func stringPtr(s string) *string {
	return &s
}
