// Package jalali converts between Gregorian and Jalali calendar dates.
package jalali

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// JalaliToCarbon converts Jalali date string to time.Time
// Format: Y/m/d (e.g., "1403/08/09")
func JalaliToCarbon(jalaliDate string) (time.Time, error) {
	parts := strings.Split(jalaliDate, "/")
	if len(parts) != 3 {
		return time.Time{}, fmt.Errorf("invalid jalali date format: expected Y/m/d")
	}

	year, err := strconv.Atoi(parts[0])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid year: %w", err)
	}

	month, err := strconv.Atoi(parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid month: %w", err)
	}

	day, err := strconv.Atoi(parts[2])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid day: %w", err)
	}

	return gregorianFromJalali(year, month, day)
}

// gregorianFromJalali finds the Gregorian date that maps to the given Jalali date
// using gregorianToJalali, keeping JalaliToCarbon consistent with CarbonToJalali.
func gregorianFromJalali(jy, jm, jd int) (time.Time, error) {
	approxYear := jy + 621
	start := time.Date(approxYear-1, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(approxYear+1, 12, 31, 0, 0, 0, 0, time.UTC)

	for current := start; !current.After(end); current = current.AddDate(0, 0, 1) {
		gotYear, gotMonth, gotDay := gregorianToJalali(current.Year(), int(current.Month()), current.Day())
		if gotYear == jy && gotMonth == jm && gotDay == jd {
			return current, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid jalali date: %d/%02d/%02d", jy, jm, jd)
}

// CarbonToJalali converts time.Time to Jalali date string
// Format: Y/m/d (e.g., "1403/08/09")
func CarbonToJalali(t time.Time) string {
	year, month, day := gregorianToJalali(t.Year(), int(t.Month()), t.Day())
	return fmt.Sprintf("%d/%02d/%02d", year, month, day)
}

// CarbonToJalaliDateTime converts time.Time to Jalali date-time string
// Format: Y/m/d H:i (e.g., "1403/08/09 14:30")
func CarbonToJalaliDateTime(t time.Time) string {
	year, month, day := gregorianToJalali(t.Year(), int(t.Month()), t.Day())
	return fmt.Sprintf("%d/%02d/%02d %02d:%02d", year, month, day, t.Hour(), t.Minute())
}

// gregorianToJalali converts Gregorian date to Jalali date
func gregorianToJalali(gy, gm, gd int) (jy, jm, jd int) {
	gDM := []int{0, 31, 59, 90, 120, 151, 181, 212, 243, 273, 304, 334}

	if gy > 1600 {
		jy = 979
		gy -= 1600
	} else {
		jy = 0
		gy -= 621
	}

	if gm > 2 {
		gy2 := gy + 1
		if (gy2%4 == 0 && gy2%100 != 0) || (gy2%400 == 0) {
			gDM[2] = 60
		}
	}

	gy2 := gy
	if (gy2%4 == 0 && gy2%100 != 0) || (gy2%400 == 0) {
		// leap year
		if gm > 2 {
			gd++
		}
	}

	days := 365*gy + ((gy + 3) / 4) - ((gy + 99) / 100) + ((gy + 399) / 400) + gd + gDM[gm-1] - 1

	jy += 33 * (days / 12053)
	days %= 12053

	jy += 4 * (days / 1461)
	days %= 1461

	if days > 365 {
		jy += (days - 1) / 365
		days = (days - 1) % 365
	}

	if days < 186 {
		jm = 1 + days/31
		jd = 1 + (days % 31)
	} else {
		jm = 7 + (days-186)/30
		jd = 1 + ((days - 186) % 30)
	}

	return
}
