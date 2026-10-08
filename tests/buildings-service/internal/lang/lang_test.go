package lang_test

import (
	"testing"

	"metarang/buildings-service/internal/lang"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeLocale(t *testing.T) {
	assert.Equal(t, "en", lang.NormalizeLocale(""))
	assert.Equal(t, "en", lang.NormalizeLocale("EN"))
	assert.Equal(t, "fa", lang.NormalizeLocale(" FA "))
	assert.Equal(t, "en", lang.NormalizeLocale("de"))
}

func TestTAndTf(t *testing.T) {
	assert.Equal(t, "feature_id is required", lang.T("en", "feature_id is required"))
	assert.Equal(t, "feature not found", lang.T("en", "feature not found"))
	got := lang.Tf("en", "failed to get build package: %v", "boom")
	assert.Contains(t, got, "boom")
	assert.NotEmpty(t, lang.T("fa", "feature not found"))
}
