package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"metarang/shared/pkg/logger"
	"metarang/sitemap-generator-service/internal/models"
	"metarang/sitemap-generator-service/internal/repository"
	"metarang/sitemap-generator-service/internal/sitemap"
)

// ErrGenerationInProgress is returned when GenerateAll is already running.
var ErrGenerationInProgress = errors.New("sitemap generation already in progress")

// DefaultGenerationTimeout bounds a single GenerateAll run.
const DefaultGenerationTimeout = 30 * time.Minute

// MaxURLsPerFile is the sitemap protocol soft cap used for non-citizen files.
const MaxURLsPerFile = 50000

// Repository loads sitemap source rows in keyset pages.
type Repository interface {
	ListUsers(ctx context.Context, afterID uint64, limit int) ([]models.UserRow, error)
	ListVideos(ctx context.Context, afterID uint64, limit int) ([]models.VideoRow, error)
	ListCategories(ctx context.Context, afterID uint64, limit int) ([]models.CategoryRow, error)
	ListSubCategories(ctx context.Context, afterID uint64, limit int) ([]models.SubCategoryRow, error)
	ListEvents(ctx context.Context, afterID uint64, limit int) ([]models.CalendarRow, error)
	ListVersions(ctx context.Context, afterID uint64, limit int) ([]models.CalendarRow, error)
}

// Writer persists and removes generated sitemap XML files.
type Writer interface {
	Write(filename string, body []byte) error
	Remove(filename string) error
}

// Config holds generator settings.
type Config struct {
	TemplatesPath     string
	AdminPanelURL     string
	CitizenMaxURLs    int
	PageSize          int
	GenerationTimeout time.Duration
}

// SitemapService generates and exports sitemap XML files.
type SitemapService struct {
	repo   Repository
	writer Writer
	cfg    Config
	log    *logger.Logger
	mu     sync.Mutex
}

// NewSitemapService constructs a SitemapService.
func NewSitemapService(repo Repository, writer Writer, cfg Config) *SitemapService {
	if cfg.CitizenMaxURLs <= 0 {
		cfg.CitizenMaxURLs = sitemap.CitizenMaxURLs
	}
	if cfg.AdminPanelURL == "" {
		cfg.AdminPanelURL = "https://admin.metarang.com"
	}
	if cfg.PageSize <= 0 {
		cfg.PageSize = repository.DefaultPageSize
	}
	if cfg.GenerationTimeout <= 0 {
		cfg.GenerationTimeout = DefaultGenerationTimeout
	}
	return &SitemapService{repo: repo, writer: writer, cfg: cfg}
}

// WithLogger attaches a logger for scheduler error reporting.
func (s *SitemapService) WithLogger(log *logger.Logger) *SitemapService {
	s.log = log
	return s
}

// GenerateAll builds every sitemap type and writes them via Writer.
// Concurrent calls return ErrGenerationInProgress.
func (s *SitemapService) GenerateAll(ctx context.Context) error {
	if !s.mu.TryLock() {
		return ErrGenerationInProgress
	}
	defer s.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, s.cfg.GenerationTimeout)
	defer cancel()

	if err := s.generateCitizens(ctx); err != nil {
		return fmt.Errorf("citizens: %w", err)
	}
	if err := s.generateUserWallets(ctx); err != nil {
		return fmt.Errorf("user_wallets: %w", err)
	}
	if err := s.generateUserLands(ctx); err != nil {
		return fmt.Errorf("user_lands: %w", err)
	}
	if err := s.generateUserBuildings(ctx); err != nil {
		return fmt.Errorf("user_buildings: %w", err)
	}
	if err := s.generateVideos(ctx); err != nil {
		return fmt.Errorf("videos: %w", err)
	}
	if err := s.generateCategories(ctx); err != nil {
		return fmt.Errorf("categories: %w", err)
	}
	if err := s.generateSubCategories(ctx); err != nil {
		return fmt.Errorf("sub_categories: %w", err)
	}
	if err := s.generateEvents(ctx); err != nil {
		return fmt.Errorf("events: %w", err)
	}
	if err := s.generateVersions(ctx); err != nil {
		return fmt.Errorf("versions: %w", err)
	}
	return nil
}

func (s *SitemapService) generateCitizens(ctx context.Context) error {
	templates, err := sitemap.LoadTemplates(s.cfg.TemplatesPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s.cleanupCitizenFiles(0)
		}
		return err
	}
	if len(templates.Citizens) == 0 {
		return s.cleanupCitizenFiles(0)
	}

	entries := make([]models.URLEntry, 0, s.cfg.CitizenMaxURLs)
	chunkIndex := 0
	var afterID uint64

	flush := func() error {
		if len(entries) == 0 {
			return nil
		}
		body, err := sitemap.RenderURLSet(entries)
		if err != nil {
			return err
		}
		if err := s.writer.Write(sitemap.CitizenFileName(chunkIndex), body); err != nil {
			return err
		}
		chunkIndex++
		entries = entries[:0]
		return nil
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		users, err := s.repo.ListUsers(ctx, afterID, s.cfg.PageSize)
		if err != nil {
			return err
		}
		if len(users) == 0 {
			break
		}
		for _, u := range users {
			for _, loc := range sitemap.ExpandCitizenURLs(templates.Citizens, u.Code) {
				entries = append(entries, models.URLEntry{
					Loc:        loc,
					LastMod:    u.UpdatedAt,
					ChangeFreq: "daily",
					Priority:   "0.8",
				})
				if len(entries) >= s.cfg.CitizenMaxURLs {
					if err := flush(); err != nil {
						return err
					}
				}
			}
			afterID = u.ID
		}
		if len(users) < s.cfg.PageSize {
			break
		}
	}
	if err := flush(); err != nil {
		return err
	}
	return s.cleanupCitizenFiles(chunkIndex)
}

func (s *SitemapService) cleanupCitizenFiles(keepCount int) error {
	return s.cleanupChunkedFiles("citizen-sitemap.xml", keepCount)
}

func (s *SitemapService) generateUserWallets(ctx context.Context) error {
	return s.generateUserCodeSitemaps(ctx, "user-wallet-sitemaps.xml", sitemap.UserWalletURLs)
}

func (s *SitemapService) generateUserLands(ctx context.Context) error {
	return s.generateUserCodeSitemaps(ctx, "user-lands-sitemap.xml", sitemap.UserLandsURLs)
}

func (s *SitemapService) generateUserBuildings(ctx context.Context) error {
	return s.generateUserCodeSitemaps(ctx, "user-buildings-sitemap.xml", sitemap.UserBuildingsURLs)
}

func (s *SitemapService) generateUserCodeSitemaps(ctx context.Context, baseFile string, urlsForCode func(code string) []string) error {
	entries := make([]models.URLEntry, 0, s.cfg.CitizenMaxURLs)
	chunkIndex := 0
	var afterID uint64

	flush := func() error {
		if len(entries) == 0 {
			return nil
		}
		body, err := sitemap.RenderURLSet(entries)
		if err != nil {
			return err
		}
		if err := s.writer.Write(sitemap.ChunkedFileName(baseFile, chunkIndex), body); err != nil {
			return err
		}
		chunkIndex++
		entries = entries[:0]
		return nil
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		users, err := s.repo.ListUsers(ctx, afterID, s.cfg.PageSize)
		if err != nil {
			return err
		}
		if len(users) == 0 {
			break
		}
		for _, u := range users {
			for _, loc := range urlsForCode(u.Code) {
				entries = append(entries, models.URLEntry{
					Loc:        loc,
					LastMod:    u.UpdatedAt,
					ChangeFreq: "daily",
					Priority:   "0.8",
				})
				if len(entries) >= s.cfg.CitizenMaxURLs {
					if err := flush(); err != nil {
						return err
					}
				}
			}
			afterID = u.ID
		}
		if len(users) < s.cfg.PageSize {
			break
		}
	}
	if err := flush(); err != nil {
		return err
	}
	if chunkIndex == 0 {
		if err := s.writeFile(baseFile, nil); err != nil {
			return err
		}
		chunkIndex = 1
	}
	return s.cleanupChunkedFiles(baseFile, chunkIndex)
}

func (s *SitemapService) cleanupChunkedFiles(baseFile string, keepCount int) error {
	for i := keepCount; i < keepCount+100; i++ {
		if err := s.writer.Remove(sitemap.ChunkedFileName(baseFile, i)); err != nil {
			return err
		}
	}
	return nil
}

func (s *SitemapService) generateVideos(ctx context.Context) error {
	return s.writePaged(ctx, "education-single-video-sitemap.xml", MaxURLsPerFile, func(ctx context.Context, afterID uint64, limit int) ([]models.URLEntry, uint64, int, error) {
		videos, err := s.repo.ListVideos(ctx, afterID, limit)
		if err != nil {
			return nil, afterID, 0, err
		}
		var entries []models.URLEntry
		var lastID uint64
		for _, v := range videos {
			for _, loc := range sitemap.VideoURLs(v.Slug) {
				entries = append(entries, models.URLEntry{
					Loc: loc, LastMod: v.UpdatedAt, ChangeFreq: "monthly", Priority: "0.8",
				})
			}
			lastID = v.ID
		}
		return entries, lastID, len(videos), nil
	})
}

func (s *SitemapService) generateCategories(ctx context.Context) error {
	return s.writePaged(ctx, "education-category-sitemap.xml", MaxURLsPerFile, func(ctx context.Context, afterID uint64, limit int) ([]models.URLEntry, uint64, int, error) {
		cats, err := s.repo.ListCategories(ctx, afterID, limit)
		if err != nil {
			return nil, afterID, 0, err
		}
		var entries []models.URLEntry
		var lastID uint64
		for _, c := range cats {
			imageLoc := ""
			if c.Image != "" {
				imageLoc = sitemap.ImageLoc(s.cfg.AdminPanelURL, c.Image)
			}
			for _, loc := range sitemap.CategoryURLs(c.Slug) {
				entries = append(entries, models.URLEntry{
					Loc: loc, LastMod: c.UpdatedAt, ChangeFreq: "monthly", Priority: "0.8", ImageLoc: imageLoc,
				})
			}
			lastID = c.ID
		}
		return entries, lastID, len(cats), nil
	})
}

func (s *SitemapService) generateSubCategories(ctx context.Context) error {
	return s.writePaged(ctx, "education-sub-category-sitemap.xml", MaxURLsPerFile, func(ctx context.Context, afterID uint64, limit int) ([]models.URLEntry, uint64, int, error) {
		subs, err := s.repo.ListSubCategories(ctx, afterID, limit)
		if err != nil {
			return nil, afterID, 0, err
		}
		var entries []models.URLEntry
		var lastID uint64
		for _, sc := range subs {
			imageLoc := ""
			if sc.Image != "" {
				imageLoc = sitemap.ImageLoc(s.cfg.AdminPanelURL, sc.Image)
			}
			for _, loc := range sitemap.SubCategoryURLs(sc.CategorySlug, sc.Slug) {
				entries = append(entries, models.URLEntry{
					Loc: loc, LastMod: sc.UpdatedAt, ChangeFreq: "monthly", Priority: "0.8", ImageLoc: imageLoc,
				})
			}
			lastID = sc.ID
		}
		return entries, lastID, len(subs), nil
	})
}

func (s *SitemapService) generateEvents(ctx context.Context) error {
	return s.writePaged(ctx, "calendar-events-sitemap.xml", MaxURLsPerFile, func(ctx context.Context, afterID uint64, limit int) ([]models.URLEntry, uint64, int, error) {
		events, err := s.repo.ListEvents(ctx, afterID, limit)
		if err != nil {
			return nil, afterID, 0, err
		}
		var entries []models.URLEntry
		var lastID uint64
		for _, e := range events {
			for _, loc := range sitemap.EventURLs(e.ID) {
				entries = append(entries, models.URLEntry{
					Loc: loc, LastMod: e.UpdatedAt, ChangeFreq: "monthly", Priority: "0.6",
				})
			}
			lastID = e.ID
		}
		return entries, lastID, len(events), nil
	})
}

func (s *SitemapService) generateVersions(ctx context.Context) error {
	return s.writePaged(ctx, "calendar-versions-sitemap.xml", MaxURLsPerFile, func(ctx context.Context, afterID uint64, limit int) ([]models.URLEntry, uint64, int, error) {
		versions, err := s.repo.ListVersions(ctx, afterID, limit)
		if err != nil {
			return nil, afterID, 0, err
		}
		var entries []models.URLEntry
		var lastID uint64
		for _, v := range versions {
			lastID = v.ID
			if v.VersionTitle == "" {
				continue
			}
			for _, loc := range sitemap.VersionURLs(v.VersionTitle) {
				entries = append(entries, models.URLEntry{
					Loc: loc, LastMod: v.UpdatedAt, ChangeFreq: "monthly", Priority: "0.6",
				})
			}
		}
		return entries, lastID, len(versions), nil
	})
}

type pageLoader func(ctx context.Context, afterID uint64, limit int) (entries []models.URLEntry, lastID uint64, fetched int, err error)

// writePaged streams DB pages into URL entries and writes a single file
// (or numbered parts when exceeding maxPerFile).
func (s *SitemapService) writePaged(ctx context.Context, baseName string, maxPerFile int, load pageLoader) error {
	entries := make([]models.URLEntry, 0, s.cfg.PageSize*2)
	part := 0
	var afterID uint64
	wroteAny := false

	flush := func() error {
		if len(entries) == 0 {
			return nil
		}
		filename := baseName
		if part > 0 {
			filename = numberedFilename(baseName, part+1)
		}
		body, err := sitemap.RenderURLSet(entries)
		if err != nil {
			return err
		}
		if err := s.writer.Write(filename, body); err != nil {
			return err
		}
		wroteAny = true
		part++
		entries = entries[:0]
		return nil
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		page, lastID, fetched, err := load(ctx, afterID, s.cfg.PageSize)
		if err != nil {
			return err
		}
		if fetched == 0 {
			break
		}
		for _, e := range page {
			entries = append(entries, e)
			if maxPerFile > 0 && len(entries) >= maxPerFile {
				if err := flush(); err != nil {
					return err
				}
			}
		}
		afterID = lastID
		if fetched < s.cfg.PageSize {
			break
		}
	}
	if err := flush(); err != nil {
		return err
	}
	if !wroteAny {
		return s.writeFile(baseName, nil)
	}
	return nil
}

func numberedFilename(base string, n int) string {
	if len(base) > 4 && base[len(base)-4:] == ".xml" {
		return fmt.Sprintf("%s-%d.xml", base[:len(base)-4], n)
	}
	return fmt.Sprintf("%s-%d", base, n)
}

func (s *SitemapService) writeFile(filename string, entries []models.URLEntry) error {
	body, err := sitemap.RenderURLSet(entries)
	if err != nil {
		return err
	}
	return s.writer.Write(filename, body)
}

// StartScheduler runs GenerateAll immediately and then every interval until ctx is cancelled.
// onRun is an optional hook invoked after each successful or attempted run (used by tests).
func (s *SitemapService) StartScheduler(ctx context.Context, interval time.Duration, onRun func()) {
	if interval <= 0 {
		interval = 3 * time.Hour
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	if s.log != nil {
		s.log.Info("Sitemap scheduler started", "interval", interval.String())
	}

	run := func() {
		if err := s.GenerateAll(ctx); err != nil {
			if errors.Is(err, ErrGenerationInProgress) {
				if s.log != nil {
					s.log.Warn("Sitemap generation skipped; already in progress")
				}
			} else if s.log != nil {
				s.log.Error("Sitemap generation failed", "error", err)
			}
		} else if s.log != nil {
			s.log.Info("Sitemap generation completed")
		}
		if onRun != nil {
			onRun()
		}
	}

	run()
	for {
		select {
		case <-ctx.Done():
			if s.log != nil {
				s.log.Info("Sitemap scheduler stopped")
			}
			return
		case <-ticker.C:
			run()
		}
	}
}
