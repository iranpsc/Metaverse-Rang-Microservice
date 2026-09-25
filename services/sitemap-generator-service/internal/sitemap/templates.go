package sitemap

import (
	"encoding/json"
	"fmt"
	"os"
)

// Templates holds citizen URL templates keyed by language.
type Templates struct {
	Citizens map[string][]string `json:"citizens"`
}

// LoadTemplates reads templates.json from path.
func LoadTemplates(path string) (*Templates, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read templates: %w", err)
	}
	var t Templates
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	return &t, nil
}
