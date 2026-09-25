package sitemap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// LanguageTemplates is one citizens language in templates.json, in file order.
type LanguageTemplates struct {
	Language string
	URLs     []string
}

// Templates is the citizens section of templates.json.
type Templates struct {
	Citizens []LanguageTemplates
	// HasCitizens is false when the citizens key is absent.
	HasCitizens bool
}

// LoadTemplates reads templates.json. Language key order is the order in the file.
func LoadTemplates(path string) (Templates, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Templates{}, fmt.Errorf("read templates: %w", err)
	}
	return ParseTemplates(data)
}

// ParseTemplates decodes templates.json. Missing citizens is not an error.
func ParseTemplates(data []byte) (Templates, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return Templates{}, fmt.Errorf("parse templates: %w", err)
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return Templates{}, fmt.Errorf("parse templates: expected object")
	}

	var out Templates
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return Templates{}, fmt.Errorf("parse templates: %w", err)
		}
		key, ok := keyTok.(string)
		if !ok {
			return Templates{}, fmt.Errorf("parse templates: expected string key")
		}
		if key != "citizens" {
			var skip json.RawMessage
			if err := dec.Decode(&skip); err != nil {
				return Templates{}, fmt.Errorf("parse templates: %w", err)
			}
			continue
		}
		langs, err := parseCitizens(dec)
		if err != nil {
			return Templates{}, err
		}
		out.Citizens = langs
		out.HasCitizens = true
	}
	return out, nil
}

func parseCitizens(dec *json.Decoder) ([]LanguageTemplates, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("parse templates citizens: %w", err)
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		return nil, fmt.Errorf("parse templates: citizens must be an object")
	}
	var langs []LanguageTemplates
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("parse templates citizens: %w", err)
		}
		lang, ok := keyTok.(string)
		if !ok {
			return nil, fmt.Errorf("parse templates: expected language key")
		}
		var urls []string
		if err := dec.Decode(&urls); err != nil {
			return nil, fmt.Errorf("parse templates language %s: %w", lang, err)
		}
		langs = append(langs, LanguageTemplates{Language: lang, URLs: urls})
	}
	if _, err := dec.Token(); err != nil {
		return nil, fmt.Errorf("parse templates citizens: %w", err)
	}
	return langs, nil
}

// ExpandCitizenURLs replaces the [code] placeholder in every template.
func ExpandCitizenURLs(code string, templates Templates) []string {
	var urls []string
	for _, lang := range templates.Citizens {
		for _, tmpl := range lang.URLs {
			urls = append(urls, strings.ReplaceAll(tmpl, "[code]", code))
		}
	}
	return urls
}
