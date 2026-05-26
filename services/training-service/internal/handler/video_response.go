package handler

import (
	"fmt"
	"os"
	"strings"

	commonpb "metargb/shared/pb/common"
	trainingpb "metargb/shared/pb/training"
	"metargb/training-service/internal/service"
)

// buildVideoResponse builds a VideoResponse from video details.
func buildVideoResponse(video *service.VideoDetails) (*trainingpb.VideoResponse, error) {
	if video == nil || video.Video == nil {
		return nil, fmt.Errorf("invalid video data")
	}

	resp := &trainingpb.VideoResponse{
		Id:          video.Video.ID,
		Title:       video.Video.Title,
		Slug:        getStringValue(video.Video.Slug),
		Description: video.Video.Description,
		FileName:    video.Video.FileName,
		CreatorCode: video.Video.CreatorCode,
		CreatedAt:   video.CreatedAtJalali,
		ImageUrl:    buildUploadURL(video.Video.Image),
		VideoUrl:    buildVideoFileURL(video.Video.FileName),
	}

	if video.Creator != nil {
		resp.Creator = &commonpb.UserBasic{
			Id:    video.Creator.ID,
			Name:  video.Creator.Name,
			Code:  video.Creator.Code,
			Email: video.Creator.Email,
		}
		if video.Creator.ProfilePhoto != "" {
			resp.Creator.ProfilePhoto = buildUploadURL(video.Creator.ProfilePhoto)
		}
	}

	if video.Category != nil {
		resp.Category = &trainingpb.CategoryInfo{
			Id:   video.Category.ID,
			Name: video.Category.Name,
			Slug: video.Category.Slug,
		}
	}
	if video.SubCategory != nil {
		resp.SubCategory = &trainingpb.SubCategoryInfo{
			Id:   video.SubCategory.ID,
			Name: video.SubCategory.Name,
			Slug: video.SubCategory.Slug,
		}
	}

	if video.Stats != nil {
		resp.Stats = &trainingpb.VideoStats{
			ViewsCount:    video.Stats.ViewsCount,
			LikesCount:    video.Stats.LikesCount,
			DislikesCount: video.Stats.DislikesCount,
			CommentsCount: video.Stats.CommentsCount,
		}
	}

	return resp, nil
}

// buildUploadURL prefixes resource paths with ADMIN_PANEL_URL/uploads/ (Laravel admin_panel_url).
func buildUploadURL(path string) string {
	if path == "" {
		return ""
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}

	adminURL := strings.TrimSuffix(os.Getenv("ADMIN_PANEL_URL"), "/")
	resourcePath := strings.TrimPrefix(path, "/")
	if adminURL != "" {
		return fmt.Sprintf("%s/uploads/%s", adminURL, resourcePath)
	}
	return fmt.Sprintf("/uploads/%s", resourcePath)
}

func buildVideoFileURL(fileName string) string {
	return buildUploadURL(fileName)
}
