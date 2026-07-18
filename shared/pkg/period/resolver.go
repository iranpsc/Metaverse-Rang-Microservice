// Package period resolves reporting periods for analytics and summaries.
package period

import (
	"fmt"
	"strconv"
	"time"

	ptime "github.com/yaa110/go-persian-calendar"
)

// ValidPeriods matches Laravel PeriodResolver::PERIODS.
var ValidPeriods = []string{"daily", "weekly", "monthly", "yearly"}

// PeriodBucket is a single chart bucket with a Jalali label.
type PeriodBucket struct {
	Start time.Time
	End   time.Time
	Label string
}

// PeriodWindow is the resolved period window and chart buckets.
type PeriodWindow struct {
	Period      string
	Start       time.Time
	End         time.Time
	Granularity string
	Buckets     []PeriodBucket
}

// PreviousWindow is the immediately preceding period of equal length.
type PreviousWindow struct {
	Start time.Time
	End   time.Time
}

// ResolvePeriod ports Laravel App\Services\WalletHistory\PeriodResolver::resolve.
func ResolvePeriod(period string, reference time.Time) (*PeriodWindow, error) {
	if !isValidPeriod(period) {
		return nil, fmt.Errorf("invalid period [%s] provided", period)
	}

	end := endOfSecond(reference)
	var start time.Time
	switch period {
	case "daily":
		start = startOfSecond(end.Add(-24 * time.Hour))
	case "weekly":
		start = startOfDay(end.AddDate(0, 0, -6))
	case "monthly":
		start = startOfDay(end.AddDate(0, 0, -29))
	case "yearly":
		start = startOfMonth(end.AddDate(0, -11, 0))
	}

	return &PeriodWindow{
		Period:      period,
		Start:       start,
		End:         end,
		Granularity: granularityFor(period),
		Buckets:     buildBuckets(period, start, end),
	}, nil
}

// ResolvePrevious ports Laravel PeriodResolver::resolvePrevious.
func ResolvePrevious(period string, reference time.Time) (*PreviousWindow, error) {
	current, err := ResolvePeriod(period, reference)
	if err != nil {
		return nil, err
	}
	duration := current.End.Sub(current.Start)
	return &PreviousWindow{
		Start: current.Start.Add(-(duration + time.Second)),
		End:   current.Start.Add(-time.Second),
	}, nil
}

// NormalizePeriod returns period if valid, otherwise "daily".
func NormalizePeriod(period string) string {
	if isValidPeriod(period) {
		return period
	}
	return "daily"
}

func isValidPeriod(period string) bool {
	for _, p := range ValidPeriods {
		if p == period {
			return true
		}
	}
	return false
}

func granularityFor(period string) string {
	switch period {
	case "daily":
		return "hourly"
	case "weekly":
		return "daily"
	case "monthly":
		return "weekly"
	case "yearly":
		return "monthly"
	default:
		return ""
	}
}

func buildBuckets(period string, start, end time.Time) []PeriodBucket {
	switch period {
	case "daily":
		return hourlyBuckets(end)
	case "weekly":
		return dailyBuckets(end, 7)
	case "monthly":
		return weeklyBuckets(start, end)
	case "yearly":
		return monthlyBuckets(end, 12)
	default:
		return nil
	}
}

func hourlyBuckets(end time.Time) []PeriodBucket {
	// Chronological order (oldest → newest), consistent with daily/weekly/monthly buckets.
	buckets := make([]PeriodBucket, 0, 24)
	for offset := 23; offset >= 0; offset-- {
		bucketEnd := endOfHour(end.Add(-time.Duration(offset) * time.Hour))
		bucketStart := startOfHour(bucketEnd)
		buckets = append(buckets, PeriodBucket{
			Start: bucketStart,
			End:   bucketEnd,
			Label: bucketStart.Format("15:04"),
		})
	}
	return buckets
}

func dailyBuckets(end time.Time, days int) []PeriodBucket {
	buckets := make([]PeriodBucket, 0, days)
	for offset := days - 1; offset >= 0; offset-- {
		bucketDate := end.AddDate(0, 0, -offset)
		bucketStart := startOfDay(bucketDate)
		bucketEnd := endOfDay(bucketDate)
		buckets = append(buckets, PeriodBucket{
			Start: bucketStart,
			End:   bucketEnd,
			Label: ptime.New(bucketStart).Format("yyyy/MM/dd"),
		})
	}
	return buckets
}

func weeklyBuckets(start, end time.Time) []PeriodBucket {
	buckets := make([]PeriodBucket, 0)
	cursor := startOfDay(start)
	for !cursor.After(end) {
		bucketStart := cursor
		bucketEnd := endOfDay(cursor.AddDate(0, 0, 6))
		if bucketEnd.After(end) {
			bucketEnd = end
		}
		buckets = append(buckets, PeriodBucket{
			Start: bucketStart,
			End:   bucketEnd,
			Label: ptime.New(bucketStart).Format("yyyy/MM/dd"),
		})
		cursor = cursor.AddDate(0, 0, 7)
	}
	return buckets
}

func monthlyBuckets(end time.Time, months int) []PeriodBucket {
	buckets := make([]PeriodBucket, 0, months)
	for offset := months - 1; offset >= 0; offset-- {
		bucketDate := end.AddDate(0, -offset, 0)
		bucketStart := startOfMonth(bucketDate)
		bucketEnd := endOfMonth(bucketDate)
		pt := ptime.New(bucketStart)
		buckets = append(buckets, PeriodBucket{
			Start: bucketStart,
			End:   bucketEnd,
			Label: pt.Month().String() + " " + strconv.Itoa(pt.Year()),
		})
	}
	return buckets
}

func endOfSecond(t time.Time) time.Time {
	return t.Truncate(time.Second).Add(time.Second - time.Nanosecond)
}

func startOfSecond(t time.Time) time.Time {
	return t.Truncate(time.Second)
}

func startOfHour(t time.Time) time.Time {
	return t.Truncate(time.Hour)
}

func endOfHour(t time.Time) time.Time {
	return startOfHour(t).Add(time.Hour - time.Nanosecond)
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func endOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 23, 59, 59, int(time.Second-time.Nanosecond), t.Location())
}

func startOfMonth(t time.Time) time.Time {
	y, m, _ := t.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, t.Location())
}

func endOfMonth(t time.Time) time.Time {
	return startOfMonth(t).AddDate(0, 1, 0).Add(-time.Nanosecond)
}
