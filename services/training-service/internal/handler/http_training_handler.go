package handler

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"

	commonpb "metarang/shared/pb/common"
	trainingpb "metarang/shared/pb/training"
	"metarang/shared/pkg/sentry"
	"metarang/training-service/internal/middleware"
)

// videoAPI is the subset of VideoService RPCs used by the HTTP layer.
type videoAPI interface {
	GetVideos(context.Context, *trainingpb.GetVideosRequest) (*trainingpb.VideosResponse, error)
	GetVideo(context.Context, *trainingpb.GetVideoRequest) (*trainingpb.VideoResponse, error)
	GetVideoByFileName(context.Context, *trainingpb.GetVideoByFileNameRequest) (*trainingpb.VideoResponse, error)
	SearchVideos(context.Context, *trainingpb.SearchVideosRequest) (*trainingpb.VideosResponse, error)
	AddInteraction(context.Context, *trainingpb.AddInteractionRequest) (*commonpb.Empty, error)
}

// categoryAPI is the subset of CategoryService RPCs used by the HTTP layer.
type categoryAPI interface {
	GetCategories(context.Context, *trainingpb.GetCategoriesRequest) (*trainingpb.CategoriesResponse, error)
	GetCategory(context.Context, *trainingpb.GetCategoryRequest) (*trainingpb.CategoryResponse, error)
	GetSubCategory(context.Context, *trainingpb.GetSubCategoryRequest) (*trainingpb.SubCategoryResponse, error)
	GetCategoryVideos(context.Context, *trainingpb.GetCategoryVideosRequest) (*trainingpb.VideosResponse, error)
}

// commentAPI is the subset of CommentService RPCs used by the HTTP layer.
type commentAPI interface {
	GetComments(context.Context, *trainingpb.GetCommentsRequest) (*trainingpb.CommentsResponse, error)
	AddComment(context.Context, *trainingpb.AddCommentRequest) (*trainingpb.CommentResponse, error)
	UpdateComment(context.Context, *trainingpb.UpdateCommentRequest) (*trainingpb.CommentResponse, error)
	DeleteComment(context.Context, *trainingpb.DeleteCommentRequest) (*commonpb.Empty, error)
	AddCommentInteraction(context.Context, *trainingpb.AddCommentInteractionRequest) (*commonpb.Empty, error)
	ReportComment(context.Context, *trainingpb.ReportCommentRequest) (*commonpb.Empty, error)
}

// replyAPI is the subset of ReplyService RPCs used by the HTTP layer.
type replyAPI interface {
	GetReplies(context.Context, *trainingpb.GetRepliesRequest) (*trainingpb.RepliesResponse, error)
	AddReply(context.Context, *trainingpb.AddReplyRequest) (*trainingpb.CommentResponse, error)
	UpdateReply(context.Context, *trainingpb.UpdateReplyRequest) (*trainingpb.CommentResponse, error)
	DeleteReply(context.Context, *trainingpb.DeleteReplyRequest) (*commonpb.Empty, error)
	AddReplyInteraction(context.Context, *trainingpb.AddReplyInteractionRequest) (*commonpb.Empty, error)
}

// HTTPTrainingHandler serves Kong-facing REST routes for training-service.
type HTTPTrainingHandler struct {
	video    videoAPI
	category categoryAPI
	comment  commentAPI
	reply    replyAPI
}

// NewHTTPTrainingHandler wraps gRPC handlers for local HTTP use.
func NewHTTPTrainingHandler(video videoAPI, category categoryAPI, comment commentAPI, reply replyAPI) *HTTPTrainingHandler {
	return &HTTPTrainingHandler{
		video:    video,
		category: category,
		comment:  comment,
		reply:    reply,
	}
}

// RegisterHTTPRoutes registers training REST routes and /health.
func (h *HTTPTrainingHandler) RegisterHTTPRoutes(
	mux *http.ServeMux,
	authMiddleware func(http.Handler) http.Handler,
	optionalAuthMiddleware func(http.Handler) http.Handler,
) {
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	mux.Handle("/api/video-tutorials", optionalAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			h.GetVideoByFileName(w, r)
			return
		}
		http.NotFound(w, r)
	})))

	mux.Handle("/api/tutorials/categories", optionalAuthMiddleware(http.HandlerFunc(h.GetCategories)))
	mux.Handle("/api/tutorials/categories/", optionalAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		categoryPath := strings.TrimPrefix(r.URL.Path, "/api/tutorials/categories/")
		parts := strings.Split(categoryPath, "/")

		if len(parts) == 1 && parts[0] != "" {
			if r.Method == http.MethodGet {
				h.GetCategory(w, r)
			} else {
				http.NotFound(w, r)
			}
		} else if len(parts) == 2 && parts[1] == "videos" {
			if r.Method == http.MethodGet {
				h.GetCategoryVideos(w, r)
			} else {
				http.NotFound(w, r)
			}
		} else if len(parts) == 2 && parts[1] != "" {
			if r.Method == http.MethodGet {
				h.GetSubCategory(w, r)
			} else {
				http.NotFound(w, r)
			}
		} else {
			http.NotFound(w, r)
		}
	})))

	mux.Handle("/api/tutorials/search", optionalAuthMiddleware(http.HandlerFunc(h.SearchVideos)))

	mux.Handle("/api/tutorials/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		videoPath := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/tutorials/"), "/")
		parts := strings.Split(videoPath, "/")

		if len(parts) == 2 && parts[1] == "interactions" && r.Method == http.MethodPost {
			authMiddleware(http.HandlerFunc(h.AddInteraction)).ServeHTTP(w, r)
			return
		}

		optionalAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if videoPath == "" {
				if r.Method == http.MethodGet {
					h.GetVideos(w, r)
				} else {
					http.NotFound(w, r)
				}
				return
			}

			if len(parts) >= 2 && parts[1] == "comments" {
				if len(parts) >= 4 {
					action := parts[3]
					switch action {
					case "interactions":
						if r.Method == http.MethodPost {
							authMiddleware(http.HandlerFunc(h.AddCommentInteraction)).ServeHTTP(w, r)
						} else {
							http.NotFound(w, r)
						}
					case "like":
						if r.Method == http.MethodPost {
							authMiddleware(http.HandlerFunc(h.AddCommentLike)).ServeHTTP(w, r)
						} else {
							http.NotFound(w, r)
						}
					case "dislike":
						if r.Method == http.MethodPost {
							authMiddleware(http.HandlerFunc(h.AddCommentDislike)).ServeHTTP(w, r)
						} else {
							http.NotFound(w, r)
						}
					case "report":
						if r.Method == http.MethodPost {
							authMiddleware(http.HandlerFunc(h.ReportComment)).ServeHTTP(w, r)
						} else {
							http.NotFound(w, r)
						}
					default:
						http.NotFound(w, r)
					}
				} else if len(parts) == 3 {
					switch r.Method {
					case http.MethodPut, http.MethodPost:
						authMiddleware(http.HandlerFunc(h.UpdateComment)).ServeHTTP(w, r)
					case http.MethodDelete:
						authMiddleware(http.HandlerFunc(h.DeleteComment)).ServeHTTP(w, r)
					default:
						http.NotFound(w, r)
					}
				} else if len(parts) == 2 {
					switch r.Method {
					case http.MethodGet:
						h.GetComments(w, r)
					case http.MethodPost:
						authMiddleware(http.HandlerFunc(h.AddComment)).ServeHTTP(w, r)
					default:
						http.NotFound(w, r)
					}
				} else {
					http.NotFound(w, r)
				}
				return
			}

			if len(parts) == 1 {
				if r.Method == http.MethodGet {
					h.GetVideo(w, r)
				} else {
					http.NotFound(w, r)
				}
				return
			}

			http.NotFound(w, r)
		})).ServeHTTP(w, r)
	}))

	mux.Handle("/api/tutorials", optionalAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			h.GetVideos(w, r)
			return
		}
		http.NotFound(w, r)
	})))

	mux.Handle("/api/comments/", optionalAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		commentPath := strings.TrimPrefix(r.URL.Path, "/api/comments/")
		parts := strings.Split(commentPath, "/")

		if len(parts) >= 2 && parts[1] == "replies" {
			if len(parts) == 2 {
				if r.Method == http.MethodGet {
					h.GetReplies(w, r)
				} else {
					http.NotFound(w, r)
				}
			} else if len(parts) == 3 {
				switch r.Method {
				case http.MethodPut:
					h.UpdateReply(w, r)
				case http.MethodDelete:
					h.DeleteReply(w, r)
				default:
					http.NotFound(w, r)
				}
			} else if len(parts) == 4 && parts[3] == "interactions" {
				if r.Method == http.MethodPost {
					h.AddReplyInteraction(w, r)
				} else {
					http.NotFound(w, r)
				}
			} else {
				http.NotFound(w, r)
			}
		} else if len(parts) == 2 && parts[1] == "reply" {
			if r.Method == http.MethodPost {
				authMiddleware(http.HandlerFunc(h.AddReply)).ServeHTTP(w, r)
			} else {
				http.NotFound(w, r)
			}
		} else {
			http.NotFound(w, r)
		}
	})))
}

// StartHTTPServer starts the public HTTP server (behind Kong).
func StartHTTPServer(
	httpHandler *HTTPTrainingHandler,
	port string,
	authMiddleware func(http.Handler) http.Handler,
	optionalAuthMiddleware func(http.Handler) http.Handler,
) error {
	mux := http.NewServeMux()
	httpHandler.RegisterHTTPRoutes(mux, authMiddleware, optionalAuthMiddleware)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: sentry.HTTPMiddleware(mux),
	}
	return server.ListenAndServe()
}

// GetVideos handles GET /api/tutorials
func (h *HTTPTrainingHandler) GetVideos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	page, perPage := parsePagination(r, 1, 18)
	grpcReq := &trainingpb.GetVideosRequest{
		Pagination: &commonpb.PaginationRequest{Page: page, PerPage: perPage},
	}

	if v := r.URL.Query().Get("category_id"); v != "" {
		if id, err := strconv.ParseUint(v, 10, 64); err == nil && id > 0 {
			grpcReq.CategoryId = id
		}
	}
	if v := r.URL.Query().Get("sub_category_id"); v != "" {
		if id, err := strconv.ParseUint(v, 10, 64); err == nil && id > 0 {
			grpcReq.SubCategoryId = id
		}
	}

	resp, err := h.video.GetVideos(r.Context(), grpcReq)
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}
	writeJSON(w, http.StatusOK, videosToJSON(resp))
}

// GetVideo handles GET /api/tutorials/{slug}
func (h *HTTPTrainingHandler) GetVideo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	slug := extractSlugFromPathTraining(r.URL.Path, "/api/tutorials/")
	if slug == "" {
		writeError(w, http.StatusBadRequest, "slug is required")
		return
	}

	grpcReq := &trainingpb.GetVideoRequest{
		Slug:      slug,
		UserId:    userIDFromOptional(r),
		IpAddress: getIPAddress(r),
	}

	resp, err := h.video.GetVideo(trainingContextWithUser(r), grpcReq)
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}
	writeJSON(w, http.StatusOK, videoToJSON(resp))
}

// SearchVideos handles POST /api/tutorials/search
func (h *HTTPTrainingHandler) SearchVideos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		SearchTerm string `json:"searchTerm"`
	}
	if err := decodeRequestBody(r, &req); err != nil {
		if isEOF(err) {
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
	resp, err := h.video.SearchVideos(r.Context(), &trainingpb.SearchVideosRequest{
		Query:      req.SearchTerm,
		Pagination: &commonpb.PaginationRequest{Page: page, PerPage: perPage},
	})
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}
	writeJSON(w, http.StatusOK, videosToJSON(resp))
}

// AddInteraction handles POST /api/tutorials/{video}/interactions
func (h *HTTPTrainingHandler) AddInteraction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, ok := requireUser(w, r)
	if !ok {
		return
	}

	videoID := extractIDFromPathWithSuffix(r.URL.Path, "/api/tutorials/", "/interactions")
	if videoID == 0 {
		writeError(w, http.StatusBadRequest, "invalid video ID")
		return
	}

	liked, err := parseLikedFromRequest(r)
	if err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "liked query parameter or request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	_, err = h.video.AddInteraction(r.Context(), &trainingpb.AddInteractionRequest{
		VideoId:   videoID,
		UserId:    userID,
		Liked:     liked,
		IpAddress: getIPAddress(r),
	})
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

// GetVideoByFileName handles POST /api/video-tutorials
func (h *HTTPTrainingHandler) GetVideoByFileName(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		URL string `json:"url"`
	}
	if err := decodeRequestBody(r, &req); err != nil {
		if isEOF(err) {
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

	resp, err := h.video.GetVideoByFileName(r.Context(), &trainingpb.GetVideoByFileNameRequest{
		FileName:  req.URL,
		IpAddress: getIPAddress(r),
	})
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data": modalVideoToJSON(resp),
	})
}

// GetCategories handles GET /api/tutorials/categories
func (h *HTTPTrainingHandler) GetCategories(w http.ResponseWriter, r *http.Request) {
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

	resp, err := h.category.GetCategories(r.Context(), &trainingpb.GetCategoriesRequest{
		Pagination: &commonpb.PaginationRequest{Page: page, PerPage: perPage},
	})
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}
	writeJSON(w, http.StatusOK, categoriesToJSON(resp))
}

// GetCategory handles GET /api/tutorials/categories/{slug}
func (h *HTTPTrainingHandler) GetCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	slug := extractSlugFromPathTraining(r.URL.Path, "/api/tutorials/categories/")
	if slug == "" {
		writeError(w, http.StatusBadRequest, "category slug is required")
		return
	}

	resp, err := h.category.GetCategory(r.Context(), &trainingpb.GetCategoryRequest{Slug: slug})
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}
	writeJSON(w, http.StatusOK, categoryToJSON(resp))
}

// GetCategoryVideos handles GET /api/tutorials/categories/{slug}/videos
func (h *HTTPTrainingHandler) GetCategoryVideos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	path := strings.Trim(strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/tutorials/categories/"), "/videos"), "/")
	if path == "" {
		writeError(w, http.StatusBadRequest, "category slug is required")
		return
	}

	page, perPage := parsePagination(r, 1, 18)
	resp, err := h.category.GetCategoryVideos(r.Context(), &trainingpb.GetCategoryVideosRequest{
		CategorySlug: path,
		Pagination:   &commonpb.PaginationRequest{Page: page, PerPage: perPage},
	})
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}
	writeJSON(w, http.StatusOK, videosToJSON(resp))
}

// GetSubCategory handles GET /api/tutorials/categories/{cat}/{sub}
func (h *HTTPTrainingHandler) GetSubCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/tutorials/categories/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 {
		writeError(w, http.StatusBadRequest, "category and subcategory slugs are required")
		return
	}

	resp, err := h.category.GetSubCategory(r.Context(), &trainingpb.GetSubCategoryRequest{
		CategorySlug:    parts[0],
		SubCategorySlug: parts[1],
	})
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}

	result := subCategoryToJSON(resp)
	h.attachSubCategoryVideos(r, resp, result)
	writeJSON(w, http.StatusOK, result)
}

func (h *HTTPTrainingHandler) attachSubCategoryVideos(r *http.Request, subCategory *trainingpb.SubCategoryResponse, resp map[string]interface{}) {
	if subCategory == nil || subCategory.Id == 0 {
		resp["videos"] = []map[string]interface{}{}
		return
	}
	if _, ok := resp["videos"]; ok {
		return
	}

	perPage := subCategory.VideosCount
	if perPage <= 0 {
		perPage = 1000
	}

	videosResp, err := h.video.GetVideos(r.Context(), &trainingpb.GetVideosRequest{
		SubCategoryId: subCategory.Id,
		Pagination:    &commonpb.PaginationRequest{Page: 1, PerPage: perPage},
	})
	if err != nil {
		resp["videos"] = []map[string]interface{}{}
		return
	}

	videos := make([]map[string]interface{}, 0, len(videosResp.Videos))
	for _, video := range videosResp.Videos {
		videos = append(videos, videoToJSON(video))
	}
	resp["videos"] = videos
}

// GetComments handles GET /api/tutorials/{video}/comments
func (h *HTTPTrainingHandler) GetComments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	videoID := extractIDFromPathWithSuffix(r.URL.Path, "/api/tutorials/", "/comments")
	if videoID == 0 {
		writeError(w, http.StatusBadRequest, "invalid video ID")
		return
	}

	page, perPage := parsePagination(r, 1, 10)
	resp, err := h.comment.GetComments(trainingContextWithUser(r), &trainingpb.GetCommentsRequest{
		VideoId:    videoID,
		UserId:     userIDFromOptional(r),
		Pagination: &commonpb.PaginationRequest{Page: page, PerPage: perPage},
	})
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}
	writeJSON(w, http.StatusOK, commentsToJSON(resp))
}

// AddComment handles POST /api/tutorials/{video}/comments
func (h *HTTPTrainingHandler) AddComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, ok := requireUser(w, r)
	if !ok {
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
		if isEOF(err) {
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

	resp, err := h.comment.AddComment(r.Context(), &trainingpb.AddCommentRequest{
		VideoId: videoID,
		UserId:  userID,
		Content: req.Content,
	})
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}
	writeJSON(w, http.StatusOK, commentToJSON(resp))
}

// AddCommentLike handles POST .../comments/{id}/like
func (h *HTTPTrainingHandler) AddCommentLike(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	q.Set("liked", "1")
	r.URL.RawQuery = q.Encode()
	h.AddCommentInteraction(w, r)
}

// AddCommentDislike handles POST .../comments/{id}/dislike
func (h *HTTPTrainingHandler) AddCommentDislike(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	q.Set("liked", "0")
	r.URL.RawQuery = q.Encode()
	h.AddCommentInteraction(w, r)
}

// UpdateComment handles PUT/POST /api/tutorials/{video}/comments/{comment}
func (h *HTTPTrainingHandler) UpdateComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, ok := requireUser(w, r)
	if !ok {
		return
	}

	commentID := extractCommentIDFromPath(r.URL.Path)
	if commentID == 0 {
		writeError(w, http.StatusBadRequest, "invalid comment ID")
		return
	}

	var req struct {
		Content string `json:"content"`
		Method  string `json:"_method"`
	}
	if err := decodeRequestBody(r, &req); err != nil {
		if isEOF(err) {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}
	if r.Method == http.MethodPost && req.Method != "" && !strings.EqualFold(req.Method, "put") {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if req.Content == "" {
		writeValidationError(w, "content is required")
		return
	}

	resp, err := h.comment.UpdateComment(r.Context(), &trainingpb.UpdateCommentRequest{
		CommentId: commentID,
		UserId:    userID,
		Content:   req.Content,
	})
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}
	writeJSON(w, http.StatusOK, commentToJSON(resp))
}

// DeleteComment handles DELETE /api/tutorials/{video}/comments/{comment}
func (h *HTTPTrainingHandler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, ok := requireUser(w, r)
	if !ok {
		return
	}

	commentID := extractCommentIDFromPath(r.URL.Path)
	if commentID == 0 {
		writeError(w, http.StatusBadRequest, "invalid comment ID")
		return
	}

	_, err := h.comment.DeleteComment(r.Context(), &trainingpb.DeleteCommentRequest{
		CommentId: commentID,
		UserId:    userID,
	})
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

// AddCommentInteraction handles POST .../comments/{comment}/interactions
func (h *HTTPTrainingHandler) AddCommentInteraction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, ok := requireUser(w, r)
	if !ok {
		return
	}

	commentID := extractCommentIDFromPath(r.URL.Path)
	if commentID == 0 {
		writeError(w, http.StatusBadRequest, "invalid comment ID")
		return
	}

	liked, err := parseLikedFromRequest(r)
	if err != nil {
		if err == io.EOF {
			writeError(w, http.StatusBadRequest, "liked query parameter or request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	_, err = h.comment.AddCommentInteraction(r.Context(), &trainingpb.AddCommentInteractionRequest{
		CommentId: commentID,
		UserId:    userID,
		Liked:     liked,
		IpAddress: getIPAddress(r),
	})
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

// ReportComment handles POST .../comments/{comment}/report
func (h *HTTPTrainingHandler) ReportComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, ok := requireUser(w, r)
	if !ok {
		return
	}

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
		if isEOF(err) {
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

	_, err := h.comment.ReportComment(r.Context(), &trainingpb.ReportCommentRequest{
		CommentId: commentID,
		UserId:    userID,
		Content:   req.Content,
	})
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

// GetReplies handles GET /api/comments/{comment}/replies
func (h *HTTPTrainingHandler) GetReplies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	commentID := extractIDFromPathWithSuffix(r.URL.Path, "/api/comments/", "/replies")
	if commentID == 0 {
		writeError(w, http.StatusBadRequest, "invalid comment ID")
		return
	}

	page, perPage := parsePagination(r, 1, 10)
	resp, err := h.reply.GetReplies(r.Context(), &trainingpb.GetRepliesRequest{
		CommentId:  commentID,
		Pagination: &commonpb.PaginationRequest{Page: page, PerPage: perPage},
	})
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}
	writeJSON(w, http.StatusOK, repliesToJSON(resp))
}

// AddReply handles POST /api/comments/{comment}/reply
func (h *HTTPTrainingHandler) AddReply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, ok := requireUser(w, r)
	if !ok {
		return
	}

	commentID := extractIDFromPathWithSuffix(r.URL.Path, "/api/comments/", "/reply")
	if commentID == 0 {
		writeError(w, http.StatusBadRequest, "invalid comment ID")
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := decodeRequestBody(r, &req); err != nil {
		if isEOF(err) {
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

	resp, err := h.reply.AddReply(r.Context(), &trainingpb.AddReplyRequest{
		ParentCommentId: commentID,
		UserId:          userID,
		Content:         req.Content,
	})
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}
	writeJSON(w, http.StatusOK, commentToJSON(resp))
}

// UpdateReply handles PUT /api/comments/{comment}/replies/{reply}
func (h *HTTPTrainingHandler) UpdateReply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, ok := requireUser(w, r)
	if !ok {
		return
	}

	replyID := extractReplyIDFromPath(r.URL.Path)
	if replyID == 0 {
		writeError(w, http.StatusBadRequest, "invalid reply ID")
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := decodeRequestBody(r, &req); err != nil {
		if isEOF(err) {
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

	resp, err := h.reply.UpdateReply(r.Context(), &trainingpb.UpdateReplyRequest{
		ReplyId: replyID,
		UserId:  userID,
		Content: req.Content,
	})
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}
	writeJSON(w, http.StatusOK, commentToJSON(resp))
}

// DeleteReply handles DELETE /api/comments/{comment}/replies/{reply}
func (h *HTTPTrainingHandler) DeleteReply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, ok := requireUser(w, r)
	if !ok {
		return
	}

	replyID := extractReplyIDFromPath(r.URL.Path)
	if replyID == 0 {
		writeError(w, http.StatusBadRequest, "invalid reply ID")
		return
	}

	_, err := h.reply.DeleteReply(r.Context(), &trainingpb.DeleteReplyRequest{
		ReplyId: replyID,
		UserId:  userID,
	})
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

// AddReplyInteraction handles POST .../replies/{reply}/interactions
func (h *HTTPTrainingHandler) AddReplyInteraction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID, ok := requireUser(w, r)
	if !ok {
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
		if isEOF(err) {
			writeError(w, http.StatusBadRequest, "request body is required")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return
	}

	_, err := h.reply.AddReplyInteraction(r.Context(), &trainingpb.AddReplyInteractionRequest{
		ReplyId:   replyID,
		UserId:    userID,
		Liked:     req.Liked,
		IpAddress: getIPAddress(r),
	})
	if err != nil {
		writeGRPCErrorTraining(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{})
}

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
	return strings.Trim(path, "/")
}

func extractIDFromPathWithSuffix(path, prefix, suffix string) uint64 {
	path = strings.TrimPrefix(path, prefix)
	path = strings.TrimSuffix(path, suffix)
	path = strings.Trim(path, "/")
	// For report/interactions nested under comments, strip remaining path segments.
	if idx := strings.Index(path, "/"); idx != -1 {
		path = path[:idx]
	}
	id, _ := strconv.ParseUint(path, 10, 64)
	return id
}

func extractCommentIDFromPath(path string) uint64 {
	if strings.Contains(path, "/comments/") {
		parts := strings.Split(path, "/comments/")
		if len(parts) > 1 {
			commentPart := strings.Split(parts[1], "/")[0]
			id, _ := strconv.ParseUint(commentPart, 10, 64)
			return id
		}
	}
	return 0
}

func extractReplyIDFromPath(path string) uint64 {
	if strings.Contains(path, "/replies/") {
		parts := strings.Split(path, "/replies/")
		if len(parts) > 1 {
			replyPart := strings.Split(parts[1], "/")[0]
			id, _ := strconv.ParseUint(replyPart, 10, 64)
			return id
		}
	}
	return 0
}

func userIDFromOptional(r *http.Request) uint64 {
	userCtx, err := middleware.GetUserFromRequest(r)
	if err != nil {
		return 0
	}
	return userCtx.UserID
}
