package handler

import (
	"testing"

	featurespb "metarang/shared/pb/features"
)

func TestEntryConfigRequestPreservesOmittedFields(t *testing.T) {
	existing := &featurespb.BuildingEntryConfig{
		FeePsc: "10.00", FeeIrr: "5.00", About: "keep", IsActive: true,
		LevelScopeType: "exact", LevelSlug: "level-3",
	}
	req := entryConfigRequest(7, map[string]interface{}{"about": "new"}, existing)
	if req.FeatureId != 7 || req.About != "new" || req.FeePsc != "10.00" || req.FeeIrr != "5.00" || !req.IsActive {
		t.Fatalf("request=%+v", req)
	}
	if req.LevelScopeType != "exact" || req.LevelSlug != "level-3" {
		t.Fatalf("scope lost: %+v", req)
	}

	created := entryConfigRequest(7, map[string]interface{}{"fee_psc": "1.50"}, nil)
	if created.IsActive || created.FeePsc != "1.50" || created.FeeIrr != "" {
		t.Fatalf("create request=%+v", created)
	}
}
