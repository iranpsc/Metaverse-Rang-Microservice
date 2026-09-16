package service

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"

	"metarang/financial-service/internal/config"
	"metarang/financial-service/internal/constants"
	"metarang/financial-service/internal/models"
	"metarang/financial-service/internal/sadad"
	notificationspb "metarang/shared/pb/notifications"
)

func logPaymentWarning(format string, args ...interface{}) {
	log.Printf("financial-service: "+format, args...)
}

func (s *orderService) requestSadadPayment(orderID uint64, amount int32, asset string, rate float64) (string, string, error) {
	returnURL, err := s.sadadCallbackReturnURL()
	if err != nil {
		return "", "", err
	}

	amountRials := amountInRials(amount, rate)
	multiplexingData, err := s.buildMultiplexingData(asset, amountRials)
	if err != nil {
		return "", "", err
	}

	response, err := s.sadadClient.RequestPayment(sadad.RequestParams{
		MerchantID:       s.sadadConfig.SadadMerchantID,
		TerminalID:       s.sadadConfig.SadadTerminalID,
		SignData:         s.sadadConfig.SadadTransactionKey,
		OrderID:          int64(orderID),
		Amount:           amountRials,
		ReturnURL:        returnURL,
		MultiplexingData: multiplexingData,
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to request payment: %w", err)
	}
	if !response.Success() {
		logPaymentWarning(
			"Sadad PaymentRequest rejected order_id=%d amount_rials=%d return_url=%q multiplexing=%s res_code=%s description=%q",
			orderID,
			amountRials,
			returnURL,
			formatMultiplexingForLog(multiplexingData),
			response.ResCode,
			response.Description,
		)
		return "", "", fmt.Errorf("%w: %s", ErrPaymentFailed, sadadFailureMessage(response))
	}

	return response.URL(), response.Token, nil
}

func amountInRials(amount int32, rate float64) int64 {
	return int64(float64(amount) * rate)
}

func sadadFailureMessage(response *sadad.RequestResponse) string {
	msg := response.Description
	if msg == "" {
		msg = response.Error().Message()
	}
	if response.ResCode != "" {
		return fmt.Sprintf("%s (ResCode=%s)", msg, response.ResCode)
	}
	return msg
}

func (s *orderService) storeTransactionToken(ctx context.Context, transaction *models.Transaction, token string) {
	tokenInt, err := strconv.ParseInt(token, 10, 64)
	if err != nil {
		return
	}

	transaction.Token = &tokenInt
	if err := s.transactionRepo.Update(ctx, transaction); err != nil {
		logPaymentWarning("failed to update transaction with token: %v", err)
	}
}

// buildMultiplexingData mirrors the production Laravel Sadad driver:
// Type=Amount with a single IBAN row for the full payment amount.
// Sadad rejects Percentage splits that include a zero-value row (common cause of
// Description "عملیات ناموفق بود" / ResCode 1104).
func (s *orderService) buildMultiplexingData(asset string, amountRials int64) (*sadad.MultiplexingData, error) {
	if s.sadadConfig.SadadSandbox {
		return nil, nil
	}
	if amountRials <= 0 {
		return nil, fmt.Errorf("%w: invalid multiplexing amount", ErrPaymentFailed)
	}

	rialIban := strings.TrimSpace(s.sadadConfig.SadadPaymentIdentityRial)
	nonRialIban := strings.TrimSpace(s.sadadConfig.SadadPaymentIdentityNonRial)
	if rialIban == "" || nonRialIban == "" {
		return nil, fmt.Errorf("%w: payment IBANs not configured for multiplexing", ErrPaymentFailed)
	}

	// IRR → loan/rial IBAN; all other assets → main/non-rial IBAN (same as Laravel MultiplexingData::forAsset).
	iban := nonRialIban
	if asset == "irr" {
		iban = rialIban
	}

	return &sadad.MultiplexingData{
		Type: "Amount",
		MultiplexingRows: []sadad.MultiplexingRow{
			{IbanNumber: iban, Value: amountRials},
		},
	}, nil
}

func formatMultiplexingForLog(data *sadad.MultiplexingData) string {
	if data == nil {
		return "none"
	}
	parts := make([]string, 0, len(data.MultiplexingRows))
	for _, row := range data.MultiplexingRows {
		parts = append(parts, fmt.Sprintf("%s=%d", maskIban(row.IbanNumber), row.Value))
	}
	return fmt.Sprintf("type=%s rows=[%s]", data.Type, strings.Join(parts, ","))
}

func maskIban(iban string) string {
	iban = strings.TrimSpace(iban)
	if len(iban) <= 8 {
		return iban
	}
	return iban[:4] + "…" + iban[len(iban)-4:]
}

func (s *orderService) HandleCallback(ctx context.Context, orderID uint64, token string, resCode string, additionalParams map[string]string) (string, error) {
	order, user, transaction, err := s.findCallbackOrderAndTransaction(ctx, orderID)
	if err != nil {
		return "", err
	}

	if resCode == "0" {
		verifyResCode, err := s.handleSuccessfulSadadCallback(ctx, order, user, transaction, token, additionalParams)
		if err != nil {
			return "", err
		}
		if verifyResCode != "" {
			resCode = verifyResCode
		}
	} else {
		if err := s.markOrderAndTransactionFailed(ctx, order, transaction, resCode); err != nil {
			return "", err
		}
	}

	return s.buildPaymentVerifyRedirectURL(orderID, resCode)
}

func (s *orderService) findCallbackOrderAndTransaction(ctx context.Context, orderID uint64) (*models.Order, *models.User, *models.Transaction, error) {
	order, user, err := s.orderRepo.FindByIDWithUser(ctx, orderID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to find order: %w", err)
	}
	if order == nil {
		return nil, nil, nil, ErrOrderNotFound
	}

	transaction, err := s.transactionRepo.FindByPayable(ctx, constants.OrderPayableType, orderID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to find transaction: %w", err)
	}
	if transaction == nil {
		return nil, nil, nil, fmt.Errorf("transaction not found for order")
	}

	return order, user, transaction, nil
}

func (s *orderService) handleSuccessfulSadadCallback(ctx context.Context, order *models.Order, user *models.User, transaction *models.Transaction, token string, additionalParams map[string]string) (string, error) {
	rate, err := s.variableRepo.GetRate(ctx, order.Asset)
	if err != nil {
		return "", fmt.Errorf("failed to get rate: %w", err)
	}

	verifyResponse, err := s.verifySadadPayment(transaction, token)
	if err != nil {
		if markErr := s.markOrderAndTransactionFailed(ctx, order, transaction, "-1"); markErr != nil {
			return "", fmt.Errorf("failed to verify payment: %w; failed to mark order failed: %v", err, markErr)
		}
		return "-1", nil
	}
	if !verifyResponse.Success() {
		failureCode := verifyResponse.ResCode
		if failureCode == "" {
			failureCode = "-1"
		}
		if markErr := s.markOrderAndTransactionFailed(ctx, order, transaction, failureCode); markErr != nil {
			return "", fmt.Errorf("payment verification failed with code %s; failed to mark order failed: %v", failureCode, markErr)
		}
		return failureCode, nil
	}

	refID := parseInt64OrDefault(verifyResponse.RetrivalRefNo, 0)
	cardPan := cardPanFromCallback(additionalParams, verifyResponse)
	if err := s.finalizeSuccessfulPayment(ctx, order, transaction, refID, rate, cardPan); err != nil {
		return "", err
	}

	s.processReferral(ctx, order)
	s.sendPaymentTransactionSMS(ctx, user, order, rate)
	return "", nil
}

func (s *orderService) finalizeSuccessfulPayment(ctx context.Context, order *models.Order, transaction *models.Transaction, refID int64, rate float64, cardPan string) error {
	canGetBonus, err := s.orderPolicy.CanGetBonus(ctx, order.UserID, order.Asset)
	if err != nil {
		return fmt.Errorf("failed to check bonus eligibility: %w", err)
	}

	var bonus float64
	var firstOrder *models.FirstOrder
	if canGetBonus {
		bonus = order.Amount * constants.FirstOrderBonusRate
		firstOrder = &models.FirstOrder{
			UserID: order.UserID,
			Type:   order.Asset,
			Amount: order.Amount,
			Date:   s.jalaliConverter.NowJalali(),
			Bonus:  bonus,
		}
	}

	order.Status = constants.StatusSuccess
	transaction.Status = constants.StatusSuccess
	transaction.RefID = &refID

	payment := &models.Payment{
		UserID:  order.UserID,
		RefID:   refID,
		CardPan: cardPan,
		Gateway: constants.SadadGateway,
		Amount:  order.Amount * rate,
		Product: order.Asset,
	}

	if s.db != nil {
		if err := s.finalizeSuccessfulPaymentTx(ctx, order, transaction, payment, firstOrder); err != nil {
			return err
		}
	} else {
		if err := s.orderRepo.Update(ctx, order); err != nil {
			return fmt.Errorf("failed to update order: %w", err)
		}
		if err := s.transactionRepo.Update(ctx, transaction); err != nil {
			return fmt.Errorf("failed to update transaction: %w", err)
		}
		if err := s.paymentRepo.Create(ctx, payment); err != nil {
			return fmt.Errorf("failed to create payment record: %w", err)
		}
		if firstOrder != nil {
			if err := s.firstOrderRepo.Create(ctx, firstOrder); err != nil {
				return fmt.Errorf("failed to create first order record: %w", err)
			}
		}
	}

	walletAmount := order.Amount
	if canGetBonus {
		walletAmount += bonus
	}
	if err := s.addWalletBalance(ctx, order.UserID, order.Asset, walletAmount); err != nil {
		return err
	}

	return nil
}

func (s *orderService) finalizeSuccessfulPaymentTx(ctx context.Context, order *models.Order, transaction *models.Transaction, payment *models.Payment, firstOrder *models.FirstOrder) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin payment transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.orderRepo.UpdateWithTx(ctx, tx, order); err != nil {
		return err
	}
	if err := s.transactionRepo.UpdateWithTx(ctx, tx, transaction); err != nil {
		return err
	}
	if err := s.paymentRepo.CreateWithTx(ctx, tx, payment); err != nil {
		return err
	}
	if firstOrder != nil {
		if err := s.firstOrderRepo.CreateWithTx(ctx, tx, firstOrder); err != nil {
			return fmt.Errorf("failed to create first order record: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit payment transaction: %w", err)
	}

	return nil
}

func (s *orderService) verifySadadPayment(transaction *models.Transaction, token string) (*sadad.VerificationResponse, error) {
	verifyToken := token
	if verifyToken == "" && transaction.Token != nil {
		verifyToken = strconv.FormatInt(*transaction.Token, 10)
	}

	return s.sadadClient.VerifyPayment(sadad.VerificationParams{
		SignData: s.sadadConfig.SadadTransactionKey,
		Token:    verifyToken,
	})
}

func (s *orderService) processReferral(ctx context.Context, order *models.Order) {
	if s.referralProcessor == nil {
		return
	}
	if err := s.referralProcessor.ProcessReferral(ctx, order.UserID, order.ID, order.Asset, order.Amount); err != nil {
		logPaymentWarning("failed to process referral for order %d: %v", order.ID, err)
	}
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

func (s *orderService) markOrderAndTransactionFailed(ctx context.Context, order *models.Order, transaction *models.Transaction, resCode string) error {
	statusCode := parseStatusCode(resCode)

	order.Status = statusCode
	if err := s.orderRepo.Update(ctx, order); err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	transaction.Status = statusCode
	if err := s.transactionRepo.Update(ctx, transaction); err != nil {
		return fmt.Errorf("failed to update transaction status: %w", err)
	}

	return nil
}

func parseStatusCode(resCode string) int32 {
	parsed, err := strconv.ParseInt(resCode, 10, 32)
	if err != nil {
		return int32(constants.StatusUnknown)
	}
	return int32(parsed)
}

func parseInt64OrDefault(value string, defaultValue int64) int64 {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func (s *orderService) sadadCallbackReturnURL() (string, error) {
	callbackURL := strings.TrimSpace(s.sadadConfig.SadadCallbackURL)
	if callbackURL == "" {
		return "", fmt.Errorf("%w: SADAD_CALLBACK_URL is not configured", ErrPaymentFailed)
	}

	normalized, ok := config.NormalizePaymentCallbackURL(callbackURL)
	if !ok {
		return "", fmt.Errorf("%w: Sadad ReturnUrl must be the API callback /api/order/callback, not the frontend verify page", ErrPaymentFailed)
	}

	return normalized, nil
}

func (s *orderService) buildPaymentVerifyRedirectURL(orderID uint64, resCode string) (string, error) {
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
	if s.walletTopUp == nil {
		return fmt.Errorf("wallet client not configured")
	}

	if err := s.walletTopUp.AddBalance(ctx, userID, asset, amount); err != nil {
		return fmt.Errorf("wallet AddBalance failed: %w", err)
	}

	return nil
}

func (s *orderService) sendPaymentTransactionSMS(ctx context.Context, user *models.User, order *models.Order, rate float64) {
	if s.smsClient == nil {
		return
	}
	if user == nil || strings.TrimSpace(user.Phone) == "" {
		return
	}

	_, err := s.smsClient.SendSMS(ctx, &notificationspb.SendSMSRequest{
		Phone:    strings.TrimSpace(user.Phone),
		Template: "transaction",
		Tokens: map[string]string{
			"token10": assetDisplayName(order.Asset),
			"token":   formatSMSAmount(order.Amount),
			"token2":  formatSMSAmount(order.Amount * rate),
		},
	})
	if err != nil {
		logPaymentWarning("failed to send payment transaction SMS: %v", err)
	}
}

func assetDisplayName(asset string) string {
	switch asset {
	case "yellow":
		return "زرد"
	case "red":
		return "قرمز"
	case "blue":
		return "آبی"
	case "psc":
		return "PSC"
	case "irr":
		return "ریال"
	default:
		return asset
	}
}

func formatSMSAmount(amount float64) string {
	if amount == float64(int64(amount)) {
		return fmt.Sprintf("%d", int64(amount))
	}
	return fmt.Sprintf("%g", amount)
}
