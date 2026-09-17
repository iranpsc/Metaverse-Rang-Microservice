package service

import (
	"time"

	"metarang/shared/pkg/helpers"
)

// FormatJalaliDate formats a time.Time to Jalali format Y/m/d.
func FormatJalaliDate(t time.Time) string {
	return helpers.FormatJalaliDate(t)
}

func FormatJalaliDateTime(t time.Time) string {
	return helpers.FormatJalaliDateTimeDash(t)
}

func formatJalaliTime(t time.Time) string {
	return helpers.FormatJalaliTime(t)
}
