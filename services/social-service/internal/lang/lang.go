package lang

import (
	_ "embed"
	"strings"

	"metarang/shared/pkg/i18n"
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
		panic("social-service: failed to load translations: " + err.Error())
	}
}

func T(locale, key string) string {
	return loader.T(NormalizeLocale(locale), key)
}

func NormalizeLocale(locale string) string {
	if strings.ToLower(strings.TrimSpace(locale)) == "fa" {
		return "fa"
	}
	return "en"
}
