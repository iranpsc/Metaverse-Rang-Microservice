// Package service implements business logic for the features service.
//
// BuyRequestService (this file) is a legacy alternate implementation of buy-request flows that mirrors
// MarketplaceService. It is not constructed in cmd/server/main.go; production uses MarketplaceService
// for all FeatureMarketplaceService RPCs. Kept for reference and potential extraction of shared helpers.
package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"metarang/features-service/internal/client"
	"metarang/features-service/internal/constants"
	"metarang/features-service/internal/models"
	"metarang/features-service/internal/repository"
	"metarang/shared/pkg/logger"
)

// BuyRequestService handles buy requests with gRPC cross-service calls
type BuyRequestService struct {
	featureRepo      *repository.FeatureRepository
	propertiesRepo   *repository.PropertiesRepository
	tradeRepo        *repository.TradeRepository
	buyRequestRepo   *repository.BuyRequestRepository
	sellRequestRepo  *repository.SellRequestRepository
	lockedAssetRepo  *repository.LockedAssetRepository
	hourlyProfitRepo *repository.HourlyProfitRepository
	commercialClient *client.CommercialClient
	db               *sql.DB
	log              *logger.Logger
}

func NewBuyRequestService(
	featureRepo *repository.FeatureRepository,
	propertiesRepo *repository.PropertiesRepository,
	tradeRepo *repository.TradeRepository,
	buyRequestRepo *repository.BuyRequestRepository,
	sellRequestRepo *repository.SellRequestRepository,
	lockedAssetRepo *repository.LockedAssetRepository,
	hourlyProfitRepo *repository.HourlyProfitRepository,
	commercialClient *client.CommercialClient,
	db *sql.DB,
	log *logger.Logger,
) *BuyRequestService {
	return &BuyRequestService{
		featureRepo:      featureRepo,
		propertiesRepo:   propertiesRepo,
		tradeRepo:        tradeRepo,
		buyRequestRepo:   buyRequestRepo,
		sellRequestRepo:  sellRequestRepo,
		lockedAssetRepo:  lockedAssetRepo,
		hourlyProfitRepo: hourlyProfitRepo,
		commercialClient: commercialClient,
		db:               db,
		log:              log,
	}
}

// SendBuyRequest creates a buy request with locked assets using gRPC
func (s *BuyRequestService) SendBuyRequest(ctx context.Context, buyerID, featureID uint64, pricePSC, priceIRR float64, note string) (uint64, error) {
	// Get feature and seller
	feature, properties, err := s.featureRepo.FindByID(ctx, featureID)
	if err != nil {
		return 0, fmt.Errorf("feature not found: %w", err)
	}

	sellerID := feature.OwnerID

	// Validate price against minimum_price_percentage
	totalRequestedPrice := priceIRR + (pricePSC * s.getVariableRate(ctx, "psc"))
	color := constants.GetColor(properties.Karbari)
	colorRate := s.getVariableRate(ctx, color)
	totalFeaturePrice := properties.Stability * colorRate

	floorPercentage := float64(properties.MinimumPricePercentage)
	actualPercentage := (totalRequestedPrice / totalFeaturePrice) * 100

	if actualPercentage < floorPercentage {
		return 0, fmt.Errorf("شما مجاز به ارسال درخواست خرید به کمتر از %.0f%% قیمت ملک نمی باشید", floorPercentage)
	}

	// Calculate amounts with fees
	buyerChargePSC := constants.CalculateBuyerCharge(pricePSC)
	buyerChargeIRR := constants.CalculateBuyerCharge(priceIRR)

	// Check buyer balance via gRPC
	hasPSC, _ := s.commercialClient.CheckBalance(ctx, buyerID, "psc", buyerChargePSC)
	hasIRR, _ := s.commercialClient.CheckBalance(ctx, buyerID, "irr", buyerChargeIRR)
	if !hasPSC || !hasIRR {
		return 0, fmt.Errorf("موجودی شما کافی نمی باشد")
	}

	// Create buy request
	requestID, err := s.buyRequestRepo.Create(ctx, buyerID, sellerID, featureID, note, pricePSC, priceIRR)
	if err != nil {
		return 0, err
	}

	// Deduct buyer's wallet via gRPC (lock funds)
	if err := s.commercialClient.DeductBalance(ctx, buyerID, "psc", buyerChargePSC); err != nil {
		return 0, fmt.Errorf("failed to lock PSC: %w", err)
	}
	if err := s.commercialClient.DeductBalance(ctx, buyerID, "irr", buyerChargeIRR); err != nil {
		// Rollback PSC
		_ = s.commercialClient.AddBalance(ctx, buyerID, "psc", buyerChargePSC)
		return 0, fmt.Errorf("failed to lock IRR: %w", err)
	}

	// Create locked asset record
	if _, err := s.lockedAssetRepo.Create(ctx, requestID, featureID, buyerChargePSC, buyerChargeIRR); err != nil {
		s.log.Error("Failed to create locked asset", "error", err)
	}

	// Create transactions via gRPC
	_, _ = s.commercialClient.CreateTransaction(ctx, buyerID, "psc", buyerChargePSC, "withdraw", 0, "App\\Models\\BuyFeatureRequest", requestID)
	_, _ = s.commercialClient.CreateTransaction(ctx, buyerID, "irr", buyerChargeIRR, "withdraw", 0, "App\\Models\\BuyFeatureRequest", requestID)

	s.log.Info("Buy request created with locked assets",
		"request_id", requestID,
		"buyer_id", buyerID,
		"feature_id", featureID,
		"psc_locked", buyerChargePSC,
		"irr_locked", buyerChargeIRR,
	)

	// TODO: Send notifications via Notifications Service

	return requestID, nil
}

// AcceptBuyRequest accepts a request and releases locked assets using gRPC
func (s *BuyRequestService) AcceptBuyRequest(ctx context.Context, requestID, sellerID uint64) error {
	// Get buy request
	buyRequest, err := s.buyRequestRepo.FindByID(ctx, requestID)
	if err != nil {
		return fmt.Errorf("buy request not found: %w", err)
	}

	// Verify seller
	if buyRequest.SellerID != sellerID {
		return fmt.Errorf("unauthorized: not the seller")
	}

	// Get feature
	feature, properties, err := s.featureRepo.FindByID(ctx, buyRequest.FeatureID)
	if err != nil {
		return fmt.Errorf("feature not found: %w", err)
	}

	// Check underpriced restriction
	if err := s.checkUnderpricedRestriction(ctx, feature, properties); err != nil {
		return err
	}

	// Get locked assets to verify they exist
	_, err = s.lockedAssetRepo.GetByBuyRequestID(ctx, requestID)
	if err != nil {
		return fmt.Errorf("locked assets not found: %w", err)
	}

	pscAmount := buyRequest.PricePSC
	irrAmount := buyRequest.PriceIRR
	pscFee := constants.CalculateFee(pscAmount)
	irrFee := constants.CalculateFee(irrAmount)

	// Pay seller via gRPC (price - fee)
	if err := s.commercialClient.AddBalance(ctx, sellerID, "psc", pscAmount-pscFee); err != nil {
		return err
	}
	if err := s.commercialClient.AddBalance(ctx, sellerID, "irr", irrAmount-irrFee); err != nil {
		return err
	}

	// Pay RGB platform via gRPC (fee × 2)
	rgbUserID, err := s.getRGBUserID(ctx)
	if err == nil {
		_ = s.commercialClient.AddBalance(ctx, rgbUserID, "psc", pscFee*2)
		_ = s.commercialClient.AddBalance(ctx, rgbUserID, "irr", irrFee*2)
	}

	// Create trade
	tradeID, err := s.tradeRepo.Create(ctx, buyRequest.FeatureID, buyRequest.BuyerID, sellerID, irrAmount, pscAmount)
	if err != nil {
		return err
	}

	// Create commission
	s.createCommission(ctx, tradeID, pscFee*2, irrFee*2)

	// Create transactions for seller via gRPC
	_, _ = s.commercialClient.CreateTransaction(ctx, sellerID, "psc", pscAmount-pscFee, "deposit", 1, "App\\Models\\Trade", tradeID)
	_, _ = s.commercialClient.CreateTransaction(ctx, sellerID, "irr", irrAmount-irrFee, "deposit", 1, "App\\Models\\Trade", tradeID)

	// Transfer ownership
	if err := s.featureRepo.UpdateOwner(ctx, feature.ID, buyRequest.BuyerID); err != nil {
		return err
	}

	// Update properties
	buyerName := s.getUserName(ctx, buyRequest.BuyerID)
	isUnder18 := s.isUserUnder18(ctx, buyRequest.BuyerID)
	pricingLimit := constants.DefaultPublicPricingLimit
	if isUnder18 {
		pricingLimit = constants.DefaultUnder18PricingLimit
	}

	newStatus := constants.ChangeStatusToSoldAndNotPriced(properties.Karbari)
	if err := s.propertiesRepo.UpdateStatus(ctx, feature.ID, newStatus, buyerName, "", pricingLimit); err != nil {
		return err
	}

	// Transfer hourly profit
	withdrawProfitDays, _ := s.getUserVariableWithdrawProfit(ctx, buyRequest.BuyerID)
	if withdrawProfitDays == 0 {
		withdrawProfitDays = 10
	}

	oldProfit, _ := s.hourlyProfitRepo.GetByFeatureAndUser(ctx, feature.ID, sellerID)
	if oldProfit != nil && oldProfit.Amount > 0 {
		_ = s.commercialClient.AddBalance(ctx, sellerID, oldProfit.Asset, oldProfit.Amount)
	}

	_ = s.hourlyProfitRepo.TransferProfitToNewOwner(ctx, feature.ID, sellerID, buyRequest.BuyerID, constants.GetColor(properties.Karbari), withdrawProfitDays)

	// Update request and delete locked asset
	_ = s.buyRequestRepo.UpdateStatus(ctx, requestID, 1)
	_ = s.buyRequestRepo.SoftDelete(ctx, requestID)
	_ = s.lockedAssetRepo.Delete(ctx, requestID)

	// Cancel other requests and refund
	allRequests, _ := s.buyRequestRepo.GetAllForFeature(ctx, buyRequest.FeatureID)
	for _, req := range allRequests {
		if req.ID != requestID {
			s.refundBuyRequest(ctx, req.ID)
		}
	}

	// Update sell requests
	_ = s.sellRequestRepo.UpdateAllForFeatureToCompleted(ctx, buyRequest.FeatureID)

	s.log.Info("Buy request accepted",
		"request_id", requestID,
		"feature_id", buyRequest.FeatureID,
		"buyer_id", buyRequest.BuyerID,
		"seller_id", sellerID,
		"trade_id", tradeID,
	)

	return nil
}

// refundBuyRequest refunds a cancelled request using gRPC
func (s *BuyRequestService) refundBuyRequest(ctx context.Context, requestID uint64) {
	buyRequest, err := s.buyRequestRepo.FindByID(ctx, requestID)
	if err != nil {
		return
	}

	lockedAsset, err := s.lockedAssetRepo.GetByBuyRequestID(ctx, requestID)
	if err != nil {
		return
	}

	// Refund buyer via gRPC
	_ = s.commercialClient.AddBalance(ctx, buyRequest.BuyerID, "psc", lockedAsset.PSC)
	_ = s.commercialClient.AddBalance(ctx, buyRequest.BuyerID, "irr", lockedAsset.IRR)

	// Delete locked asset and soft delete request
	_ = s.lockedAssetRepo.Delete(ctx, requestID)
	_ = s.buyRequestRepo.SoftDelete(ctx, requestID)

	s.log.Info("Buy request refunded", "request_id", requestID, "buyer_id", buyRequest.BuyerID)
}

// Helper methods
func (s *BuyRequestService) checkUnderpricedRestriction(ctx context.Context, feature *models.Feature, properties *models.FeatureProperties) error {
	// Reuse from marketplace_service_grpc.go logic
	isUnderpriced, _ := s.sellRequestRepo.IsUnderpriced(ctx, feature.ID)
	if !isUnderpriced {
		return nil
	}

	latestSellReq, _ := s.sellRequestRepo.GetLatestUnderpricedForSeller(ctx, feature.OwnerID)
	if latestSellReq == nil {
		return nil
	}

	latestTrade, _ := s.tradeRepo.GetLatestUnderpricedForSeller(ctx, feature.OwnerID, latestSellReq.FeatureID)
	if latestTrade == nil || !s.tradeRepo.IsWithin24Hours(latestTrade) {
		return nil
	}

	hours, minutes := s.tradeRepo.GetTimeRemaining(latestTrade)
	var elapsedTime string
	if hours < 1 {
		elapsedTime = fmt.Sprintf("%d دقیقه", minutes)
	} else {
		elapsedTime = fmt.Sprintf("%d ساعت", hours)
	}

	return fmt.Errorf("شما در ۲۴ ساعت گذشته ملکی با زیر قیمت ۱۰۰٪ بفروش رسانده اید. برای پذیرش این درخواست باید %s صبر کنید", elapsedTime)
}

func (s *BuyRequestService) getVariableRate(ctx context.Context, asset string) float64 {
	var rate float64
	query := "SELECT value FROM variables WHERE `key` = ?"
	if err := s.db.QueryRowContext(ctx, query, asset).Scan(&rate); err != nil {
		return 1.0
	}
	return rate
}

func (s *BuyRequestService) getRGBUserID(ctx context.Context) (uint64, error) {
	var rgbID uint64
	err := s.db.QueryRowContext(ctx, "SELECT id FROM users WHERE code = ?", constants.RGBUserCode).Scan(&rgbID)
	return rgbID, err
}

func (s *BuyRequestService) getUserName(ctx context.Context, userID uint64) string {
	var name string
	_ = s.db.QueryRowContext(ctx, "SELECT name FROM users WHERE id = ?", userID).Scan(&name)
	return name
}

func (s *BuyRequestService) isUserUnder18(ctx context.Context, userID uint64) bool {
	var birthdate sql.NullTime
	err := s.db.QueryRowContext(ctx, "SELECT birthdate FROM kycs WHERE user_id = ?", userID).Scan(&birthdate)
	if err != nil || !birthdate.Valid {
		return false
	}
	// Calculate age accurately
	age := time.Since(birthdate.Time).Hours() / 24 / 365.25
	return age < 18
}

func (s *BuyRequestService) getUserVariableWithdrawProfit(ctx context.Context, userID uint64) (int, error) {
	var days int
	err := s.db.QueryRowContext(ctx, "SELECT withdraw_profit FROM user_variables WHERE user_id = ?", userID).Scan(&days)
	return days, err
}

func (s *BuyRequestService) createCommission(ctx context.Context, tradeID uint64, psc, irr float64) {
	query := "INSERT INTO comissions (trade_id, psc, irr, created_at, updated_at) VALUES (?, ?, ?, NOW(), NOW())"
	_, _ = s.db.ExecContext(ctx, query, tradeID, psc, irr)
}

// ListBuyRequests lists all buy requests for a buyer
func (s *BuyRequestService) ListBuyRequests(ctx context.Context, buyerID uint64) ([]*BuyRequestDetail, error) {
	requests, err := s.buyRequestRepo.ListByBuyerID(ctx, buyerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list buy requests: %w", err)
	}

	details := make([]*BuyRequestDetail, 0, len(requests))
	for _, req := range requests {
		detail, err := s.buildBuyRequestDetail(ctx, req)
		if err != nil {
			s.log.Error("Failed to build buy request detail", "error", err, "request_id", req.ID)
			continue
		}
		details = append(details, detail)
	}

	return details, nil
}

// ListReceivedBuyRequests lists all buy requests received by a seller
func (s *BuyRequestService) ListReceivedBuyRequests(ctx context.Context, sellerID uint64) ([]*BuyRequestDetail, error) {
	requests, err := s.buyRequestRepo.ListBySellerID(ctx, sellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list received buy requests: %w", err)
	}

	details := make([]*BuyRequestDetail, 0, len(requests))
	for _, req := range requests {
		detail, err := s.buildBuyRequestDetail(ctx, req)
		if err != nil {
			s.log.Error("Failed to build buy request detail", "error", err, "request_id", req.ID)
			continue
		}
		details = append(details, detail)
	}

	return details, nil
}

// RejectBuyRequest rejects a buy request and refunds the buyer
func (s *BuyRequestService) RejectBuyRequest(ctx context.Context, requestID, sellerID uint64) error {
	buyRequest, err := s.buyRequestRepo.FindByID(ctx, requestID)
	if err != nil {
		return fmt.Errorf("buy request not found: %w", err)
	}
	if buyRequest == nil {
		return fmt.Errorf("buy request not found")
	}

	// Verify seller
	if buyRequest.SellerID != sellerID {
		return fmt.Errorf("unauthorized: not the seller")
	}

	// Get locked assets
	lockedAsset, err := s.lockedAssetRepo.GetByBuyRequestID(ctx, requestID)
	if err != nil {
		return fmt.Errorf("locked assets not found: %w", err)
	}

	// Refund buyer
	if err := s.commercialClient.AddBalance(ctx, buyRequest.BuyerID, "psc", lockedAsset.PSC); err != nil {
		return fmt.Errorf("failed to refund PSC: %w", err)
	}
	if err := s.commercialClient.AddBalance(ctx, buyRequest.BuyerID, "irr", lockedAsset.IRR); err != nil {
		return fmt.Errorf("failed to refund IRR: %w", err)
	}

	// Delete transactions (via commercial service - handled by deleting locked asset)
	// Delete locked asset and buy request
	if err := s.lockedAssetRepo.Delete(ctx, requestID); err != nil {
		s.log.Error("Failed to delete locked asset", "error", err)
	}
	if err := s.buyRequestRepo.Delete(ctx, requestID); err != nil {
		return fmt.Errorf("failed to delete buy request: %w", err)
	}

	s.log.Info("Buy request rejected", "request_id", requestID, "seller_id", sellerID)
	return nil
}

// DeleteBuyRequest deletes a buy request (buyer cancels their own offer)
func (s *BuyRequestService) DeleteBuyRequest(ctx context.Context, requestID, buyerID uint64) error {
	buyRequest, err := s.buyRequestRepo.FindByID(ctx, requestID)
	if err != nil {
		return fmt.Errorf("buy request not found: %w", err)
	}
	if buyRequest == nil {
		return fmt.Errorf("buy request not found")
	}

	// Verify buyer
	if buyRequest.BuyerID != buyerID {
		return fmt.Errorf("unauthorized: not the buyer")
	}

	// Get locked assets
	lockedAsset, err := s.lockedAssetRepo.GetByBuyRequestID(ctx, requestID)
	if err != nil {
		return fmt.Errorf("locked assets not found: %w", err)
	}

	// Refund buyer
	if err := s.commercialClient.AddBalance(ctx, buyRequest.BuyerID, "psc", lockedAsset.PSC); err != nil {
		return fmt.Errorf("failed to refund PSC: %w", err)
	}
	if err := s.commercialClient.AddBalance(ctx, buyRequest.BuyerID, "irr", lockedAsset.IRR); err != nil {
		return fmt.Errorf("failed to refund IRR: %w", err)
	}

	// Delete locked asset and buy request
	if err := s.lockedAssetRepo.Delete(ctx, requestID); err != nil {
		s.log.Error("Failed to delete locked asset", "error", err)
	}
	if err := s.buyRequestRepo.Delete(ctx, requestID); err != nil {
		return fmt.Errorf("failed to delete buy request: %w", err)
	}

	s.log.Info("Buy request deleted", "request_id", requestID, "buyer_id", buyerID)
	return nil
}

// UpdateGracePeriod updates the grace period for a buy request
func (s *BuyRequestService) UpdateGracePeriod(ctx context.Context, requestID, sellerID uint64, gracePeriodDays int32) error {
	if gracePeriodDays < 1 || gracePeriodDays > 30 {
		return fmt.Errorf("grace period must be between 1 and 30 days")
	}

	buyRequest, err := s.buyRequestRepo.FindByID(ctx, requestID)
	if err != nil {
		return fmt.Errorf("buy request not found: %w", err)
	}
	if buyRequest == nil {
		return fmt.Errorf("buy request not found")
	}

	// Verify seller
	if buyRequest.SellerID != sellerID {
		return fmt.Errorf("unauthorized: not the seller")
	}

	// Check status is pending
	if buyRequest.Status != 0 {
		return fmt.Errorf("buy request is not pending")
	}

	// Calculate grace period timestamp
	gracePeriod := sql.NullTime{
		Time:  time.Now().AddDate(0, 0, int(gracePeriodDays)),
		Valid: true,
	}

	if err := s.buyRequestRepo.UpdateGracePeriod(ctx, requestID, gracePeriod); err != nil {
		return fmt.Errorf("failed to update grace period: %w", err)
	}

	s.log.Info("Grace period updated", "request_id", requestID, "grace_period_days", gracePeriodDays)
	return nil
}

// BuyRequestDetail contains all information needed for a buy request response
type BuyRequestDetail struct {
	ID                   uint64
	BuyerID              uint64
	SellerID             uint64
	FeatureID            uint64
	Status               int
	Note                 string
	PricePSC             float64
	PriceIRR             float64
	RequestedGracePeriod *time.Time
	CreatedAt            time.Time
	// Loaded relationships
	BuyerCode          string
	BuyerProfilePhoto  string
	SellerCode         string
	FeatureProperties  *models.FeatureProperties
	FeatureCoordinates []*models.Coordinate
}

// buildBuyRequestDetail builds a detailed buy request response with all relationships loaded
func (s *BuyRequestService) buildBuyRequestDetail(ctx context.Context, req *models.BuyFeatureRequest) (*BuyRequestDetail, error) {
	detail := &BuyRequestDetail{
		ID:        req.ID,
		BuyerID:   req.BuyerID,
		SellerID:  req.SellerID,
		FeatureID: req.FeatureID,
		Status:    req.Status,
		Note:      req.Note,
		PricePSC:  req.PricePSC,
		PriceIRR:  req.PriceIRR,
		CreatedAt: req.CreatedAt,
	}

	if req.RequestedGracePeriod.Valid {
		detail.RequestedGracePeriod = &req.RequestedGracePeriod.Time
	}

	// Get buyer code
	buyerCode, err := s.getUserCode(ctx, req.BuyerID)
	if err == nil {
		detail.BuyerCode = buyerCode
	}

	// Get buyer profile photo
	buyerPhoto, err := s.getLatestProfilePhoto(ctx, req.BuyerID)
	if err == nil {
		detail.BuyerProfilePhoto = buyerPhoto
	}

	// Get seller code
	sellerCode, err := s.getUserCode(ctx, req.SellerID)
	if err == nil {
		detail.SellerCode = sellerCode
	}

	// Get feature properties
	_, properties, err := s.featureRepo.FindByID(ctx, req.FeatureID)
	if err == nil {
		detail.FeatureProperties = properties
	}

	// Get feature coordinates (we need geometry repo)
	// For now, we'll load them in the handler using geometryRepo

	return detail, nil
}

// getUserCode gets user code from database
func (s *BuyRequestService) getUserCode(ctx context.Context, userID uint64) (string, error) {
	var code string
	err := s.db.QueryRowContext(ctx, "SELECT code FROM users WHERE id = ?", userID).Scan(&code)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("user not found")
	}
	return code, err
}

// getLatestProfilePhoto gets the latest profile photo URL for a user
func (s *BuyRequestService) getLatestProfilePhoto(ctx context.Context, userID uint64) (string, error) {
	var url string
	query := `
		SELECT url 
		FROM images 
		WHERE imageable_type = 'App\\Models\\User' AND imageable_id = ?
		ORDER BY created_at DESC
		LIMIT 1
	`
	err := s.db.QueryRowContext(ctx, query, userID).Scan(&url)
	if err == sql.ErrNoRows {
		return "", nil // No photo is not an error
	}
	return url, err
}
