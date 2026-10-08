package service

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	authpb "metarang/shared/pb/auth"
	levelspb "metarang/shared/pb/levels"
	"metarang/shared/pkg/jalali"
)

// KYCPort fetches KYC status/details from auth-service.
type KYCPort interface {
	GetKYC(ctx context.Context, userID uint64) (*authpb.KYCResponse, error)
}

// LevelsPort fetches user levels and gems from levels-service.
type LevelsPort interface {
	GetUserLevel(ctx context.Context, userID uint64) (*levelspb.UserLevelResponse, error)
	GetLevelGem(ctx context.Context, levelID uint64) (*levelspb.LevelGem, error)
}

// UserSearchService provides user search functionality
type UserSearchService struct {
	db     *sql.DB
	kyc    KYCPort
	levels LevelsPort
}

func NewUserSearchService(db *sql.DB, kyc KYCPort, levels LevelsPort) *UserSearchService {
	return &UserSearchService{
		db:     db,
		kyc:    kyc,
		levels: levels,
	}
}

// UserSearchLevelGem is the compact gem payload for search results.
type UserSearchLevelGem struct {
	ID    uint64 `json:"id"`
	Name  string `json:"name"`
	Image string `json:"image"`
}

// UserSearchLevelItem is one achieved level card in search results.
type UserSearchLevelItem struct {
	ID   uint64              `json:"id"`
	Slug string              `json:"slug"`
	Gem  *UserSearchLevelGem `json:"gem"`
}

// UserSearchResult represents a search result
type UserSearchResult struct {
	ID          uint64                 `json:"id"`
	Code        string                 `json:"code"`
	Name        string                 `json:"name"`
	DisplayName string                 `json:"-"` // Internal use
	Image       *string                `json:"image"`
	Level       string                 `json:"level"`
	Verified    bool                   `json:"verified"`
	Age         int32                  `json:"age"`
	Levels      []*UserSearchLevelItem `json:"levels"`
}

// SearchUsers searches for users by code or name
func (s *UserSearchService) SearchUsers(
	ctx context.Context,
	searchTerm string,
	limit int,
) ([]*UserSearchResult, error) {
	query := `
		SELECT u.id, u.code, u.name
		FROM users u
		LEFT JOIN kycs k ON k.user_id = u.id
		WHERE u.code LIKE ? OR u.name LIKE ? OR CONCAT(IFNULL(k.fname, ''), ' ', IFNULL(k.lname, '')) LIKE ?
		LIMIT ?
	`

	term := "%" + searchTerm + "%"
	rows, err := s.db.QueryContext(ctx, query, term, term, term, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var results []*UserSearchResult
	for rows.Next() {
		var r UserSearchResult

		if err := rows.Scan(&r.ID, &r.Code, &r.Name); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}

		r.Levels = []*UserSearchLevelItem{}

		photo, _ := s.getLatestProfilePhoto(ctx, r.ID)
		r.Image = photo

		s.enrichFromAuth(ctx, &r)
		s.enrichFromLevels(ctx, &r)

		results = append(results, &r)
	}

	return results, nil
}

func (s *UserSearchService) enrichFromAuth(ctx context.Context, r *UserSearchResult) {
	if s.kyc == nil {
		return
	}
	kyc, err := s.kyc.GetKYC(ctx, r.ID)
	if err != nil || kyc == nil {
		return
	}
	r.Verified = kyc.Status == 1
	if !r.Verified {
		r.Age = 0
		return
	}
	if fullName := strings.TrimSpace(kyc.Fname + " " + kyc.Lname); fullName != "" {
		r.Name = fullName
	}
	r.Age = ageFromKYCBirthdate(kyc.Birthdate)
}

func (s *UserSearchService) enrichFromLevels(ctx context.Context, r *UserSearchResult) {
	if s.levels == nil {
		return
	}
	resp, err := s.levels.GetUserLevel(ctx, r.ID)
	if err != nil || resp == nil {
		return
	}

	rawLevels := make([]*levelspb.Level, 0, 1+len(resp.PreviousLevels))
	if resp.LatestLevel != nil {
		rawLevels = append(rawLevels, resp.LatestLevel)
		r.Level = resp.LatestLevel.Name
	}
	rawLevels = append(rawLevels, resp.PreviousLevels...)

	items := make([]*UserSearchLevelItem, 0, len(rawLevels))
	seen := make(map[uint64]struct{}, len(rawLevels))
	for _, lvl := range rawLevels {
		if lvl == nil {
			continue
		}
		if _, ok := seen[lvl.Id]; ok {
			continue
		}
		seen[lvl.Id] = struct{}{}

		item := &UserSearchLevelItem{
			ID:   lvl.Id,
			Slug: lvl.Slug,
			Gem:  gemFromLevel(lvl),
		}
		if item.Gem == nil || item.Gem.ID == 0 || item.Gem.Name == "" {
			if gem, gerr := s.levels.GetLevelGem(ctx, lvl.Id); gerr == nil && gem != nil {
				item.Gem = mapGem(gem)
			}
		}
		items = append(items, item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		return slugGreater(items[i].Slug, items[j].Slug)
	})
	r.Levels = items
}

func gemFromLevel(lvl *levelspb.Level) *UserSearchLevelGem {
	if lvl == nil || lvl.Gem == nil {
		return nil
	}
	return mapGem(lvl.Gem)
}

func mapGem(gem *levelspb.LevelGem) *UserSearchLevelGem {
	if gem == nil {
		return nil
	}
	return &UserSearchLevelGem{
		ID:    gem.Id,
		Name:  gem.Name,
		Image: gem.PngFile,
	}
}

// slugGreater reports whether a should sort before b for descending slug order.
// Numeric slugs compare as integers; otherwise lexicographic.
func slugGreater(a, b string) bool {
	ai, aErr := strconv.Atoi(strings.TrimSpace(a))
	bi, bErr := strconv.Atoi(strings.TrimSpace(b))
	if aErr == nil && bErr == nil {
		return ai > bi
	}
	return a > b
}

// ageFromKYCBirthdate computes age in full years from a KYC birthdate string.
// Accepts Jalali Y/m/d or Gregorian YYYY-MM-DD.
func ageFromKYCBirthdate(birthdate string) int32 {
	birthdate = strings.TrimSpace(birthdate)
	if birthdate == "" {
		return 0
	}

	var birth time.Time
	var err error
	if strings.Contains(birthdate, "/") {
		birth, err = jalali.JalaliToCarbon(birthdate)
	} else {
		birth, err = time.Parse("2006-01-02", birthdate)
	}
	if err != nil {
		return 0
	}

	now := time.Now()
	age := now.Year() - birth.Year()
	anniversary := time.Date(now.Year(), birth.Month(), birth.Day(), 0, 0, 0, 0, time.UTC)
	if now.Before(anniversary) {
		age--
	}
	if age < 0 {
		return 0
	}
	return int32(age)
}

// getLatestProfilePhoto retrieves user's latest profile photo
func (s *UserSearchService) getLatestProfilePhoto(ctx context.Context, userID uint64) (*string, error) {
	query := `
		SELECT url FROM images 
		WHERE imageable_type = 'App\\Models\\User' 
		  AND imageable_id = ? 
		ORDER BY id DESC 
		LIMIT 1
	`

	var url string
	err := s.db.QueryRowContext(ctx, query, userID).Scan(&url)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &url, nil
}
