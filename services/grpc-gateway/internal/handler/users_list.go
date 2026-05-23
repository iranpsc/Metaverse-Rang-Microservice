package handler

import pb "metargb/shared/pb/auth"

// userListLevelToHTTP maps a list Level proto to Laravel UserResource level shape.
func userListLevelToHTTP(lvl *pb.Level) map[string]interface{} {
	if lvl == nil {
		return nil
	}
	out := map[string]interface{}{
		"id": lvl.Id,
	}
	if lvl.Title != "" {
		out["name"] = lvl.Title
	}
	if lvl.Slug != "" {
		out["slug"] = lvl.Slug
	}
	if lvl.Score != 0 {
		out["score"] = lvl.Score
	}
	if lvl.ImageUrl != "" {
		out["image"] = lvl.ImageUrl
	}
	return out
}
