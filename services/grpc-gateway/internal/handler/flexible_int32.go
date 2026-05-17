package handler

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"metargb/shared/pkg/helpers"
)

// flexibleInt32 unmarshals JSON numbers or numeric strings (including Persian digits).
type flexibleInt32 int32

func (f *flexibleInt32) UnmarshalJSON(data []byte) error {
	data = bytesTrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		*f = 0
		return nil
	}

	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		s = strings.TrimSpace(helpers.NormalizePersianNumbers(s))
		if s == "" {
			*f = 0
			return nil
		}
		n, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return fmt.Errorf("invalid integer value %q: %w", s, err)
		}
		*f = flexibleInt32(n)
		return nil
	}

	var n int32
	if err := json.Unmarshal(data, &n); err == nil {
		*f = flexibleInt32(n)
		return nil
	}

	var f64 float64
	if err := json.Unmarshal(data, &f64); err != nil {
		return fmt.Errorf("invalid integer value: %w", err)
	}
	*f = flexibleInt32(f64)
	return nil
}

func (f flexibleInt32) Int32() int32 {
	return int32(f)
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}
