package handler

import (
	"net/http"

	pb "metargb/shared/pb/auth"
)

const listUsersPerPage int32 = 20

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
	if lvl.ImageUrl != "" {
		out["image"] = lvl.ImageUrl
	}
	return out
}

// buildListUserItemHTTP maps a user list proto item to Laravel UserResource JSON.
func buildListUserItemHTTP(item *pb.UserListItem) map[string]interface{} {
	userData := map[string]interface{}{
		"id":    item.Id,
		"name":  item.Name,
		"code":  item.Code,
		"score": item.Score,
	}

	if item.ProfilePhoto != "" {
		userData["profile_photo"] = item.ProfilePhoto
	}

	levelsData := map[string]interface{}{
		"current":  nil,
		"previous": []interface{}{},
	}
	if item.Levels != nil {
		if item.Levels.Current != nil {
			levelsData["current"] = userListLevelToHTTP(item.Levels.Current)
		}
		if len(item.Levels.Previous) > 0 {
			previous := make([]interface{}, 0, len(item.Levels.Previous))
			for _, lvl := range item.Levels.Previous {
				if lvl != nil {
					previous = append(previous, userListLevelToHTTP(lvl))
				}
			}
			levelsData["previous"] = previous
		}
	}
	userData["levels"] = levelsData

	return userData
}

// buildListUsersHTTPResponse formats GET /api/users as Laravel simplePaginate JSON.
func buildListUsersHTTPResponse(r *http.Request, resp *pb.ListUsersResponse) map[string]interface{} {
	currentPage := int32(1)
	hasMore := false
	if resp.Meta != nil {
		currentPage = resp.Meta.CurrentPage
		if currentPage <= 0 {
			currentPage = 1
		}
		hasMore = resp.Meta.NextPageUrl != ""
	}

	users := make([]map[string]interface{}, 0, len(resp.Data))
	for _, item := range resp.Data {
		users = append(users, buildListUserItemHTTP(item))
	}

	response := map[string]interface{}{
		"data": users,
	}

	response["links"] = buildSimplePaginationLinks(r, currentPage, hasMore)

	itemCount := len(users)
	var from interface{}
	var to interface{}
	if itemCount > 0 {
		fromVal := int((currentPage-1)*listUsersPerPage) + 1
		from = fromVal
		to = fromVal + itemCount - 1
	}

	response["meta"] = map[string]interface{}{
		"current_page": currentPage,
		"from":         from,
		"path":         requestPath(r),
		"per_page":     listUsersPerPage,
		"to":           to,
	}

	return response
}
