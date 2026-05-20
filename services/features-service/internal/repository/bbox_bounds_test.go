package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBboxBoundsFromPoints(t *testing.T) {
	points := []string{
		"10,20",
		"30,20",
		"30,40",
		"10,40",
	}

	minX, maxX, minY, maxY, err := bboxBoundsFromPoints(points)
	require.NoError(t, err)
	assert.Equal(t, "10", minX)
	assert.Equal(t, "30", maxX)
	assert.Equal(t, "20", minY)
	assert.Equal(t, "40", maxY)
}
