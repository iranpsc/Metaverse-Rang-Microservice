package handler

import (
	trainingpb "metarang/shared/pb/training"
)

func videoToJSON(video *trainingpb.VideoResponse) map[string]interface{} {
	resp := map[string]interface{}{
		"id":          video.Id,
		"title":       video.Title,
		"slug":        video.Slug,
		"description": video.Description,
		"image_url":   video.ImageUrl,
		"video_url":   video.VideoUrl,
		"created_at":  video.CreatedAt,
	}

	if video.Creator != nil {
		creator := map[string]interface{}{
			"name": video.Creator.Name,
			"code": video.Creator.Code,
		}
		if video.Creator.ProfilePhoto != "" {
			creator["image"] = video.Creator.ProfilePhoto
		}
		resp["creator"] = creator
	}

	if video.Category != nil {
		resp["category"] = map[string]interface{}{
			"id":   video.Category.Id,
			"name": video.Category.Name,
			"slug": video.Category.Slug,
		}
	}

	if video.SubCategory != nil {
		resp["sub_category"] = map[string]interface{}{
			"id":   video.SubCategory.Id,
			"name": video.SubCategory.Name,
			"slug": video.SubCategory.Slug,
		}
	}

	if video.Stats != nil {
		resp["views_count"] = video.Stats.ViewsCount
		resp["likes_count"] = video.Stats.LikesCount
		resp["dislikes_count"] = video.Stats.DislikesCount
		resp["comments_count"] = video.Stats.CommentsCount
	}

	if video.UserInteraction != nil {
		resp["user_interaction"] = map[string]interface{}{
			"liked": *video.UserInteraction,
		}
	}

	return resp
}

func videosToJSON(resp *trainingpb.VideosResponse) map[string]interface{} {
	videos := make([]map[string]interface{}, 0, len(resp.Videos))
	for _, video := range resp.Videos {
		videos = append(videos, videoToJSON(video))
	}

	result := map[string]interface{}{
		"data": videos,
	}

	if resp.Pagination != nil {
		result["meta"] = map[string]interface{}{
			"current_page": resp.Pagination.CurrentPage,
			"per_page":     resp.Pagination.PerPage,
			"total":        resp.Pagination.Total,
			"last_page":    resp.Pagination.LastPage,
		}
	}

	return result
}

func modalVideoToJSON(video *trainingpb.VideoResponse) map[string]interface{} {
	resp := map[string]interface{}{
		"id":           video.Id,
		"title":        video.Title,
		"description":  video.Description,
		"video":        video.VideoUrl,
		"image":        video.ImageUrl,
		"creator_code": video.CreatorCode,
	}
	if video.Stats != nil {
		resp["views"] = video.Stats.ViewsCount
		resp["likes"] = video.Stats.LikesCount
		resp["dislikes"] = video.Stats.DislikesCount
	}
	return resp
}

func categoryToJSON(category *trainingpb.CategoryResponse) map[string]interface{} {
	resp := map[string]interface{}{
		"id":   category.Id,
		"name": category.Name,
		"slug": category.Slug,
	}

	if category.Description != "" {
		resp["description"] = category.Description
	}

	if category.ImageUrl != "" {
		resp["image"] = category.ImageUrl
	}
	if category.IconUrl != "" {
		resp["icon"] = category.IconUrl
	}

	applyCategoryCountsToJSON(resp, category.VideosCount, category.Stats)

	if len(category.SubCategories) > 0 {
		subCats := make([]map[string]interface{}, 0, len(category.SubCategories))
		for _, subCat := range category.SubCategories {
			subCats = append(subCats, subCategoryInfoToJSON(subCat))
		}
		resp["subcategories"] = subCats
	}

	return resp
}

func categoriesToJSON(resp *trainingpb.CategoriesResponse) map[string]interface{} {
	categories := make([]map[string]interface{}, 0, len(resp.Categories))
	for _, category := range resp.Categories {
		categories = append(categories, categoryToJSON(category))
	}

	result := map[string]interface{}{
		"data": categories,
	}

	if resp.Pagination != nil {
		result["meta"] = map[string]interface{}{
			"current_page": resp.Pagination.CurrentPage,
			"per_page":     resp.Pagination.PerPage,
			"total":        resp.Pagination.Total,
			"last_page":    resp.Pagination.LastPage,
		}
	}

	return result
}

func subCategoryToJSON(subCategory *trainingpb.SubCategoryResponse) map[string]interface{} {
	resp := map[string]interface{}{
		"id":          subCategory.Id,
		"name":        subCategory.Name,
		"slug":        subCategory.Slug,
		"description": subCategory.Description,
	}

	if subCategory.ImageUrl != "" {
		resp["image"] = subCategory.ImageUrl
	}
	if subCategory.IconUrl != "" {
		resp["icon"] = subCategory.IconUrl
	}

	if subCategory.Category != nil {
		resp["category"] = map[string]interface{}{
			"id":   subCategory.Category.Id,
			"name": subCategory.Category.Name,
			"slug": subCategory.Category.Slug,
		}
	}

	applyCategoryCountsToJSON(resp, subCategory.VideosCount, subCategory.Stats)

	return resp
}

func subCategoryInfoToJSON(subCategory *trainingpb.SubCategoryInfo) map[string]interface{} {
	resp := map[string]interface{}{
		"id":          subCategory.Id,
		"name":        subCategory.Name,
		"slug":        subCategory.Slug,
		"description": subCategory.Description,
	}

	if subCategory.ImageUrl != "" {
		resp["image"] = subCategory.ImageUrl
	}
	if subCategory.IconUrl != "" {
		resp["icon"] = subCategory.IconUrl
	}

	applyCategoryCountsToJSON(resp, subCategory.VideosCount, subCategory.Stats)

	return resp
}

func applyCategoryCountsToJSON(resp map[string]interface{}, videosCount int32, stats *trainingpb.VideoStats) {
	if stats != nil {
		resp["views_count"] = stats.ViewsCount
		resp["likes_count"] = stats.LikesCount
		resp["dislikes_count"] = stats.DislikesCount
	}
	resp["videos_count"] = videosCount
}

func commentToJSON(comment *trainingpb.CommentResponse) map[string]interface{} {
	resp := map[string]interface{}{
		"id":         comment.Id,
		"video_id":   comment.VideoId,
		"content":    comment.Content,
		"created_at": comment.CreatedAt,
	}
	if comment.UpdatedAt != "" {
		resp["updated_at"] = comment.UpdatedAt
	}

	if comment.ParentId > 0 {
		resp["parent_id"] = comment.ParentId
	}

	if comment.User != nil {
		user := map[string]interface{}{
			"id":   comment.User.Id,
			"name": comment.User.Name,
			"code": comment.User.Code,
		}
		if comment.User.ProfilePhoto != "" {
			user["image"] = comment.User.ProfilePhoto
		}
		resp["user"] = user
	}

	if comment.Stats != nil {
		resp["likes"] = comment.Stats.LikesCount
		resp["dislikes"] = comment.Stats.DislikesCount
		resp["replies_count"] = comment.Stats.RepliesCount
	}

	if comment.UserInteraction != nil {
		resp["user_interaction"] = *comment.UserInteraction
	}

	if comment.ParentId > 0 {
		resp["is_reply"] = true
	} else {
		resp["is_reply"] = false
	}

	return resp
}

func commentsToJSON(resp *trainingpb.CommentsResponse) map[string]interface{} {
	comments := make([]map[string]interface{}, 0, len(resp.Comments))
	for _, comment := range resp.Comments {
		comments = append(comments, commentToJSON(comment))
	}

	result := map[string]interface{}{
		"data": comments,
	}

	if resp.Pagination != nil {
		result["links"] = map[string]interface{}{
			"next": nil,
		}
		result["meta"] = map[string]interface{}{
			"current_page": resp.Pagination.CurrentPage,
			"per_page":     resp.Pagination.PerPage,
			"total":        resp.Pagination.Total,
			"last_page":    resp.Pagination.LastPage,
		}
	}

	return result
}

func repliesToJSON(resp *trainingpb.RepliesResponse) map[string]interface{} {
	replies := make([]map[string]interface{}, 0, len(resp.Replies))
	for _, reply := range resp.Replies {
		replies = append(replies, commentToJSON(reply))
	}

	result := map[string]interface{}{
		"data": replies,
	}

	if resp.Pagination != nil {
		result["links"] = map[string]interface{}{
			"next": nil,
		}
		result["meta"] = map[string]interface{}{
			"current_page": resp.Pagination.CurrentPage,
			"per_page":     resp.Pagination.PerPage,
			"total":        resp.Pagination.Total,
			"last_page":    resp.Pagination.LastPage,
		}
	}

	return result
}
