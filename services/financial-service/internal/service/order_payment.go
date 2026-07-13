package service

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"metarang/financial-service/internal/config"
	"metarang/financial-service/internal/models"
	"metarang/financial-service/internal/sadad"
	commercialpb "metarang/shared/pb/commercial"
)

func (s *orderService) requestSadadPayment(orderID uint64, amount int32, asset string, rate float64) (string, string, error) {
	paymentIdentity := s.getPaymentIdentity(asset)
	if paymentIdentity == "" && !s.sadadConfig.SadadSandbox {
		return "", "", fmt.Errorf("%w: payment identity not configured for asset %s", ErrPaymentFailed, asset)
	}

	returnURL, err := s.sadadCallbackReturnURL()
	if err != nil {
		return "", "", err
	}

	response, err := s.sadadClient.RequestPayment(sadad.RequestParams{
		MerchantID:      s.sadadConfig.SadadMerchantID,
		TerminalID:      s.sadadConfig.SadadTerminalID,
		TransactionKey:  s.sadadConfig.SadadTransactionKey,
		OrderID:         fmt.Sprintf("%d", orderID),
		Amount:          amountInRials(amount, rate),
		ReturnURL:       returnURL,
		PaymentIdentity: paymentIdentity,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to request payment: %w", err)
	}
	if !response.Success() {
		return "", "", fmt.Errorf("%w: %s", ErrPaymentFailed, sadadFailureMessage(response))
	}

	return response.URL(), response.Token, nil
}

func amountInRials(amount int32, rate float64) int64 {
	return int64(float64(amount) * rate)
}

func sadadFailureMessage(response *sadad.RequestResponse) string {
	if response.Description != "" {
		return response.Description
	}
	return response.Error().Message()
}

func (s *orderService) storeTransactionToken(ctx context.Context, transaction *models.Transaction, token string) {
	tokenInt, err := strconv.ParseInt(token, 10, 64)
	if err != nil {
		return
	}

	transaction.Token = &tokenInt
	if err := s.transactionRepo.Update(ctx, transaction); err != nil {
		fmt.Printf("Warning: failed to update transaction with token: %v\n", err)
	}
}

func (s *orderService) getPaymentIdentity(asset string) string {
	if asset == "irr" {
		return s.sadadConfig.SadadPaymentIdentityRial
	}
	return s.sadadConfig.SadadPaymentIdentityNonRial
}

func (s *orderService) HandleCallback(ctx context.Context, orderID uint64, token string, resCode string, additionalParams map[string]string) (string, error) {
	order, transaction, err := s.findCallbackOrderAndTransaction(ctx, orderID)
	if err != nil {
		return "", err
	}

	if resCode == "0" {
		if err := s.handleSuccessfulSadadCallback(ctx, order, transaction, token, additionalParams); err != nil {
			return "", err
		}
	} else {
		s.markOrderAndTransactionFailed(ctx, order, transaction, resCode)
	}

	return s.buildPaymentVerifyRedirectURL(orderID, token, resCode, additionalParams)
}

func (s *orderService) findCallbackOrderAndTransaction(ctx context.Context, orderID uint64) (*models.Order, *models.Transaction, error) {
	order, _, err := s.orderRepo.FindByIDWithUser(ctx, orderID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find order: %w", err)
	}
	if order == nil {
		return nil, nil, ErrOrderNotFound
	}

	transaction, err := s.transactionRepo.FindByPayable(ctx, orderPayableType, orderID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find transaction: %w", err)
	}
	if transaction == nil {
		return nil, nil, fmt.Errorf("transaction not found for order")
	}

	return order, transaction, nil
}

func (s *orderService) handleSuccessfulSadadCallback(ctx context.Context, order *models.Order, transaction *models.Transaction, token string, additionalParams map[string]string) error {
	rate, err := s.variableRepo.GetRate(ctx, order.Asset)
	if err != nil {
		return fmt.Errorf("failed to get rate: %w", err)
	}

	verifyResponse, err := s.verifySadadPayment(transaction, token)
	if err != nil {
		return nil
	}
	if !verifyResponse.Success() {
		s.markOrderFailed(ctx, order, verifyResponse.ResCode)
		return nil
	}

	refID := parseInt64OrDefault(verifyResponse.RetrivalRefNo, 0)
	if err := s.markPaymentSucceeded(ctx, order, transaction, refID); err != nil {
		return err
	}
	if err := s.creditWallet(ctx, order); err != nil {
		return err
	}

	return s.createPaymentRecord(ctx, order, refID, rate, cardPanFromCallback(additionalParams, verifyResponse))
}

func (s *orderService) verifySadadPayment(transaction *models.Transaction, token string) (*sadad.VerificationResponse, error) {
	verifyToken := token
	if verifyToken == "" && transaction.Token != nil {
		verifyToken = strconv.FormatInt(*transaction.Token, 10)
	}

	return s.sadadClient.VerifyPayment(sadad.VerificationParams{
		TransactionKey: s.sadadConfig.SadadTransactionKey,
		Token:          verifyToken,
	})
}

func (s *orderService) markPaymentSucceeded(ctx context.Context, order *models.Order, transaction *models.Transaction, refID int64) error {
	order.Status = statusSuccess
	if err := s.orderRepo.Update(ctx, order); err != nil {
		return fmt.Errorf("failed to update order: %w", err)
	}

	transaction.Status = statusSuccess
	transaction.RefID = &refID
	if err := s.transactionRepo.Update(ctx, transaction); err != nil {
		return fmt.Errorf("failed to update transaction: %w", err)
	}

	return nil
}

func (s *orderService) creditWallet(ctx context.Context, order *models.Order) error {
	canGetBonus, err := s.orderPolicy.CanGetBonus(ctx, order.UserID, order.Asset)
	if err != nil {
		return fmt.Errorf("failed to check bonus eligibility: %w", err)
	}

	if !canGetBonus {
		if err := s.addWalletBalance(ctx, order.UserID, order.Asset, order.Amount); err != nil {
			return fmt.Errorf("failed to add wallet balance: %w", err)
		}
		return nil
	}

	bonus := order.Amount * firstOrderBonusRate
	totalAmount := order.Amount + bonus
	if err := s.addWalletBalance(ctx, order.UserID, order.Asset, totalAmount); err != nil {
		return fmt.Errorf("failed to add wallet balance with bonus: %w", err)
	}

	firstOrder := &models.FirstOrder{
		UserID: order.UserID,
		Type:   order.Asset,
		Amount: order.Amount,
		Date:   s.jalaliConverter.NowJalali(),
		Bonus:  bonus,
	}
	if err := s.firstOrderRepo.Create(ctx, firstOrder); err != nil {
		return fmt.Errorf("failed to create first order record: %w", err)
	}

	return nil
}

func (s *orderService) createPaymentRecord(ctx context.Context, order *models.Order, refID int64, rate float64, cardPan string) error {
	payment := &models.Payment{
		UserID:  order.UserID,
		RefID:   refID,
		CardPan: cardPan,
		Gateway: sadadGateway,
		Amount:  order.Amount * rate,
		Product: order.Asset,
	}
	if err := s.paymentRepo.Create(ctx, payment); err != nil {
		return fmt.Errorf("failed to create payment record: %w", err)
	}

	return nil
}

func cardPanFromCallback(additionalParams map[string]string, verifyResponse *sadad.VerificationResponse) string {
	for _, key := range []string{"PrimaryAccNo", "CardMaskPan", "card_pan"} {
		if cardPan := additionalParams[key]; cardPan != "" {
			return cardPan
		}
	}
	if verifyResponse.CardNumberMasked != "" {
		return verifyResponse.CardNumberMasked
	}
	return "card-hash"
}

func (s *orderService) markOrderAndTransactionFailed(ctx context.Context, order *models.Order, transaction *models.Transaction, resCode string) {
	statusCode := parseStatusCode(resCode)

	order.Status = statusCode
	s.orderRepo.Update(ctx, order)

	transaction.Status = statusCode
	s.transactionRepo.Update(ctx, transaction)
}

func (s *orderService) markOrderFailed(ctx context.Context, order *models.Order, resCode string) {
	order.Status = parseStatusCode(resCode)
	s.orderRepo.Update(ctx, order)
}

func parseStatusCode(resCode string) int32 {
	return int32(parseInt64OrDefault(resCode, int64(statusUnknown)))
}

func parseInt64OrDefault(value string, defaultValue int64) int64 {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func (s *orderService) sadadCallbackReturnURL() (string, error) {
	callbackURL := strings.TrimSpace(config.ResolveSadadCallbackURL())
	if callbackURL == "" {
		return "", fmt.Errorf("%w: SADAD_CALLBACK_URL is not configured", ErrPaymentFailed)
	}
	if strings.Contains(callbackURL, "/payment/verify") {
		return "", fmt.Errorf("%w: Sadad ReturnUrl must be the API callback /api/order/callback, not the frontend verify page", ErrPaymentFailed)
	}
	if !strings.Contains(callbackURL, "/api/order/callback") {
		return "", fmt.Errorf("%w: Sadad ReturnUrl must include /api/order/callback", ErrPaymentFailed)
	}
	return strings.TrimSuffix(callbackURL, "/"), nil
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
	for k, v := range additionalParams {
		if strings.EqualFold(k, "OrderId") || strings.EqualFold(k, "order_id") {
			continue
		}
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
