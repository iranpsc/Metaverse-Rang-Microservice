package sitemap

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	fileCitizen          = "citizen-sitemap.xml"
	fileVideos           = "education_single_video-sitemap.xml"
	fileCategories       = "education_category-sitemap.xml"
	fileSubCategories    = "education_sub_category-sitemap.xml"
	fileCalendarEvents   = "calendar_events-sitemap.xml"
	fileCalendarVersions = "calendar_versions-sitemap.xml"
	defaultAdminPanelURL = "https://admin.rgb.irpsc.com"
)

// Options controls one generation run.
type Options struct {
	OutputDir      string
	TemplatesPath  string
	AdminPanelURL  string
	MaxURLsPerFile int
	Templates      *Templates
}

// Result reports files written by a run.
type Result struct {
	Files []string
}

// Generate reads source rows and writes sitemap files into OutputDir.
func Generate(ctx context.Context, source Source, opts Options) (Result, error) {
	if source == nil {
		return Result{}, fmt.Errorf("sitemap source is nil")
	}
	if opts.OutputDir == "" {
		return Result{}, fmt.Errorf("sitemap output directory is empty")
	}
	if opts.MaxURLsPerFile <= 0 {
		opts.MaxURLsPerFile = MaxURLsPerFile
	}
	admin := strings.TrimRight(opts.AdminPanelURL, "/")
	if admin == "" {
		admin = defaultAdminPanelURL
	}

	templates, err := resolveTemplates(opts)
	if err != nil {
		return Result{}, err
	}

	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create sitemap directory: %w", err)
	}

	var written []string
	if templates.HasCitizens {
		citizenFiles, err := writeCitizens(ctx, source, opts, templates)
		if err != nil {
			return Result{}, err
		}
		written = append(written, citizenFiles...)
		if err := removeStaleCitizenFiles(opts.OutputDir, citizenFiles); err != nil {
			return Result{}, err
		}
	} else {
		log.Printf("templates.json has no citizens key; leaving citizen sitemaps unchanged")
	}

	videos, err := source.Videos(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("load videos: %w", err)
	}
	if err := writeNamed(opts.OutputDir, fileVideos, videoEntries(videos)); err != nil {
		return Result{}, err
	}
	written = append(written, fileVideos)

	categories, err := source.Categories(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("load categories: %w", err)
	}
	if err := writeNamed(opts.OutputDir, fileCategories, categoryEntries(categories, admin)); err != nil {
		return Result{}, err
	}
	written = append(written, fileCategories)

	subs, err := source.SubCategories(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("load subcategories: %w", err)
	}
	if err := writeNamed(opts.OutputDir, fileSubCategories, subCategoryEntries(subs, admin)); err != nil {
		return Result{}, err
	}
	written = append(written, fileSubCategories)

	events, err := source.CalendarEvents(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("load calendar events: %w", err)
	}
	if err := writeNamed(opts.OutputDir, fileCalendarEvents, eventEntries(events)); err != nil {
		return Result{}, err
	}
	written = append(written, fileCalendarEvents)

	versions, err := source.CalendarVersions(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("load calendar versions: %w", err)
	}
	if err := writeNamed(opts.OutputDir, fileCalendarVersions, versionEntries(versions)); err != nil {
		return Result{}, err
	}
	written = append(written, fileCalendarVersions)

	log.Printf("wrote %d sitemap file(s) to %s", len(written), opts.OutputDir)
	return Result{Files: written}, nil
}

func resolveTemplates(opts Options) (Templates, error) {
	if opts.Templates != nil {
		return *opts.Templates, nil
	}
	if opts.TemplatesPath == "" {
		return Templates{}, fmt.Errorf("templates path is empty")
	}
	templates, err := LoadTemplates(opts.TemplatesPath)
	if err != nil {
		return Templates{}, err
	}
	return templates, nil
}

func writeCitizens(ctx context.Context, source Source, opts Options, templates Templates) ([]string, error) {
	users, err := source.Users(ctx)
	if err != nil {
		return nil, fmt.Errorf("load users: %w", err)
	}

	var files []string
	var batch []Entry
	fileIndex := 1
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		name := citizenFileName(fileIndex)
		if err := writeNamed(opts.OutputDir, name, batch); err != nil {
			return err
		}
		files = append(files, name)
		batch = batch[:0]
		fileIndex++
		return nil
	}

	for _, user := range users {
		if strings.TrimSpace(user.Code) == "" {
			continue
		}
		for _, loc := range ExpandCitizenURLs(user.Code, templates) {
			if strings.TrimSpace(loc) == "" {
				continue
			}
			batch = append(batch, Entry{
				Loc:        loc,
				LastMod:    user.UpdatedAt,
				HasLastMod: user.HasUpdated,
				ChangeFreq: ChangeFreqDaily,
				Priority:   PriorityCitizen,
			})
			if len(batch) >= opts.MaxURLsPerFile {
				if err := flush(); err != nil {
					return nil, err
				}
			}
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return files, nil
}

func citizenFileName(index int) string {
	if index <= 1 {
		return fileCitizen
	}
	return fmt.Sprintf("citizen-sitemap-%d.xml", index)
}

func removeStaleCitizenFiles(dir string, keep []string) error {
	keepSet := make(map[string]struct{}, len(keep))
	for _, name := range keep {
		keepSet[name] = struct{}{}
	}
	matches, err := filepath.Glob(filepath.Join(dir, "citizen-sitemap*.xml"))
	if err != nil {
		return fmt.Errorf("list citizen sitemaps: %w", err)
	}
	for _, path := range matches {
		name := filepath.Base(path)
		if _, ok := keepSet[name]; ok {
			continue
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove stale sitemap %s: %w", name, err)
		}
		log.Printf("removed stale sitemap %s", name)
	}
	return nil
}

func writeNamed(dir, name string, entries []Entry) error {
	path := filepath.Join(dir, name)
	if err := WriteFileAtomic(path, Render(entries)); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	log.Printf("wrote %s (%d urls)", name, len(entries))
	return nil
}

func videoEntries(videos []Video) []Entry {
	var entries []Entry
	for _, video := range videos {
		if strings.TrimSpace(video.Slug) == "" {
			continue
		}
		for _, lang := range []string{"fa", "en"} {
			entries = append(entries, Entry{
				Loc:        fmt.Sprintf("https://rgb.irpsc.com/%s/education/watch/%s", lang, video.Slug),
				LastMod:    video.UpdatedAt,
				HasLastMod: video.HasUpdated,
				ChangeFreq: ChangeFreqMonthly,
				Priority:   PriorityVideo,
			})
		}
	}
	return entries
}

func categoryEntries(categories []Category, admin string) []Entry {
	var entries []Entry
	for _, category := range categories {
		if strings.TrimSpace(category.Slug) == "" {
			continue
		}
		images := imageLocs(admin, category.Image)
		for _, lang := range []string{"fa", "en"} {
			entries = append(entries, Entry{
				Loc:        fmt.Sprintf("https://rgb.irpsc.com/%s/education/category/%s", lang, category.Slug),
				LastMod:    category.UpdatedAt,
				HasLastMod: category.HasUpdated,
				ChangeFreq: ChangeFreqMonthly,
				Priority:   PriorityVideo,
				Images:     images,
			})
		}
	}
	return entries
}

func subCategoryEntries(subs []SubCategory, admin string) []Entry {
	var entries []Entry
	for _, sub := range subs {
		if strings.TrimSpace(sub.Slug) == "" || strings.TrimSpace(sub.CategorySlug) == "" {
			continue
		}
		images := imageLocs(admin, sub.Image)
		for _, lang := range []string{"fa", "en"} {
			entries = append(entries, Entry{
				Loc:        fmt.Sprintf("https://rgb.irpsc.com/%s/education/category/%s/%s", lang, sub.CategorySlug, sub.Slug),
				LastMod:    sub.UpdatedAt,
				HasLastMod: sub.HasUpdated,
				ChangeFreq: ChangeFreqMonthly,
				Priority:   PriorityVideo,
				Images:     images,
			})
		}
	}
	return entries
}

func eventEntries(events []CalendarEvent) []Entry {
	var entries []Entry
	for _, event := range events {
		for _, lang := range []string{"fa", "en"} {
			entries = append(entries, Entry{
				Loc:        fmt.Sprintf("https://metarang.com/%s/calendar/%d", lang, event.ID),
				LastMod:    event.UpdatedAt,
				HasLastMod: event.HasUpdated,
				ChangeFreq: ChangeFreqMonthly,
				Priority:   PriorityCalendar,
			})
		}
	}
	return entries
}

func versionEntries(versions []CalendarVersion) []Entry {
	var entries []Entry
	for _, version := range versions {
		title := strings.TrimSpace(version.VersionTitle)
		if title == "" {
			continue
		}
		for _, lang := range []string{"fa", "en"} {
			entries = append(entries, Entry{
				Loc:        fmt.Sprintf("https://metarang.com/%s/versions/%s", lang, version.VersionTitle),
				LastMod:    version.UpdatedAt,
				HasLastMod: version.HasUpdated,
				ChangeFreq: ChangeFreqMonthly,
				Priority:   PriorityCalendar,
			})
		}
	}
	return entries
}

func imageLocs(admin, image string) []string {
	image = strings.TrimSpace(image)
	if image == "" {
		return nil
	}
	return []string{admin + "/uploads/" + image}
}
