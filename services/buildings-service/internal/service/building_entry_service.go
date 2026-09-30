package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/shopspring/decimal"

	"metarang/buildings-service/internal/models"
	pb "metarang/shared/pb/features"
	levelspb "metarang/shared/pb/levels"
	"metarang/shared/pkg/auth"
)

const (
	entryScopeExact     = "exact"
	entryScopeAndUpper  = "and_upper"
	entryScopeAndLower  = "and_lower"
	entryAboutMaxRunes  = 1000
	entrySuccessMessage = "entry successful"
	exitSuccessMessage  = "exit successful"
	entryWindow         = 24 * time.Hour
	entryPayableType    = `App\Models\BuildingEntrySession`
	entryTxnWithdraw    = "withdraw"
	entryTxnDeposit     = "deposit"
)

var entryCouponCodePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type entryStore interface {
	UpsertConfig(ctx context.Context, config models.BuildingEntryConfig) error
	FindConfig(ctx context.Context, featureID uint64) (*models.BuildingEntryConfig, error)
	CreateCoupon(ctx context.Context, coupon models.BuildingEntryCoupon) (*models.BuildingEntryCoupon, error)
	ListCoupons(ctx context.Context, featureID uint64) ([]models.BuildingEntryCoupon, error)
	FindCouponByCode(ctx context.Context, featureID uint64, code string) (*models.BuildingEntryCoupon, error)
	FindCurrentAccess(ctx context.Context, featureID, userID uint64, validAfter time.Time) (*models.BuildingEntryAccess, error)
	WithUserLock(ctx context.Context, featureID, userID uint64, fn func(context.Context) error) error
	ReopenSession(ctx context.Context, featureID, userID, sessionID uint64, validAfter time.Time) error
	CreateSession(ctx context.Context, session models.BuildingEntrySession, validAfter time.Time) (uint64, error)
	DeleteSession(ctx context.Context, featureID, userID, sessionID uint64) error
	CloseActiveSession(ctx context.Context, featureID, userID uint64) error
}

type entryFeatureStore interface {
	FindByID(ctx context.Context, id uint64) (*models.Feature, *models.FeatureProperties, error)
}

type entryBuildingStore interface {
	FindByFeatureID(ctx context.Context, featureID uint64) ([]*pb.Building, error)
}

type entryLevelsClient interface {
	GetUserLevel(ctx context.Context, userID uint64) (*levelspb.UserLevelResponse, error)
	GetAllLevels(ctx context.Context) (*levelspb.LevelsResponse, error)
}

type entryWalletClient interface {
	DeductBalance(ctx context.Context, userID uint64, asset string, amount float64) error
	AddBalance(ctx context.Context, userID uint64, asset string, amount float64) error
	RecordTransaction(ctx context.Context, userID uint64, asset string, amount float64, action string, status int32, payableType string, payableID uint64) error
}

// BuildingEntryService configures entry fees and records paid visits.
type BuildingEntryService struct {
	repo      entryStore
	features  entryFeatureStore
	buildings entryBuildingStore
	levels    entryLevelsClient
	wallet    entryWalletClient
	now       func() time.Time
}

func NewBuildingEntryService(
	repo entryStore,
	features entryFeatureStore,
	buildings entryBuildingStore,
	levels entryLevelsClient,
	wallet entryWalletClient,
) *BuildingEntryService {
	return &BuildingEntryService{
		repo:      repo,
		features:  features,
		buildings: buildings,
		levels:    nilIfPointerNil(levels),
		wallet:    nilIfPointerNil(wallet),
		now:       time.Now,
	}
}

func nilIfPointerNil[T any](client T) T {
	var zero T
	if any(client) == nil {
		return zero
	}
	value := reflect.ValueOf(client)
	if value.Kind() == reflect.Ptr && value.IsNil() {
		return zero
	}
	return client
}

func (s *BuildingEntryService) SetConfig(ctx context.Context, req *pb.SetBuildingEntryConfigRequest) (*pb.BuildingEntryConfig, error) {
	if _, _, _, err := s.ownedFeature(ctx, req.FeatureId); err != nil {
		return nil, err
	}
	if err := s.requireBuilding(ctx, req.FeatureId); err != nil {
		return nil, err
	}

	feePSC, err := parseEntryFee(req.FeePsc, "fee_psc")
	if err != nil {
		return nil, err
	}
	feeIRR, err := parseEntryFee(req.FeeIrr, "fee_irr")
	if err != nil {
		return nil, err
	}
	about := strings.TrimSpace(req.About)
	if utf8.RuneCountInString(about) > entryAboutMaxRunes {
		return nil, fmt.Errorf("invalid about: must not exceed %d characters", entryAboutMaxRunes)
	}
	scopeType := strings.TrimSpace(req.LevelScopeType)
	levelSlug := strings.TrimSpace(req.LevelSlug)
	if err := s.validateLevelScope(ctx, scopeType, levelSlug); err != nil {
		return nil, err
	}

	config := models.BuildingEntryConfig{
		FeatureID:      req.FeatureId,
		FeePSC:         formatEntryFee(feePSC),
		FeeIRR:         formatEntryFee(feeIRR),
		About:          about,
		LevelScopeType: scopeType,
		LevelSlug:      levelSlug,
		IsActive:       req.IsActive,
	}
	if err := s.repo.UpsertConfig(ctx, config); err != nil {
		return nil, err
	}
	saved, err := s.repo.FindConfig(ctx, req.FeatureId)
	if err != nil {
		return nil, err
	}
	if saved == nil {
		return nil, fmt.Errorf("entry config not found")
	}
	return entryConfigProto(saved), nil
}

func (s *BuildingEntryService) GetConfig(ctx context.Context, featureID uint64) (*pb.BuildingEntryConfig, error) {
	if _, _, _, err := s.ownedFeature(ctx, featureID); err != nil {
		return nil, err
	}
	config, err := s.repo.FindConfig(ctx, featureID)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, fmt.Errorf("entry config not found")
	}
	return entryConfigProto(config), nil
}

func (s *BuildingEntryService) CreateCoupon(ctx context.Context, req *pb.CreateBuildingEntryCouponRequest) (*pb.BuildingEntryCoupon, error) {
	if _, _, _, err := s.ownedFeature(ctx, req.FeatureId); err != nil {
		return nil, err
	}
	if err := s.requireBuilding(ctx, req.FeatureId); err != nil {
		return nil, err
	}
	code := strings.TrimSpace(req.Code)
	if !entryCouponCodePattern.MatchString(code) {
		return nil, fmt.Errorf("invalid coupon code")
	}
	if req.DiscountPercentage < 1 || req.DiscountPercentage > 100 {
		return nil, fmt.Errorf("invalid discount percentage")
	}
	if req.MaxUsageCount < 1 {
		return nil, fmt.Errorf("invalid usage count")
	}

	created, err := s.repo.CreateCoupon(ctx, models.BuildingEntryCoupon{
		FeatureID:          req.FeatureId,
		Code:               code,
		DiscountPercentage: req.DiscountPercentage,
		MaxUsageCount:      req.MaxUsageCount,
	})
	if err != nil {
		return nil, err
	}
	return entryCouponProto(created), nil
}

func (s *BuildingEntryService) ListCoupons(ctx context.Context, featureID uint64) ([]*pb.BuildingEntryCoupon, error) {
	if _, _, _, err := s.ownedFeature(ctx, featureID); err != nil {
		return nil, err
	}
	coupons, err := s.repo.ListCoupons(ctx, featureID)
	if err != nil {
		return nil, err
	}
	out := make([]*pb.BuildingEntryCoupon, 0, len(coupons))
	for i := range coupons {
		out = append(out, entryCouponProto(&coupons[i]))
	}
	return out, nil
}

func (s *BuildingEntryService) Enter(ctx context.Context, req *pb.EnterBuildingRequest) (string, error) {
	feature, _, user, err := s.authenticatedFeature(ctx, req.FeatureId)
	if err != nil {
		return "", err
	}
	config, err := s.repo.FindConfig(ctx, req.FeatureId)
	if err != nil {
		return "", err
	}
	if config == nil {
		return "", fmt.Errorf("entry config not found")
	}
	if !config.IsActive {
		return "", fmt.Errorf("entry is not active")
	}
	if err := s.requireConstructionFinished(ctx, req.FeatureId); err != nil {
		return "", err
	}

	var message string
	err = s.repo.WithUserLock(ctx, req.FeatureId, user.UserID, func(ctx context.Context) error {
		var inner error
		message, inner = s.enterLocked(ctx, req, feature, user, config)
		return inner
	})
	return message, err
}

func (s *BuildingEntryService) enterLocked(
	ctx context.Context,
	req *pb.EnterBuildingRequest,
	feature *models.Feature,
	user *auth.UserContext,
	config *models.BuildingEntryConfig,
) (string, error) {
	validAfter := s.now().Add(-entryWindow)
	access, err := s.repo.FindCurrentAccess(ctx, req.FeatureId, user.UserID, validAfter)
	if err != nil {
		return "", err
	}
	if access != nil {
		if access.Inside {
			return "", fmt.Errorf("user already inside this building")
		}
		if user.UserID != feature.OwnerID {
			if err := s.ensureLevelAllowed(ctx, user.UserID, config.LevelScopeType, config.LevelSlug); err != nil {
				return "", err
			}
		}
		if err := s.repo.ReopenSession(ctx, req.FeatureId, user.UserID, access.ID, validAfter); err != nil {
			return "", err
		}
		return entrySuccessMessage, nil
	}

	session := models.BuildingEntrySession{
		FeatureID: req.FeatureId,
		UserID:    user.UserID,
		FeePSC:    formatEntryFee(decimal.Zero),
		FeeIRR:    formatEntryFee(decimal.Zero),
	}
	if user.UserID != feature.OwnerID {
		feePSC, feeIRR, couponID, err := s.priceForGuest(ctx, config, req.FeatureId, user.UserID, req.CouponCode)
		if err != nil {
			return "", err
		}
		session.FeePSC = formatEntryFee(feePSC)
		session.FeeIRR = formatEntryFee(feeIRR)
		session.CouponID = couponID
		if err := s.collectFee(ctx, user.UserID, feature.OwnerID, feePSC, feeIRR); err != nil {
			return "", err
		}
		sessionID, err := s.repo.CreateSession(ctx, session, validAfter)
		if err != nil {
			if refundErr := s.reverseFee(ctx, user.UserID, feature.OwnerID, feePSC, feeIRR); refundErr != nil {
				return "", refundErr
			}
			if strings.Contains(err.Error(), "entry window still open") {
				return s.reopenPaidWindow(ctx, req.FeatureId, user.UserID, feature.OwnerID, config, validAfter)
			}
			return "", err
		}
		if err := s.recordEntryLedger(ctx, user.UserID, feature.OwnerID, feePSC, feeIRR, sessionID); err != nil {
			if refundErr := s.reverseFee(ctx, user.UserID, feature.OwnerID, feePSC, feeIRR); refundErr != nil {
				slog.Error("building entry ledger failed and fee reversal failed",
					"feature_id", req.FeatureId, "guest_id", user.UserID, "session_id", sessionID, "error", refundErr)
				return "", fmt.Errorf("payment needs reconciliation")
			}
			if delErr := s.repo.DeleteSession(ctx, req.FeatureId, user.UserID, sessionID); delErr != nil {
				slog.Error("building entry fee reversed but session remains",
					"feature_id", req.FeatureId, "guest_id", user.UserID, "session_id", sessionID, "error", delErr)
				return "", fmt.Errorf("payment needs reconciliation")
			}
			return "", err
		}
		return entrySuccessMessage, nil
	}

	if _, err := s.repo.CreateSession(ctx, session, validAfter); err != nil {
		if strings.Contains(err.Error(), "entry window still open") {
			return s.reopenPaidWindow(ctx, req.FeatureId, user.UserID, feature.OwnerID, config, validAfter)
		}
		return "", err
	}
	return entrySuccessMessage, nil
}

func (s *BuildingEntryService) reopenPaidWindow(
	ctx context.Context,
	featureID, userID, ownerID uint64,
	config *models.BuildingEntryConfig,
	validAfter time.Time,
) (string, error) {
	access, err := s.repo.FindCurrentAccess(ctx, featureID, userID, validAfter)
	if err != nil {
		return "", err
	}
	if access == nil {
		return "", fmt.Errorf("entry window expired")
	}
	if access.Inside {
		return "", fmt.Errorf("user already inside this building")
	}
	if userID != ownerID && config != nil {
		if err := s.ensureLevelAllowed(ctx, userID, config.LevelScopeType, config.LevelSlug); err != nil {
			return "", err
		}
	}
	if err := s.repo.ReopenSession(ctx, featureID, userID, access.ID, validAfter); err != nil {
		return "", err
	}
	return entrySuccessMessage, nil
}

func (s *BuildingEntryService) Exit(ctx context.Context, featureID uint64) (string, error) {
	user, err := auth.GetUserFromContext(ctx)
	if err != nil {
		return "", fmt.Errorf("unauthorized: authentication required")
	}
	if featureID == 0 {
		return "", fmt.Errorf("invalid feature_id")
	}
	err = s.repo.WithUserLock(ctx, featureID, user.UserID, func(ctx context.Context) error {
		return s.repo.CloseActiveSession(ctx, featureID, user.UserID)
	})
	if err != nil {
		return "", err
	}
	return exitSuccessMessage, nil
}

func (s *BuildingEntryService) priceForGuest(
	ctx context.Context,
	config *models.BuildingEntryConfig,
	featureID, userID uint64,
	couponCode string,
) (decimal.Decimal, decimal.Decimal, *uint64, error) {
	if err := s.ensureLevelAllowed(ctx, userID, config.LevelScopeType, config.LevelSlug); err != nil {
		return decimal.Zero, decimal.Zero, nil, err
	}
	feePSC, err := parseEntryFee(config.FeePSC, "fee_psc")
	if err != nil {
		return decimal.Zero, decimal.Zero, nil, err
	}
	feeIRR, err := parseEntryFee(config.FeeIRR, "fee_irr")
	if err != nil {
		return decimal.Zero, decimal.Zero, nil, err
	}

	code := strings.TrimSpace(couponCode)
	if code == "" {
		return feePSC, feeIRR, nil, nil
	}
	coupon, err := s.repo.FindCouponByCode(ctx, featureID, code)
	if err != nil {
		return decimal.Zero, decimal.Zero, nil, err
	}
	if coupon == nil {
		return decimal.Zero, decimal.Zero, nil, fmt.Errorf("coupon not found")
	}
	if coupon.RealUsageCount >= coupon.MaxUsageCount {
		return decimal.Zero, decimal.Zero, nil, fmt.Errorf("coupon usage limit reached")
	}
	discountedPSC := applyEntryDiscount(feePSC, coupon.DiscountPercentage)
	discountedIRR := applyEntryDiscount(feeIRR, coupon.DiscountPercentage)
	if discountedPSC.Equal(feePSC) && discountedIRR.Equal(feeIRR) {
		return decimal.Zero, decimal.Zero, nil, fmt.Errorf("coupon discount does not reduce the fee")
	}
	return discountedPSC, discountedIRR, &coupon.ID, nil
}

func (s *BuildingEntryService) ensureLevelAllowed(ctx context.Context, userID uint64, scopeType, levelSlug string) error {
	scopeType = strings.TrimSpace(scopeType)
	levelSlug = strings.TrimSpace(levelSlug)
	if scopeType == "" {
		return nil
	}
	if s.levels == nil {
		return fmt.Errorf("levels service unavailable")
	}
	scopeLevel, err := s.findLevelBySlug(ctx, levelSlug)
	if err != nil {
		return err
	}
	userLevel, err := s.levels.GetUserLevel(ctx, userID)
	if err != nil {
		return err
	}
	if userLevel == nil {
		return fmt.Errorf("user level is outside the allowed scope")
	}
	if !levelInScope(userLevel.UserScore, scopeLevel.Score, scopeType) {
		return fmt.Errorf("user level is outside the allowed scope")
	}
	return nil
}

func (s *BuildingEntryService) validateLevelScope(ctx context.Context, scopeType, levelSlug string) error {
	if scopeType == "" && levelSlug == "" {
		return nil
	}
	if scopeType == "" || levelSlug == "" {
		return fmt.Errorf("invalid level scope")
	}
	switch scopeType {
	case entryScopeExact, entryScopeAndUpper, entryScopeAndLower:
	default:
		return fmt.Errorf("invalid level scope")
	}
	if s.levels == nil {
		return fmt.Errorf("levels service unavailable")
	}
	_, err := s.findLevelBySlug(ctx, levelSlug)
	return err
}

func (s *BuildingEntryService) findLevelBySlug(ctx context.Context, slug string) (*levelspb.Level, error) {
	resp, err := s.levels.GetAllLevels(ctx)
	if err != nil {
		return nil, err
	}
	if resp != nil {
		for _, level := range resp.Levels {
			if level != nil && level.Slug == slug {
				return level, nil
			}
		}
	}
	return nil, fmt.Errorf("invalid level_slug")
}

func (s *BuildingEntryService) collectFee(ctx context.Context, guestID, ownerID uint64, feePSC, feeIRR decimal.Decimal) error {
	if !positiveEntryFee(feePSC) && !positiveEntryFee(feeIRR) {
		return nil
	}
	if s.wallet == nil {
		return fmt.Errorf("commercial service unavailable")
	}

	chargedPSC := false
	chargedIRR := false
	creditedPSC := false
	creditedIRR := false

	move := func(op func() error, mark func()) error {
		if err := op(); err != nil {
			if paymentAmbiguous(err) {
				slog.Error("building entry payment outcome unknown",
					"guest_id", guestID, "owner_id", ownerID, "error", err)
				return fmt.Errorf("payment outcome unknown")
			}
			if refundErr := s.refundGuest(ctx, guestID, feePSC, feeIRR, chargedPSC, chargedIRR); refundErr != nil {
				slog.Error("building entry guest refund failed", "guest_id", guestID, "error", refundErr)
				return fmt.Errorf("payment needs reconciliation")
			}
			if clawErr := s.clawbackOwner(ctx, ownerID, feePSC, feeIRR, creditedPSC, creditedIRR); clawErr != nil {
				slog.Error("building entry owner clawback failed",
					"guest_id", guestID, "owner_id", ownerID, "error", clawErr)
				return fmt.Errorf("payment needs reconciliation")
			}
			return err
		}
		mark()
		return nil
	}

	if positiveEntryFee(feePSC) {
		amount, err := feeFloat(feePSC)
		if err != nil {
			return fmt.Errorf("invalid fee_psc")
		}
		if err := move(func() error { return s.wallet.DeductBalance(ctx, guestID, "psc", amount) }, func() { chargedPSC = true }); err != nil {
			return err
		}
	}
	if positiveEntryFee(feeIRR) {
		amount, err := feeFloat(feeIRR)
		if err != nil {
			return fmt.Errorf("invalid fee_irr")
		}
		if err := move(func() error { return s.wallet.DeductBalance(ctx, guestID, "irr", amount) }, func() { chargedIRR = true }); err != nil {
			return err
		}
	}
	if positiveEntryFee(feePSC) {
		amount, err := feeFloat(feePSC)
		if err != nil {
			return fmt.Errorf("invalid fee_psc")
		}
		if err := move(func() error { return s.wallet.AddBalance(ctx, ownerID, "psc", amount) }, func() { creditedPSC = true }); err != nil {
			return err
		}
	}
	if positiveEntryFee(feeIRR) {
		amount, err := feeFloat(feeIRR)
		if err != nil {
			return fmt.Errorf("invalid fee_irr")
		}
		if err := move(func() error { return s.wallet.AddBalance(ctx, ownerID, "irr", amount) }, func() { creditedIRR = true }); err != nil {
			return err
		}
	}
	return nil
}

func (s *BuildingEntryService) reverseFee(ctx context.Context, guestID, ownerID uint64, feePSC, feeIRR decimal.Decimal) error {
	if !positiveEntryFee(feePSC) && !positiveEntryFee(feeIRR) {
		return nil
	}
	if s.wallet == nil {
		return fmt.Errorf("commercial service unavailable")
	}
	if err := s.refundGuest(ctx, guestID, feePSC, feeIRR, positiveEntryFee(feePSC), positiveEntryFee(feeIRR)); err != nil {
		if paymentAmbiguous(err) {
			slog.Error("building entry refund outcome unknown", "guest_id", guestID, "owner_id", ownerID, "error", err)
			return fmt.Errorf("payment outcome unknown")
		}
		slog.Error("building entry guest refund failed", "guest_id", guestID, "owner_id", ownerID, "error", err)
		return fmt.Errorf("payment needs reconciliation")
	}
	if err := s.clawbackOwner(ctx, ownerID, feePSC, feeIRR, positiveEntryFee(feePSC), positiveEntryFee(feeIRR)); err != nil {
		slog.Error("building entry owner clawback failed", "guest_id", guestID, "owner_id", ownerID, "error", err)
		return fmt.Errorf("payment needs reconciliation")
	}
	return nil
}

func (s *BuildingEntryService) refundGuest(ctx context.Context, guestID uint64, feePSC, feeIRR decimal.Decimal, refundPSC, refundIRR bool) error {
	payCtx, cancel := detachContext(ctx)
	defer cancel()
	var refundErr error
	if refundIRR {
		amount, err := feeFloat(feeIRR)
		if err != nil {
			refundErr = errors.Join(refundErr, err)
		} else if err := s.wallet.AddBalance(payCtx, guestID, "irr", amount); err != nil {
			refundErr = errors.Join(refundErr, err)
		}
	}
	if refundPSC {
		amount, err := feeFloat(feePSC)
		if err != nil {
			refundErr = errors.Join(refundErr, err)
		} else if err := s.wallet.AddBalance(payCtx, guestID, "psc", amount); err != nil {
			refundErr = errors.Join(refundErr, err)
		}
	}
	return refundErr
}

func (s *BuildingEntryService) clawbackOwner(ctx context.Context, ownerID uint64, feePSC, feeIRR decimal.Decimal, clawPSC, clawIRR bool) error {
	payCtx, cancel := detachContext(ctx)
	defer cancel()
	var clawErr error
	if clawIRR {
		amount, err := feeFloat(feeIRR)
		if err != nil {
			clawErr = errors.Join(clawErr, err)
		} else if err := s.wallet.DeductBalance(payCtx, ownerID, "irr", amount); err != nil {
			clawErr = errors.Join(clawErr, err)
		}
	}
	if clawPSC {
		amount, err := feeFloat(feePSC)
		if err != nil {
			clawErr = errors.Join(clawErr, err)
		} else if err := s.wallet.DeductBalance(payCtx, ownerID, "psc", amount); err != nil {
			clawErr = errors.Join(clawErr, err)
		}
	}
	return clawErr
}

type entryLedgerLeg struct {
	userID uint64
	asset  string
	amount decimal.Decimal
	action string
	status int32
}

func (s *BuildingEntryService) recordEntryLedger(ctx context.Context, guestID, ownerID uint64, feePSC, feeIRR decimal.Decimal, sessionID uint64) error {
	if s.wallet == nil {
		return fmt.Errorf("commercial service unavailable")
	}
	legs := []entryLedgerLeg{
		{userID: guestID, asset: "psc", amount: feePSC, action: entryTxnWithdraw, status: 0},
		{userID: guestID, asset: "irr", amount: feeIRR, action: entryTxnWithdraw, status: 0},
		{userID: ownerID, asset: "psc", amount: feePSC, action: entryTxnDeposit, status: 1},
		{userID: ownerID, asset: "irr", amount: feeIRR, action: entryTxnDeposit, status: 1},
	}
	var written []entryLedgerLeg
	for _, leg := range legs {
		if !positiveEntryFee(leg.amount) {
			continue
		}
		if err := s.recordLedgerLeg(ctx, leg, sessionID); err != nil {
			s.reverseLedgerLegs(ctx, written, sessionID)
			return fmt.Errorf("failed to record entry transaction: %w", err)
		}
		written = append(written, leg)
	}
	return nil
}

func (s *BuildingEntryService) recordLedgerLeg(ctx context.Context, leg entryLedgerLeg, sessionID uint64) error {
	value, err := feeFloat(leg.amount)
	if err != nil {
		return err
	}
	return s.wallet.RecordTransaction(ctx, leg.userID, leg.asset, value, leg.action, leg.status, entryPayableType, sessionID)
}

func (s *BuildingEntryService) reverseLedgerLegs(ctx context.Context, legs []entryLedgerLeg, sessionID uint64) {
	for i := len(legs) - 1; i >= 0; i-- {
		leg := legs[i]
		if leg.action == entryTxnWithdraw {
			leg.action = entryTxnDeposit
			leg.status = 1
		} else {
			leg.action = entryTxnWithdraw
			leg.status = 0
		}
		if err := s.recordLedgerLeg(ctx, leg, sessionID); err != nil {
			slog.Error("building entry ledger reversal failed", "session_id", sessionID, "asset", leg.asset, "error", err)
		}
	}
}

func (s *BuildingEntryService) ownedFeature(ctx context.Context, featureID uint64) (*models.Feature, *models.FeatureProperties, *auth.UserContext, error) {
	feature, properties, user, err := s.authenticatedFeature(ctx, featureID)
	if err != nil {
		return nil, nil, nil, err
	}
	if feature.OwnerID != user.UserID {
		return nil, nil, nil, fmt.Errorf("unauthorized: user does not own this feature")
	}
	return feature, properties, user, nil
}

func (s *BuildingEntryService) authenticatedFeature(ctx context.Context, featureID uint64) (*models.Feature, *models.FeatureProperties, *auth.UserContext, error) {
	if featureID == 0 {
		return nil, nil, nil, fmt.Errorf("invalid feature_id")
	}
	user, err := auth.GetUserFromContext(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("unauthorized: authentication required")
	}
	feature, properties, err := s.features.FindByID(ctx, featureID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, nil, fmt.Errorf("feature not found")
		}
		slog.Error("failed to load feature", "feature_id", featureID, "error", err)
		return nil, nil, nil, fmt.Errorf("failed to load feature")
	}
	if feature == nil {
		return nil, nil, nil, fmt.Errorf("feature not found")
	}
	return feature, properties, user, nil
}

func (s *BuildingEntryService) requireBuilding(ctx context.Context, featureID uint64) error {
	buildings, err := s.buildings.FindByFeatureID(ctx, featureID)
	if err != nil {
		return fmt.Errorf("failed to find building: %w", err)
	}
	if len(buildings) == 0 {
		return fmt.Errorf("building not found")
	}
	return nil
}

func (s *BuildingEntryService) requireConstructionFinished(ctx context.Context, featureID uint64) error {
	buildings, err := s.buildings.FindByFeatureID(ctx, featureID)
	if err != nil {
		return fmt.Errorf("failed to find building: %w", err)
	}
	if len(buildings) == 0 {
		return fmt.Errorf("building not found")
	}
	now := s.now()
	for _, building := range buildings {
		if building == nil {
			continue
		}
		end, ok := parseLocalDateTime(building.ConstructionEndDate)
		if !ok {
			return fmt.Errorf("invalid construction end date")
		}
		if !end.Before(now) {
			return fmt.Errorf("building construction is not finished")
		}
	}
	return nil
}

func parseLocalDateTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, format := range mysqlDateTimeFormats {
		if parsed, err := time.ParseInLocation(format, value, time.Local); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func parseEntryFee(raw, field string) (decimal.Decimal, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return decimal.Zero, nil
	}
	value, err := decimal.NewFromString(raw)
	if err != nil || value.IsNegative() {
		return decimal.Zero, fmt.Errorf("invalid %s", field)
	}
	if !value.Equal(value.Truncate(2)) {
		return decimal.Zero, fmt.Errorf("invalid %s", field)
	}
	if value.GreaterThanOrEqual(decimal.RequireFromString("10000000000000000")) {
		return decimal.Zero, fmt.Errorf("invalid %s", field)
	}
	return value.Round(2), nil
}

func formatEntryFee(value decimal.Decimal) string {
	return value.StringFixed(2)
}

func applyEntryDiscount(fee decimal.Decimal, percent int32) decimal.Decimal {
	factor := decimal.NewFromInt(int64(100 - percent))
	discounted := fee.Mul(factor).Div(decimal.NewFromInt(100))
	if discounted.IsNegative() {
		return decimal.Zero
	}
	return discounted.Round(2)
}

func positiveEntryFee(value decimal.Decimal) bool {
	return value.IsPositive()
}

func feeFloat(value decimal.Decimal) (float64, error) {
	rounded := value.Round(2)
	amount, _ := rounded.Float64()
	if !decimal.NewFromFloat(amount).Round(2).Equal(rounded) {
		return 0, fmt.Errorf("invalid fee amount")
	}
	return amount, nil
}

func paymentAmbiguous(err error) bool {
	var ambiguous interface{ PaymentAmbiguous() bool }
	return err != nil && errors.As(err, &ambiguous) && ambiguous.PaymentAmbiguous()
}

func detachContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
}

func levelInScope(guestScore, scopeScore int32, scopeType string) bool {
	switch scopeType {
	case entryScopeExact:
		return guestScore == scopeScore
	case entryScopeAndUpper:
		return guestScore >= scopeScore
	case entryScopeAndLower:
		return guestScore <= scopeScore
	default:
		return false
	}
}

func entryConfigProto(config *models.BuildingEntryConfig) *pb.BuildingEntryConfig {
	if config == nil {
		return nil
	}
	feePSC, _ := parseEntryFee(config.FeePSC, "fee_psc")
	feeIRR, _ := parseEntryFee(config.FeeIRR, "fee_irr")
	return &pb.BuildingEntryConfig{
		FeatureId:      config.FeatureID,
		FeePsc:         formatEntryFee(feePSC),
		FeeIrr:         formatEntryFee(feeIRR),
		About:          config.About,
		LevelScopeType: config.LevelScopeType,
		LevelSlug:      config.LevelSlug,
		IsActive:       config.IsActive,
	}
}

func entryCouponProto(coupon *models.BuildingEntryCoupon) *pb.BuildingEntryCoupon {
	if coupon == nil {
		return nil
	}
	return &pb.BuildingEntryCoupon{
		Id:                 coupon.ID,
		FeatureId:          coupon.FeatureID,
		Code:               coupon.Code,
		DiscountPercentage: coupon.DiscountPercentage,
		MaxUsageCount:      coupon.MaxUsageCount,
		RealUsageCount:     coupon.RealUsageCount,
	}
}
