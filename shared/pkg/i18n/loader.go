package i18n

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Loader holds translations for EN and FA locales
type Loader struct {
	en map[string]string
	fa map[string]string
}

// NewLoader creates a new i18n loader from JSON content
func NewLoader(enJSON, faJSON []byte) (*Loader, error) {
	loader := &Loader{
		en: make(map[string]string),
		fa: make(map[string]string),
	}

	if len(enJSON) > 0 {
		if err := json.Unmarshal(enJSON, &loader.en); err != nil {
			return nil, fmt.Errorf("failed to load en translations: %w", err)
		}
	}

	if len(faJSON) > 0 {
		if err := json.Unmarshal(faJSON, &loader.fa); err != nil {
			return nil, fmt.Errorf("failed to load fa translations: %w", err)
		}
	}

	return loader, nil
}

// T returns the translated string for the given key and locale
// If translation is not found, returns the key itself
// Locale is case-insensitive (en, EN, fa, FA)
func (l *Loader) T(locale, key string) string {
	locale = strings.ToLower(strings.TrimSpace(locale))
	if locale != "fa" && locale != "en" {
		locale = "en"
	}

	var m map[string]string
	if locale == "fa" {
		m = l.fa
	} else {
		m = l.en
	}

	if v, ok := m[key]; ok && v != "" {
		return v
	}
	return key
}

// Tf returns the translated string formatted with the given arguments
// Uses fmt.Sprintf with the translated format string
func (l *Loader) Tf(locale, key string, a ...interface{}) string {
	format := l.T(locale, key)
	if len(a) == 0 {
		return format
	}
	return fmt.Sprintf(format, a...)
}
