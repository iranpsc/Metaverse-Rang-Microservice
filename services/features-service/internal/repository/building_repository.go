package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	pb "metarang/shared/pb/features"
)

type BuildingRepository struct {
	db *sql.DB
}

func NewBuildingRepository(db *sql.DB) *BuildingRepository {
	return &BuildingRepository{db: db}
}

// FindByFeatureID retrieves all buildings for a feature with building model data
func (r *BuildingRepository) FindByFeatureID(ctx context.Context, featureID uint64) ([]*pb.Building, error) {
	query := `
		SELECT 
			b.id, 
			b.construction_start_date, 
			b.construction_end_date, 
			b.launched_satisfaction,
			b.rotation, 
			b.position, 
			b.bubble_diameter, 
			b.information,
			bm.id as model_id,
			bm.model_id as model_model_id,
			bm.name as model_name,
			bm.sku as model_sku,
			bm.images as model_images,
			bm.attributes as model_attributes,
			bm.file as model_file,
			bm.required_satisfaction as model_required_satisfaction
		FROM buildings b
		INNER JOIN building_models bm ON b.model_id = bm.id
		WHERE b.feature_id = ?
		ORDER BY b.id ASC
	`

	rows, err := r.db.QueryContext(ctx, query, featureID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	buildings := []*pb.Building{}
	for rows.Next() {
		building := &pb.Building{}
		var constructionStartDate, constructionEndDate, launchedSatisfaction sql.NullString
		var rotation, position, bubbleDiameter, information sql.NullString
		var id uint64
		var modelID, modelModelID uint64
		var modelName, modelSKU, modelImages, modelAttributes, modelFile sql.NullString
		var modelRequiredSatisfaction sql.NullFloat64

		if err := rows.Scan(
			&id,
			&constructionStartDate,
			&constructionEndDate,
			&launchedSatisfaction,
			&rotation,
			&position,
			&bubbleDiameter,
			&information,
			&modelID,
			&modelModelID,
			&modelName,
			&modelSKU,
			&modelImages,
			&modelAttributes,
			&modelFile,
			&modelRequiredSatisfaction,
		); err != nil {
			continue
		}

		building.Id = id
		if constructionStartDate.Valid {
			building.ConstructionStartDate = constructionStartDate.String
		}
		if constructionEndDate.Valid {
			building.ConstructionEndDate = constructionEndDate.String
		}
		if launchedSatisfaction.Valid {
			building.LaunchedSatisfaction = launchedSatisfaction.String
		}
		if rotation.Valid {
			building.Rotation = rotation.String
		}
		if position.Valid {
			building.Position = position.String
		}
		if bubbleDiameter.Valid {
			building.BubbleDiameter = bubbleDiameter.String
		}
		if information.Valid {
			building.Information = information.String
		}

		model := &pb.BuildingModel{Id: modelID}
		if modelModelID > 0 {
			model.ModelId = fmt.Sprintf("%d", modelModelID)
		}
		if modelName.Valid {
			model.Name = modelName.String
		}
		if modelSKU.Valid {
			model.Sku = modelSKU.String
		}
		if modelImages.Valid {
			model.Images = modelImages.String
		}
		if modelAttributes.Valid {
			model.Attributes = modelAttributes.String
		}
		if modelFile.Valid {
			model.File = modelFile.String
		}
		if modelRequiredSatisfaction.Valid {
			model.RequiredSatisfaction = fmt.Sprintf("%.4f", modelRequiredSatisfaction.Float64)
		}

		building.Model = model
		buildings = append(buildings, building)
	}

	return buildings, nil
}

// FindByFeatureIDs retrieves buildings for multiple features keyed by feature_id.
func (r *BuildingRepository) FindByFeatureIDs(ctx context.Context, featureIDs []uint64) (map[uint64][]*pb.Building, error) {
	result := make(map[uint64][]*pb.Building, len(featureIDs))
	if len(featureIDs) == 0 {
		return result, nil
	}

	idStrs := make([]string, len(featureIDs))
	for i, id := range featureIDs {
		idStrs[i] = fmt.Sprintf("%d", id)
	}

	query := `
		SELECT 
			b.feature_id,
			b.id, 
			b.construction_start_date, 
			b.construction_end_date, 
			b.launched_satisfaction,
			b.rotation, 
			b.position, 
			b.bubble_diameter, 
			b.information,
			bm.id as model_id,
			bm.model_id as model_model_id,
			bm.name as model_name,
			bm.sku as model_sku,
			bm.images as model_images,
			bm.attributes as model_attributes,
			bm.file as model_file,
			bm.required_satisfaction as model_required_satisfaction
		FROM buildings b
		INNER JOIN building_models bm ON b.model_id = bm.id
		WHERE b.feature_id IN (` + strings.Join(idStrs, ",") + `)
		ORDER BY b.feature_id, b.id ASC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		building := &pb.Building{}
		var featureID uint64
		var constructionStartDate, constructionEndDate, launchedSatisfaction sql.NullString
		var rotation, position, bubbleDiameter, information sql.NullString
		var id uint64
		var modelID, modelModelID uint64
		var modelName, modelSKU, modelImages, modelAttributes, modelFile sql.NullString
		var modelRequiredSatisfaction sql.NullFloat64

		if err := rows.Scan(
			&featureID,
			&id,
			&constructionStartDate,
			&constructionEndDate,
			&launchedSatisfaction,
			&rotation,
			&position,
			&bubbleDiameter,
			&information,
			&modelID,
			&modelModelID,
			&modelName,
			&modelSKU,
			&modelImages,
			&modelAttributes,
			&modelFile,
			&modelRequiredSatisfaction,
		); err != nil {
			continue
		}

		building.Id = id
		if constructionStartDate.Valid {
			building.ConstructionStartDate = constructionStartDate.String
		}
		if constructionEndDate.Valid {
			building.ConstructionEndDate = constructionEndDate.String
		}
		if launchedSatisfaction.Valid {
			building.LaunchedSatisfaction = launchedSatisfaction.String
		}
		if rotation.Valid {
			building.Rotation = rotation.String
		}
		if position.Valid {
			building.Position = position.String
		}
		if bubbleDiameter.Valid {
			building.BubbleDiameter = bubbleDiameter.String
		}
		if information.Valid {
			building.Information = information.String
		}

		model := &pb.BuildingModel{Id: modelID}
		if modelModelID > 0 {
			model.ModelId = fmt.Sprintf("%d", modelModelID)
		}
		if modelName.Valid {
			model.Name = modelName.String
		}
		if modelSKU.Valid {
			model.Sku = modelSKU.String
		}
		if modelImages.Valid {
			model.Images = modelImages.String
		}
		if modelAttributes.Valid {
			model.Attributes = modelAttributes.String
		}
		if modelFile.Valid {
			model.File = modelFile.String
		}
		if modelRequiredSatisfaction.Valid {
			model.RequiredSatisfaction = fmt.Sprintf("%.4f", modelRequiredSatisfaction.Float64)
		}
		building.Model = model
		result[featureID] = append(result[featureID], building)
	}

	return result, nil
}