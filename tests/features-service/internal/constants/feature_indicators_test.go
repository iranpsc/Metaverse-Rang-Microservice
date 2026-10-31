package constants_test

import (
	"testing"

	"metarang/features-service/internal/constants"

	"github.com/stretchr/testify/assert"
)

func TestGetColor(t *testing.T) {
	assert.Equal(t, "yellow", constants.GetColor("m"))
	assert.Equal(t, "red", constants.GetColor("t"))
	assert.Equal(t, "blue", constants.GetColor("a"))
	assert.Equal(t, "", constants.GetColor("unknown"))
}

func TestGetColorPersian(t *testing.T) {
	assert.Equal(t, constants.ColorMaskoniPersian, constants.GetColorPersian("m"))
	assert.Equal(t, constants.ColorTejariPersian, constants.GetColorPersian("t"))
	assert.Equal(t, constants.ColorAmozeshiPersian, constants.GetColorPersian("a"))
	assert.Equal(t, "", constants.GetColorPersian("x"))
}

func TestGetKarbariTitle(t *testing.T) {
	assert.Equal(t, constants.TitleMaskoni, constants.GetKarbariTitle("m"))
	assert.Equal(t, constants.TitleTejari, constants.GetKarbariTitle("t"))
	assert.Equal(t, constants.TitleAmozeshi, constants.GetKarbariTitle("a"))
	assert.Equal(t, "", constants.GetKarbariTitle("unknown"))
	for _, tc := range []struct{ code, title string }{
		{constants.Edari, constants.TitleEdari},
		{constants.Behdashti, constants.TitleBehdashti},
		{constants.FazaSabz, constants.TitleFazaSabz},
		{constants.Farhangi, constants.TitleFarhangi},
		{constants.Parking, constants.TitleParking},
		{constants.Mazhabi, constants.TitleMazhabi},
		{constants.Nemayeshgah, constants.TitleNemayeshgah},
		{constants.Gardeshgari, constants.TitleGardeshgari},
	} {
		assert.Equal(t, tc.title, constants.GetKarbariTitle(tc.code))
	}
}

func TestGetKarbariCoefficient(t *testing.T) {
	assert.Equal(t, 0.1, constants.GetKarbariCoefficient("m"))
	assert.Equal(t, 0.2, constants.GetKarbariCoefficient("t"))
	assert.Equal(t, 0.3, constants.GetKarbariCoefficient("a"))
	assert.Equal(t, 1.0, constants.GetKarbariCoefficient("unknown"))
}

func TestIsLimitedFeature(t *testing.T) {
	assert.True(t, constants.IsLimitedFeature(constants.MaskoniTradingLimited))
	assert.True(t, constants.IsLimitedFeature(constants.TejariTradingLimited))
	assert.True(t, constants.IsLimitedFeature(constants.AmoozeshiTradingLimited))
	assert.False(t, constants.IsLimitedFeature(constants.MaskoniSoldAndPriced))
}

func TestIsNotAllowedToBeSold(t *testing.T) {
	assert.True(t, constants.IsNotAllowedToBeSold(constants.MaskoniNotAllowedToBeSold))
	assert.True(t, constants.IsNotAllowedToBeSold(constants.TejariNotAllowedToBeSold))
	assert.True(t, constants.IsNotAllowedToBeSold(constants.AmoozeshiNotAllowedToBeSold))
	assert.False(t, constants.IsNotAllowedToBeSold(constants.MaskoniPriced))
}

func TestIsSoldAndNotPriced(t *testing.T) {
	assert.True(t, constants.IsSoldAndNotPriced(constants.MaskoniSoldAndNotPriced))
	assert.True(t, constants.IsSoldAndNotPriced(constants.TejariSoldAndNotPriced))
	assert.True(t, constants.IsSoldAndNotPriced(constants.AmozeshiSoldAndNotPriced))
	assert.True(t, constants.IsSoldAndNotPriced(constants.MaskoniNotPriced))
	assert.False(t, constants.IsSoldAndNotPriced(constants.MaskoniSoldAndPriced))
}

func TestChangeStatusToSoldAndPriced(t *testing.T) {
	assert.Equal(t, constants.MaskoniSoldAndPriced, constants.ChangeStatusToSoldAndPriced("m"))
	assert.Equal(t, constants.TejariSoldAndPriced, constants.ChangeStatusToSoldAndPriced("t"))
	assert.Equal(t, constants.AmozeshiSoldAndPriced, constants.ChangeStatusToSoldAndPriced("a"))
	assert.Equal(t, "", constants.ChangeStatusToSoldAndPriced("x"))
}

func TestChangeStatusToSoldAndNotPriced(t *testing.T) {
	assert.Equal(t, constants.MaskoniSoldAndNotPriced, constants.ChangeStatusToSoldAndNotPriced("m"))
	assert.Equal(t, constants.TejariSoldAndNotPriced, constants.ChangeStatusToSoldAndNotPriced("t"))
	assert.Equal(t, constants.AmozeshiSoldAndNotPriced, constants.ChangeStatusToSoldAndNotPriced("a"))
	assert.Equal(t, "", constants.ChangeStatusToSoldAndNotPriced("x"))
}
