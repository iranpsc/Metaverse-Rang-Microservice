package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"metarang/features-service/internal/models"
)

// CitizenFeaturesRepository queries public citizen feature assets.
type CitizenFeaturesRepository struct {
	db *sql.DB
}

func NewCitizenFeaturesRepository(db *sql.DB) *CitizenFeaturesRepository {
	return &CitizenFeaturesRepository{db: db}
}

// CountOwnedByKarbari returns current inventory for one karbari (period-independent).
func (r *CitizenFeaturesRepository) CountOwnedByKarbari(ctx context.Context, userID uint64, karbari string) (int32, error) {
	query := `
		SELECT COUNT(*)
		FROM features f
		INNER JOIN feature_properties fp ON f.id = fp.feature_id
		WHERE f.owner_id = ? AND fp.karbari = ?
	`
	var count int32
	if err := r.db.QueryRowContext(ctx, query, userID, karbari).Scan(&count); err != nil {
		return 0, fmt.Errorf("count owned features: %w", err)
	}
	return count, nil
}

// CountTradesByKarbari counts bought or sold trades for a karbari within [start, end].
// role must be "buyer" or "seller".
func (r *CitizenFeaturesRepository) CountTradesByKarbari(
	ctx context.Context,
	userID uint64,
	role string,
	karbari string,
	start, end time.Time,
) (int32, error) {
	column := "buyer_id"
	if role == "seller" {
		column = "seller_id"
	} else if role != "buyer" {
		return 0, fmt.Errorf("invalid trade role: %s", role)
	}

	query := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM trades t
		INNER JOIN features f ON t.feature_id = f.id
		INNER JOIN feature_properties fp ON f.id = fp.feature_id
		WHERE t.%s = ?
		  AND fp.karbari = ?
		  AND t.created_at BETWEEN ? AND ?
	`, column)

	var count int32
	if err := r.db.QueryRowContext(ctx, query, userID, karbari, start, end).Scan(&count); err != nil {
		return 0, fmt.Errorf("count %s trades: %w", role, err)
	}
	return count, nil
}

// ListTradeTimestamps returns trade created_at values for chart bucketing.
// role must be "buyer" or "seller". Empty karbaris yields an empty slice.
func (r *CitizenFeaturesRepository) ListTradeTimestamps(
	ctx context.Context,
	userID uint64,
	role string,
	karbaris []string,
	start, end time.Time,
) ([]models.CitizenTradeTimestamp, error) {
	if len(karbaris) == 0 {
		return []models.CitizenTradeTimestamp{}, nil
	}

	column := "buyer_id"
	if role == "seller" {
		column = "seller_id"
	} else if role != "buyer" {
		return nil, fmt.Errorf("invalid trade role: %s", role)
	}

	placeholders := make([]string, len(karbaris))
	args := make([]interface{}, 0, 3+len(karbaris))
	args = append(args, userID)
	for i, k := range karbaris {
		placeholders[i] = "?"
		args = append(args, k)
	}
	args = append(args, start, end)

	query := fmt.Sprintf(`
		SELECT t.id, t.created_at
		FROM trades t
		INNER JOIN features f ON t.feature_id = f.id
		INNER JOIN feature_properties fp ON f.id = fp.feature_id
		WHERE t.%s = ?
		  AND fp.karbari IN (%s)
		  AND t.created_at BETWEEN ? AND ?
	`, column, strings.Join(placeholders, ","))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list %s trade timestamps: %w", role, err)
	}
	defer rows.Close()

	out := make([]models.CitizenTradeTimestamp, 0)
	for rows.Next() {
		var item models.CitizenTradeTimestamp
		if err := rows.Scan(&item.ID, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan trade timestamp: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// ListOwnedFeatures returns a page of owned features with properties and owner code.
// Empty karbaris yields an empty page. Search filters by properties id/address.
func (r *CitizenFeaturesRepository) ListOwnedFeatures(
	ctx context.Context,
	userID uint64,
	karbaris []string,
	search string,
	page, perPage int,
) ([]models.CitizenFeatureListItem, int, error) {
	if len(karbaris) == 0 {
		return []models.CitizenFeatureListItem{}, 0, nil
	}
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 15
	}

	where, args := buildOwnedFeaturesWhere(userID, karbaris, search)

	var total int
	countQuery := `
		SELECT COUNT(*)
		FROM features f
		INNER JOIN feature_properties fp ON f.id = fp.feature_id
		WHERE ` + where
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count owned features list: %w", err)
	}

	offset := (page - 1) * perPage
	listArgs := append(append([]interface{}{}, args...), perPage, offset)
	listQuery := `
		SELECT f.id, fp.id, COALESCE(fp.address, ''), COALESCE(fp.area, 0), COALESCE(fp.density, 0),
		       COALESCE(fp.karbari, ''), COALESCE(u.code, ''), COALESCE(fp.price_psc, '0'),
		       COALESCE(fp.price_irr, '0'), COALESCE(fp.label, '')
		FROM features f
		INNER JOIN feature_properties fp ON f.id = fp.feature_id
		LEFT JOIN users u ON f.owner_id = u.id
		WHERE ` + where + `
		ORDER BY f.id ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list owned features: %w", err)
	}
	defer rows.Close()

	items := make([]models.CitizenFeatureListItem, 0)
	featureIDs := make([]uint64, 0)
	for rows.Next() {
		var item models.CitizenFeatureListItem
		if err := rows.Scan(
			&item.ID, &item.VodID, &item.Address, &item.Area, &item.Density,
			&item.Karbari, &item.OwnerCode, &item.PricePSC, &item.PriceIRR, &item.Label,
		); err != nil {
			return nil, 0, fmt.Errorf("scan owned feature: %w", err)
		}
		items = append(items, item)
		featureIDs = append(featureIDs, item.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	centers, err := r.GetFeatureCenters(ctx, featureIDs)
	if err != nil {
		return nil, 0, err
	}
	imagesByFeature, err := r.getImagesByFeatureIDs(ctx, featureIDs)
	if err != nil {
		return nil, 0, err
	}

	for i := range items {
		if c, ok := centers[items[i].ID]; ok {
			center := c
			items[i].Center = &center
		}
		items[i].Images = imagesByFeature[items[i].ID]
		if items[i].Images == nil {
			items[i].Images = []models.CitizenFeatureImage{}
		}
	}

	return items, total, nil
}

// ListMapMarkers returns all owned features for the map (karbari only, no search).
func (r *CitizenFeaturesRepository) ListMapMarkers(
	ctx context.Context,
	userID uint64,
	karbaris []string,
) ([]models.CitizenFeatureMapMarker, error) {
	if len(karbaris) == 0 {
		return []models.CitizenFeatureMapMarker{}, nil
	}

	placeholders := make([]string, len(karbaris))
	args := make([]interface{}, 0, 1+len(karbaris))
	args = append(args, userID)
	for i, k := range karbaris {
		placeholders[i] = "?"
		args = append(args, k)
	}

	query := `
		SELECT f.id, COALESCE(fp.karbari, '')
		FROM features f
		INNER JOIN feature_properties fp ON f.id = fp.feature_id
		WHERE f.owner_id = ? AND fp.karbari IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY f.id ASC
	`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list map markers: %w", err)
	}
	defer rows.Close()

	markers := make([]models.CitizenFeatureMapMarker, 0)
	featureIDs := make([]uint64, 0)
	for rows.Next() {
		var m models.CitizenFeatureMapMarker
		if err := rows.Scan(&m.ID, &m.Karbari); err != nil {
			return nil, fmt.Errorf("scan map marker: %w", err)
		}
		markers = append(markers, m)
		featureIDs = append(featureIDs, m.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	centers, err := r.GetFeatureCenters(ctx, featureIDs)
	if err != nil {
		return nil, err
	}
	for i := range markers {
		if c, ok := centers[markers[i].ID]; ok {
			center := c
			markers[i].Center = &center
		}
	}
	return markers, nil
}

// GetFeatureCenters batch-computes centroids (Laravel Feature::batchComputedCenters).
func (r *CitizenFeaturesRepository) GetFeatureCenters(
	ctx context.Context,
	featureIDs []uint64,
) (map[uint64]models.CitizenFeatureCenter, error) {
	out := make(map[uint64]models.CitizenFeatureCenter)
	if len(featureIDs) == 0 {
		return out, nil
	}

	placeholders := make([]string, len(featureIDs))
	args := make([]interface{}, len(featureIDs))
	for i, id := range featureIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := `
		SELECT g.feature_id,
		       AVG(CAST(c.x AS DECIMAL(20,12))) AS cx,
		       AVG(CAST(c.y AS DECIMAL(20,12))) AS cy
		FROM coordinates c
		INNER JOIN geometries g ON c.geometry_id = g.id
		WHERE g.feature_id IN (` + strings.Join(placeholders, ",") + `)
		GROUP BY g.feature_id
	`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get feature centers: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var featureID uint64
		var cx, cy sql.NullFloat64
		if err := rows.Scan(&featureID, &cx, &cy); err != nil {
			return nil, fmt.Errorf("scan feature center: %w", err)
		}
		if cx.Valid && cy.Valid {
			out[featureID] = models.CitizenFeatureCenter{X: cx.Float64, Y: cy.Float64}
		}
	}
	return out, rows.Err()
}

func buildOwnedFeaturesWhere(userID uint64, karbaris []string, search string) (string, []interface{}) {
	placeholders := make([]string, len(karbaris))
	args := make([]interface{}, 0, 2+len(karbaris))
	args = append(args, userID)
	for i, k := range karbaris {
		placeholders[i] = "?"
		args = append(args, k)
	}

	where := `f.owner_id = ? AND fp.karbari IN (` + strings.Join(placeholders, ",") + `)`
	if search != "" {
		where += ` AND (fp.id LIKE ? OR fp.address LIKE ?)`
		like := "%" + search + "%"
		args = append(args, like, like)
	}
	return where, args
}

func (r *CitizenFeaturesRepository) getImagesByFeatureIDs(
	ctx context.Context,
	featureIDs []uint64,
) (map[uint64][]models.CitizenFeatureImage, error) {
	out := make(map[uint64][]models.CitizenFeatureImage)
	if len(featureIDs) == 0 {
		return out, nil
	}

	placeholders := make([]string, len(featureIDs))
	args := make([]interface{}, len(featureIDs))
	for i, id := range featureIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := `
		SELECT id, imageable_id, url
		FROM images
		WHERE imageable_type = 'App\\Models\\Feature'
		  AND imageable_id IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY id ASC
	`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list feature images: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var img models.CitizenFeatureImage
		var featureID uint64
		if err := rows.Scan(&img.ID, &featureID, &img.URL); err != nil {
			return nil, fmt.Errorf("scan feature image: %w", err)
		}
		out[featureID] = append(out[featureID], img)
	}
	return out, rows.Err()
}
