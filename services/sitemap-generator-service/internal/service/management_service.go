package service

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"metarang/sitemap-generator-service/internal/models"
	"metarang/sitemap-generator-service/internal/repository"
)

type ManagementService struct {
	db      *repository.ManagementRepository
	writer  *FileWriter
	service *SitemapService
}

func NewManagementService(db *repository.ManagementRepository, writer *FileWriter, generator *SitemapService) *ManagementService {
	return &ManagementService{db: db, writer: writer, service: generator}
}

func (m *ManagementService) Generate(ctx context.Context, trigger, target, language string) (*models.GenerationRun, error) {
	if target == "" {
		target = "all"
	}
	target = normalizeTarget(target)
	run := repository.NewRun(trigger, target, language)
	var id int64
	if m.db != nil {
		id, _ = m.db.CreateRun(ctx, run)
		run.ID = id
	}
	started := time.Now()
	err := m.service.GenerateTarget(ctx, target, language)
	now := time.Now().UTC()
	run.FinishedAt = &now
	d := time.Since(started).Milliseconds()
	run.DurationMS = &d
	if err != nil {
		run.Status = "failed"
		run.Error = err.Error()
	} else {
		run.Status = "succeeded"
	}
	if m.db != nil && id != 0 {
		_ = m.db.CompleteRun(ctx, id, run)
	}
	if err != nil {
		return run, err
	}
	return run, nil
}

var numberedTarget = regexp.MustCompile(`^(citizen|user-wallet|user-lands|user-buildings|education-single-video|education-category|education-sub-category|calendar-events|calendar-versions)[0-9]+\.xml$`)

func normalizeTarget(target string) string {
	if target != "all" && target != "" && !strings.HasSuffix(target, ".xml") {
		target += ".xml"
	}
	if matches := numberedTarget.FindStringSubmatch(target); len(matches) == 2 {
		return matches[1] + ".xml"
	}
	return target
}

func (m *ManagementService) ListFiles() ([]models.SitemapFile, error) {
	items, err := m.writer.List()
	if err != nil {
		return nil, err
	}
	if m.db != nil {
		for _, item := range items {
			_ = m.db.UpsertFile(context.Background(), item)
		}
	}
	return items, nil
}

func (m *ManagementService) Upload(language, filename string, r io.Reader) (models.SitemapFile, error) {
	if len(language) < 2 || filepath.Base(language) != language {
		return models.SitemapFile{}, fmt.Errorf("invalid language")
	}
	if filepath.Base(filename) != filename || !strings.HasSuffix(strings.ToLower(filename), ".xml") || strings.Contains(filename, "..") {
		return models.SitemapFile{}, fmt.Errorf("invalid sitemap filename")
	}
	if generatedSitemap.MatchString(filename) {
		return models.SitemapFile{}, fmt.Errorf("dynamic sitemap filename is reserved")
	}
	data, err := io.ReadAll(io.LimitReader(r, 20<<20))
	if err != nil {
		return models.SitemapFile{}, fmt.Errorf("read sitemap upload: %w", err)
	}
	var root struct {
		XMLName  xml.Name   `xml:""`
		URLs     []struct{} `xml:"url"`
		Children []struct{} `xml:"sitemap"`
	}
	if err := xml.Unmarshal(data, &root); err != nil || (root.XMLName.Local != "urlset" && root.XMLName.Local != "sitemapindex") {
		return models.SitemapFile{}, fmt.Errorf("uploaded file is not a sitemap urlset")
	}
	if err := m.writer.Write(filepath.Join(language, filename), data); err != nil {
		return models.SitemapFile{}, err
	}
	count := len(root.URLs)
	if root.XMLName.Local == "sitemapindex" {
		count = len(root.Children)
	}
	item := models.SitemapFile{Language: language, Filename: filename, Type: "static", URLCount: count, Status: "available", UpdatedAt: time.Now().UTC()}
	if m.db != nil {
		_ = m.db.UpsertFile(context.Background(), item)
		_, _ = m.db.CreateRun(context.Background(), &models.GenerationRun{Trigger: "static_upload", Target: filename, Language: language, Status: "succeeded"})
	}
	return item, nil
}

func (m *ManagementService) Delete(language, filename string) error {
	if err := m.writer.Remove(filepath.Join(language, filename)); err != nil {
		return err
	}
	if m.db != nil {
		_, _ = m.db.CreateRun(context.Background(), &models.GenerationRun{Trigger: "static_delete", Target: filename, Language: language, Status: "succeeded"})
	}
	return nil
}

func (m *ManagementService) Logs(ctx context.Context, limit int) ([]models.GenerationRun, error) {
	if m.db == nil {
		return []models.GenerationRun{}, nil
	}
	return m.db.Logs(ctx, limit)
}

func (m *ManagementService) LogsPage(ctx context.Context, page, perPage int) ([]models.GenerationRun, int, error) {
	if m.db == nil {
		return []models.GenerationRun{}, 0, nil
	}
	return m.db.LogsPage(ctx, page, perPage)
}
