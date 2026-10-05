package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"metarang/financial-service/internal/config"
	"metarang/financial-service/internal/constants"
	"metarang/financial-service/internal/models"
	"metarang/financial-service/internal/sadad"
	notificationspb "metarang/shared/pb/notifications"
)

const (
	sadadResCodeSuccess = "0"
	sadadResCodeUnknown = "-1"
	// callbackCompletionTimeout keeps verify and wallet credit running after the buyer leaves the bank return page.
	callbackCompletionTimeout = 90 * time.Second
)

func logPaymentWarning(format string, args ...interface{}) {
	log.Printf("financial-service: "+format, args...)
}

func (s *orderService) requestSadadPayment(orderID uint64, amount int32, asset string, rate float64) (string, string, error) {
	// Sadad posts OrderId in the return body. The ReturnUrl itself stays free of
	// an order id so the callback cannot be pointed at a different order.
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

// sadadFailureMessage returns the exact ResCode and Description from Sadad IPG.
// Does not substitute local/custom messages when Description is empty.
func sadadFailureMessage(response *sadad.RequestResponse) string {
	if response == nil {
		return "empty IPG response"
	}
	code := strings.TrimSpace(response.ResCode)
	msg := strings.TrimSpace(response.Description)
	switch {
	case code != "" && msg != "":
		return fmt.Sprintf("ResCode=%s Description=%s", code, msg)
	case code != "":
		return fmt.Sprintf("ResCode=%s", code)
	case msg != "":
		return fmt.Sprintf("Description=%s", msg)
	default:
		return "empty ResCode and Description from IPG"
	}
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
	if amountRials <= 0 {
		return nil, fmt.Errorf("%w: invalid multiplexing amount", ErrPaymentFailed)
	}

	rialIban := strings.TrimSpace(s.sadadConfig.SadadPaymentIdentityRial)
	nonRialIban := strings.TrimSpace(s.sadadConfig.SadadPaymentIdentityNonRial)
	if rialIban == "" || nonRialIban == "" {
		return nil, fmt.Errorf("%w: payment IBANs not configured for multiplexing", ErrPaymentFailed)
	}

	return &sadad.MultiplexingData{
		Type: sadad.MultiplexingTypeAmount,
		MultiplexingRows: []sadad.MultiplexingRow{
			{IbanNumber: settlementIBAN(asset, rialIban, nonRialIban), Value: amountRials},
		},
	}, nil
}

// settlementIBAN routes IRR to the loan IBAN and every other asset to the main IBAN.
func settlementIBAN(asset, rialIBAN, nonRialIBAN string) string {
	if asset == constants.AssetIRR {
		return rialIBAN
	}
	return nonRialIBAN
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

	// A completed payment must stay successful. Sadad retries the return POST, and a
	// later verify attempt is rejected by the gateway once the token is consumed.
	if order.Status == constants.StatusSuccess {
		return s.buildPaymentVerifyRedirectURL(orderID, sadadResCodeSuccess)
	}

	redirectCode := resCode
	if resCode == sadadResCodeSuccess {
		redirectCode, err = s.handleSuccessfulSadadCallback(ctx, order, user, transaction, token, additionalParams)
		if err != nil {
			return "", err
		}
	} else if err = s.markOrderAndTransactionFailed(ctx, order, transaction, resCode); err != nil {
		return "", err
	}

	return s.buildPaymentVerifyRedirectURL(orderID, redirectCode)
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
	// The buyer can leave the bank return page while verify and the wallet
	// credit are still running. Finish that work on a detached context.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), callbackCompletionTimeout)
	defer cancel()

	rate, err := s.variableRepo.GetRate(ctx, order.Asset)
	if err != nil {
		return "", fmt.Errorf("failed to get rate: %w", err)
	}

	verifyResponse, err := s.verifySadadPayment(transaction, token)
	if err != nil {
		return s.rejectVerifiedPayment(ctx, order, transaction, sadadResCodeUnknown, err)
	}
	if !verifyResponse.Success() {
		failureCode := verifyResponse.ResCode
		if failureCode == "" {
			failureCode = sadadResCodeUnknown
		}
		return s.rejectVerifiedPayment(ctx, order, transaction, failureCode, nil)
	}

	refID := parseInt64OrZero(verifyResponse.RetrivalRefNo)
	if err := s.finalizeSuccessfulPayment(ctx, order, transaction, refID, rate, cardPanFromCallback(additionalParams)); err != nil {
		if errors.Is(err, ErrOrderAlreadyPaid) {
			return sadadResCodeSuccess, nil
		}
		return "", err
	}

	s.processReferral(ctx, order)
	s.sendPaymentTransactionSMS(ctx, user, order, rate)
	return sadadResCodeSuccess, nil
}

func (s *orderService) rejectVerifiedPayment(ctx context.Context, order *models.Order, transaction *models.Transaction, resCode string, verifyErr error) (string, error) {
	if markErr := s.markOrderAndTransactionFailed(ctx, order, transaction, resCode); markErr != nil {
		if verifyErr != nil {
			return "", fmt.Errorf("failed to verify payment: %w; failed to mark order failed: %v", verifyErr, markErr)
		}
		return "", fmt.Errorf("payment verification failed with code %s; failed to mark order failed: %v", resCode, markErr)
	}
	return resCode, nil
}

func (s *orderService) finalizeSuccessfulPayment(ctx context.Context, order *models.Order, transaction *models.Transaction, refID int64, rate float64, cardPan string) error {
	canGetBonus, err := s.orderPolicy.CanGetBonus(ctx, order.UserID, order.Asset)
	if err != nil {
		return fmt.Errorf("failed to check bonus eligibility: %w", err)
	}

	firstOrder, walletAmount := s.firstOrderReward(order, canGetBonus)

	if s.db != nil {
		return s.finalizeSuccessfulPaymentTx(ctx, order, transaction, refID, cardPan, rate, firstOrder, walletAmount)
	}

	// Callers without a database connection, including unit tests, persist each row on its own.
	// Credit the wallet before those writes so a wallet failure leaves the order pending.
	markPaymentSuccessful(order, transaction, refID)
	payment := newSadadPayment(order, refID, cardPan, rate)
	return s.persistSuccessfulPayment(ctx, order, transaction, payment, firstOrder, walletAmount)
}

func (s *orderService) firstOrderReward(order *models.Order, canGetBonus bool) (*models.FirstOrder, float64) {
	if !canGetBonus {
		return nil, order.Amount
	}

	bonus := order.Amount * constants.FirstOrderBonusRate
	return &models.FirstOrder{
		UserID: order.UserID,
		Type:   order.Asset,
		Amount: order.Amount,
		Date:   s.jalaliConverter.NowJalali(),
		Bonus:  bonus,
	}, order.Amount + bonus
}

func markPaymentSuccessful(order *models.Order, transaction *models.Transaction, refID int64) {
	order.Status = constants.StatusSuccess
	transaction.Status = constants.StatusSuccess
	transaction.RefID = &refID
}

func newSadadPayment(order *models.Order, refID int64, cardPan string, rate float64) *models.Payment {
	return &models.Payment{
		UserID:  order.UserID,
		RefID:   refID,
		CardPan: cardPan,
		Gateway: constants.SadadGateway,
		Amount:  order.Amount * rate,
		Product: order.Asset,
	}
}

func (s *orderService) persistSuccessfulPayment(ctx context.Context, order *models.Order, transaction *models.Transaction, payment *models.Payment, firstOrder *models.FirstOrder, walletAmount float64) error {
	if err := s.addWalletBalance(ctx, order.UserID, order.Asset, walletAmount); err != nil {
		restorePendingPayment(order, transaction)
		return err
	}

	orderWritten := false
	transactionWritten := false
	fail := func(err error) error {
		s.compensateUncommittedCredit(ctx, order, transaction, walletAmount, orderWritten, transactionWritten)
		return err
	}

	if err := s.orderRepo.Update(ctx, order); err != nil {
		return fail(fmt.Errorf("failed to update order: %w", err))
	}
	orderWritten = true
	if err := s.transactionRepo.Update(ctx, transaction); err != nil {
		return fail(fmt.Errorf("failed to update transaction: %w", err))
	}
	transactionWritten = true
	if err := s.paymentRepo.Create(ctx, payment); err != nil {
		return fail(fmt.Errorf("failed to create payment record: %w", err))
	}
	if firstOrder != nil {
		if err := s.firstOrderRepo.Create(ctx, firstOrder); err != nil {
			return fail(fmt.Errorf("failed to create first order record: %w", err))
		}
	}
	return nil
}

func (s *orderService) compensateUncommittedCredit(ctx context.Context, order *models.Order, transaction *models.Transaction, walletAmount float64, orderWritten, transactionWritten bool) {
	if err := s.reverseWalletCredit(ctx, order.UserID, order.Asset, walletAmount); err != nil {
		logPaymentWarning("wallet credited for order %d but payment was not saved and reversal failed: %v", order.ID, err)
	}
	restorePendingPayment(order, transaction)
	if orderWritten {
		if err := s.orderRepo.Update(ctx, order); err != nil {
			logPaymentWarning("failed to restore pending order %d after payment persist failure: %v", order.ID, err)
		}
	}
	if transactionWritten {
		if err := s.transactionRepo.Update(ctx, transaction); err != nil {
			logPaymentWarning("failed to restore pending transaction %s after payment persist failure: %v", transaction.ID, err)
		}
	}
}

func restorePendingPayment(order *models.Order, transaction *models.Transaction) {
	order.Status = constants.OrderStatusPending
	transaction.Status = constants.TransactionStatusPending
	transaction.RefID = nil
}

func (s *orderService) finalizeSuccessfulPaymentTx(ctx context.Context, order *models.Order, transaction *models.Transaction, refID int64, cardPan string, rate float64, firstOrder *models.FirstOrder, walletAmount float64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin payment transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Claim the pending order inside the transaction so a concurrent callback
	// cannot credit the wallet a second time. The row lock waits until the
	// winner commits or rolls back.
	claimed, err := s.orderRepo.ClaimUnpaidWithTx(ctx, tx, order.ID, constants.StatusSuccess)
	if err != nil {
		return err
	}
	if !claimed {
		return ErrOrderAlreadyPaid
	}

	markPaymentSuccessful(order, transaction, refID)
	payment := newSadadPayment(order, refID, cardPan, rate)

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

	// Credit before commit. A wallet failure rolls the order back to pending
	// so the bank callback can be retried without recording a paid order.
	if err := s.addWalletBalance(ctx, order.UserID, order.Asset, walletAmount); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		if revErr := s.reverseWalletCredit(ctx, order.UserID, order.Asset, walletAmount); revErr != nil {
			logPaymentWarning("wallet credited for order %d but database commit failed and reversal failed: commit=%v reversal=%v", order.ID, err, revErr)
			return fmt.Errorf("failed to commit payment transaction: %w; wallet reversal: %v", err, revErr)
		}
		logPaymentWarning("wallet credit reversed after commit failure for order %d: %v", order.ID, err)
		return fmt.Errorf("failed to commit payment transaction: %w", err)
	}

	return nil
}

func (s *orderService) verifySadadPayment(transaction *models.Transaction, token string) (*sadad.VerificationResponse, error) {
	callbackToken := strings.TrimSpace(token)
	if transaction.Token != nil && callbackToken != "" && !callbackTokenMatches(*transaction.Token, callbackToken) {
		return nil, fmt.Errorf("callback token does not match stored payment token")
	}

	verifyToken := callbackToken
	if verifyToken == "" && transaction.Token != nil {
		verifyToken = strconv.FormatInt(*transaction.Token, 10)
	}

	return s.sadadClient.VerifyPayment(sadad.VerificationParams{
		SignData: s.sadadConfig.SadadTransactionKey,
		Token:    verifyToken,
	})
}

func callbackTokenMatches(stored int64, callbackToken string) bool {
	parsed, err := strconv.ParseInt(strings.TrimSpace(callbackToken), 10, 64)
	if err != nil {
		return false
	}
	return parsed == stored
}

func (s *orderService) processReferral(ctx context.Context, order *models.Order) {
	if s.referralProcessor == nil || order.Asset == constants.AssetIRR {
		return
	}
	if err := s.referralProcessor.ProcessReferral(ctx, order.UserID, order.ID, order.Asset, order.Amount); err != nil {
		logPaymentWarning("failed to process referral for order %d: %v", order.ID, err)
	}
}

// cardPanFromCallback extracts the masked card number (PAN) from the Sadad callback POST.
// Per VPG Help v1.10 section 6.5, Sadad sends PrimaryAccNo (masked PAN) in the callback body.
// The Verify response does not include a card number field per the documented output.
func cardPanFromCallback(additionalParams map[string]string) string {
	for _, key := range []string{"PrimaryAccNo", "CardMaskPan", "card_pan"} {
		if cardPan := additionalParams[key]; cardPan != "" {
			return cardPan
		}
	}
	return ""
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

func parseInt64OrZero(value string) int64 {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
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
		logPaymentWarning("wallet credit failed user_id=%d asset=%s amount=%v: %v", userID, asset, amount, err)
		return fmt.Errorf("wallet AddBalance failed: %w", err)
	}

	return nil
}

func (s *orderService) reverseWalletCredit(ctx context.Context, userID uint64, asset string, amount float64) error {
	if s.walletTopUp == nil {
		return fmt.Errorf("wallet client not configured")
	}
	if err := s.walletTopUp.ReverseBalance(ctx, userID, asset, amount); err != nil {
		return fmt.Errorf("wallet credit reversal failed: %w", err)
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

var assetDisplayNames = map[string]string{
	"yellow":           "زرد",
	"red":              "قرمز",
	"blue":             "آبی",
	"psc":              "PSC",
	constants.AssetIRR: "ریال",
}

func assetDisplayName(asset string) string {
	if name, ok := assetDisplayNames[asset]; ok {
		return name
	}
	return asset
}

func formatSMSAmount(amount float64) string {
	if amount == float64(int64(amount)) {
		return fmt.Sprintf("%d", int64(amount))
	}
	return fmt.Sprintf("%g", amount)
}
