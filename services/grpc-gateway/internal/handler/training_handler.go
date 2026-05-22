package handler

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metargb/grpc-gateway/internal/middleware"
	pb "metargb/shared/pb/auth"
	commonpb "metargb/shared/pb/common"
	trainingpb "metargb/shared/pb/training"
)

type TrainingHandler struct {
	trainingClient trainingpb.VideoServiceClient
	categoryClient trainingpb.CategoryServiceClient
	commentClient  trainingpb.CommentServiceClient
	replyClient    trainingpb.ReplyServiceClient
	authClient     pb.AuthServiceClient
}

func NewTrainingHandler(trainingConn *grpc.ClientConn, authConn *grpc.ClientConn) *TrainingHandler {
	return &TrainingHandler{
		trainingClient: trainingpb.NewVideoServiceClient(trainingConn),
		categoryClient: trainingpb.NewCategoryServiceClient(trainingConn),
		commentClient:  trainingpb.NewCommentServiceClient(trainingConn),
		replyClient:    trainingpb.NewReplyServiceClient(trainingConn),
		authClient:     pb.NewAuthServiceClient(authConn),
	}
}

// GetVideos handles GET /api/tutorials
func (h *TrainingHandler) GetVideos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	page, perPage := parsePagination(r, 1, 18)

	grpcReq := &trainingpb.GetVideosRequest{
		Pagination: &commonpb.PaginationRequest{
			Page:    page,
			PerPage: perPage,
		},
	}

	resp, err := h.trainingClient.GetVideos(r.Context(), grpcReq)
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}

	writeJSON(w, http.StatusOK, buildVideosResponse(resp))
}

// GetVideo handles GET /api/tutorials/{slug}
func (h *TrainingHandler) GetVideo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	slug := extractSlugFromPathTraining(r.URL.Path, "/api/tutorials/")
	if slug == "" {
		writeError(w, http.StatusBadRequest, "slug is required")
		return
	}

	var userID uint64
	// Get user from context (optional - set by optionalAuthMiddleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err == nil {
		userID = userCtx.UserID
	}

	ipAddress := getIPAddress(r)

	grpcReq := &trainingpb.GetVideoRequest{
		Slug:      slug,
		UserId:    userID,
		IpAddress: ipAddress,
	}

	resp, err := h.trainingClient.GetVideo(r.Context(), grpcReq)
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}

	writeJSON(w, http.StatusOK, buildVideoResponse(resp))
}

// SearchVideos handles POST /api/tutorials/search
func (h *TrainingHandler) SearchVideos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		SearchTerm string `json:"searchTerm"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	if req.SearchTerm == "" {
		writeValidationError(w, "searchTerm is required")
		return
	}

	page, perPage := parsePagination(r, 1, 18)

	grpcReq := &trainingpb.SearchVideosRequest{
		Query: req.SearchTerm,
		Pagination: &commonpb.PaginationRequest{
			Page:    page,
			PerPage: perPage,
		},
	}

	resp, err := h.trainingClient.SearchVideos(r.Context(), grpcReq)
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}

	writeJSON(w, http.StatusOK, buildVideosResponse(resp))
}

// AddInteraction handles POST /api/tutorials/{video}/interactions
func (h *TrainingHandler) AddInteraction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	videoID := extractIDFromPathWithSuffix(r.URL.Path, "/api/tutorials/", "/interactions")
	if videoID == 0 {
		writeError(w, http.StatusBadRequest, "invalid video ID")
		return
	}

	var req struct {
		Liked bool `json:"liked"`
	}

	// Try query parameter first (per API spec)
	likedStr := r.URL.Query().Get("liked")
	if likedStr != "" {
		req.Liked = likedStr == "1" || likedStr == "true"
	} else {
		// Try request body (JSON or form-data)
		if err := decodeRequestBody(r, &req); err != nil {
			if err == io.EOF {
				writeError(w, http.StatusBadRequest, "request body is required")
			} else {
				writeError(w, http.StatusBadRequest, "invalid request body")
			}
			return
		}
	}

	ipAddress := getIPAddress(r)

	grpcReq := &trainingpb.AddInteractionRequest{
		VideoId:   videoID,
		UserId:    userCtx.UserID,
		Liked:     req.Liked,
		IpAddress: ipAddress,
	}

	_, err = h.trainingClient.AddInteraction(r.Context(), grpcReq)
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

// GetVideoByFileName handles POST /api/video-tutorials (v1 modal lookup)
func (h *TrainingHandler) GetVideoByFileName(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		URL string `json:"url"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	if req.URL == "" {
		writeValidationError(w, "url is required")
		return
	}

	ipAddress := getIPAddress(r)

	grpcReq := &trainingpb.GetVideoByFileNameRequest{
		FileName:  req.URL,
		IpAddress: ipAddress,
	}

	resp, err := h.trainingClient.GetVideoByFileName(r.Context(), grpcReq)
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": buildModalVideoResponse(resp),
	})
}

// GetCategories handles GET /api/tutorials/categories
func (h *TrainingHandler) GetCategories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	page, perPage := parsePagination(r, 1, 30)
	if count := r.URL.Query().Get("count"); count != "" {
		if parsed, err := strconv.ParseInt(count, 10, 32); err == nil && parsed > 0 {
			perPage = int32(parsed)
		}
	}

	grpcReq := &trainingpb.GetCategoriesRequest{
		Pagination: &commonpb.PaginationRequest{
			Page:    page,
			PerPage: perPage,
		},
	}

	resp, err := h.categoryClient.GetCategories(r.Context(), grpcReq)
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}

	writeJSON(w, http.StatusOK, buildCategoriesResponse(resp))
}

// GetCategory handles GET /api/tutorials/categories/{category:slug}
func (h *TrainingHandler) GetCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	slug := extractSlugFromPathTraining(r.URL.Path, "/api/tutorials/categories/")
	if slug == "" {
		writeError(w, http.StatusBadRequest, "category slug is required")
		return
	}

	grpcReq := &trainingpb.GetCategoryRequest{
		Slug: slug,
	}

	resp, err := h.categoryClient.GetCategory(r.Context(), grpcReq)
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}

	writeJSON(w, http.StatusOK, buildCategoryResponse(resp))
}

// GetCategoryVideos handles GET /api/tutorials/categories/{category:slug}/videos
func (h *TrainingHandler) GetCategoryVideos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract category slug from path like /api/tutorials/categories/{slug}/videos
	path := strings.TrimPrefix(r.URL.Path, "/api/tutorials/categories/")
	path = strings.TrimSuffix(path, "/videos")
	path = strings.Trim(path, "/")
	if path == "" {
		writeError(w, http.StatusBadRequest, "category slug is required")
		return
	}

	page, perPage := parsePagination(r, 1, 18)
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if parsed, err := strconv.ParseInt(pp, 10, 32); err == nil && parsed > 0 {
			perPage = int32(parsed)
		}
	}

	grpcReq := &trainingpb.GetCategoryVideosRequest{
		CategorySlug: path,
		Pagination: &commonpb.PaginationRequest{
			Page:    page,
			PerPage: perPage,
		},
	}

	resp, err := h.categoryClient.GetCategoryVideos(r.Context(), grpcReq)
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}

	writeJSON(w, http.StatusOK, buildVideosResponse(resp))
}

// GetSubCategory handles GET /api/tutorials/categories/{category:slug}/{subCategory:slug}
func (h *TrainingHandler) GetSubCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract slugs from path like /api/tutorials/categories/{category}/{subcategory}
	path := strings.TrimPrefix(r.URL.Path, "/api/tutorials/categories/")
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		writeError(w, http.StatusBadRequest, "category and subcategory slugs are required")
		return
	}

	grpcReq := &trainingpb.GetSubCategoryRequest{
		CategorySlug:    parts[0],
		SubCategorySlug: parts[1],
	}

	resp, err := h.categoryClient.GetSubCategory(r.Context(), grpcReq)
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}

	writeJSON(w, http.StatusOK, buildSubCategoryResponse(resp))
}

// GetComments handles GET /api/tutorials/{video}/comments
func (h *TrainingHandler) GetComments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract video ID from path like /api/tutorials/{video}/comments
	videoID := extractIDFromPathWithSuffix(r.URL.Path, "/api/tutorials/", "/comments")
	if videoID == 0 {
		writeError(w, http.StatusBadRequest, "invalid video ID")
		return
	}

	page, perPage := parsePagination(r, 1, 10)

	grpcReq := &trainingpb.GetCommentsRequest{
		VideoId: videoID,
		Pagination: &commonpb.PaginationRequest{
			Page:    page,
			PerPage: perPage,
		},
	}

	resp, err := h.commentClient.GetComments(r.Context(), grpcReq)
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}

	writeJSON(w, http.StatusOK, buildCommentsResponse(resp))
}

// AddComment handles POST /api/tutorials/{video}/comments
func (h *TrainingHandler) AddComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	videoID := extractIDFromPathWithSuffix(r.URL.Path, "/api/tutorials/", "/comments")
	if videoID == 0 {
		writeError(w, http.StatusBadRequest, "invalid video ID")
		return
	}

	var req struct {
		Content string `json:"content"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	if req.Content == "" {
		writeValidationError(w, "content is required")
		return
	}

	grpcReq := &trainingpb.AddCommentRequest{
		VideoId: videoID,
		UserId:  userCtx.UserID,
		Content: req.Content,
	}

	resp, err := h.commentClient.AddComment(r.Context(), grpcReq)
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}

	writeJSON(w, http.StatusOK, buildCommentResponse(resp))
}

// UpdateComment handles PUT /api/tutorials/{video}/comments/{comment}
func (h *TrainingHandler) UpdateComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Extract video ID and comment ID from path
	commentID := extractCommentIDFromPath(r.URL.Path)
	if commentID == 0 {
		writeError(w, http.StatusBadRequest, "invalid comment ID")
		return
	}

	var req struct {
		Content string `json:"content"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	if req.Content == "" {
		writeValidationError(w, "content is required")
		return
	}

	grpcReq := &trainingpb.UpdateCommentRequest{
		CommentId: commentID,
		UserId:    userCtx.UserID,
		Content:   req.Content,
	}

	resp, err := h.commentClient.UpdateComment(r.Context(), grpcReq)
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}

	writeJSON(w, http.StatusOK, buildCommentResponse(resp))
}

// DeleteComment handles DELETE /api/tutorials/{video}/comments/{comment}
func (h *TrainingHandler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	commentID := extractCommentIDFromPath(r.URL.Path)
	if commentID == 0 {
		writeError(w, http.StatusBadRequest, "invalid comment ID")
		return
	}

	grpcReq := &trainingpb.DeleteCommentRequest{
		CommentId: commentID,
		UserId:    userCtx.UserID,
	}

	_, err = h.commentClient.DeleteComment(r.Context(), grpcReq)
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

// AddCommentInteraction handles POST /api/tutorials/{video}/comments/{comment}/interactions
func (h *TrainingHandler) AddCommentInteraction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	commentID := extractCommentIDFromPath(r.URL.Path)
	if commentID == 0 {
		writeError(w, http.StatusBadRequest, "invalid comment ID")
		return
	}

	var req struct {
		Liked bool `json:"liked"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	ipAddress := getIPAddress(r)

	grpcReq := &trainingpb.AddCommentInteractionRequest{
		CommentId: commentID,
		UserId:    userCtx.UserID,
		Liked:     req.Liked,
		IpAddress: ipAddress,
	}

	_, err = h.commentClient.AddCommentInteraction(r.Context(), grpcReq)
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

// ReportComment handles POST /api/tutorials/{video}/comments/{comment}/report
func (h *TrainingHandler) ReportComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Extract video ID and comment ID from path
	videoID := extractIDFromPathWithSuffix(r.URL.Path, "/api/tutorials/", "/comments")
	commentID := extractCommentIDFromPath(r.URL.Path)
	if videoID == 0 || commentID == 0 {
		writeError(w, http.StatusBadRequest, "invalid video or comment ID")
		return
	}

	var req struct {
		Content string `json:"content"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	if req.Content == "" {
		writeValidationError(w, "content is required")
		return
	}

	grpcReq := &trainingpb.ReportCommentRequest{
		CommentId: commentID,
		UserId:    userCtx.UserID,
		Content:   req.Content,
	}

	_, err = h.commentClient.ReportComment(r.Context(), grpcReq)
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

// GetReplies handles GET /api/comments/{comment}/replies
func (h *TrainingHandler) GetReplies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract comment ID from path like /api/comments/{comment}/replies
	commentID := extractIDFromPathWithSuffix(r.URL.Path, "/api/comments/", "/replies")
	if commentID == 0 {
		writeError(w, http.StatusBadRequest, "invalid comment ID")
		return
	}

	page, perPage := parsePagination(r, 1, 10)

	grpcReq := &trainingpb.GetRepliesRequest{
		CommentId: commentID,
		Pagination: &commonpb.PaginationRequest{
			Page:    page,
			PerPage: perPage,
		},
	}

	resp, err := h.replyClient.GetReplies(r.Context(), grpcReq)
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}

	writeJSON(w, http.StatusOK, buildRepliesResponse(resp))
}

// AddReply handles POST /api/comments/{comment}/reply
func (h *TrainingHandler) AddReply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Extract comment ID from path like /api/comments/{comment}/reply
	commentID := extractIDFromPathWithSuffix(r.URL.Path, "/api/comments/", "/reply")
	if commentID == 0 {
		writeError(w, http.StatusBadRequest, "invalid comment ID")
		return
	}

	var req struct {
		Content string `json:"content"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	if req.Content == "" {
		writeValidationError(w, "content is required")
		return
	}

	grpcReq := &trainingpb.AddReplyRequest{
		ParentCommentId: commentID,
		UserId:          userCtx.UserID,
		Content:         req.Content,
	}

	resp, err := h.replyClient.AddReply(r.Context(), grpcReq)
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}

	writeJSON(w, http.StatusOK, buildCommentResponse(resp))
}

// UpdateReply handles PUT /api/comments/{comment}/replies/{reply}
func (h *TrainingHandler) UpdateReply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	// Extract reply ID from path like /api/comments/{comment}/replies/{reply}
	replyID := extractReplyIDFromPath(r.URL.Path)
	if replyID == 0 {
		writeError(w, http.StatusBadRequest, "invalid reply ID")
		return
	}

	var req struct {
		Content string `json:"content"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	if req.Content == "" {
		writeValidationError(w, "content is required")
		return
	}

	grpcReq := &trainingpb.UpdateReplyRequest{
		ReplyId: replyID,
		UserId:  userCtx.UserID,
		Content: req.Content,
	}

	resp, err := h.replyClient.UpdateReply(r.Context(), grpcReq)
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}

	writeJSON(w, http.StatusOK, buildCommentResponse(resp))
}

// DeleteReply handles DELETE /api/comments/{comment}/replies/{reply}
func (h *TrainingHandler) DeleteReply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	replyID := extractReplyIDFromPath(r.URL.Path)
	if replyID == 0 {
		writeError(w, http.StatusBadRequest, "invalid reply ID")
		return
	}

	grpcReq := &trainingpb.DeleteReplyRequest{
		ReplyId: replyID,
		UserId:  userCtx.UserID,
	}

	_, err = h.replyClient.DeleteReply(r.Context(), grpcReq)
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

// AddReplyInteraction handles POST /api/comments/{comment}/replies/{reply}/interactions
func (h *TrainingHandler) AddReplyInteraction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get user from context (set by auth middleware)
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	replyID := extractReplyIDFromPath(r.URL.Path)
	if replyID == 0 {
		writeError(w, http.StatusBadRequest, "invalid reply ID")
		return
	}

	var req struct {
		Liked bool `json:"liked"`
	}

	if err := decodeRequestBody(r, &req); err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	ipAddress := getIPAddress(r)

	grpcReq := &trainingpb.AddReplyInteractionRequest{
		ReplyId:   replyID,
		UserId:    userCtx.UserID,
		Liked:     req.Liked,
		IpAddress: ipAddress,
	}

	_, err = h.replyClient.AddReplyInteraction(r.Context(), grpcReq)
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

// Helper functions
func parsePagination(r *http.Request, defaultPage, defaultPerPage int32) (int32, int32) {
	page := defaultPage
	perPage := defaultPerPage

	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.ParseInt(p, 10, 32); err == nil && parsed > 0 {
			page = int32(parsed)
		}
	}
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if parsed, err := strconv.ParseInt(pp, 10, 32); err == nil && parsed > 0 {
			perPage = int32(parsed)
		}
	}

	return page, perPage
}

func extractSlugFromPathTraining(path, prefix string) string {
	path = strings.TrimPrefix(path, prefix)
	path = strings.Trim(path, "/")
	return path
}

func extractIDFromPathWithSuffix(path, prefix, suffix string) uint64 {
	path = strings.TrimPrefix(path, prefix)
	path = strings.TrimSuffix(path, suffix)
	path = strings.Trim(path, "/")
	id, _ := strconv.ParseUint(path, 10, 64)
	return id
}

func getIPAddress(r *http.Request) string {
	ip := r.Header.Get("X-Forwarded-For")
	if ip == "" {
		ip = r.Header.Get("X-Real-IP")
	}
	if ip == "" {
		ip = r.RemoteAddr
	}
	return ip
}

func extractCommentIDFromPath(path string) uint64 {
	// Extract comment ID from paths like:
	// /api/tutorials/{video}/comments/{comment}
	// /api/tutorials/{video}/comments/{comment}/interactions
	// /api/tutorials/{video}/comments/{comment}/report
	if strings.Contains(path, "/comments/") {
		parts := strings.Split(path, "/comments/")
		if len(parts) > 1 {
			commentPart := parts[1]
			commentPart = strings.Split(commentPart, "/")[0]
			id, _ := strconv.ParseUint(commentPart, 10, 64)
			return id
		}
	}
	return 0
}

func extractReplyIDFromPath(path string) uint64 {
	// Extract reply ID from paths like:
	// /api/comments/{comment}/replies/{reply}
	// /api/comments/{comment}/replies/{reply}/interactions
	if strings.Contains(path, "/replies/") {
		parts := strings.Split(path, "/replies/")
		if len(parts) > 1 {
			replyPart := parts[1]
			replyPart = strings.Split(replyPart, "/")[0]
			id, _ := strconv.ParseUint(replyPart, 10, 64)
			return id
		}
	}
	return 0
}

func buildVideoResponse(video *trainingpb.VideoResponse) map[string]interface{} {
	// Build response matching VideoTutorialResource format
	resp := map[string]interface{}{
		"id":          video.Id,
		"title":       video.Title,
		"slug":        video.Slug,
		"description": video.Description,
		"image_url":   video.ImageUrl,
		"video_url":   video.VideoUrl,
		"created_at":  video.CreatedAt,
	}

	// Add creator
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

	// Add category
	if video.Category != nil {
		resp["category"] = map[string]interface{}{
			"id":   video.Category.Id,
			"name": video.Category.Name,
			"slug": video.Category.Slug,
		}
	}

	// Add subcategory
	if video.SubCategory != nil {
		resp["sub_category"] = map[string]interface{}{
			"id":   video.SubCategory.Id,
			"name": video.SubCategory.Name,
			"slug": video.SubCategory.Slug,
		}
	}

	// Add stats
	if video.Stats != nil {
		resp["views_count"] = video.Stats.ViewsCount
		resp["likes_count"] = video.Stats.LikesCount
		resp["dislikes_count"] = video.Stats.DislikesCount
		resp["comments_count"] = video.Stats.CommentsCount
	}

	return resp
}

func buildVideosResponse(resp *trainingpb.VideosResponse) map[string]interface{} {
	videos := make([]map[string]interface{}, 0, len(resp.Videos))
	for _, video := range resp.Videos {
		videos = append(videos, buildVideoResponse(video))
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

func buildModalVideoResponse(video *trainingpb.VideoResponse) map[string]interface{} {
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

func buildCategoryResponse(category *trainingpb.CategoryResponse) map[string]interface{} {
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
			subCats = append(subCats, buildSubCategoryInfoJSON(subCat))
		}
		resp["subcategories"] = subCats
	}

	return resp
}

func buildCategoriesResponse(resp *trainingpb.CategoriesResponse) map[string]interface{} {
	categories := make([]map[string]interface{}, 0, len(resp.Categories))
	for _, category := range resp.Categories {
		categories = append(categories, buildCategoryResponse(category))
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

func buildSubCategoryResponse(subCategory *trainingpb.SubCategoryResponse) map[string]interface{} {
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

func buildSubCategoryInfoJSON(subCategory *trainingpb.SubCategoryInfo) map[string]interface{} {
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

func buildCommentResponse(comment *trainingpb.CommentResponse) map[string]interface{} {
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

	if comment.ParentId > 0 {
		resp["is_reply"] = true
	} else {
		resp["is_reply"] = false
	}

	return resp
}

func buildCommentsResponse(resp *trainingpb.CommentsResponse) map[string]interface{} {
	comments := make([]map[string]interface{}, 0, len(resp.Comments))
	for _, comment := range resp.Comments {
		comments = append(comments, buildCommentResponse(comment))
	}

	result := map[string]interface{}{
		"data": comments,
	}

	if resp.Pagination != nil {
		result["links"] = map[string]interface{}{
			"next": nil, // Simple pagination - would need to calculate next URL
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

func buildRepliesResponse(resp *trainingpb.RepliesResponse) map[string]interface{} {
	replies := make([]map[string]interface{}, 0, len(resp.Replies))
	for _, reply := range resp.Replies {
		replies = append(replies, buildCommentResponse(reply))
	}

	result := map[string]interface{}{
		"data": replies,
	}

	if resp.Pagination != nil {
		result["links"] = map[string]interface{}{
			"next": nil, // Simple pagination
		}
	}

	return result
}

func writeGRPCErrorTraining(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	switch st.Code() {
	case codes.NotFound:
		writeError(w, http.StatusNotFound, st.Message())
	case codes.InvalidArgument:
		writeValidationError(w, st.Message())
	case codes.Unauthenticated:
		writeError(w, http.StatusUnauthorized, st.Message())
	case codes.PermissionDenied:
		writeError(w, http.StatusForbidden, st.Message())
	default:
		writeError(w, http.StatusInternalServerError, st.Message())
	}
}
