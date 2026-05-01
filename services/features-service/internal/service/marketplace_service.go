package service

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"metargb/features-service/internal/client"
	"metargb/features-service/internal/constants"
	"metargb/features-service/internal/events"
	"metargb/features-service/internal/metrics"
	"metargb/features-service/internal/models"
	"metargb/features-service/internal/repository"
	pb "metargb/shared/pb/features"
	"metargb/shared/pkg/logger"
)

// MarketplaceService implements marketplace logic with gRPC cross-service calls
// This version uses CommercialClient instead of direct SQL for wallet operations
type MarketplaceService struct {
	featureRepo        *repository.FeatureRepository
	propertiesRepo     *repository.PropertiesRepository
	geometryRepo       *repository.GeometryRepository
	tradeRepo          *repository.TradeRepository
	buyRequestRepo     *repository.BuyRequestRepository
	sellRequestRepo    *repository.SellRequestRepository
	lockedAssetRepo    *repository.LockedAssetRepository
	hourlyProfitRepo   *repository.HourlyProfitRepository
	featureLimitRepo   *repository.FeatureLimitRepository
	systemVariableRepo *repository.SystemVariableRepository
	variableRepo       *repository.VariableRepository
	commercialClient   *client.CommercialClient
	notificationClient *client.NotificationClient
	eventBroadcaster   events.EventBroadcaster
	metrics            *metrics.MarketplaceMetrics
	db                 *sql.DB
	log                *logger.Logger
}

func NewMarketplaceService(
	featureRepo *repository.FeatureRepository,
	propertiesRepo *repository.PropertiesRepository,
	geometryRepo *repository.GeometryRepository,
	tradeRepo *repository.TradeRepository,
	buyRequestRepo *repository.BuyRequestRepository,
	sellRequestRepo *repository.SellRequestRepository,
	lockedAssetRepo *repository.LockedAssetRepository,
	hourlyProfitRepo *repository.HourlyProfitRepository,
	featureLimitRepo *repository.FeatureLimitRepository,
	variableRepo *repository.VariableRepository,
	commercialClient *client.CommercialClient,
	notificationClient *client.NotificationClient,
	eventBroadcaster events.EventBroadcaster,
	marketplaceMetrics *metrics.MarketplaceMetrics,
	db *sql.DB,
	log *logger.Logger,
) *MarketplaceService {
	return &MarketplaceService{
		featureRepo:        featureRepo,
		propertiesRepo:     propertiesRepo,
		geometryRepo:       geometryRepo,
		tradeRepo:          tradeRepo,
		buyRequestRepo:     buyRequestRepo,
		sellRequestRepo:    sellRequestRepo,
		lockedAssetRepo:    lockedAssetRepo,
		hourlyProfitRepo:   hourlyProfitRepo,
		featureLimitRepo:   featureLimitRepo,
		systemVariableRepo: repository.NewSystemVariableRepository(db),
		variableRepo:       variableRepo,
		commercialClient:   commercialClient,
		notificationClient: notificationClient,
		eventBroadcaster:   eventBroadcaster,
		metrics:            marketplaceMetrics,
		db:                 db,
		log:                log,
	}
}

// BuyFeature implements the three-path buy logic using gRPC
// Returns updated feature after purchase
func (s *MarketplaceService) BuyFeature(ctx context.Context, featureID, buyerID uint64) (*pb.Feature, error) {
	// Load feature with properties and owner
	feature, properties, err := s.featureRepo.FindByID(ctx, featureID)
	if err != nil {
		return nil, fmt.Errorf("feature not found: %w", err)
	}

	// Get owner code
	var ownerCode string
	err = s.db.QueryRowContext(ctx, "SELECT code FROM users WHERE id = ?", feature.OwnerID).Scan(&ownerCode)
	if err != nil {
		return nil, fmt.Errorf("failed to get owner: %w", err)
	}

	// Route to appropriate buy path
	if constants.IsLimitedFeature(properties.RGB) {
		if err := s.handleLimitedFeature(ctx, feature, properties, buyerID); err != nil {
			return nil, err
		}
	} else if ownerCode == constants.RGBUserCode {
		if err := s.buyFromRGB(ctx, feature, properties, buyerID); err != nil {
			return nil, err
		}
	} else {
		if err := s.buyFromUser(ctx, feature, properties, buyerID); err != nil {
			return nil, err
		}
	}

	// Return updated feature (reload to get latest state)
	// We'll need to call GetFeature service method, but for now return basic info
	updatedFeature, updatedProperties, err := s.featureRepo.FindByID(ctx, featureID)
	if err != nil {
		return nil, fmt.Errorf("failed to reload feature: %w", err)
	}

	// Load geometry
	geometry, err := s.geometryRepo.GetByFeatureID(ctx, featureID)
	if err != nil {
		geometry = nil
	}

	// Convert to protobuf (basic conversion - full hydration would require feature service)
	pbFeature := models.FeatureToPB(updatedFeature, updatedProperties, geometry)
	return pbFeature, nil
}

// handleLimitedFeature - Path A with gRPC wallet operations
func (s *MarketplaceService) handleLimitedFeature(ctx context.Context, feature *models.Feature, properties *models.FeatureProperties, buyerID uint64) error {
	// Get feature limitation
	limitation, err := s.featureLimitRepo.GetLimitationByPropertyID(ctx, properties.ID)
	if err != nil || limitation == nil {
		return fmt.Errorf("خطایی رخ داده است. لطفا با پشتیبانی تماس بگیرید")
	}

	// Get buyer info
	var buyerName string
	var buyerDynastyID sql.NullInt64
	var buyerBirthdate sql.NullTime
	err = s.db.QueryRowContext(ctx,
		"SELECT u.name, u.dynasty_id, k.birthdate FROM users u LEFT JOIN kycs k ON u.id = k.user_id WHERE u.id = ?",
		buyerID,
	).Scan(&buyerName, &buyerDynastyID, &buyerBirthdate)
	if err != nil {
		return err
	}

	isUnder18 := false
	if buyerBirthdate.Valid {
		age := time.Since(buyerBirthdate.Time).Hours() / 24 / 365
		isUnder18 = age < 18
	}

	// Check buyer balance for color using gRPC
	color := constants.GetColor(properties.Karbari)
	if limitation.PriceLimit {
		hasBalance, err := s.commercialClient.CheckBalance(ctx, buyerID, color, properties.Stability)
		if err != nil || !hasBalance {
			return fmt.Errorf("برای خرید این ملک شما نیاز به %.2f لیتر رنگ %s دارید!",
				properties.Stability, constants.GetColorPersian(properties.Karbari))
		}
	}

	// Deduct buyer's color wallet via gRPC
	if err := s.commercialClient.DeductBalance(ctx, buyerID, color, properties.Stability); err != nil {
		return fmt.Errorf("failed to deduct buyer wallet: %w", err)
	}

	// Credit seller's color wallet via gRPC
	if err := s.commercialClient.AddBalance(ctx, feature.OwnerID, color, properties.Stability); err != nil {
		// Rollback buyer deduction
		s.commercialClient.AddBalance(ctx, buyerID, color, properties.Stability)
		return fmt.Errorf("failed to credit seller wallet: %w", err)
	}

	// Transfer ownership
	if err := s.featureRepo.UpdateOwner(ctx, feature.ID, buyerID); err != nil {
		return err
	}

	// Update properties
	pricingLimit := constants.DefaultPublicPricingLimit
	if isUnder18 {
		pricingLimit = constants.DefaultUnder18PricingLimit
	}

	newStatus := constants.ChangeStatusToSoldAndNotPriced(properties.Karbari)
	if err := s.propertiesRepo.UpdateStatus(ctx, feature.ID, newStatus, buyerName, "", pricingLimit); err != nil {
		return err
	}

	// Create trade
	tradeID, err := s.tradeRepo.Create(ctx, feature.ID, buyerID, feature.OwnerID, 0, 0)
	if err != nil {
		return err
	}

	s.log.Info("Limited feature purchased", "trade_id", tradeID, "feature_id", feature.ID, "buyer_id", buyerID)

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordTrade("limited", 0, 0)
	}

	// Create hourly profit
	withdrawProfitDays, err := s.getUserVariableWithdrawProfit(ctx, buyerID)
	if err != nil {
		withdrawProfitDays = 10
	}

	_, err = s.hourlyProfitRepo.Create(ctx, buyerID, feature.ID, color, withdrawProfitDays)
	if err != nil {
		s.log.Error("Failed to create hourly profit", "error", err)
	}

	// Track limited feature purchase
	if err := s.featureLimitRepo.TrackLimitedPurchase(ctx, buyerID, limitation.ID, feature.ID); err != nil {
		s.log.Error("Failed to track limited purchase", "error", err)
	}

	// Send notification to buyer
	if s.notificationClient != nil {
		colorPersian := constants.GetColorPersian(properties.Karbari)
		if err := s.notificationClient.SendBuyFeatureNotification(ctx, buyerID, feature.ID, true, colorPersian, properties.Stability, 0, 0); err != nil {
			s.log.Warn("Failed to send buy feature notification", "error", err)
		}
	}

	// Broadcast feature status change
	if s.eventBroadcaster != nil {
		if err := s.eventBroadcaster.BroadcastFeatureStatusChanged(ctx, feature.ID, newStatus); err != nil {
			s.log.Warn("Failed to broadcast feature status changed", "error", err)
		}
	}

	return nil
}

// buyFromRGB - Path B with gRPC wallet operations
func (s *MarketplaceService) buyFromRGB(ctx context.Context, feature *models.Feature, properties *models.FeatureProperties, buyerID uint64) error {
	// Get buyer info
	var buyerName string
	var buyerBirthdate sql.NullTime
	err := s.db.QueryRowContext(ctx,
		"SELECT u.name, k.birthdate FROM users u LEFT JOIN kycs k ON u.id = k.user_id WHERE u.id = ?",
		buyerID,
	).Scan(&buyerName, &buyerBirthdate)
	if err != nil {
		return err
	}

	isUnder18 := false
	if buyerBirthdate.Valid {
		age := time.Since(buyerBirthdate.Time).Hours() / 24 / 365
		isUnder18 = age < 18
	}

	color := constants.GetColor(properties.Karbari)

	// Check buyer balance via gRPC
	hasBalance, err := s.commercialClient.CheckBalance(ctx, buyerID, color, properties.Stability)
	if err != nil || !hasBalance {
		return fmt.Errorf("برای خرید این ملک شما نیاز به %.2f لیتر رنگ %s دارید!",
			properties.Stability, constants.GetColorPersian(properties.Karbari))
	}

	// Deduct buyer's wallet via gRPC
	if err := s.commercialClient.DeductBalance(ctx, buyerID, color, properties.Stability); err != nil {
		return err
	}

	// Credit RGB's wallet via gRPC
	if err := s.commercialClient.AddBalance(ctx, feature.OwnerID, color, properties.Stability); err != nil {
		// Rollback
		s.commercialClient.AddBalance(ctx, buyerID, color, properties.Stability)
		return err
	}

	// Transfer ownership
	if err := s.featureRepo.UpdateOwner(ctx, feature.ID, buyerID); err != nil {
		return err
	}

	// Update properties
	pricingLimit := constants.DefaultPublicPricingLimit
	if isUnder18 {
		pricingLimit = constants.DefaultUnder18PricingLimit
	}

	newStatus := constants.ChangeStatusToSoldAndNotPriced(properties.Karbari)
	if err := s.propertiesRepo.UpdateStatus(ctx, feature.ID, newStatus, buyerName, "", pricingLimit); err != nil {
		return err
	}

	// Create trade
	_, err = s.tradeRepo.Create(ctx, feature.ID, buyerID, feature.OwnerID, 0, 0)
	if err != nil {
		return err
	}

	// Create hourly profit
	withdrawProfitDays, _ := s.getUserVariableWithdrawProfit(ctx, buyerID)
	if withdrawProfitDays == 0 {
		withdrawProfitDays = 10
	}

	_, err = s.hourlyProfitRepo.Create(ctx, buyerID, feature.ID, color, withdrawProfitDays)
	if err != nil {
		s.log.Error("Failed to create hourly profit", "error", err)
	}

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordTrade("rgb", 0, 0)
	}

	// Send notification to buyer
	if s.notificationClient != nil {
		colorPersian := constants.GetColorPersian(properties.Karbari)
		if err := s.notificationClient.SendBuyFeatureNotification(ctx, buyerID, feature.ID, true, colorPersian, properties.Stability, 0, 0); err != nil {
			s.log.Warn("Failed to send buy feature notification", "error", err)
		}
	}

	// Broadcast feature status change
	if s.eventBroadcaster != nil {
		if err := s.eventBroadcaster.BroadcastFeatureStatusChanged(ctx, feature.ID, newStatus); err != nil {
			s.log.Warn("Failed to broadcast feature status changed", "error", err)
		}
	}

	return nil
}

// buyFromUser - Path C with gRPC wallet operations and transactions
func (s *MarketplaceService) buyFromUser(ctx context.Context, feature *models.Feature, properties *models.FeatureProperties, buyerID uint64) error {
	// Check underpriced restriction
	if err := s.checkUnderpricedRestriction(ctx, feature, properties); err != nil {
		return err
	}

	// Get buyer info
	var buyerName string
	var buyerBirthdate sql.NullTime
	err := s.db.QueryRowContext(ctx,
		"SELECT u.name, k.birthdate FROM users u LEFT JOIN kycs k ON u.id = k.user_id WHERE u.id = ?",
		buyerID,
	).Scan(&buyerName, &buyerBirthdate)
	if err != nil {
		return err
	}

	isUnder18 := false
	if buyerBirthdate.Valid {
		age := time.Since(buyerBirthdate.Time).Hours() / 24 / 365
		isUnder18 = age < 18
	}

	// Parse prices
	pricePSC := parseFloat(properties.PricePSC)
	priceIRR := parseFloat(properties.PriceIRR)

	// Calculate amounts with fees
	buyerChargePSC := constants.CalculateBuyerCharge(pricePSC)
	buyerChargeIRR := constants.CalculateBuyerCharge(priceIRR)
	sellerPayPSC := constants.CalculateSellerPayment(pricePSC)
	sellerPayIRR := constants.CalculateSellerPayment(priceIRR)
	platformFeePSC := constants.CalculatePlatformFee(pricePSC)
	platformFeeIRR := constants.CalculatePlatformFee(priceIRR)

	// Check buyer balance via gRPC
	hasPSC, _ := s.commercialClient.CheckBalance(ctx, buyerID, "psc", buyerChargePSC)
	hasIRR, _ := s.commercialClient.CheckBalance(ctx, buyerID, "irr", buyerChargeIRR)
	if !hasPSC || !hasIRR {
		return fmt.Errorf("موجودی شما کافی نمی باشد")
	}

	// Deduct from buyer via gRPC
	if err := s.commercialClient.DeductBalance(ctx, buyerID, "psc", buyerChargePSC); err != nil {
		return err
	}
	if err := s.commercialClient.DeductBalance(ctx, buyerID, "irr", buyerChargeIRR); err != nil {
		// Rollback PSC
		s.commercialClient.AddBalance(ctx, buyerID, "psc", buyerChargePSC)
		return err
	}

	// Pay seller via gRPC
	if err := s.commercialClient.AddBalance(ctx, feature.OwnerID, "psc", sellerPayPSC); err != nil {
		return err
	}
	if err := s.commercialClient.AddBalance(ctx, feature.OwnerID, "irr", sellerPayIRR); err != nil {
		return err
	}

	// Pay RGB platform via gRPC
	rgbUserID, err := s.getRGBUserID(ctx)
	if err == nil {
		s.commercialClient.AddBalance(ctx, rgbUserID, "psc", platformFeePSC)
		s.commercialClient.AddBalance(ctx, rgbUserID, "irr", platformFeeIRR)
	}

	// Create trade
	tradeID, err := s.tradeRepo.Create(ctx, feature.ID, buyerID, feature.OwnerID, priceIRR, pricePSC)
	if err != nil {
		return err
	}

	// Create commission via direct SQL (Commercial service doesn't have commission endpoint yet)
	s.createCommission(ctx, tradeID, platformFeePSC, platformFeeIRR)

	// Transfer ownership
	if err := s.featureRepo.UpdateOwner(ctx, feature.ID, buyerID); err != nil {
		return err
	}

	// Update properties
	pricingLimit := constants.DefaultPublicPricingLimit
	if isUnder18 {
		pricingLimit = constants.DefaultUnder18PricingLimit
	}

	newStatus := constants.ChangeStatusToSoldAndNotPriced(properties.Karbari)
	if err := s.propertiesRepo.UpdateStatus(ctx, feature.ID, newStatus, buyerName, "", pricingLimit); err != nil {
		return err
	}

	// Transfer hourly profit
	withdrawProfitDays, _ := s.getUserVariableWithdrawProfit(ctx, buyerID)
	if withdrawProfitDays == 0 {
		withdrawProfitDays = 10
	}

	oldProfit, err := s.hourlyProfitRepo.GetByFeatureAndUser(ctx, feature.ID, feature.OwnerID)
	if err == nil && oldProfit != nil && oldProfit.Amount > 0 {
		// Add accumulated profit to seller's wallet via gRPC
		if err := s.commercialClient.AddBalance(ctx, feature.OwnerID, oldProfit.Asset, oldProfit.Amount); err != nil {
			s.log.Error("Failed to transfer profit to seller", "error", err)
		}
	}

	// Transfer profit to new owner
	_ = constants.GetColor(properties.Karbari) // Color for potential future use
	if err := s.hourlyProfitRepo.TransferProfitToNewOwner(ctx, feature.ID, feature.OwnerID, buyerID, withdrawProfitDays); err != nil {
		s.log.Error("Failed to transfer hourly profit", "error", err)
	}

	// Cancel all pending buy requests
	if err := s.buyRequestRepo.CancelAllForFeature(ctx, feature.ID); err != nil {
		s.log.Error("Failed to cancel buy requests", "error", err)
	}

	// Update sell requests
	if err := s.sellRequestRepo.UpdateAllForFeatureToCompleted(ctx, feature.ID); err != nil {
		s.log.Error("Failed to update sell requests", "error", err)
	}

	s.log.Info("Feature purchased from user",
		"trade_id", tradeID,
		"feature_id", feature.ID,
		"buyer_id", buyerID,
		"seller_id", feature.OwnerID,
	)

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordTrade("user", buyerChargePSC, buyerChargeIRR)
	}

	// Send notifications to buyer and seller
	if s.notificationClient != nil {
		// Notify buyer - user-to-user purchase (PSC + IRR)
		if err := s.notificationClient.SendBuyFeatureNotification(ctx, buyerID, feature.ID, false, "", 0, buyerChargePSC, buyerChargeIRR); err != nil {
			s.log.Warn("Failed to send buy feature notification to buyer", "error", err)
		}

		// Notify seller - they received payment
		if err := s.notificationClient.SendNotification(ctx, feature.OwnerID, "sellFeature", "فروش ملک",
			fmt.Sprintf("ملک %d با موفقیت فروخته شد.", feature.ID),
			map[string]string{
				"feature_id": fmt.Sprintf("%d", feature.ID),
				"trade_id":   fmt.Sprintf("%d", tradeID),
				"related-to": "transactions",
			}); err != nil {
			s.log.Warn("Failed to send sell feature notification to seller", "error", err)
		}
	}

	// Broadcast feature status change
	if s.eventBroadcaster != nil {
		if err := s.eventBroadcaster.BroadcastFeatureStatusChanged(ctx, feature.ID, newStatus); err != nil {
			s.log.Warn("Failed to broadcast feature status changed", "error", err)
		}
	}

	return nil
}

// Helper methods
func (s *MarketplaceService) checkUnderpricedRestriction(ctx context.Context, feature *models.Feature, properties *models.FeatureProperties) error {
	isUnderpriced, err := s.sellRequestRepo.IsUnderpriced(ctx, feature.ID)
	if err != nil || !isUnderpriced {
		return nil
	}

	latestSellReq, err := s.sellRequestRepo.GetLatestUnderpricedForSeller(ctx, feature.OwnerID)
	if err != nil || latestSellReq == nil {
		return nil
	}

	latestTrade, err := s.tradeRepo.GetLatestUnderpricedForSeller(ctx, feature.OwnerID, latestSellReq.FeatureID)
	if err != nil || latestTrade == nil {
		return nil
	}

	if !s.tradeRepo.IsWithin24Hours(latestTrade) {
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

func (s *MarketplaceService) getUserVariableWithdrawProfit(ctx context.Context, userID uint64) (int, error) {
	var days int
	err := s.db.QueryRowContext(ctx, "SELECT withdraw_profit FROM user_variables WHERE user_id = ?", userID).Scan(&days)
	if err != nil {
		return 10, nil
	}
	return days, nil
}

func (s *MarketplaceService) getRGBUserID(ctx context.Context) (uint64, error) {
	var rgbID uint64
	err := s.db.QueryRowContext(ctx, "SELECT id FROM users WHERE code = ?", constants.RGBUserCode).Scan(&rgbID)
	return rgbID, err
}

func (s *MarketplaceService) createCommission(ctx context.Context, tradeID uint64, psc, irr float64) error {
	query := "INSERT INTO comissions (trade_id, psc, irr, created_at, updated_at) VALUES (?, ?, ?, NOW(), NOW())"
	_, err := s.db.ExecContext(ctx, query, tradeID, psc, irr)
	return err
}

func (s *MarketplaceService) createCommissionWithTx(ctx context.Context, tx *sql.Tx, tradeID uint64, psc, irr float64) error {
	query := "INSERT INTO comissions (trade_id, psc, irr, created_at, updated_at) VALUES (?, ?, ?, NOW(), NOW())"
	_, err := tx.ExecContext(ctx, query, tradeID, psc, irr)
	return err
}

func parseFloat(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

// SendBuyRequest creates a buy request for a feature
// Implements POST /api/buy-requests/store/{feature}
func (s *MarketplaceService) SendBuyRequest(ctx context.Context, req *pb.SendBuyRequestRequest) (*models.BuyFeatureRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	buyerID := req.BuyerId
	featureID := req.FeatureId
	pricePSC := parseFloat(req.PricePsc)
	priceIRR := parseFloat(req.PriceIrr)
	note := req.Note

	// Get feature and seller
	feature, properties, err := s.featureRepo.FindByID(ctx, featureID)
	if err != nil {
		return nil, fmt.Errorf("feature not found: %w", err)
	}

	sellerID := feature.OwnerID

	// Check if buyer has pending request
	hasPending, _ := s.buyRequestRepo.HasPendingRequest(ctx, buyerID, featureID)
	if hasPending {
		return nil, fmt.Errorf("you already have a pending buy request for this feature")
	}

	// Validate price - cannot be both zero
	if pricePSC == 0 && priceIRR == 0 {
		return nil, fmt.Errorf("price_psc and price_irr cannot both be zero")
	}

	// Validate price against minimum_price_percentage
	totalRequestedPrice := priceIRR + (pricePSC * s.getVariableRate(ctx, "psc"))
	color := constants.GetColor(properties.Karbari)
	colorRate := s.getVariableRate(ctx, color)
	totalFeaturePrice := properties.Stability * colorRate

	floorPercentage := float64(properties.MinimumPricePercentage)
	actualPercentage := (totalRequestedPrice / totalFeaturePrice) * 100

	if actualPercentage < floorPercentage {
		return nil, fmt.Errorf("شما مجاز به ارسال درخواست خرید به کمتر از %.0f%% قیمت ملک نمی باشید!", floorPercentage)
	}

	// Calculate amounts with fees
	buyerChargePSC := constants.CalculateBuyerCharge(pricePSC)
	buyerChargeIRR := constants.CalculateBuyerCharge(priceIRR)

	// Check buyer balance via gRPC
	if s.commercialClient != nil {
		hasPSC, _ := s.commercialClient.CheckBalance(ctx, buyerID, "psc", buyerChargePSC)
		hasIRR, _ := s.commercialClient.CheckBalance(ctx, buyerID, "irr", buyerChargeIRR)
		if !hasPSC {
			return nil, fmt.Errorf("موجودی psc شما کافی نیست!")
		}
		if !hasIRR {
			return nil, fmt.Errorf("موجودی ریال شما کافی نیست!")
		}
	}

	// Create buy request
	requestID, err := s.buyRequestRepo.Create(ctx, buyerID, sellerID, featureID, note, pricePSC, priceIRR)
	if err != nil {
		return nil, err
	}

	// Deduct buyer's wallet via gRPC (lock funds)
	if s.commercialClient != nil {
		if err := s.commercialClient.DeductBalance(ctx, buyerID, "psc", buyerChargePSC); err != nil {
			return nil, fmt.Errorf("failed to lock PSC: %w", err)
		}
		if err := s.commercialClient.DeductBalance(ctx, buyerID, "irr", buyerChargeIRR); err != nil {
			// Rollback PSC
			s.commercialClient.AddBalance(ctx, buyerID, "psc", buyerChargePSC)
			return nil, fmt.Errorf("failed to lock IRR: %w", err)
		}

		// Create locked asset record
		if _, err := s.lockedAssetRepo.Create(ctx, requestID, featureID, buyerChargePSC, buyerChargeIRR); err != nil {
			s.log.Error("Failed to create locked asset", "error", err)
		}

		// Create transactions via gRPC
		s.commercialClient.CreateTransaction(ctx, buyerID, "psc", buyerChargePSC, "withdraw", 0, "App\\Models\\BuyFeatureRequest", requestID)
		s.commercialClient.CreateTransaction(ctx, buyerID, "irr", buyerChargeIRR, "withdraw", 0, "App\\Models\\BuyFeatureRequest", requestID)
	}

	// Get the created request
	buyRequest, err := s.buyRequestRepo.FindByID(ctx, requestID)
	if err != nil {
		return nil, err
	}

	s.log.Info("Buy request created",
		"request_id", requestID,
		"buyer_id", buyerID,
		"feature_id", featureID,
	)

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordBuyRequest("pending")
		// Update locked assets gauge
		s.updateLockedAssetsMetrics(ctx)
	}

	// Send notifications to buyer and seller
	if s.notificationClient != nil {
		// Notify buyer
		if err := s.notificationClient.SendBuyRequestNotification(ctx, buyerID, "buyer", requestID, featureID, pricePSC, priceIRR); err != nil {
			s.log.Warn("Failed to send buy request notification to buyer", "error", err)
		}
		// Notify seller
		if err := s.notificationClient.SendBuyRequestNotification(ctx, sellerID, "seller", requestID, featureID, pricePSC, priceIRR); err != nil {
			s.log.Warn("Failed to send buy request notification to seller", "error", err)
		}
	}

	return buyRequest, nil
}

// AcceptBuyRequest accepts a buy request
// Implements POST /api/buy-requests/accept/{buyFeatureRequest}
// Note: This operation involves both gRPC wallet operations and DB updates.
// Wallet operations via gRPC cannot be rolled back if DB operations fail.
// If DB operations fail after wallet operations, manual reconciliation may be needed.
// TODO: Implement full transaction support with repository WithTx methods for atomicity.
func (s *MarketplaceService) AcceptBuyRequest(ctx context.Context, requestID, sellerID uint64) (*models.BuyFeatureRequest, error) {
	// Get buy request
	buyRequest, err := s.buyRequestRepo.FindByID(ctx, requestID)
	if err != nil || buyRequest == nil {
		return nil, fmt.Errorf("buy request not found: %w", err)
	}

	// Verify seller
	if buyRequest.SellerID != sellerID {
		return nil, fmt.Errorf("unauthorized: not the seller")
	}

	// Check status is pending
	if buyRequest.Status != 0 {
		return nil, fmt.Errorf("buy request is not pending")
	}

	// Get feature
	feature, properties, err := s.featureRepo.FindByID(ctx, buyRequest.FeatureID)
	if err != nil {
		return nil, fmt.Errorf("feature not found: %w", err)
	}

	// Check underpriced restriction
	if err := s.checkUnderpricedRestriction(ctx, feature, properties); err != nil {
		return nil, err
	}

	// Get locked assets (not used in this function but kept for consistency)
	_, err = s.lockedAssetRepo.GetByBuyRequestID(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("locked assets not found: %w", err)
	}

	pscAmount := buyRequest.PricePSC
	irrAmount := buyRequest.PriceIRR
	pscFee := constants.CalculateFee(pscAmount)
	irrFee := constants.CalculateFee(irrAmount)

	// Perform wallet operations via gRPC first (these cannot be rolled back)
	if s.commercialClient != nil {
		// Pay seller via gRPC (price - fee)
		if err := s.commercialClient.AddBalance(ctx, sellerID, "psc", pscAmount-pscFee); err != nil {
			return nil, fmt.Errorf("failed to pay seller PSC: %w", err)
		}
		if err := s.commercialClient.AddBalance(ctx, sellerID, "irr", irrAmount-irrFee); err != nil {
			// Attempt rollback of PSC (best effort)
			s.commercialClient.DeductBalance(ctx, sellerID, "psc", pscAmount-pscFee)
			return nil, fmt.Errorf("failed to pay seller IRR: %w", err)
		}

		// Pay RGB platform via gRPC (fee × 2)
		rgbUserID, err := s.getRGBUserID(ctx)
		if err == nil {
			s.commercialClient.AddBalance(ctx, rgbUserID, "psc", pscFee*2)
			s.commercialClient.AddBalance(ctx, rgbUserID, "irr", irrFee*2)
		}

		// Transfer hourly profit to seller if any
		oldProfit, _ := s.hourlyProfitRepo.GetByFeatureAndUser(ctx, feature.ID, sellerID)
		if oldProfit != nil && oldProfit.Amount > 0 {
			if err := s.commercialClient.AddBalance(ctx, sellerID, oldProfit.Asset, oldProfit.Amount); err != nil {
				s.log.Warn("Failed to transfer profit to seller", "error", err)
			}
		}
	}

	// Wrap all DB operations in a transaction for atomicity
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Auto-rollback if not committed

	// Create trade within transaction
	// Note: We need to use a transaction-aware version or execute directly
	tradeID, err := s.tradeRepo.Create(ctx, buyRequest.FeatureID, buyRequest.BuyerID, sellerID, irrAmount, pscAmount)
	if err != nil {
		return nil, fmt.Errorf("failed to create trade: %w", err)
	}

	// Create transactions for seller via gRPC (outside transaction - these are external)
	if s.commercialClient != nil {
		s.commercialClient.CreateTransaction(ctx, sellerID, "psc", pscAmount-pscFee, "deposit", 1, "App\\Models\\Trade", tradeID)
		s.commercialClient.CreateTransaction(ctx, sellerID, "irr", irrAmount-irrFee, "deposit", 1, "App\\Models\\Trade", tradeID)
	}

	// Create commission within transaction
	if err := s.createCommissionWithTx(ctx, tx, tradeID, pscFee*2, irrFee*2); err != nil {
		return nil, fmt.Errorf("failed to create commission: %w", err)
	}

	// Transfer ownership within transaction
	if err := s.featureRepo.UpdateOwnerWithTx(ctx, tx, feature.ID, buyRequest.BuyerID); err != nil {
		return nil, fmt.Errorf("failed to transfer ownership: %w", err)
	}

	// Update properties within transaction
	buyerName := s.getUserName(ctx, buyRequest.BuyerID)
	isUnder18 := s.isUserUnder18(ctx, buyRequest.BuyerID)
	pricingLimit := constants.DefaultPublicPricingLimit
	if isUnder18 {
		pricingLimit = constants.DefaultUnder18PricingLimit
	}

	newStatus := constants.ChangeStatusToSoldAndNotPriced(properties.Karbari)
	if err := s.propertiesRepo.UpdateStatusWithTx(ctx, tx, feature.ID, newStatus, buyerName, "", pricingLimit); err != nil {
		return nil, fmt.Errorf("failed to update properties: %w", err)
	}

	// Transfer hourly profit within transaction
	withdrawProfitDays, _ := s.getUserVariableWithdrawProfit(ctx, buyRequest.BuyerID)
	if withdrawProfitDays == 0 {
		withdrawProfitDays = 10
	}

	if err := s.hourlyProfitRepo.TransferProfitToNewOwnerWithTx(ctx, tx, feature.ID, sellerID, buyRequest.BuyerID, withdrawProfitDays); err != nil {
		return nil, fmt.Errorf("failed to transfer hourly profit: %w", err)
	}

	// Update request status and soft delete within transaction
	if err := s.buyRequestRepo.UpdateStatusWithTx(ctx, tx, requestID, 1); err != nil {
		return nil, fmt.Errorf("failed to update request status: %w", err)
	}
	if err := s.buyRequestRepo.SoftDeleteWithTx(ctx, tx, requestID); err != nil {
		return nil, fmt.Errorf("failed to soft delete request: %w", err)
	}
	if err := s.lockedAssetRepo.DeleteWithTx(ctx, tx, requestID); err != nil {
		return nil, fmt.Errorf("failed to delete locked asset: %w", err)
	}

	// Cancel other requests and refund (outside transaction - involves gRPC)
	allRequests, _ := s.buyRequestRepo.GetAllForFeature(ctx, buyRequest.FeatureID)
	for _, req := range allRequests {
		if req.ID != requestID {
			s.refundBuyRequest(ctx, req.ID)
		}
	}

	// Update sell requests within transaction
	if err := s.sellRequestRepo.UpdateAllForFeatureToCompletedWithTx(ctx, tx, buyRequest.FeatureID); err != nil {
		return nil, fmt.Errorf("failed to update sell requests: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.log.Info("Buy request accepted",
		"request_id", requestID,
		"feature_id", buyRequest.FeatureID,
		"buyer_id", buyRequest.BuyerID,
		"seller_id", sellerID,
	)

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordBuyRequest("accepted")
		s.metrics.RecordTrade("user", pscAmount, irrAmount)
		s.updateLockedAssetsMetrics(ctx)
	}

	// Send notifications to buyer and seller
	if s.notificationClient != nil {
		// Get trade ID for notification
		latestTrade, _ := s.tradeRepo.GetLatestForFeature(ctx, buyRequest.FeatureID)
		var tradeID uint64
		if latestTrade != nil {
			tradeID = latestTrade.ID
		}

		// Notify buyer - user-to-user purchase (PSC + IRR)
		if err := s.notificationClient.SendBuyFeatureNotification(ctx, buyRequest.BuyerID, buyRequest.FeatureID, false, "", 0, pscAmount, irrAmount); err != nil {
			s.log.Warn("Failed to send buy feature notification to buyer", "error", err)
		}

		// Notify seller - they received payment
		// Using a generic notification since Laravel sends sellFeature notification
		if err := s.notificationClient.SendNotification(ctx, sellerID, "sellFeature", "فروش ملک",
			fmt.Sprintf("ملک %d با موفقیت فروخته شد.", buyRequest.FeatureID),
			map[string]string{
				"feature_id": fmt.Sprintf("%d", buyRequest.FeatureID),
				"trade_id":   fmt.Sprintf("%d", tradeID),
				"related-to": "transactions",
			}); err != nil {
			s.log.Warn("Failed to send sell feature notification to seller", "error", err)
		}
	}

	// Broadcast feature status change
	if s.eventBroadcaster != nil {
		if err := s.eventBroadcaster.BroadcastFeatureStatusChanged(ctx, feature.ID, newStatus); err != nil {
			s.log.Warn("Failed to broadcast feature status changed", "error", err)
		}
	}

	// Return the request (will be soft-deleted but still accessible)
	return buyRequest, nil
}

// CreateSellRequest creates a sell request for a feature
// Implements POST /api/sell-requests/store/{feature}
func (s *MarketplaceService) CreateSellRequest(ctx context.Context, req *pb.CreateSellRequestRequest) (*models.SellFeatureRequest, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	sellerID := req.SellerId
	featureID := req.FeatureId

	// Get feature and properties
	feature, properties, err := s.featureRepo.FindByID(ctx, featureID)
	if err != nil {
		return nil, fmt.Errorf("feature not found: %w", err)
	}

	// Verify ownership
	if feature.OwnerID != sellerID {
		return nil, fmt.Errorf("unauthorized: not the owner")
	}

	// Get pricing limits from system variables
	publicPricingLimit, under18PricingLimit, err := s.systemVariableRepo.GetPricingLimits(ctx)
	if err != nil {
		publicPricingLimit = constants.DefaultPublicPricingLimit
		under18PricingLimit = constants.DefaultUnder18PricingLimit
	}

	// Check if user is under 18
	isUnder18 := s.isUserUnder18(ctx, sellerID)

	// Parse request - either explicit prices or percentage
	var requestedPricePSC, requestedPriceIRR float64
	var pricingPercentage int

	hasExplicitPrices := (req.PricePsc != "" && req.PricePsc != "0") || (req.PriceIrr != "" && req.PriceIrr != "0")
	hasPercentage := req.MinimumPricePercentage > 0

	// Validation: mutually exclusive
	if hasExplicitPrices && hasPercentage {
		return nil, fmt.Errorf("price_psc/price_irr and minimum_price_percentage are mutually exclusive")
	}

	if !hasExplicitPrices && !hasPercentage {
		return nil, fmt.Errorf("either price_psc/price_irr or minimum_price_percentage is required")
	}

	if hasPercentage {
		// Calculate prices from percentage
		if req.MinimumPricePercentage < 80 {
			return nil, fmt.Errorf("minimum_price_percentage must be at least 80")
		}

		// Check age-based limit
		if isUnder18 && req.MinimumPricePercentage < int32(under18PricingLimit) {
			return nil, fmt.Errorf("شما مجاز به فروش زمین خود به کمتر از %d درصد قیمت خرید ملک نمی باشید", under18PricingLimit)
		} else if !isUnder18 && req.MinimumPricePercentage < int32(publicPricingLimit) {
			return nil, fmt.Errorf("شما مجاز به فروش زمین خود به کمتر از %d درصد قیمت خرید ملک نمی باشید", publicPricingLimit)
		}

		// Calculate total price from stability and color rate
		color := constants.GetColor(properties.Karbari)
		colorRate := s.getVariableRate(ctx, color)
		pscRate := s.getVariableRate(ctx, "psc")

		totalPrice := properties.Stability * colorRate * float64(req.MinimumPricePercentage) / 100.0

		// Split 50/50 between PSC and IRR
		requestedPricePSC = (totalPrice * 0.5) / pscRate
		requestedPriceIRR = totalPrice * 0.5
		pricingPercentage = int(req.MinimumPricePercentage)
	} else {
		// Validate explicit prices
		var err error
		if req.PricePsc != "" {
			requestedPricePSC, err = strconv.ParseFloat(req.PricePsc, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid price_psc: %w", err)
			}
		}
		if req.PriceIrr != "" {
			requestedPriceIRR, err = strconv.ParseFloat(req.PriceIrr, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid price_irr: %w", err)
			}
		}

		// At least one must be non-zero
		if requestedPricePSC == 0 && requestedPriceIRR == 0 {
			return nil, fmt.Errorf("at least one of price_psc or price_irr must be greater than 0")
		}

		// Calculate implied percentage
		pscRate := s.getVariableRate(ctx, "psc")
		color := constants.GetColor(properties.Karbari)
		colorRate := s.getVariableRate(ctx, color)

		totalRequestedPrice := requestedPriceIRR + (requestedPricePSC * pscRate)
		totalTradedPrice := properties.Stability * colorRate

		if totalTradedPrice > 0 {
			pricingPercentage = int((totalRequestedPrice / totalTradedPrice) * 100)
		} else {
			pricingPercentage = 100
		}

		// Check pricing floor
		if isUnder18 && pricingPercentage < under18PricingLimit {
			return nil, fmt.Errorf("شما مجاز به فروش زمین خود به کمتر از %d درصد قیمت خرید ملک نمی باشید", under18PricingLimit)
		} else if !isUnder18 && pricingPercentage < publicPricingLimit {
			return nil, fmt.Errorf("شما مجاز به فروش زمین خود به کمتر از %d درصد قیمت خرید ملک نمی باشید", publicPricingLimit)
		}
	}

	// Create sell request
	sellRequestID, err := s.sellRequestRepo.Create(ctx, sellerID, featureID, requestedPricePSC, requestedPriceIRR, pricingPercentage)
	if err != nil {
		return nil, fmt.Errorf("failed to create sell request: %w", err)
	}

	// Update feature properties: RGB status and pricing
	newRGBStatus := constants.ChangeStatusToSoldAndPriced(properties.Karbari)
	pricePSCStr := fmt.Sprintf("%.10f", requestedPricePSC)
	priceIRRStr := fmt.Sprintf("%.10f", requestedPriceIRR)

	if err := s.propertiesRepo.Update(ctx, featureID, map[string]interface{}{
		"rgb":                      newRGBStatus,
		"price_psc":                pricePSCStr,
		"price_irr":                priceIRRStr,
		"minimum_price_percentage": pricingPercentage,
	}); err != nil {
		return nil, fmt.Errorf("failed to update feature properties: %w", err)
	}

	// Broadcast FeatureStatusChanged event
	if s.eventBroadcaster != nil {
		if err := s.eventBroadcaster.BroadcastFeatureStatusChanged(ctx, featureID, newRGBStatus); err != nil {
			s.log.Warn("Failed to broadcast feature status changed", "error", err)
		}
	}

	// Send notification to seller
	if s.notificationClient != nil {
		if err := s.notificationClient.SendSellRequestNotification(ctx, sellerID, featureID, properties.ID); err != nil {
			s.log.Warn("Failed to send sell request notification", "error", err)
		}
	}

	// Get created sell request
	sellRequest, err := s.sellRequestRepo.FindByID(ctx, sellRequestID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve created sell request: %w", err)
	}

	s.log.Info("Sell request created",
		"request_id", sellRequestID,
		"seller_id", sellerID,
		"feature_id", featureID,
		"pricing_percentage", pricingPercentage,
	)

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordSellRequest()
	}

	return sellRequest, nil
}

// ListSellRequests lists all sell requests for a seller
// Implements GET /api/sell-requests
func (s *MarketplaceService) ListSellRequests(ctx context.Context, sellerID uint64) ([]*models.SellFeatureRequest, error) {
	requests, err := s.sellRequestRepo.ListBySellerID(ctx, sellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list sell requests: %w", err)
	}
	return requests, nil
}

// DeleteSellRequest deletes a sell request and reverts feature status
// Implements DELETE /api/sell-requests/{sellRequest}
func (s *MarketplaceService) DeleteSellRequest(ctx context.Context, sellRequestID, sellerID uint64) error {
	// Get sell request
	sellRequest, err := s.sellRequestRepo.FindByID(ctx, sellRequestID)
	if err != nil {
		return fmt.Errorf("sell request not found: %w", err)
	}
	if sellRequest == nil {
		return fmt.Errorf("sell request not found")
	}

	// Verify ownership
	if sellRequest.SellerID != sellerID {
		return fmt.Errorf("unauthorized: not the seller")
	}

	// Get feature and properties
	feature, properties, err := s.featureRepo.FindByID(ctx, sellRequest.FeatureID)
	if err != nil {
		return fmt.Errorf("feature not found: %w", err)
	}

	// Revert RGB status to SoldAndNotPriced
	newRGBStatus := constants.ChangeStatusToSoldAndNotPriced(properties.Karbari)
	if err := s.propertiesRepo.Update(ctx, feature.ID, map[string]interface{}{
		"rgb": newRGBStatus,
	}); err != nil {
		return fmt.Errorf("failed to update feature properties: %w", err)
	}

	// Delete sell request
	if err := s.sellRequestRepo.Delete(ctx, sellRequestID); err != nil {
		return fmt.Errorf("failed to delete sell request: %w", err)
	}

	// Broadcast FeatureStatusChanged event
	if s.eventBroadcaster != nil {
		if err := s.eventBroadcaster.BroadcastFeatureStatusChanged(ctx, feature.ID, newRGBStatus); err != nil {
			s.log.Warn("Failed to broadcast feature status changed", "error", err)
		}
	}

	s.log.Info("Sell request deleted",
		"request_id", sellRequestID,
		"seller_id", sellerID,
		"feature_id", feature.ID,
	)

	return nil
}

// RequestGracePeriod sets requested_grace_period on a pending buy request (deprecated RPC; prefer UpdateGracePeriod).
// gracePeriod is the number of days as a decimal string (1–30). buyerID is a legacy proto field name: it must be the
// seller's user id, matching Laravel BuyFeatureRequestPolicy::addGracePeriod (seller-only).
func (s *MarketplaceService) RequestGracePeriod(ctx context.Context, requestID, sellerID uint64, gracePeriod string) error {
	days, err := strconv.ParseInt(gracePeriod, 10, 32)
	if err != nil || days < 1 || days > 30 {
		return fmt.Errorf("grace period must be between 1 and 30 days")
	}
	return s.UpdateGracePeriod(ctx, requestID, sellerID, int32(days))
}

// ListBuyRequests lists all buy requests for a buyer
// Implements GET /api/buy-requests
func (s *MarketplaceService) ListBuyRequests(ctx context.Context, buyerID uint64) ([]*models.BuyFeatureRequest, error) {
	requests, err := s.buyRequestRepo.ListByBuyerID(ctx, buyerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list buy requests: %w", err)
	}
	return requests, nil
}

// ListReceivedBuyRequests lists all buy requests received by a seller
// Implements GET /api/buy-requests/recieved
func (s *MarketplaceService) ListReceivedBuyRequests(ctx context.Context, sellerID uint64) ([]*models.BuyFeatureRequest, error) {
	requests, err := s.buyRequestRepo.ListBySellerID(ctx, sellerID)
	if err != nil {
		return nil, fmt.Errorf("failed to list received buy requests: %w", err)
	}
	return requests, nil
}

// RejectBuyRequest rejects a buy request and refunds the buyer
// Implements POST /api/buy-requests/reject/{buyFeatureRequest}
func (s *MarketplaceService) RejectBuyRequest(ctx context.Context, requestID, sellerID uint64) error {
	buyRequest, err := s.buyRequestRepo.FindByID(ctx, requestID)
	if err != nil || buyRequest == nil {
		return fmt.Errorf("buy request not found: %w", err)
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

	if s.commercialClient != nil {
		// Refund buyer
		if err := s.commercialClient.AddBalance(ctx, buyRequest.BuyerID, "psc", lockedAsset.PSC); err != nil {
			return fmt.Errorf("failed to refund PSC: %w", err)
		}
		if err := s.commercialClient.AddBalance(ctx, buyRequest.BuyerID, "irr", lockedAsset.IRR); err != nil {
			return fmt.Errorf("failed to refund IRR: %w", err)
		}
	}

	// Delete locked asset and buy request
	if err := s.lockedAssetRepo.Delete(ctx, requestID); err != nil {
		s.log.Error("Failed to delete locked asset", "error", err)
	}
	if err := s.buyRequestRepo.Delete(ctx, requestID); err != nil {
		return fmt.Errorf("failed to delete buy request: %w", err)
	}

	s.log.Info("Buy request rejected", "request_id", requestID, "seller_id", sellerID)

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordBuyRequest("rejected")
		s.updateLockedAssetsMetrics(ctx)
	}

	return nil
}

// DeleteBuyRequest deletes a buy request (buyer cancels their own offer)
// Implements DELETE /api/buy-requests/delete/{buyFeatureRequest}
func (s *MarketplaceService) DeleteBuyRequest(ctx context.Context, requestID, buyerID uint64) error {
	buyRequest, err := s.buyRequestRepo.FindByID(ctx, requestID)
	if err != nil || buyRequest == nil {
		return fmt.Errorf("buy request not found: %w", err)
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

	if s.commercialClient != nil {
		// Refund buyer
		if err := s.commercialClient.AddBalance(ctx, buyRequest.BuyerID, "psc", lockedAsset.PSC); err != nil {
			return fmt.Errorf("failed to refund PSC: %w", err)
		}
		if err := s.commercialClient.AddBalance(ctx, buyRequest.BuyerID, "irr", lockedAsset.IRR); err != nil {
			return fmt.Errorf("failed to refund IRR: %w", err)
		}
	}

	// Delete locked asset and buy request
	if err := s.lockedAssetRepo.Delete(ctx, requestID); err != nil {
		s.log.Error("Failed to delete locked asset", "error", err)
	}
	if err := s.buyRequestRepo.Delete(ctx, requestID); err != nil {
		return fmt.Errorf("failed to delete buy request: %w", err)
	}

	s.log.Info("Buy request deleted", "request_id", requestID, "buyer_id", buyerID)

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordBuyRequest("cancelled")
		s.updateLockedAssetsMetrics(ctx)
	}

	return nil
}

// UpdateGracePeriod updates the grace period for a buy request
// Implements POST /api/buy-requests/add-grace-period/{buyFeatureRequest}
func (s *MarketplaceService) UpdateGracePeriod(ctx context.Context, requestID, sellerID uint64, gracePeriodDays int32) error {
	if gracePeriodDays < 1 || gracePeriodDays > 30 {
		return fmt.Errorf("grace period must be between 1 and 30 days")
	}

	buyRequest, err := s.buyRequestRepo.FindByID(ctx, requestID)
	if err != nil || buyRequest == nil {
		return fmt.Errorf("buy request not found: %w", err)
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

// Helper methods
func (s *MarketplaceService) refundBuyRequest(ctx context.Context, requestID uint64) {
	buyRequest, err := s.buyRequestRepo.FindByID(ctx, requestID)
	if err != nil {
		return
	}

	lockedAsset, err := s.lockedAssetRepo.GetByBuyRequestID(ctx, requestID)
	if err != nil {
		return
	}

	if s.commercialClient != nil {
		// Refund buyer via gRPC
		s.commercialClient.AddBalance(ctx, buyRequest.BuyerID, "psc", lockedAsset.PSC)
		s.commercialClient.AddBalance(ctx, buyRequest.BuyerID, "irr", lockedAsset.IRR)
	}

	// Delete locked asset and soft delete request
	s.lockedAssetRepo.Delete(ctx, requestID)
	s.buyRequestRepo.SoftDelete(ctx, requestID)

	s.log.Info("Buy request refunded", "request_id", requestID, "buyer_id", buyRequest.BuyerID)
}

func (s *MarketplaceService) getVariableRate(ctx context.Context, asset string) float64 {
	return s.variableRepo.GetRateWithCache(ctx, asset)
}

// updateLockedAssetsMetrics updates the locked assets gauge by querying all pending buy requests
func (s *MarketplaceService) updateLockedAssetsMetrics(ctx context.Context) {
	if s.metrics == nil {
		return
	}

	var totalPSC, totalIRR float64
	query := `
		SELECT COALESCE(SUM(la.psc), 0), COALESCE(SUM(la.irr), 0)
		FROM locked_assets la
		INNER JOIN buy_feature_requests bfr ON la.buy_feature_request_id = bfr.id
		WHERE bfr.status = 0 AND bfr.deleted_at IS NULL
	`
	err := s.db.QueryRowContext(ctx, query).Scan(&totalPSC, &totalIRR)
	if err != nil {
		s.log.Warn("Failed to update locked assets metrics", "error", err)
		return
	}

	s.metrics.UpdateLockedAssets(totalPSC, totalIRR)
}

func (s *MarketplaceService) getUserName(ctx context.Context, userID uint64) string {
	var name string
	s.db.QueryRowContext(ctx, "SELECT name FROM users WHERE id = ?", userID).Scan(&name)
	return name
}

func (s *MarketplaceService) isUserUnder18(ctx context.Context, userID uint64) bool {
	var birthdate sql.NullTime
	err := s.db.QueryRowContext(ctx, "SELECT birthdate FROM kycs WHERE user_id = ?", userID).Scan(&birthdate)
	if err != nil || !birthdate.Valid {
		return false
	}
	// Calculate age accurately
	age := time.Since(birthdate.Time).Hours() / 24 / 365.25
	return age < 18
}

// GetUserCode gets user code from database (exported for handler use)
func (s *MarketplaceService) GetUserCode(ctx context.Context, userID uint64) (string, error) {
	var code string
	err := s.db.QueryRowContext(ctx, "SELECT code FROM users WHERE id = ?", userID).Scan(&code)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("user not found")
	}
	return code, err
}

// GetLatestProfilePhoto gets the latest profile photo URL for a user (exported for handler use)
func (s *MarketplaceService) GetLatestProfilePhoto(ctx context.Context, userID uint64) (string, error) {
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
