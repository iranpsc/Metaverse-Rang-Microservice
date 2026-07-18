package helpers

import (
	"fmt"
	"time"

	ptime "github.com/yaa110/go-persian-calendar"
)

// FormatJalaliDate converts Gregorian date to Jalali format Y/m/d
// Example: 2025-10-30 -> 1403/08/09
func FormatJalaliDate(t time.Time) string {
	pt := ptime.New(t)
	return pt.Format("yyyy/MM/dd")
}

// FormatJalaliDateTime converts Gregorian datetime to Jalali format Y/m/d H:m:s
// Example: 2025-10-30 14:30:45 -> 1403/08/09 14:30:45
func FormatJalaliDateTime(t time.Time) string {
	pt := ptime.New(t)
	return pt.Format("yyyy/MM/dd HH:mm:ss")
}

// FormatJalaliDateTimeLaravel matches Laravel jdate(...)->format('Y-m-d H:i:s').
func FormatJalaliDateTimeLaravel(t time.Time) string {
	pt := ptime.New(t)
	return pt.Format("yyyy-MM-dd HH:mm:ss")
}

// FormatJalaliTime formats just the time part H:m:s
func FormatJalaliTime(t time.Time) string {
	return t.Format("15:04:05")
}

// ParseJalaliDate parses a Jalali date string to Gregorian time.Time
// Example: "1403/08/09" -> 2025-10-30
func ParseJalaliDate(jalaliDate string) (time.Time, error) {
	// Parse format: yyyy/MM/dd
	// For go-persian-calendar, we need to manually parse the components
	var year, month, day int
	_, err := time.Parse("2006/01/02", jalaliDate) // Just for validation
	if err == nil {
		// If it's a valid Gregorian date, return it as-is
		return time.Parse("2006/01/02", jalaliDate)
	}

	// Try parsing as Jalali date components
	_, err = fmt.Sscanf(jalaliDate, "%d/%d/%d", &year, &month, &day)
	if err != nil {
		return time.Time{}, err
	}

	// Create a Persian time and convert to Gregorian
	pt := ptime.Date(year, ptime.Month(month), day, 0, 0, 0, 0, ptime.Iran())
	return pt.Time(), nil
}

// ParseJalaliDateTime parses a Jalali datetime string to Gregorian time.Time
// Example: "1403/08/09 14:30:45" -> 2025-10-30 14:30:45
func ParseJalaliDateTime(jalaliDateTime string) (time.Time, error) {
	// Parse format: yyyy/MM/dd HH:mm:ss
	var year, month, day, hour, min, sec int
	_, err := fmt.Sscanf(jalaliDateTime, "%d/%d/%d %d:%d:%d", &year, &month, &day, &hour, &min, &sec)
	if err != nil {
		return time.Time{}, err
	}

	// Create a Persian time and convert to Gregorian
	pt := ptime.Date(year, ptime.Month(month), day, hour, min, sec, 0, ptime.Iran())
	return pt.Time(), nil
}

// NowJalali returns current time formatted in Jalali
func NowJalali() string {
	return FormatJalaliDate(time.Now())
}

// NowJalaliDateTime returns current datetime formatted in Jalali
func NowJalaliDateTime() string {
	return FormatJalaliDateTime(time.Now())
}
