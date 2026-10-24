package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var languageCode = regexp.MustCompile(`^[a-z]{2,3}(?:-[a-z0-9]{2,8})*$`)

// TranslationSource fetches active language codes, falling back to the last successful response.
type TranslationSource struct {
	url       string
	cachePath string
	client    *http.Client
}

func NewTranslationSource(adminURL, cachePath string) *TranslationSource {
	return &TranslationSource{
		url:       strings.TrimRight(adminURL, "/") + "/api/translations?active=true",
		cachePath: cachePath,
		client:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *TranslationSource) Active(ctx context.Context) ([]string, error) {
	languages, err := s.fetch(ctx)
	if err == nil {
		if err := s.save(languages); err != nil {
			return nil, fmt.Errorf("cache active translations: %w", err)
		}
		return languages, nil
	}
	data, cacheErr := os.ReadFile(s.cachePath)
	if cacheErr != nil {
		return nil, fmt.Errorf("fetch active translations: %w; read cache: %v", err, cacheErr)
	}
	var cached []string
	if cacheErr = json.Unmarshal(data, &cached); cacheErr != nil {
		return nil, fmt.Errorf("fetch active translations: %w; decode cache: %v", err, cacheErr)
	}
	if cacheErr = validateLanguages(cached); cacheErr != nil {
		return nil, fmt.Errorf("fetch active translations: %w; invalid cache: %v", err, cacheErr)
	}
	return cached, nil
}

func (s *TranslationSource) fetch(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("admin translations returned HTTP %d", resp.StatusCode)
	}
	var payload struct {
		Data []struct {
			Code string `json:"code"`
		} `json:"data"`
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Data == nil {
		return nil, fmt.Errorf("admin translations response has no data array")
	}
	seen := make(map[string]bool)
	var languages []string
	for _, item := range payload.Data {
		code := strings.ToLower(strings.TrimSpace(item.Code))
		if !languageCode.MatchString(code) {
			return nil, fmt.Errorf("invalid translation code %q", item.Code)
		}
		if !seen[code] {
			languages = append(languages, code)
			seen[code] = true
		}
	}
	sort.Strings(languages)
	if languages == nil {
		languages = []string{}
	}
	return languages, nil
}

func (s *TranslationSource) save(languages []string) error {
	data, err := json.Marshal(languages)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.cachePath), 0o755); err != nil {
		return err
	}
	tmp := s.cachePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.cachePath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func validateLanguages(languages []string) error {
	for _, language := range languages {
		if !languageCode.MatchString(language) {
			return fmt.Errorf("invalid language %q", language)
		}
	}
	return nil
}
