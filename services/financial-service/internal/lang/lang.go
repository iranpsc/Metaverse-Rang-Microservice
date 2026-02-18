package lang

import (
	_ "embed"
	"strings"

	"metargb/shared/pkg/i18n"
)

//go:embed en.json
var enJSON []byte

//go:embed fa.json
var faJSON []byte

var loader *i18n.Loader

func init() {
	var err error
	loader, err = i18n.NewLoader(enJSON, faJSON)
	if err != nil {
		panic("financial-service: failed to load translations: " + err.Error())
	}
}

// T returns translated string for key using locale (EN or FA)
func T(locale, key string) string {
	return loader.T(normalizeLocale(locale), key)
}

// Tf returns translated format string with args
func Tf(locale, key string, a ...interface{}) string {
	return loader.Tf(normalizeLocale(locale), key, a...)
}

// NormalizeLocale returns "en" or "fa", defaulting to "en"
func NormalizeLocale(locale string) string {
	return normalizeLocale(locale)
}

func normalizeLocale(locale string) string {
	locale = strings.ToLower(strings.TrimSpace(locale))
	if locale == "fa" {
		return "fa"
	}
	return "en"
}
