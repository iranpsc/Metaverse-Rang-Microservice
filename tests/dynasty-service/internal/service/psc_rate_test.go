package service_test

import (
	"math"
	"testing"

	"metarang/dynasty-service/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRialsToPSC(t *testing.T) {
	for _, tc := range []struct {
		name  string
		rials int64
		rate  float64
		want  float64
		valid bool
	}{
		{"fractional prize", 1000, 400, 2.5, true},
		{"fractional rate", 1000, 400.5, 1000 / 400.5, true},
		{"large prize", 3000000000, 400, 7500000, true},
		{"zero prize", 0, 400, 0, true},
		{"negative prize", -1, 400, 0, false},
		{"overflow", 1000, math.SmallestNonzeroFloat64, 0, false},
		{"zero rate", 1000, 0, 0, false},
		{"negative rate", 1000, -1, 0, false},
		{"nan rate", 1000, math.NaN(), 0, false},
		{"infinite rate", 1000, math.Inf(1), 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := service.RialsToPSC(tc.rials, tc.rate)
			if tc.valid {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			} else {
				require.Error(t, err)
				assert.Zero(t, got)
			}
		})
	}
}
