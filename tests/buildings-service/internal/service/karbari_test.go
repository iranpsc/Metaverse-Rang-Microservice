package service_test

import (
	"testing"

	"metarang/buildings-service/internal/constants"
	"metarang/buildings-service/internal/service"

	"github.com/stretchr/testify/assert"
)

func TestKarbariLabel(t *testing.T) {
	assert.Equal(t, "مسکونی", service.KarbariLabel("m"))
	assert.Equal(t, "تجاری", service.KarbariLabel("t"))
	assert.Equal(t, "نامشخص", service.KarbariLabel("unknown"))
}

func TestGetKarbariCoefficient(t *testing.T) {
	assert.Equal(t, 0.1, constants.GetKarbariCoefficient(constants.Maskoni))
	assert.Equal(t, 0.2, constants.GetKarbariCoefficient(constants.Tejari))
	assert.Equal(t, 1.0, constants.GetKarbariCoefficient("unknown"))
}
