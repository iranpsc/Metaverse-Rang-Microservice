// Package utils provides shared utility helpers for the support service.
package utils

import (
	"fmt"
	"time"
)

// JalaliDate represents a Jalali (Persian) date
type JalaliDate struct {
	Year   int
	Month  int
	Day    int
	Hour   int
	Minute int
	Second int
}

// FormatJalaliDate converts Gregorian date to Jalali and formats it
func FormatJalaliDate(t time.Time) string {
	jd := GregorianToJalali(t)
	return fmt.Sprintf("%04d/%02d/%02d", jd.Year, jd.Month, jd.Day)
}

// FormatJalaliTime formats time as HH:MM:SS
func FormatJalaliTime(t time.Time) string {
	return fmt.Sprintf("%02d:%02d:%02d", t.Hour(), t.Minute(), t.Second())
}

// FormatJalaliDateTime formats both date and time
func FormatJalaliDateTime(t time.Time) string {
	jd := GregorianToJalali(t)
	return fmt.Sprintf("%04d/%02d/%02d %02d:%02d:%02d",
		jd.Year, jd.Month, jd.Day, t.Hour(), t.Minute(), t.Second())
}

// GregorianToJalali converts Gregorian date to Jalali
func GregorianToJalali(t time.Time) JalaliDate {
	gy := t.Year()
	gm := int(t.Month())
	gd := t.Day()

	gDM := []int{0, 31, 59, 90, 120, 151, 181, 212, 243, 273, 304, 334}

	var jy int
	if gy > 1600 {
		jy = 979
		gy -= 1600
	} else {
		jy = 0
		gy -= 621
	}

	if gm > 2 {
		gy2 := gy + 1
		if gy2%4 == 0 && (gy2%100 != 0 || gy2%400 == 0) {
			gDM[2] = 60
		}
	} else {
		if gy%4 == 0 && (gy%100 != 0 || gy%400 == 0) {
			gDM[2] = 60
		}
	}

	gy2 := gy
	if gm > 2 {
		gy2++
	}

	days := 365*gy + ((gy2 + 3) / 4) - ((gy2 + 99) / 100) + ((gy2 + 399) / 400) - 80 + gd + gDM[gm-1]
	jy += 33 * (days / 12053)
	days %= 12053

	jy += 4 * (days / 1461)
	days %= 1461

	if days > 365 {
		jy += (days - 1) / 365
		days = (days - 1) % 365
	}

	var jm, jd int
	if days < 186 {
		jm = 1 + days/31
		jd = 1 + (days % 31)
	} else {
		jm = 7 + (days-186)/30
		jd = 1 + ((days - 186) % 30)
	}

	return JalaliDate{
		Year:   jy,
		Month:  jm,
		Day:    jd,
		Hour:   t.Hour(),
		Minute: t.Minute(),
		Second: t.Second(),
	}
}
