package models

import (
	"encoding/json"
	"strings"
)

// ParsePrivacyJSON decodes settings.privacy JSON into 0|1 flags.
//
// Stored values may be booleans (legacy Laravel) or integers (Go updates).
// Missing keys are filled from DefaultPrivacySettings so a single-key update
// cannot blank the rest of the public profile.
func ParsePrivacyJSON(raw string) map[string]int {
	privacy := DefaultPrivacySettings()
	if strings.TrimSpace(raw) == "" {
		return privacy
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil || parsed == nil {
		return privacy
	}

	for key, value := range parsed {
		privacy[key] = PrivacyValueToInt(value)
	}
	return privacy
}

// ParsePublicPrivacyJSON decodes settings.privacy for the public citizen
// profile. Public profile privacy is opt-out, so every omitted key is public;
// only an explicitly stored zero hides a field.
func ParsePublicPrivacyJSON(raw string) map[string]int {
	privacy := DefaultPrivacySettings()
	for key := range privacy {
		privacy[key] = 1
	}
	if strings.TrimSpace(raw) == "" {
		return privacy
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil || parsed == nil {
		return privacy
	}

	for key, value := range parsed {
		privacy[key] = PrivacyValueToInt(value)
	}
	return privacy
}

// PrivacyValueToInt normalizes a JSON privacy value to 0 (private) or 1 (public).
func PrivacyValueToInt(value interface{}) int {
	switch v := value.(type) {
	case bool:
		if v {
			return 1
		}
		return 0
	case float64:
		if v != 0 {
			return 1
		}
		return 0
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			f, ferr := v.Float64()
			if ferr != nil || f == 0 {
				return 0
			}
			return 1
		}
		if i != 0 {
			return 1
		}
		return 0
	case int:
		if v != 0 {
			return 1
		}
		return 0
	case int32:
		if v != 0 {
			return 1
		}
		return 0
	case int64:
		if v != 0 {
			return 1
		}
		return 0
	case string:
		if v == "1" || strings.EqualFold(v, "true") {
			return 1
		}
		return 0
	default:
		return 0
	}
}

// PrivacyIntToBoolMap converts 0|1 privacy flags to the citizen profile bool map.
func PrivacyIntToBoolMap(privacy map[string]int) map[string]bool {
	if privacy == nil {
		privacy = DefaultPrivacySettings()
	}
	out := make(map[string]bool, len(privacy))
	for key, value := range privacy {
		out[key] = value != 0
	}
	return out
}

// PrivacyIntToInt32Map converts 0|1 privacy flags to the citizen user-info map.
func PrivacyIntToInt32Map(privacy map[string]int) map[string]int32 {
	if privacy == nil {
		privacy = DefaultPrivacySettings()
	}
	out := make(map[string]int32, len(privacy))
	for key, value := range privacy {
		out[key] = int32(value)
	}
	return out
}
