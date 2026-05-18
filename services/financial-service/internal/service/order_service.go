package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"

	"metargb/financial-service/internal/models"
	"metargb/financial-service/internal/parsian"
	"metargb/financial-service/internal/repository"
	commercialpb "metargb/shared/pb/commercial"
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
	HandleCallback(ctx context.Context, orderID uint64, status int32, token int64, additionalParams map[string]string) (string, error)
}

type orderService struct {
	orderRepo       repository.OrderRepository
	transactionRepo repository.TransactionRepository
	paymentRepo     repository.PaymentRepository
	variableRepo    repository.VariableRepository
	firstOrderRepo  repository.FirstOrderRepository
	parsianClient   ParsianClient
	orderPolicy     OrderPolicy
	jalaliConverter JalaliConverter
	walletClient    commercialpb.WalletServiceClient
	merchantID      string
	loanMerchantID  string
	callbackURL     string
	frontendURL     string
}

type OrderConfig struct {
	ParsianMerchantID            string
	ParsianLoanAccountMerchantID string
	ParsianCallbackURL           string
	FrontendURL                  string
}

func NewOrderService(
	orderRepo repository.OrderRepository,
	transactionRepo repository.TransactionRepository,
	paymentRepo repository.PaymentRepository,
	variableRepo repository.VariableRepository,
	firstOrderRepo repository.FirstOrderRepository,
	parsianClient ParsianClient,
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
		parsianClient:   parsianClient,
		orderPolicy:     orderPolicy,
		jalaliConverter: jalaliConverter,
		walletClient:    walletClient,
		merchantID:      config.ParsianMerchantID,
		loanMerchantID:  config.ParsianLoanAccountMerchantID,
		callbackURL:     config.ParsianCallbackURL,
		frontendURL:     config.FrontendURL,
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

	merchantID := s.merchantID
	if asset == "irr" {
		merchantID = s.loanMerchantID
	}

	amountInRials := int64(float64(amount) * rate)

	params := parsian.RequestParams{
		MerchantID:     merchantID,
		OrderID:        fmt.Sprintf("%d", order.ID),
		Amount:         amountInRials,
		CallbackURL:    s.callbackURL,
		AdditionalData: "",
		Originator:     "",
	}

	response, err := s.parsianClient.RequestPayment(params)
	if err != nil {
		return "", fmt.Errorf("failed to request payment: %w", err)
	}

	if !response.Success() {
		return "", fmt.Errorf("%w: %s", ErrPaymentFailed, response.Error().Message())
	}

	transaction.Token = &response.Token
	err = s.transactionRepo.Update(ctx, transaction)
	if err != nil {
		fmt.Printf("Warning: failed to update transaction with token: %v\n", err)
	}

	return response.URL(), nil
}

func (s *orderService) HandleCallback(ctx context.Context, orderID uint64, status int32, token int64, additionalParams map[string]string) (string, error) {
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

	redirectURL := s.frontendURL + "/metaverse/payment/verify"
	u, err := url.Parse(redirectURL)
	if err != nil {
		return "", fmt.Errorf("invalid frontend URL: %w", err)
	}

	q := u.Query()
	q.Set("OrderId", fmt.Sprintf("%d", orderID))
	q.Set("status", fmt.Sprintf("%d", status))
	q.Set("Token", fmt.Sprintf("%d", token))

	for k, v := range additionalParams {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	if status == 0 {
		rate, err := s.variableRepo.GetRate(ctx, order.Asset)
		if err != nil {
			return u.String(), fmt.Errorf("failed to get rate: %w", err)
		}

		merchantID := s.merchantID
		if order.Asset == "irr" {
			merchantID = s.loanMerchantID
		}

		verifyToken := token
		if verifyToken == 0 && transaction.Token != nil {
			verifyToken = *transaction.Token
		}

		verifyParams := parsian.VerificationParams{
			MerchantID: merchantID,
			Token:      verifyToken,
		}

		verifyResponse, err := s.parsianClient.VerifyPayment(verifyParams)
		if err != nil {
			return u.String(), nil
		}

		if verifyResponse.Success() {
			order.Status = verifyResponse.Status
			err = s.orderRepo.Update(ctx, order)
			if err != nil {
				return u.String(), fmt.Errorf("failed to update order: %w", err)
			}

			transaction.Status = verifyResponse.Status
			refID := verifyResponse.ReferenceID
			transaction.RefID = &refID
			err = s.transactionRepo.Update(ctx, transaction)
			if err != nil {
				return u.String(), fmt.Errorf("failed to update transaction: %w", err)
			}

			canGetBonus, err := s.orderPolicy.CanGetBonus(ctx, order.UserID, order.Asset)
			if err != nil {
				return u.String(), fmt.Errorf("failed to check bonus eligibility: %w", err)
			}

			amount := order.Amount * rate

			if canGetBonus {
				bonus := order.Amount * 0.5
				totalAmount := order.Amount + bonus

				if err := s.addWalletBalance(ctx, order.UserID, order.Asset, totalAmount); err != nil {
					fmt.Printf("Warning: failed to add wallet balance with bonus: %v\n", err)
				}

				jalaliDate := s.jalaliConverter.NowJalali()
				firstOrder := &models.FirstOrder{
					UserID: order.UserID,
					Type:   order.Asset,
					Amount: order.Amount,
					Date:   jalaliDate,
					Bonus:  bonus,
				}
				err = s.firstOrderRepo.Create(ctx, firstOrder)
				if err != nil {
					fmt.Printf("Warning: failed to create first order record: %v\n", err)
				}
			} else {
				if err := s.addWalletBalance(ctx, order.UserID, order.Asset, order.Amount); err != nil {
					fmt.Printf("Warning: failed to add wallet balance: %v\n", err)
				}
			}

			cardPan := additionalParams["CardMaskPan"]
			if cardPan == "" {
				cardPan = additionalParams["card_pan"]
			}
			if cardPan == "" {
				cardPan = verifyResponse.CardHash
			}
			if cardPan == "" {
				cardPan = "card-hash"
			}

			payment := &models.Payment{
				UserID:  order.UserID,
				RefID:   verifyResponse.ReferenceID,
				CardPan: cardPan,
				Gateway: "parsian",
				Amount:  amount,
				Product: order.Asset,
			}
			err = s.paymentRepo.Create(ctx, payment)
			if err != nil {
				fmt.Printf("Warning: failed to create payment record: %v\n", err)
			}

			// Referral and notifications remain in commercial-service scope for future wiring
		} else {
			order.Status = verifyResponse.Status
			s.orderRepo.Update(ctx, order)
		}
	} else {
		order.Status = status
		s.orderRepo.Update(ctx, order)
		transaction.Status = status
		s.transactionRepo.Update(ctx, transaction)
	}

	return u.String(), nil
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
