package service

import (
	"fmt"
	"time"
)

type JalaliConverter interface {
	NowJalali() string
	FormatJalaliDate(t time.Time) string
}

type jalaliConverter struct{}

func NewJalaliConverter() JalaliConverter {
	return &jalaliConverter{}
}

func (c *jalaliConverter) NowJalali() string {
	return c.FormatJalaliDate(time.Now())
}

func (c *jalaliConverter) FormatJalaliDate(t time.Time) string {
	// Use shared helpers for Jalali conversion
	// For now, using a simple implementation
	// TODO: Import actual jalali converter from shared/pkg/helpers if available
	jy, jm, jd := gregorianToJalali(t.Year(), int(t.Month()), t.Day())
	return fmt.Sprintf("%04d/%02d/%02d", jy, jm, jd)
}

// gregorianToJalali converts Gregorian date to Jalali (Persian) date
func gregorianToJalali(gy, gm, gd int) (int, int, int) {
	var jy, jm, jd int
	gDN := 365*gy + ((gy + 3) / 4) - ((gy + 99) / 100) + ((gy + 399) / 400)

	for i := 0; i < gm-1; i++ {
		gDN += daysInGregorianMonth(i+1, gy)
	}
	gDN += gd

	jDN := gDN - 79
	jNp := jDN / 12053
	jDN = jDN % 12053

	jy = 979 + 33*jNp + 4*(jDN/1461)
	jDN = jDN % 1461

	if jDN >= 366 {
		jy += (jDN - 1) / 365
		jDN = (jDN - 1) % 365
	}

	if jDN < 186 {
		jm = 1 + jDN/31
		jd = 1 + (jDN % 31)
	} else {
		jm = 7 + (jDN-186)/30
		jd = 1 + ((jDN - 186) % 30)
	}

	return jy, jm, jd
}

func daysInGregorianMonth(month, year int) int {
	if month == 2 {
		if isLeapYear(year) {
			return 29
		}
		return 28
	}
	if month == 4 || month == 6 || month == 9 || month == 11 {
		return 30
	}
	return 31
}

func isLeapYear(year int) bool {
	return (year%4 == 0 && year%100 != 0) || (year%400 == 0)
}
