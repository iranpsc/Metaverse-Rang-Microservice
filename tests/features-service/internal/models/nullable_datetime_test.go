package models_test

import (
	"testing"
	"time"

	"metarang/features-service/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNullableDateTime_ScanNil(t *testing.T) {
	var n models.NullableDateTime
	require.NoError(t, n.Scan(nil))
	assert.False(t, n.Valid)
}

func TestNullableDateTime_ScanTime(t *testing.T) {
	want := time.Date(2026, 9, 30, 17, 42, 0, 0, time.UTC)
	var n models.NullableDateTime
	require.NoError(t, n.Scan(want))
	require.True(t, n.Valid)
	assert.True(t, want.Equal(n.Time))
}

func TestNullableDateTime_ScanVarcharBytes(t *testing.T) {
	var n models.NullableDateTime
	require.NoError(t, n.Scan([]byte("2026-09-30 17:42:00")))
	require.True(t, n.Valid)
	assert.Equal(t, 2026, n.Time.Year())
	assert.Equal(t, time.September, n.Time.Month())
	assert.Equal(t, 30, n.Time.Day())
	assert.Equal(t, 17, n.Time.Hour())
	assert.Equal(t, 42, n.Time.Minute())
}

func TestNullableDateTime_ScanEmptyAndZeroDates(t *testing.T) {
	for _, raw := range []any{"", []byte(""), "0000-00-00", "0000-00-00 00:00:00"} {
		var n models.NullableDateTime
		require.NoError(t, n.Scan(raw), "scan %v", raw)
		assert.False(t, n.Valid, "scan %v", raw)
	}
}

func TestNullableDateTime_ScanInvalid(t *testing.T) {
	var n models.NullableDateTime
	err := n.Scan("not-a-date")
	require.Error(t, err)
	assert.False(t, n.Valid)
}

func TestNullableDateTime_Value(t *testing.T) {
	var unset models.NullableDateTime
	v, err := unset.Value()
	require.NoError(t, err)
	assert.Nil(t, v)

	want := time.Date(2026, 9, 30, 17, 42, 0, 0, time.Local)
	set := models.NullableDateTime{}
	set.Time, set.Valid = want, true
	v, err = set.Value()
	require.NoError(t, err)
	assert.Equal(t, want, v)
}
