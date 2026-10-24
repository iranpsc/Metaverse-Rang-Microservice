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
)

type raceEntryRepo struct {
	*entryRepoMock
	reads int
}

func (r *raceEntryRepo) FindCurrentAccess(_ context.Context, _, _ uint64, _ time.Time) (*models.BuildingEntryAccess, error) {
	r.reads++
	if r.reads == 1 {
		return nil, nil
	}
	r.access = &models.BuildingEntryAccess{ID: 9, Inside: false}
	return r.access, nil
}
func (r *raceEntryRepo) CreateSession(context.Context, models.BuildingEntrySession, time.Time) (uint64, error) {
	return 0, errors.New("entry window still open")
}

func TestBuildingEntryReopensWindowAfterInsertRace(t *testing.T) {
	repo := &raceEntryRepo{entryRepoMock: &entryRepoMock{config: &models.BuildingEntryConfig{
		FeatureID: 7, FeePSC: "0.00", FeeIRR: "0.00", IsActive: true,
	}}}
	svc := service.NewBuildingEntryService(repo,
		&entryFeatureMock{feature: &models.Feature{ID: 7, OwnerID: 1}},
		&entryBuildingMock{buildings: finishedBuilding()}, nil, nil)
	// The owner has no fee to reverse; the competing insert is resolved by reopening.
	message, err := svc.Enter(userCtx(1), &pb.EnterBuildingRequest{FeatureId: 7})
	if err != nil || message != "entry successful" || !repo.reopened || repo.reads != 2 {
		t.Fatalf("message=%q reopened=%v reads=%d err=%v", message, repo.reopened, repo.reads, err)
	}
}

func TestBuildingEntryReversesWrittenLedgerLegs(t *testing.T) {
	repo := &entryRepoMock{config: &models.BuildingEntryConfig{
		FeatureID: 7, FeePSC: "10.00", FeeIRR: "20.00", IsActive: true,
	}}
	wallet := &entryWalletMock{txnScript: []error{nil, nil, errors.New("ledger down"), nil, nil}}
	svc := service.NewBuildingEntryService(repo,
		&entryFeatureMock{feature: &models.Feature{ID: 7, OwnerID: 1}},
		&entryBuildingMock{buildings: finishedBuilding()}, nil, wallet)
	_, err := svc.Enter(userCtx(9), &pb.EnterBuildingRequest{FeatureId: 7})
	if err == nil || !strings.Contains(err.Error(), "record entry transaction") {
		t.Fatalf("ledger error=%v", err)
	}
	if len(wallet.txns) != 5 || wallet.txns[3].action != "deposit" || wallet.txns[4].action != "deposit" {
		t.Fatalf("ledger reversal=%+v", wallet.txns)
	}
	if !repo.deleted {
		t.Fatal("session retained after ledger failure")
	}
}
