package service

import (
	"time"

	"metargb/shared/pkg/helpers"
)

// FormatJalaliDate formats a time.Time to Jalali format Y/m/d.
func FormatJalaliDate(t time.Time) string {
	return helpers.FormatJalaliDate(t)
}

// FormatJalaliDateTime formats a time.Time to Jalali format Y-m-d H:i:s (Laravel-compatible).
func FormatJalaliDateTime(t time.Time) string {
	return helpers.FormatJalaliDateTimeLaravel(t)
}
