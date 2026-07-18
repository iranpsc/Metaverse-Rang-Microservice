package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	pb "metarang/shared/pb/levels"
	"metarang/shared/pkg/helpers"
)

// LevelRepository handles level database operations
// Implements Laravel's Level model and relationships
type LevelRepository struct {
	db            *sql.DB
	adminPanelURL string
}

type LevelRepositoryInterface interface {
	GetUserLatestLevel(ctx context.Context, userID uint64) (*pb.Level, error)
	GetLevelsBelowScore(ctx context.Context, score int32) ([]*pb.Level, error)
	GetNextLevel(ctx context.Context, currentScore int32) (*pb.Level, error)
	GetAllLevels(ctx context.Context) ([]*pb.Level, error)
	FindByID(ctx context.Context, id uint64) (*pb.Level, error)
	FindBySlug(ctx context.Context, slug string) (*pb.Level, error)
	GetLevelGeneralInfo(ctx context.Context, levelID uint64) (*pb.LevelGeneralInfo, error)
	GetLevelPrize(ctx context.Context, levelID uint64) (*pb.LevelPrize, error)
	GetLevelGem(ctx context.Context, levelID uint64) (*pb.LevelGem, error)
	GetLevelGift(ctx context.Context, levelID uint64) (*pb.LevelGift, error)
	GetLevelLicenses(ctx context.Context, levelID uint64) (*pb.LevelLicense, error)
	GetNextLevelForScore(ctx context.Context, userID uint64, score int32) (*pb.Level, error)
	AttachLevelToUser(ctx context.Context, userID, levelID uint64) error
	HasUserReceivedPrize(ctx context.Context, userID, prizeID uint64) (bool, error)
	RecordReceivedPrize(ctx context.Context, userID, prizeID uint64) error
	GetVariableRate(ctx context.Context, name string) (float64, error)
}

func NewLevelRepository(db *sql.DB, adminPanelURL string) *LevelRepository {
	return &LevelRepository{
		db:            db,
		adminPanelURL: strings.TrimSuffix(adminPanelURL, "/"),
	}
}

// formatImageURL formats image URL with admin_panel_url + /uploads/ prefix
// Implements Laravel: config('app.admin_panel_url') . '/uploads/' . $this->image->url
func (r *LevelRepository) formatImageURL(imageURL string) string {
	if imageURL == "" {
		return ""
	}
	// If already a full URL, return as-is
	if strings.HasPrefix(imageURL, "http://") || strings.HasPrefix(imageURL, "https://") {
		return imageURL
	}
	// If admin_panel_url is not configured, return relative path
	if r.adminPanelURL == "" {
		path := strings.TrimPrefix(imageURL, "/")
		if !strings.HasPrefix(path, "uploads/") {
			return "/uploads/" + path
		}
		return "/" + path
	}
	// Prefix with admin_panel_url/uploads/
	path := strings.TrimPrefix(imageURL, "/")
	if !strings.HasPrefix(path, "uploads/") {
		path = "uploads/" + path
	}
	return r.adminPanelURL + "/" + path
}

// formatJalaliDateTime formats time.Time to Jalali format Y/m/d H:i:s
// Implements Laravel: jdate($this->created_at)->format('Y/m/d H:i:s')
func (r *LevelRepository) formatJalaliDateTime(t time.Time) string {
	return helpers.FormatJalaliDateTime(t)
}

// GetUserLatestLevel retrieves user's latest achieved level
// Implements Laravel: $user->levels()->orderByDesc('id')->first()
func (r *LevelRepository) GetUserLatestLevel(ctx context.Context, userID uint64) (*pb.Level, error) {
	query := `
		SELECT l.id, l.name, l.slug, CAST(l.score AS UNSIGNED) as score, l.background_image,
		       COALESCE(i.url, '') as image_url,
		       COALESCE(lg.fbx_file, '') as gem_fbx_file
		FROM levels l
		INNER JOIN level_user lu ON l.id = lu.level_id
		LEFT JOIN images i ON i.imageable_id = l.id AND i.imageable_type = 'App\\Models\\Levels\\Level'
		LEFT JOIN level_gems lg ON lg.level_id = l.id
		WHERE lu.user_id = ?
		ORDER BY l.id DESC
		LIMIT 1
	`

	var level pb.Level
	var imageURL sql.NullString
	var backgroundImage sql.NullString
	var gemFbxFile string

	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&level.Id,
		&level.Name,
		&level.Slug,
		&level.Score,
		&backgroundImage,
		&imageURL,
		&gemFbxFile,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user has no level")
		}
		return nil, err
	}

	if imageURL.Valid && imageURL.String != "" {
		level.ImageUrl = r.formatImageURL(imageURL.String)
	}
	if backgroundImage.Valid {
		level.BackgroundImage = backgroundImage.String
	}

	level.Gem = &pb.LevelGem{
		LevelId: level.Id,
		FbxFile: gemFbxFile,
	}

	return &level, nil
}

// GetLevelsBelowScore retrieves all levels with score less than given score
// Implements Laravel: Level::where('score', '<', $score)->orderBy('score')->get()
func (r *LevelRepository) GetLevelsBelowScore(ctx context.Context, score int32) ([]*pb.Level, error) {
	query := `
		SELECT l.id, l.name, l.slug, CAST(l.score AS UNSIGNED) as score, l.background_image,
		       COALESCE(i.url, '') as image_url
		FROM levels l
		LEFT JOIN images i ON i.imageable_id = l.id AND i.imageable_type = 'App\\Models\\Levels\\Level'
		WHERE CAST(l.score AS UNSIGNED) < ?
		ORDER BY CAST(l.score AS UNSIGNED) ASC
	`

	rows, err := r.db.QueryContext(ctx, query, score)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var levels []*pb.Level
	for rows.Next() {
		var level pb.Level
		var imageURL sql.NullString
		var backgroundImage sql.NullString

		if err := rows.Scan(&level.Id, &level.Name, &level.Slug, &level.Score, &backgroundImage, &imageURL); err != nil {
			return nil, err
		}

		if imageURL.Valid && imageURL.String != "" {
			level.ImageUrl = r.formatImageURL(imageURL.String)
		}
		if backgroundImage.Valid {
			level.BackgroundImage = backgroundImage.String
		}

		levels = append(levels, &level)
	}

	return levels, nil
}

// GetNextLevel retrieves the next level above current score
// Used for calculating progress percentage
func (r *LevelRepository) GetNextLevel(ctx context.Context, currentScore int32) (*pb.Level, error) {
	query := `
		SELECT l.id, l.name, l.slug, CAST(l.score AS UNSIGNED) as score, l.background_image,
		       COALESCE(i.url, '') as image_url
		FROM levels l
		LEFT JOIN images i ON i.imageable_id = l.id AND i.imageable_type = 'App\\Models\\Levels\\Level'
		WHERE CAST(l.score AS UNSIGNED) > ?
		ORDER BY CAST(l.score AS UNSIGNED) ASC
		LIMIT 1
	`

	var level pb.Level
	var imageURL sql.NullString
	var backgroundImage sql.NullString

	err := r.db.QueryRowContext(ctx, query, currentScore).Scan(
		&level.Id,
		&level.Name,
		&level.Slug,
		&level.Score,
		&backgroundImage,
		&imageURL,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No next level
		}
		return nil, err
	}

	if imageURL.Valid && imageURL.String != "" {
		level.ImageUrl = r.formatImageURL(imageURL.String)
	}
	if backgroundImage.Valid {
		level.BackgroundImage = backgroundImage.String
	}

	return &level, nil
}

// GetAllLevels retrieves all levels
// Implements Laravel: Level::select('id', 'name', 'slug')->with('image')->get()
func (r *LevelRepository) GetAllLevels(ctx context.Context) ([]*pb.Level, error) {
	query := `
		SELECT l.id, l.name, l.slug, CAST(l.score AS UNSIGNED) as score, l.background_image,
		       COALESCE(i.url, '') as image_url
		FROM levels l
		LEFT JOIN images i ON i.imageable_id = l.id AND i.imageable_type = 'App\\Models\\Levels\\Level'
		ORDER BY CAST(l.score AS UNSIGNED) ASC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var levels []*pb.Level
	for rows.Next() {
		var level pb.Level
		var imageURL sql.NullString
		var backgroundImage sql.NullString

		if err := rows.Scan(&level.Id, &level.Name, &level.Slug, &level.Score, &backgroundImage, &imageURL); err != nil {
			return nil, err
		}

		if imageURL.Valid && imageURL.String != "" {
			level.ImageUrl = r.formatImageURL(imageURL.String)
		}
		if backgroundImage.Valid {
			level.BackgroundImage = backgroundImage.String
		}

		levels = append(levels, &level)
	}

	return levels, nil
}

// FindByID retrieves a level by ID with all relationships
// Implements Laravel: Level::find($id)->load('image', 'generalInfo')
func (r *LevelRepository) FindByID(ctx context.Context, id uint64) (*pb.Level, error) {
	query := `
		SELECT l.id, l.name, l.slug, CAST(l.score AS UNSIGNED) as score, l.background_image,
		       COALESCE(i.url, '') as image_url
		FROM levels l
		LEFT JOIN images i ON i.imageable_id = l.id AND i.imageable_type = 'App\\Models\\Levels\\Level'
		WHERE l.id = ?
	`

	var level pb.Level
	var imageURL sql.NullString
	var backgroundImage sql.NullString

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&level.Id,
		&level.Name,
		&level.Slug,
		&level.Score,
		&backgroundImage,
		&imageURL,
	)

	if err != nil {
		return nil, err
	}

	if imageURL.Valid && imageURL.String != "" {
		level.ImageUrl = r.formatImageURL(imageURL.String)
	}
	if backgroundImage.Valid {
		level.BackgroundImage = backgroundImage.String
	}

	// Load general info
	generalInfo, err := r.GetLevelGeneralInfo(ctx, id)
	if err == nil {
		level.GeneralInfo = generalInfo
	}

	return &level, nil
}

// FindBySlug retrieves a level by slug
// Implements Laravel: Level::where('slug', $slug)->firstOrFail()
func (r *LevelRepository) FindBySlug(ctx context.Context, slug string) (*pb.Level, error) {
	query := `
		SELECT l.id, l.name, l.slug, CAST(l.score AS UNSIGNED) as score, l.background_image,
		       COALESCE(i.url, '') as image_url
		FROM levels l
		LEFT JOIN images i ON i.imageable_id = l.id AND i.imageable_type = 'App\\Models\\Levels\\Level'
		WHERE l.slug = ?
	`

	var level pb.Level
	var imageURL sql.NullString
	var backgroundImage sql.NullString

	err := r.db.QueryRowContext(ctx, query, slug).Scan(
		&level.Id,
		&level.Name,
		&level.Slug,
		&level.Score,
		&backgroundImage,
		&imageURL,
	)

	if err != nil {
		return nil, err
	}

	if imageURL.Valid && imageURL.String != "" {
		level.ImageUrl = r.formatImageURL(imageURL.String)
	}
	if backgroundImage.Valid {
		level.BackgroundImage = backgroundImage.String
	}

	// Load general info
	generalInfo, err := r.GetLevelGeneralInfo(ctx, level.Id)
	if err == nil {
		level.GeneralInfo = generalInfo
	}

	return &level, nil
}

// GetLevelGeneralInfo retrieves general information for a level
// Implements Laravel: $level->generalInfo
func (r *LevelRepository) GetLevelGeneralInfo(ctx context.Context, levelID uint64) (*pb.LevelGeneralInfo, error) {
	query := `
		SELECT id, level_id, score, ` + "`rank`" + `, description, subcategories,
		       persian_font, english_font, file_volume, used_colors, points, ` + "`lines`" + `,
		       has_animation, designer, model_designer, creation_date,
		       COALESCE(png_file, '') as png_file,
		       COALESCE(fbx_file, '') as fbx_file,
		       COALESCE(gif_file, '') as gif_file
		FROM level_general_infos
		WHERE level_id = ?
	`

	var info pb.LevelGeneralInfo
	var rank sql.NullInt64
	var description, persianFont, englishFont, usedColors, designer, modelDesigner, creationDate sql.NullString
	var subcategories, points, lines sql.NullInt64
	var fileVolume sql.NullFloat64
	var hasAnimation sql.NullInt64

	err := r.db.QueryRowContext(ctx, query, levelID).Scan(
		&info.Id,
		&info.LevelId,
		&info.Score,
		&rank,
		&description,
		&subcategories,
		&persianFont,
		&englishFont,
		&fileVolume,
		&usedColors,
		&points,
		&lines,
		&hasAnimation,
		&designer,
		&modelDesigner,
		&creationDate,
		&info.PngFile,
		&info.FbxFile,
		&info.GifFile,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Missing general info is allowed per API docs
		}
		return nil, err
	}

	// Convert nullable fields to proto string/bool types
	if rank.Valid {
		info.Rank = fmt.Sprintf("%d", rank.Int64)
	}
	if description.Valid {
		info.Description = description.String
	}
	if subcategories.Valid {
		info.Subcategories = fmt.Sprintf("%d", subcategories.Int64)
	}
	if persianFont.Valid {
		info.PersianFont = persianFont.String
	}
	if englishFont.Valid {
		info.EnglishFont = englishFont.String
	}
	if fileVolume.Valid {
		info.FileVolume = fmt.Sprintf("%g", fileVolume.Float64)
	}
	if usedColors.Valid {
		info.UsedColors = usedColors.String
	}
	if points.Valid {
		info.Points = fmt.Sprintf("%d", points.Int64)
	}
	if lines.Valid {
		info.Lines = fmt.Sprintf("%d", lines.Int64)
	}
	info.HasAnimation = hasAnimation.Valid && hasAnimation.Int64 != 0
	if designer.Valid {
		info.Designer = designer.String
	}
	if modelDesigner.Valid {
		info.ModelDesigner = modelDesigner.String
	}
	if creationDate.Valid {
		info.CreationDate = creationDate.String
	}

	// Format file URLs with admin_panel_url prefix (for png_file, fbx_file, gif_file)
	if info.PngFile != "" {
		info.PngFile = r.formatImageURL(info.PngFile)
	}
	if info.FbxFile != "" {
		info.FbxFile = r.formatImageURL(info.FbxFile)
	}
	if info.GifFile != "" {
		info.GifFile = r.formatImageURL(info.GifFile)
	}

	return &info, nil
}

// GetLevelPrize retrieves prize information for a level
// Implements Laravel: $level->prize
func (r *LevelRepository) GetLevelPrize(ctx context.Context, levelID uint64) (*pb.LevelPrize, error) {
	query := `
		SELECT id, level_id, psc, blue, red, yellow, effect, satisfaction, created_at
		FROM level_prizes
		WHERE level_id = ?
	`

	var prize pb.LevelPrize
	var psc, blue, red, yellow, effect int64
	var satisfaction float64
	var createdAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, levelID).Scan(
		&prize.Id,
		&prize.LevelId,
		&psc,
		&blue,
		&red,
		&yellow,
		&effect,
		&satisfaction,
		&createdAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Missing prize is allowed per API docs
		}
		return nil, err
	}

	// Convert integers to strings for consistency with Laravel
	prize.Psc = fmt.Sprintf("%d", psc)
	prize.Blue = fmt.Sprintf("%d", blue)
	prize.Red = fmt.Sprintf("%d", red)
	prize.Yellow = fmt.Sprintf("%d", yellow)
	prize.Effect = effect

	// Format satisfaction to 2 decimal places as per API docs
	prize.Satisfaction = fmt.Sprintf("%.2f", satisfaction)

	// Format created_at in Jalali format Y/m/d H:i:s as per API docs
	if createdAt.Valid {
		prize.CreatedAt = r.formatJalaliDateTime(createdAt.Time)
	}

	return &prize, nil
}

// GetLevelGem retrieves gem information for a level
// Implements Laravel: $level->gem
func (r *LevelRepository) GetLevelGem(ctx context.Context, levelID uint64) (*pb.LevelGem, error) {
	query := `
		SELECT id, level_id, name, description, thread, points, volume, color,
		       has_animation, ` + "`lines`" + `, png_file, fbx_file, encryption, designer
		FROM level_gems
		WHERE level_id = ?
	`

	var gem pb.LevelGem
	var encryptionInt int8

	err := r.db.QueryRowContext(ctx, query, levelID).Scan(
		&gem.Id,
		&gem.LevelId,
		&gem.Name,
		&gem.Description,
		&gem.Thread,
		&gem.Points,
		&gem.Volume,
		&gem.Color,
		&gem.HasAnimation,
		&gem.Lines,
		&gem.PngFile,
		&gem.FbxFile,
		&encryptionInt,
		&gem.Designer,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Missing gem is allowed per API docs
		}
		return nil, err
	}

	gem.Encryption = encryptionInt != 0

	return &gem, nil
}

// GetLevelGift retrieves gift information for a level
// Implements Laravel: $level->gift
func (r *LevelRepository) GetLevelGift(ctx context.Context, levelID uint64) (*pb.LevelGift, error) {
	query := `
		SELECT id, level_id, name, description, monthly_capacity_count, store_capacity,
		       sell_capacity, features, sell, vod_document_registration, seller_link,
		       designer, three_d_model_volume, three_d_model_points, three_d_model_lines,
		       has_animation, png_file, fbx_file, gif_file, rent, vod_count,
		       start_vod_id, end_vod_id
		FROM level_gifts
		WHERE level_id = ?
	`

	var gift pb.LevelGift
	var storeCapacity, sellCapacity, sell, vodDocReg, hasAnim, rent int8

	err := r.db.QueryRowContext(ctx, query, levelID).Scan(
		&gift.Id,
		&gift.LevelId,
		&gift.Name,
		&gift.Description,
		&gift.MonthlyCapacityCount,
		&storeCapacity,
		&sellCapacity,
		&gift.Features,
		&sell,
		&vodDocReg,
		&gift.SellerLink,
		&gift.Designer,
		&gift.ThreeDModelVolume,
		&gift.ThreeDModelPoints,
		&gift.ThreeDModelLines,
		&hasAnim,
		&gift.PngFile,
		&gift.FbxFile,
		&gift.GifFile,
		&rent,
		&gift.VodCount,
		&gift.StartVodId,
		&gift.EndVodId,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Missing gift is allowed per API docs
		}
		return nil, err
	}

	gift.StoreCapacity = storeCapacity != 0
	gift.SellCapacity = sellCapacity != 0
	gift.Sell = sell != 0
	gift.VodDocumentRegistration = vodDocReg != 0
	gift.HasAnimation = hasAnim != 0
	gift.Rent = rent != 0

	return &gift, nil
}

// GetLevelLicenses retrieves license information for a level
// Implements Laravel: $level->licenses
func (r *LevelRepository) GetLevelLicenses(ctx context.Context, levelID uint64) (*pb.LevelLicense, error) {
	query := `
		SELECT id, level_id, create_union, add_memeber_to_union, observation_license,
		       gate_license, lawyer_license, city_counsile_entry,
		       establish_special_residential_property, establish_property_on_surface,
		       judge_entry, upload_image, delete_image, inter_level_general_points,
		       inter_level_special_points, rent_out_satisfaction,
		       access_to_answer_questions_unit, create_challenge_questions, upload_music
		FROM level_licenses
		WHERE level_id = ?
	`

	var license pb.LevelLicense
	var createUnion, addMember, obsLicense, gateLicense, lawyerLicense, cityEntry,
		establishSpecialProp, establishPropSurface, judgeEntry, uploadImg, deleteImg,
		interGenPoints, interSpecialPoints, rentSatisfaction, accessQuestions,
		createChallenge, uploadMusic int8

	err := r.db.QueryRowContext(ctx, query, levelID).Scan(
		&license.Id,
		&license.LevelId,
		&createUnion,
		&addMember,
		&obsLicense,
		&gateLicense,
		&lawyerLicense,
		&cityEntry,
		&establishSpecialProp,
		&establishPropSurface,
		&judgeEntry,
		&uploadImg,
		&deleteImg,
		&interGenPoints,
		&interSpecialPoints,
		&rentSatisfaction,
		&accessQuestions,
		&createChallenge,
		&uploadMusic,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Missing licenses is allowed per API docs
		}
		return nil, err
	}

	license.CreateUnion = createUnion != 0
	license.AddMemeberToUnion = addMember != 0
	license.ObservationLicense = obsLicense != 0
	license.GateLicense = gateLicense != 0
	license.LawyerLicense = lawyerLicense != 0
	license.CityCounsileEntry = cityEntry != 0
	license.EstablishSpecialResidentialProperty = establishSpecialProp != 0
	license.EstablishPropertyOnSurface = establishPropSurface != 0
	license.JudgeEntry = judgeEntry != 0
	license.UploadImage = uploadImg != 0
	license.DeleteImage = deleteImg != 0
	license.InterLevelGeneralPoints = interGenPoints != 0
	license.InterLevelSpecialPoints = interSpecialPoints != 0
	license.RentOutSatisfaction = rentSatisfaction != 0
	license.AccessToAnswerQuestionsUnit = accessQuestions != 0
	license.CreateChallengeQuestions = createChallenge != 0
	license.UploadMusic = uploadMusic != 0

	return &license, nil
}

// GetNextLevelForScore finds the next level a user should achieve based on their score
// Implements Laravel: Level::where('score', '<=', $user->score)->whereNotIn('id', $user->levels->pluck('id'))->first()
func (r *LevelRepository) GetNextLevelForScore(ctx context.Context, userID uint64, score int32) (*pb.Level, error) {
	query := `
		SELECT l.id, l.name, l.slug, CAST(l.score AS UNSIGNED) as score, l.background_image,
		       COALESCE(i.url, '') as image_url
		FROM levels l
		LEFT JOIN images i ON i.imageable_id = l.id AND i.imageable_type = 'App\\Models\\Levels\\Level'
		WHERE CAST(l.score AS UNSIGNED) <= ?
		  AND l.id NOT IN (
		      SELECT level_id FROM level_user WHERE user_id = ?
		  )
		ORDER BY CAST(l.score AS UNSIGNED) DESC
		LIMIT 1
	`

	var level pb.Level
	var imageURL sql.NullString
	var backgroundImage sql.NullString

	err := r.db.QueryRowContext(ctx, query, score, userID).Scan(
		&level.Id,
		&level.Name,
		&level.Slug,
		&level.Score,
		&backgroundImage,
		&imageURL,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No new level to achieve
		}
		return nil, err
	}

	if imageURL.Valid && imageURL.String != "" {
		level.ImageUrl = r.formatImageURL(imageURL.String)
	}
	if backgroundImage.Valid {
		level.BackgroundImage = backgroundImage.String
	}

	return &level, nil
}

// AttachLevelToUser attaches a level to a user
// Implements Laravel: $user->levels()->attach($level_id)
func (r *LevelRepository) AttachLevelToUser(ctx context.Context, userID, levelID uint64) error {
	query := "INSERT INTO level_user (user_id, level_id, created_at, updated_at) VALUES (?, ?, NOW(), NOW())"
	_, err := r.db.ExecContext(ctx, query, userID, levelID)
	return err
}

// HasUserReceivedPrize checks if user has received prize for a level
// Implements Laravel: $user->recievedLevelPrizes()->where('level_prize_id', $prize_id)->exists()
func (r *LevelRepository) HasUserReceivedPrize(ctx context.Context, userID, prizeID uint64) (bool, error) {
	query := "SELECT COUNT(*) FROM recieved_level_prizes WHERE user_id = ? AND level_prize_id = ?"
	var count int
	err := r.db.QueryRowContext(ctx, query, userID, prizeID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// RecordReceivedPrize records that user has received a prize
// Implements Laravel: $user->recievedLevelPrizes()->attach($prize_id)
func (r *LevelRepository) RecordReceivedPrize(ctx context.Context, userID, prizeID uint64) error {
	query := "INSERT INTO recieved_level_prizes (user_id, level_prize_id, created_at, updated_at) VALUES (?, ?, NOW(), NOW())"
	_, err := r.db.ExecContext(ctx, query, userID, prizeID)
	return err
}

// GetVariableRate returns numeric value from system_variables table.
func (r *LevelRepository) GetVariableRate(ctx context.Context, name string) (float64, error) {
	query := "SELECT value FROM system_variables WHERE name = ? LIMIT 1"
	var value string
	if err := r.db.QueryRowContext(ctx, query, name).Scan(&value); err != nil {
		return 0, err
	}

	rate, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse variable %s rate: %w", name, err)
	}
	return rate, nil
}
