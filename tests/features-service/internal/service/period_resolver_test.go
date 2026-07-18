package service_test

import (
	"strconv"
	"testing"
	"time"

	"metarang/features-service/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ptime "github.com/yaa110/go-persian-calendar"
)

func TestPeriodResolver_InvalidPeriod(t *testing.T) {
	_, err := service.ResolvePeriod("invalid", time.Now())
	require.Error(t, err)
}

func TestPeriodResolver_Daily(t *testing.T) {
	ref := time.Date(2026, 5, 15, 14, 30, 45, 0, time.Local)
	window, err := service.ResolvePeriod("daily", ref)
	require.NoError(t, err)

	assert.Equal(t, "daily", window.Period)
	assert.Equal(t, "hourly", window.Granularity)
	assert.Equal(t, 24, len(window.Buckets))

	expectedEnd := ref.Truncate(time.Second).Add(time.Second - time.Nanosecond)
	expectedStart := expectedEnd.Add(-24 * time.Hour).Truncate(time.Second)
	assert.True(t, window.End.Equal(expectedEnd), "end=%v expected=%v", window.End, expectedEnd)
	assert.True(t, window.Start.Equal(expectedStart), "start=%v expected=%v", window.Start, expectedStart)

	// Laravel reverses hourly buckets so newest hour is first.
	first := window.Buckets[0]
	assert.Equal(t, first.Start.Format("15:04"), first.Label)
	assert.Equal(t, expectedEnd.Truncate(time.Hour), first.Start.Truncate(time.Hour))
}

func TestPeriodResolver_Weekly(t *testing.T) {
	ref := time.Date(2026, 5, 15, 14, 30, 45, 0, time.Local)
	window, err := service.ResolvePeriod("weekly", ref)
	require.NoError(t, err)

	assert.Equal(t, "weekly", window.Period)
	assert.Equal(t, "daily", window.Granularity)
	assert.Equal(t, 7, len(window.Buckets))

	expectedEnd := ref.Truncate(time.Second).Add(time.Second - time.Nanosecond)
	expectedStart := startOfDayLocal(expectedEnd.AddDate(0, 0, -6))
	assert.True(t, window.Start.Equal(expectedStart))

	for i, bucket := range window.Buckets {
		day := expectedEnd.AddDate(0, 0, -(6 - i))
		assert.True(t, bucket.Start.Equal(startOfDayLocal(day)), "bucket %d start", i)
		assert.Equal(t, ptime.New(bucket.Start).Format("yyyy/MM/dd"), bucket.Label)
	}
}

func TestPeriodResolver_Monthly(t *testing.T) {
	ref := time.Date(2026, 5, 15, 14, 30, 45, 0, time.Local)
	window, err := service.ResolvePeriod("monthly", ref)
	require.NoError(t, err)

	assert.Equal(t, "monthly", window.Period)
	assert.Equal(t, "weekly", window.Granularity)
	assert.GreaterOrEqual(t, len(window.Buckets), 4)
	assert.LessOrEqual(t, len(window.Buckets), 5)

	expectedEnd := ref.Truncate(time.Second).Add(time.Second - time.Nanosecond)
	expectedStart := startOfDayLocal(expectedEnd.AddDate(0, 0, -29))
	assert.True(t, window.Start.Equal(expectedStart))
	assert.Equal(t, ptime.New(window.Buckets[0].Start).Format("yyyy/MM/dd"), window.Buckets[0].Label)
}

func TestPeriodResolver_Yearly(t *testing.T) {
	ref := time.Date(2026, 5, 15, 14, 30, 45, 0, time.Local)
	window, err := service.ResolvePeriod("yearly", ref)
	require.NoError(t, err)

	assert.Equal(t, "yearly", window.Period)
	assert.Equal(t, "monthly", window.Granularity)
	assert.Equal(t, 12, len(window.Buckets))

	expectedEnd := ref.Truncate(time.Second).Add(time.Second - time.Nanosecond)
	expectedStart := startOfMonthLocal(expectedEnd.AddDate(0, -11, 0))
	assert.True(t, window.Start.Equal(expectedStart))

	first := window.Buckets[0]
	pt := ptime.New(first.Start)
	assert.Equal(t, pt.Month().String()+" "+strconv.Itoa(pt.Year()), first.Label)
}

func startOfDayLocal(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func startOfMonthLocal(t time.Time) time.Time {
	y, m, _ := t.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, t.Location())
}
