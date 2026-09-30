package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"metarang/buildings-service/internal/models"
	"metarang/buildings-service/internal/service"
	pb "metarang/shared/pb/features"
	levelspb "metarang/shared/pb/levels"
	"metarang/shared/pkg/auth"
)

type entryRepoMock struct {
	config    *models.BuildingEntryConfig
	coupons   []models.BuildingEntryCoupon
	sessions  []models.BuildingEntrySession
	access    *models.BuildingEntryAccess
	active    bool
	reopened  bool
	cutoff    time.Time
	saveErr   error
	inLock    bool
	createID  uint64
	createErr error
	deleted   bool
}

func (m *entryRepoMock) UpsertConfig(_ context.Context, config models.BuildingEntryConfig) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	copied := config
	m.config = &copied
	return nil
}

func (m *entryRepoMock) FindConfig(context.Context, uint64) (*models.BuildingEntryConfig, error) {
	if m.config == nil {
		return nil, nil
	}
	copied := *m.config
	return &copied, nil
}

func (m *entryRepoMock) CreateCoupon(_ context.Context, coupon models.BuildingEntryCoupon) (*models.BuildingEntryCoupon, error) {
	coupon.ID = uint64(len(m.coupons) + 1)
	m.coupons = append(m.coupons, coupon)
	created := coupon
	return &created, nil
}

func (m *entryRepoMock) ListCoupons(context.Context, uint64) ([]models.BuildingEntryCoupon, error) {
	return m.coupons, nil
}

func (m *entryRepoMock) FindCouponByCode(_ context.Context, _ uint64, code string) (*models.BuildingEntryCoupon, error) {
	for i := range m.coupons {
		if m.coupons[i].Code == code {
			copied := m.coupons[i]
			return &copied, nil
		}
	}
	return nil, nil
}

func (m *entryRepoMock) FindCurrentAccess(_ context.Context, _, _ uint64, validAfter time.Time) (*models.BuildingEntryAccess, error) {
	m.cutoff = validAfter
	if m.access == nil {
		return nil, nil
	}
	copied := *m.access
	return &copied, nil
}

func (m *entryRepoMock) WithUserLock(ctx context.Context, _, _ uint64, fn func(context.Context) error) error {
	m.inLock = true
	defer func() { m.inLock = false }()
	return fn(ctx)
}

func (m *entryRepoMock) ReopenSession(context.Context, uint64, uint64, uint64, time.Time) error {
	if m.access == nil {
		return errors.New("entry window expired")
	}
	if m.access.Inside {
		return errors.New("user already inside this building")
	}
	m.access.Inside = true
	m.reopened = true
	m.active = true
	return nil
}

func (m *entryRepoMock) CreateSession(_ context.Context, session models.BuildingEntrySession, _ time.Time) (uint64, error) {
	if !m.inLock {
		return 0, errors.New("session write without entry lock")
	}
	if m.createErr != nil {
		return 0, m.createErr
	}
	if m.access != nil && m.access.Inside {
		return 0, errors.New("user already inside this building")
	}
	if m.access != nil {
		return 0, errors.New("entry window still open")
	}
	m.sessions = append(m.sessions, session)
	m.active = true
	id := uint64(len(m.sessions))
	m.access = &models.BuildingEntryAccess{ID: id, Inside: true}
	m.createID = id
	return id, nil
}

func (m *entryRepoMock) DeleteSession(context.Context, uint64, uint64, uint64) error {
	m.deleted = true
	m.sessions = nil
	m.access = nil
	m.active = false
	return nil
}

func (m *entryRepoMock) CloseActiveSession(context.Context, uint64, uint64) error {
	if !m.active {
		return errors.New("active session not found")
	}
	m.active = false
	return nil
}

type entryFeatureMock struct {
	feature *models.Feature
}

func (m *entryFeatureMock) FindByID(context.Context, uint64) (*models.Feature, *models.FeatureProperties, error) {
	if m.feature == nil {
		return nil, nil, errors.New("sql: no rows in result set")
	}
	return m.feature, &models.FeatureProperties{}, nil
}

type entryBuildingMock struct {
	buildings []*pb.Building
}

func (m *entryBuildingMock) FindByFeatureID(context.Context, uint64) ([]*pb.Building, error) {
	return m.buildings, nil
}

type entryLevelsMock struct {
	userScore   int32
	latestScore int32
	levels      []*levelspb.Level
}

func (m *entryLevelsMock) GetUserLevel(context.Context, uint64) (*levelspb.UserLevelResponse, error) {
	latest := m.latestScore
	if latest == 0 {
		latest = m.userScore
	}
	return &levelspb.UserLevelResponse{
		LatestLevel: &levelspb.Level{Slug: "guest", Score: latest},
		UserScore:   m.userScore,
	}, nil
}

func (m *entryLevelsMock) GetAllLevels(context.Context) (*levelspb.LevelsResponse, error) {
	return &levelspb.LevelsResponse{Levels: m.levels}, nil
}

type walletOp struct {
	action string
	userID uint64
	asset  string
	amount float64
}

type entryWalletMock struct {
	ops    []walletOp
	txns   []walletOp
	err    error
	addErr error
	script []error
	txnErr error
}

func (m *entryWalletMock) take(action string, userID uint64, asset string, amount float64) error {
	m.ops = append(m.ops, walletOp{action: action, userID: userID, asset: asset, amount: amount})
	if len(m.script) > 0 {
		err := m.script[0]
		m.script = m.script[1:]
		return err
	}
	if action == "deduct" {
		return m.err
	}
	return m.addErr
}

func (m *entryWalletMock) DeductBalance(_ context.Context, userID uint64, asset string, amount float64) error {
	return m.take("deduct", userID, asset, amount)
}

func (m *entryWalletMock) AddBalance(_ context.Context, userID uint64, asset string, amount float64) error {
	return m.take("add", userID, asset, amount)
}

func (m *entryWalletMock) RecordTransaction(_ context.Context, userID uint64, asset string, amount float64, action string, _ int32, _ string, _ uint64) error {
	m.txns = append(m.txns, walletOp{action: action, userID: userID, asset: asset, amount: amount})
	return m.txnErr
}

func userCtx(userID uint64) context.Context {
	return context.WithValue(context.Background(), auth.UserContextKey{}, &auth.UserContext{UserID: userID})
}

func finishedBuilding() []*pb.Building {
	return []*pb.Building{{ConstructionEndDate: "2000-01-01 00:00:00"}}
}

func TestBuildingEntry_OwnerEntersFree(t *testing.T) {
	repo := &entryRepoMock{config: &models.BuildingEntryConfig{
		FeatureID: 7, FeePSC: "10.00", FeeIRR: "1000.00", IsActive: true,
	}}
	wallet := &entryWalletMock{}
	svc := service.NewBuildingEntryService(
		repo,
		&entryFeatureMock{feature: &models.Feature{ID: 7, OwnerID: 1}},
		&entryBuildingMock{buildings: finishedBuilding()},
		nil,
		wallet,
	)

	message, err := svc.Enter(userCtx(1), &pb.EnterBuildingRequest{FeatureId: 7})
	if err != nil {
		t.Fatal(err)
	}
	if message != "entry successful" {
		t.Fatalf("message=%q", message)
	}
	if len(wallet.ops) != 0 {
		t.Fatalf("owner was charged: %+v", wallet.ops)
	}
	if len(repo.sessions) != 1 || repo.sessions[0].FeePSC != "0.00" || repo.sessions[0].CouponID != nil {
		t.Fatalf("session=%+v", repo.sessions)
	}
}

func TestBuildingEntry_GuestPaysDiscountedFees(t *testing.T) {
	couponID := uint64(4)
	repo := &entryRepoMock{
		config: &models.BuildingEntryConfig{
			FeatureID: 7, FeePSC: "10.00", FeeIRR: "1000.00", IsActive: true,
			LevelScopeType: "and_upper", LevelSlug: "level-3",
		},
		coupons: []models.BuildingEntryCoupon{{
			ID: couponID, FeatureID: 7, Code: "SAVE20", DiscountPercentage: 20, MaxUsageCount: 2,
		}},
	}
	wallet := &entryWalletMock{}
	svc := service.NewBuildingEntryService(
		repo,
		&entryFeatureMock{feature: &models.Feature{ID: 7, OwnerID: 1}},
		&entryBuildingMock{buildings: finishedBuilding()},
		&entryLevelsMock{userScore: 30, levels: []*levelspb.Level{{Slug: "level-3", Score: 30}}},
		wallet,
	)

	message, err := svc.Enter(userCtx(9), &pb.EnterBuildingRequest{FeatureId: 7, CouponCode: "SAVE20"})
	if err != nil {
		t.Fatal(err)
	}
	if message != "entry successful" {
		t.Fatalf("message=%q", message)
	}
	want := []walletOp{
		{action: "deduct", userID: 9, asset: "psc", amount: 8},
		{action: "deduct", userID: 9, asset: "irr", amount: 800},
		{action: "add", userID: 1, asset: "psc", amount: 8},
		{action: "add", userID: 1, asset: "irr", amount: 800},
	}
	if len(wallet.ops) != len(want) {
		t.Fatalf("ops=%+v", wallet.ops)
	}
	for i := range want {
		if wallet.ops[i] != want[i] {
			t.Fatalf("op %d = %+v, want %+v", i, wallet.ops[i], want[i])
		}
	}
	if repo.sessions[0].CouponID == nil || *repo.sessions[0].CouponID != couponID {
		t.Fatalf("coupon not recorded: %+v", repo.sessions[0])
	}
	if len(wallet.txns) != 4 {
		t.Fatalf("ledger=%+v", wallet.txns)
	}
}

func TestBuildingEntry_RejectsLevelAndUnfinishedConstruction(t *testing.T) {
	repo := &entryRepoMock{config: &models.BuildingEntryConfig{
		FeatureID: 7, FeePSC: "10.00", FeeIRR: "5.00", IsActive: true,
		LevelScopeType: "exact", LevelSlug: "level-3",
	}}
	wallet := &entryWalletMock{}
	levels := &entryLevelsMock{userScore: 10, levels: []*levelspb.Level{{Slug: "level-3", Score: 30}}}
	svc := service.NewBuildingEntryService(
		repo,
		&entryFeatureMock{feature: &models.Feature{ID: 7, OwnerID: 1}},
		&entryBuildingMock{buildings: finishedBuilding()},
		levels,
		wallet,
	)
	if _, err := svc.Enter(userCtx(9), &pb.EnterBuildingRequest{FeatureId: 7}); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("level error=%v", err)
	}
	if len(wallet.ops) != 0 {
		t.Fatalf("charged despite level rejection: %+v", wallet.ops)
	}

	svc = service.NewBuildingEntryService(
		repo,
		&entryFeatureMock{feature: &models.Feature{ID: 7, OwnerID: 1}},
		&entryBuildingMock{buildings: []*pb.Building{{ConstructionEndDate: "2999-01-01 00:00:00"}}},
		levels,
		wallet,
	)
	repo.config.LevelScopeType = ""
	repo.config.LevelSlug = ""
	if _, err := svc.Enter(userCtx(9), &pb.EnterBuildingRequest{FeatureId: 7}); err == nil || !strings.Contains(err.Error(), "not finished") {
		t.Fatalf("construction error=%v", err)
	}
}

func TestBuildingEntry_ExitAndCouponListUsage(t *testing.T) {
	repo := &entryRepoMock{active: true, coupons: []models.BuildingEntryCoupon{{
		ID: 1, FeatureID: 7, Code: "SAVE20", DiscountPercentage: 10, MaxUsageCount: 5, RealUsageCount: 3,
	}}}
	svc := service.NewBuildingEntryService(
		repo,
		&entryFeatureMock{feature: &models.Feature{ID: 7, OwnerID: 1}},
		&entryBuildingMock{buildings: finishedBuilding()},
		nil,
		nil,
	)
	message, err := svc.Exit(userCtx(9), 7)
	if err != nil {
		t.Fatal(err)
	}
	if message != "exit successful" || repo.active {
		t.Fatalf("message=%q active=%v", message, repo.active)
	}

	coupons, err := svc.ListCoupons(userCtx(1), 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(coupons) != 1 || coupons[0].RealUsageCount != 3 || coupons[0].MaxUsageCount != 5 {
		t.Fatalf("coupons=%+v", coupons[0])
	}
}

func TestBuildingEntry_ReentryWithinWindowIsFree(t *testing.T) {
	repo := &entryRepoMock{
		config: &models.BuildingEntryConfig{
			FeatureID: 7, FeePSC: "10.00", FeeIRR: "1000.00", IsActive: true,
		},
		access: &models.BuildingEntryAccess{ID: 4, Inside: false},
		active: false,
	}
	wallet := &entryWalletMock{}
	svc := service.NewBuildingEntryService(
		repo,
		&entryFeatureMock{feature: &models.Feature{ID: 7, OwnerID: 1}},
		&entryBuildingMock{buildings: finishedBuilding()},
		nil,
		wallet,
	)

	message, err := svc.Enter(userCtx(9), &pb.EnterBuildingRequest{FeatureId: 7, CouponCode: "SAVE20"})
	if err != nil {
		t.Fatal(err)
	}
	if message != "entry successful" || !repo.reopened || !repo.access.Inside {
		t.Fatalf("message=%q reopened=%v inside=%v", message, repo.reopened, repo.access.Inside)
	}
	if len(wallet.ops) != 0 || len(repo.sessions) != 0 {
		t.Fatalf("re-entry charged or opened a new window: ops=%+v sessions=%d", wallet.ops, len(repo.sessions))
	}
	if age := time.Since(repo.cutoff); age < 23*time.Hour || age > 25*time.Hour {
		t.Fatalf("window cutoff=%s age=%s", repo.cutoff, age)
	}
}

func TestBuildingEntry_ExpiredWindowChargesAgain(t *testing.T) {
	repo := &entryRepoMock{config: &models.BuildingEntryConfig{
		FeatureID: 7, FeePSC: "10.00", FeeIRR: "4.00", IsActive: true,
	}}
	wallet := &entryWalletMock{}
	svc := service.NewBuildingEntryService(
		repo,
		&entryFeatureMock{feature: &models.Feature{ID: 7, OwnerID: 1}},
		&entryBuildingMock{buildings: finishedBuilding()},
		nil,
		wallet,
	)
	if _, err := svc.Enter(userCtx(9), &pb.EnterBuildingRequest{FeatureId: 7}); err != nil {
		t.Fatal(err)
	}
	if len(wallet.ops) != 4 || len(repo.sessions) != 1 {
		t.Fatalf("ops=%d sessions=%d", len(wallet.ops), len(repo.sessions))
	}
}

type ambiguousWalletErr struct{}

func (ambiguousWalletErr) Error() string          { return "service_unavailable: timeout" }
func (ambiguousWalletErr) PaymentAmbiguous() bool { return true }

func TestBuildingEntry_RefundsGuestWhenOwnerClawbackFails(t *testing.T) {
	repo := &entryRepoMock{config: &models.BuildingEntryConfig{
		FeatureID: 7, FeePSC: "10.00", FeeIRR: "4.00", IsActive: true,
	}}
	wallet := &entryWalletMock{script: []error{
		nil, nil, nil, errors.New("owner irr credit failed"),
		nil, nil,
		errors.New("owner psc clawback failed"),
	}}
	svc := service.NewBuildingEntryService(
		repo,
		&entryFeatureMock{feature: &models.Feature{ID: 7, OwnerID: 1}},
		&entryBuildingMock{buildings: finishedBuilding()},
		nil,
		wallet,
	)
	_, err := svc.Enter(userCtx(9), &pb.EnterBuildingRequest{FeatureId: 7})
	if err == nil || !strings.Contains(err.Error(), "reconciliation") {
		t.Fatalf("error=%v", err)
	}
	if len(repo.sessions) != 0 {
		t.Fatalf("session stored after failed payment: %+v", repo.sessions)
	}
	var refundedPSC, refundedIRR bool
	for _, op := range wallet.ops {
		if op.action == "add" && op.userID == 9 && op.asset == "psc" {
			refundedPSC = true
		}
		if op.action == "add" && op.userID == 9 && op.asset == "irr" {
			refundedIRR = true
		}
	}
	if !refundedPSC || !refundedIRR {
		t.Fatalf("guest was not refunded: %+v", wallet.ops)
	}
}

func TestBuildingEntry_AmbiguousCreditDoesNotRefund(t *testing.T) {
	repo := &entryRepoMock{config: &models.BuildingEntryConfig{
		FeatureID: 7, FeePSC: "10.00", FeeIRR: "4.00", IsActive: true,
	}}
	wallet := &entryWalletMock{script: []error{nil, nil, ambiguousWalletErr{}}}
	svc := service.NewBuildingEntryService(
		repo,
		&entryFeatureMock{feature: &models.Feature{ID: 7, OwnerID: 1}},
		&entryBuildingMock{buildings: finishedBuilding()},
		nil,
		wallet,
	)
	_, err := svc.Enter(userCtx(9), &pb.EnterBuildingRequest{FeatureId: 7})
	if err == nil || !strings.Contains(err.Error(), "outcome unknown") {
		t.Fatalf("error=%v", err)
	}
	for _, op := range wallet.ops {
		if op.action == "add" && op.userID == 9 {
			t.Fatalf("ambiguous credit was refunded: %+v", wallet.ops)
		}
	}
}

func TestBuildingEntry_SessionInsertFailureRefundsGuest(t *testing.T) {
	repo := &entryRepoMock{
		config: &models.BuildingEntryConfig{
			FeatureID: 7, FeePSC: "10.00", FeeIRR: "4.00", IsActive: true,
		},
		createErr: errors.New("insert failed"),
	}
	wallet := &entryWalletMock{script: []error{
		nil, nil, nil, nil,
		nil, nil,
		errors.New("owner clawback failed"), nil,
	}}
	svc := service.NewBuildingEntryService(
		repo,
		&entryFeatureMock{feature: &models.Feature{ID: 7, OwnerID: 1}},
		&entryBuildingMock{buildings: finishedBuilding()},
		nil,
		wallet,
	)
	_, err := svc.Enter(userCtx(9), &pb.EnterBuildingRequest{FeatureId: 7})
	if err == nil || !strings.Contains(err.Error(), "reconciliation") {
		t.Fatalf("error=%v", err)
	}
	refunded := false
	for _, op := range wallet.ops {
		if op.action == "add" && op.userID == 9 {
			refunded = true
		}
	}
	if !refunded {
		t.Fatalf("guest was not refunded: %+v", wallet.ops)
	}
}

func TestBuildingEntry_ReentryChecksLevel(t *testing.T) {
	repo := &entryRepoMock{
		config: &models.BuildingEntryConfig{
			FeatureID: 7, FeePSC: "10.00", FeeIRR: "4.00", IsActive: true,
			LevelScopeType: "exact", LevelSlug: "level-3",
		},
		access: &models.BuildingEntryAccess{ID: 4, Inside: false},
	}
	wallet := &entryWalletMock{}
	svc := service.NewBuildingEntryService(
		repo,
		&entryFeatureMock{feature: &models.Feature{ID: 7, OwnerID: 1}},
		&entryBuildingMock{buildings: finishedBuilding()},
		&entryLevelsMock{userScore: 10, latestScore: 100, levels: []*levelspb.Level{{Slug: "level-3", Score: 30}}},
		wallet,
	)
	_, err := svc.Enter(userCtx(9), &pb.EnterBuildingRequest{FeatureId: 7})
	if err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("error=%v", err)
	}
	if repo.reopened || len(wallet.ops) != 0 {
		t.Fatalf("reopened=%v ops=%+v", repo.reopened, wallet.ops)
	}
}

func TestBuildingEntry_CouponThatDoesNotChangeFeeIsRejected(t *testing.T) {
	repo := &entryRepoMock{
		config: &models.BuildingEntryConfig{
			FeatureID: 7, FeePSC: "0.01", FeeIRR: "0.01", IsActive: true,
		},
		coupons: []models.BuildingEntryCoupon{{
			ID: 3, FeatureID: 7, Code: "TINY", DiscountPercentage: 50, MaxUsageCount: 5,
		}},
	}
	wallet := &entryWalletMock{}
	svc := service.NewBuildingEntryService(
		repo,
		&entryFeatureMock{feature: &models.Feature{ID: 7, OwnerID: 1}},
		&entryBuildingMock{buildings: finishedBuilding()},
		nil,
		wallet,
	)
	_, err := svc.Enter(userCtx(9), &pb.EnterBuildingRequest{FeatureId: 7, CouponCode: "TINY"})
	if err == nil || !strings.Contains(err.Error(), "does not reduce") {
		t.Fatalf("error=%v", err)
	}
	if len(wallet.ops) != 0 || len(repo.sessions) != 0 {
		t.Fatalf("ops=%+v sessions=%d", wallet.ops, len(repo.sessions))
	}
}
