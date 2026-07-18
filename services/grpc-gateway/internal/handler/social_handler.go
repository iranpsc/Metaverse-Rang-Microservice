package handler

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"google.golang.org/grpc"

	"metarang/grpc-gateway/internal/middleware"
	socialpb "metarang/shared/pb/social"
)

const followListPerPage int32 = 10

type SocialHandler struct {
	followClient    socialpb.FollowServiceClient
	challengeClient socialpb.ChallengeServiceClient
}

func NewSocialHandler(socialConn *grpc.ClientConn, _ *grpc.ClientConn) *SocialHandler {
	return &SocialHandler{
		followClient:    socialpb.NewFollowServiceClient(socialConn),
		challengeClient: socialpb.NewChallengeServiceClient(socialConn),
	}
}

// getUserIDFromToken extracts user ID from context (set by auth middleware)
func (h *SocialHandler) getUserIDFromToken(r *http.Request) (uint64, error) {
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		return 0, err
	}
	return userCtx.UserID, nil
}

// GetFollowers handles GET /api/followers
func (h *SocialHandler) GetFollowers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getUserIDFromToken(r)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	grpcReq := &socialpb.GetFollowersRequest{UserId: userID}
	resp, err := h.followClient.GetFollowers(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, buildFollowListHTTPResponse(r, resp.Data))
}

// GetFollowing handles GET /api/following
func (h *SocialHandler) GetFollowing(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getUserIDFromToken(r)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	grpcReq := &socialpb.GetFollowingRequest{UserId: userID}
	resp, err := h.followClient.GetFollowing(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, buildFollowListHTTPResponse(r, resp.Data))
}

// Follow handles GET /api/follow/{user}
func (h *SocialHandler) Follow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getUserIDFromToken(r)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	// Extract target user ID from path: /api/follow/{user}
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/follow/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		writeError(w, http.StatusBadRequest, "user ID is required")
		return
	}

	targetUserID, err := strconv.ParseUint(pathParts[0], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	grpcReq := &socialpb.FollowRequest{
		UserId:       userID,
		TargetUserId: targetUserID,
	}
	_, err = h.followClient.Follow(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// buildFollowListHTTPResponse formats a follow list as Laravel simplePaginate JSON
// with 10 items per page and the FollowResource field shape.
func buildFollowListHTTPResponse(r *http.Request, resources []*socialpb.FollowResource) map[string]interface{} {
	page := int32(1)
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.ParseInt(pageStr, 10, 32); err == nil && p > 0 {
			page = int32(p)
		}
	}

	total := int32(len(resources))
	start := (page - 1) * followListPerPage
	if start > total {
		start = total
	}
	end := start + followListPerPage
	if end > total {
		end = total
	}
	pageSlice := resources[start:end]
	hasMore := end < total

	data := make([]map[string]interface{}, 0, len(pageSlice))
	for _, resource := range pageSlice {
		data = append(data, followResourceJSON(resource))
	}

	response := map[string]interface{}{
		"data":  data,
		"links": buildSimplePaginationLinks(r, page, hasMore),
	}

	itemCount := len(data)
	var from interface{}
	var to interface{}
	if itemCount > 0 {
		fromVal := int((page-1)*followListPerPage) + 1
		from = fromVal
		to = fromVal + itemCount - 1
	}

	response["meta"] = map[string]interface{}{
		"current_page": page,
		"from":         from,
		"path":         requestPath(r),
		"per_page":     followListPerPage,
		"to":           to,
	}

	return response
}

func followResourceJSON(resource *socialpb.FollowResource) map[string]interface{} {
	canFollow := false
	canUnfollow := false
	canRemoveFollower := false
	if resource.Can != nil {
		canFollow = resource.Can.Follow
		canUnfollow = resource.Can.Unfollow
		canRemoveFollower = resource.Can.RemoveFollower
	}

	return map[string]interface{}{
		"id":            resource.Id,
		"name":          resource.Name,
		"code":          resource.Code,
		"profile_photo": resource.ProfilePhoto,
		"level":         resource.Level,
		"online":        resource.Online,
		"followed":      resource.Followed,
		"can": map[string]bool{
			"follow":          canFollow,
			"unfollow":        canUnfollow,
			"remove_follower": canRemoveFollower,
		},
	}
}

// Unfollow handles GET /api/unfollow/{user}
func (h *SocialHandler) Unfollow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getUserIDFromToken(r)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	// Extract target user ID from path: /api/unfollow/{user}
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/unfollow/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		writeError(w, http.StatusBadRequest, "user ID is required")
		return
	}

	targetUserID, err := strconv.ParseUint(pathParts[0], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	grpcReq := &socialpb.UnfollowRequest{
		UserId:       userID,
		TargetUserId: targetUserID,
	}
	_, err = h.followClient.Unfollow(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Remove handles GET /api/remove/{user}
func (h *SocialHandler) Remove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getUserIDFromToken(r)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	// Extract target user ID from path: /api/remove/{user}
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/remove/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		writeError(w, http.StatusBadRequest, "user ID is required")
		return
	}

	targetUserID, err := strconv.ParseUint(pathParts[0], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	grpcReq := &socialpb.RemoveRequest{
		UserId:       userID,
		TargetUserId: targetUserID,
	}
	_, err = h.followClient.Remove(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetTimings handles GET /api/challenge/timings
func (h *SocialHandler) GetTimings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getUserIDFromToken(r)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	grpcReq := &socialpb.GetTimingsRequest{UserId: userID}
	resp, err := h.challengeClient.GetTimings(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": resp.Data})
}

// GetQuestion handles GET /api/challenge/question
func (h *SocialHandler) GetQuestion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getUserIDFromToken(r)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	grpcReq := &socialpb.GetQuestionRequest{UserId: userID}
	resp, err := h.challengeClient.GetQuestion(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp.Data)
}

// SubmitAnswer handles POST /api/challenge/answer
func (h *SocialHandler) SubmitAnswer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, err := h.getUserIDFromToken(r)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	var req struct {
		QuestionID uint64 `json:"question_id"`
		AnswerID   uint64 `json:"answer_id"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	if req.QuestionID == 0 {
		writeError(w, http.StatusUnprocessableEntity, "question_id is required")
		return
	}
	if req.AnswerID == 0 {
		writeError(w, http.StatusUnprocessableEntity, "answer_id is required")
		return
	}

	grpcReq := &socialpb.SubmitAnswerRequest{
		UserId:     userID,
		QuestionId: req.QuestionID,
		AnswerId:   req.AnswerID,
	}
	resp, err := h.challengeClient.SubmitAnswer(r.Context(), grpcReq)
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, resp.Data)
}

// GetAdvertisement handles GET /api/challenge/advertisement
func (h *SocialHandler) GetAdvertisement(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if _, err := h.getUserIDFromToken(r); err != nil {
		writeGRPCError(w, err)
		return
	}

	resp, err := h.challengeClient.GetAdvertisement(r.Context(), &socialpb.GetAdvertisementRequest{})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	ads := make([]map[string]interface{}, 0, len(resp.Advertisements))
	for _, ad := range resp.Advertisements {
		ads = append(ads, map[string]interface{}{
			"code":             ad.Code,
			"title":            ad.Title,
			"description":      ad.Description,
			"investment_value": ad.InvestmentValue,
			"ends_at":          ad.EndsAt,
			"video_url":        ad.VideoUrl,
			"image_url":        ad.ImageUrl,
			"url":              ad.Url,
			"investment_asset": ad.InvestmentAsset,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": ads})
}
