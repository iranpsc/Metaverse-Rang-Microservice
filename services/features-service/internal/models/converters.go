package models

import (
	"fmt"

	pb "metarang/shared/pb/features"
)

// FeatureToPB converts internal Feature model to protobuf message
func FeatureToPB(feature *Feature, properties *FeatureProperties, geometry *Geometry) *pb.Feature {
	pbFeature := &pb.Feature{
		Id:      feature.ID,
		OwnerId: feature.OwnerID,
	}

	if properties != nil {
		pbFeature.Properties = PropertiesToPB(properties)
	}

	if geometry != nil {
		pbFeature.Geometry = &pb.Geometry{
			Id:   geometry.ID,
			Type: geometry.Type,
		}
	}

	return pbFeature
}

// PropertiesToPB converts FeatureProperties to protobuf
func PropertiesToPB(props *FeatureProperties) *pb.FeatureProperties {
	return &pb.FeatureProperties{
		Id:                     props.ID,
		Address:                props.Address,
		Density:                int32(props.Density),
		Area:                   fmt.Sprintf("%.2f", props.Area),
		Stability:              fmt.Sprintf("%.2f", props.Stability),
		Label:                  props.Label,
		Karbari:                props.Karbari,
		Owner:                  props.Owner,
		Rgb:                    props.RGB,
		PricePsc:               props.PricePSC,
		PriceIrr:               props.PriceIRR,
		MinimumPricePercentage: int32(props.MinimumPricePercentage),
	}
}

// FeaturesToPB converts slice of Features to protobuf messages
func FeaturesToPB(features []*Feature) []*pb.Feature {
	result := make([]*pb.Feature, 0, len(features))
	for _, f := range features {
		result = append(result, FeatureToPB(f, nil, nil))
	}
	return result
}
