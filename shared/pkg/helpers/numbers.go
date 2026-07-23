package helpers

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// FormatCompactNumber formats a number in compact notation (e.g., 1.2K, 3.4M)
// Mimics Laravel's formatCompactNumber helper exactly
// Laravel logic:
// - If number < 1000: return with 3 decimals, trim trailing zeros and dot
// - If number >= 1000: divide by appropriate power of 1000, format with 3 decimals, trim trailing zeros and dot, add unit
func FormatCompactNumber(num float64) string {
	if num < 1000 {
		// Format with 3 decimals
		formatted := fmt.Sprintf("%.3f", num)
		// Trim trailing zeros
		formatted = strings.TrimRight(formatted, "0")
		// Trim trailing dot
		formatted = strings.TrimRight(formatted, ".")
		return formatted
	}

	units := []string{"K", "M", "B", "T"}
	power := int(math.Floor(math.Log(num) / math.Log(1000)))

	if power > len(units) {
		power = len(units)
	}

	// Calculate the value divided by the appropriate power of 1000
	value := num / math.Pow(1000, float64(power))

	// Format with 3 decimals
	formatted := fmt.Sprintf("%.3f", value)
	// Trim trailing zeros
	formatted = strings.TrimRight(formatted, "0")
	// Trim trailing dot
	formatted = strings.TrimRight(formatted, ".")

	// Add unit (power-1 because arrays are 0-indexed but power starts at 1)
	return formatted + units[power-1]
}

// NumberFormat formats a number with decimals (equivalent to PHP's number_format)
func NumberFormat(num float64, decimals int) string {
	format := "%." + strconv.Itoa(decimals) + "f"
	return fmt.Sprintf(format, num)
}

// NumberFormatWithSeparator formats a number with thousand separators and decimals
func NumberFormatWithSeparator(num float64, decimals int, decPoint, thousandsSep string) string {
	// Format the number with decimals
	formatted := NumberFormat(num, decimals)

	// Split into integer and decimal parts
	parts := strings.Split(formatted, ".")
	intPart := parts[0]
	decPart := ""
	if len(parts) > 1 {
		decPart = parts[1]
	}

	// Add thousands separator to integer part
	var result strings.Builder
	negative := strings.HasPrefix(intPart, "-")
	if negative {
		intPart = intPart[1:]
		result.WriteString("-")
	}

	// Add thousands separator from right to left
	for i, digit := range reverse(intPart) {
		if i > 0 && i%3 == 0 {
			result.WriteString(thousandsSep)
		}
		result.WriteRune(digit)
	}

	// Reverse the result
	intPartFormatted := reverse(result.String())

	// Combine with decimal part
	if decimals > 0 {
		return intPartFormatted + decPoint + decPart
	}
	return intPartFormatted
}

// reverse reverses a string
func reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// NormalizePersianNumbers converts Persian/Arabic numerals to Latin
func NormalizePersianNumbers(input string) string {
	// Persian digits: ۰۱۲۳۴۵۶۷۸۹
	// Arabic digits: ٠١٢٣٤٥٦٧٨٩
	// Latin digits:  0123456789

	persianToLatin := map[rune]rune{
		'۰': '0', '۱': '1', '۲': '2', '۳': '3', '۴': '4',
		'۵': '5', '۶': '6', '۷': '7', '۸': '8', '۹': '9',
		'٠': '0', '١': '1', '٢': '2', '٣': '3', '٤': '4',
		'٥': '5', '٦': '6', '٧': '7', '٨': '8', '٩': '9',
	}

	var result strings.Builder
	for _, char := range input {
		if latinDigit, found := persianToLatin[char]; found {
			result.WriteRune(latinDigit)
		} else {
			result.WriteRune(char)
		}
	}

	return result.String()
}

// ParseCompactNumber parses a compact number string (e.g. "1.2K", "3.4M") or a plain
// numeric string (e.g. "1500.5") into float64. Empty input returns 0.
func ParseCompactNumber(s string) (float64, error) {
	s = strings.TrimSpace(NormalizePersianNumbers(s))
	if s == "" {
		return 0, nil
	}

	multiplier := 1.0
	switch suffix := s[len(s)-1]; suffix {
	case 'K', 'k':
		multiplier = 1_000
		s = strings.TrimSpace(s[:len(s)-1])
	case 'M', 'm':
		multiplier = 1_000_000
		s = strings.TrimSpace(s[:len(s)-1])
	case 'B', 'b':
		multiplier = 1_000_000_000
		s = strings.TrimSpace(s[:len(s)-1])
	case 'T', 't':
		multiplier = 1_000_000_000_000
		s = strings.TrimSpace(s[:len(s)-1])
	}

	if s == "" {
		return 0, fmt.Errorf("invalid compact number")
	}

	value, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return value * multiplier, nil
}

// ParseFloat parses a string to float64 after normalizing Persian numbers
func ParseFloat(s string) (float64, error) {
	normalized := NormalizePersianNumbers(s)
	return strconv.ParseFloat(normalized, 64)
}

// ParseInt parses a string to int64 after normalizing Persian numbers
func ParseInt(s string) (int64, error) {
	normalized := NormalizePersianNumbers(s)
	return strconv.ParseInt(normalized, 10, 64)
}
