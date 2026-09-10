package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	commonpb "metarang/shared/pb/common"
	trainingpb "metarang/shared/pb/training"
	"metarang/training-service/internal/handler"
)

func TestHTTPContract_GetVideoSuccessAndFilters(t *testing.T) {
	liked := true
	video := &mockVideoAPI{
		GetVideoFunc: func(_ context.Context, req *trainingpb.GetVideoRequest) (*trainingpb.VideoResponse, error) {
			if req.Slug != "intro" || req.UserId != 2 || req.IpAddress == "" {
				t.Fatalf("req=%+v", req)
			}
			return &trainingpb.VideoResponse{
				Id: 1, Title: "Intro", Slug: "intro", VideoUrl: "/v.mp4", ImageUrl: "/i.jpg",
				Creator:         &commonpb.UserBasic{Name: "C", Code: "c"},
				Stats:           &trainingpb.VideoStats{ViewsCount: 10, LikesCount: 2, DislikesCount: 1, CommentsCount: 3},
				UserInteraction: &liked,
			}, nil
		},
		GetVideosFunc: func(_ context.Context, req *trainingpb.GetVideosRequest) (*trainingpb.VideosResponse, error) {
			if req.CategoryId != 3 || req.SubCategoryId != 4 {
				t.Fatalf("filters=%+v", req)
			}
			return &trainingpb.VideosResponse{
				Videos:     []*trainingpb.VideoResponse{{Id: 9, Title: "Filtered", Slug: "filtered"}},
				Pagination: &commonpb.PaginationMeta{CurrentPage: 1, PerPage: 18, Total: 1, LastPage: 1},
			}, nil
		},
		GetVideoByFileNameFunc: func(_ context.Context, req *trainingpb.GetVideoByFileNameRequest) (*trainingpb.VideoResponse, error) {
			if req.FileName != "clip.mp4" {
				t.Fatalf("file=%s", req.FileName)
			}
			return &trainingpb.VideoResponse{
				Id: 3, Title: "Clip", VideoUrl: "/uploads/clip.mp4", CreatorCode: "c1",
				Stats: &trainingpb.VideoStats{ViewsCount: 5, LikesCount: 1, DislikesCount: 0},
			}, nil
		},
	}
	h := handler.NewHTTPTrainingHandler(video, &mockCategoryAPI{}, &mockCommentAPI{}, &mockReplyAPI{})
	optional := func(next http.Handler) http.Handler {
		return withUser(2)(forwardedForMW("203.0.113.1")(next))
	}
	mux := newTestMux(h, identityMW, optional)

	rr := doJSON(mux, http.MethodGet, "/api/tutorials/intro", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("get video code=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"user_interaction":{"liked":true}`) {
		t.Fatalf("body=%s", rr.Body.String())
	}

	rr = doJSON(mux, http.MethodGet, "/api/tutorials?category_id=3&sub_category_id=4", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("list code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doJSON(mux, http.MethodPost, "/api/video-tutorials", `{"url":"clip.mp4"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("modal code=%d body=%s", rr.Code, rr.Body.String())
	}
	var modal map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &modal); err != nil {
		t.Fatal(err)
	}
	data, _ := modal["data"].(map[string]interface{})
	if data["views"] == nil || data["likes"] == nil {
		t.Fatalf("expected stats in modal payload: %v", data)
	}
}

func forwardedForMW(ip string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Set("X-Forwarded-For", ip)
			next.ServeHTTP(w, r)
		})
	}
}

func TestHTTPContract_GetCommentsAndRepliesJSON(t *testing.T) {
	liked := true
	comment := &mockCommentAPI{
		GetCommentsFunc: func(_ context.Context, req *trainingpb.GetCommentsRequest) (*trainingpb.CommentsResponse, error) {
			if req.VideoId != 5 || req.UserId != 2 {
				t.Fatalf("comments req=%+v", req)
			}
			return &trainingpb.CommentsResponse{
				Comments: []*trainingpb.CommentResponse{
					{
						Id: 11, VideoId: 5, Content: "hello", CreatedAt: "1403-01-01", UpdatedAt: "1403-01-02",
						User:            &commonpb.UserBasic{Id: 3, Name: "Ann", Code: "a1", ProfilePhoto: "p.jpg"},
						Stats:           &trainingpb.CommentStats{LikesCount: 2, DislikesCount: 1, RepliesCount: 4},
						UserInteraction: &liked,
					},
					{Id: 12, VideoId: 5, Content: "plain", CreatedAt: "1403-01-03"},
				},
				Pagination: &commonpb.PaginationMeta{CurrentPage: 1, PerPage: 10, Total: 2, LastPage: 1},
			}, nil
		},
	}
	reply := &mockReplyAPI{
		GetRepliesFunc: func(_ context.Context, req *trainingpb.GetRepliesRequest) (*trainingpb.RepliesResponse, error) {
			if req.CommentId != 11 {
				t.Fatalf("replies req=%+v", req)
			}
			return &trainingpb.RepliesResponse{
				Replies: []*trainingpb.CommentResponse{
					{
						Id: 21, VideoId: 5, ParentId: 11, Content: "r", CreatedAt: "1403-01-04",
						User:  &commonpb.UserBasic{Id: 4, Name: "Bob", Code: "b1"},
						Stats: &trainingpb.CommentStats{LikesCount: 1},
					},
				},
				Pagination: &commonpb.PaginationMeta{CurrentPage: 1, PerPage: 10, Total: 1, LastPage: 1},
			}, nil
		},
	}
	h := handler.NewHTTPTrainingHandler(&mockVideoAPI{}, &mockCategoryAPI{}, comment, reply)
	mux := newTestMux(h, identityMW, withUser(2))

	rr := doJSON(mux, http.MethodGet, "/api/tutorials/5/comments", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("get comments code=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"user_interaction":true`) || !strings.Contains(rr.Body.String(), `"is_reply":false`) {
		t.Fatalf("comments body=%s", rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"current_page"`) {
		t.Fatalf("expected pagination meta: %s", rr.Body.String())
	}

	rr = doJSON(mux, http.MethodGet, "/api/comments/11/replies", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("get replies code=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"is_reply":true`) || !strings.Contains(rr.Body.String(), `"parent_id":11`) {
		t.Fatalf("replies body=%s", rr.Body.String())
	}
}

func TestHTTPContract_DirectHandlerBranches(t *testing.T) {
	video := &mockVideoAPI{
		GetVideoFunc: func(context.Context, *trainingpb.GetVideoRequest) (*trainingpb.VideoResponse, error) {
			return nil, status.Error(codes.NotFound, "missing")
		},
		GetVideosFunc: func(context.Context, *trainingpb.GetVideosRequest) (*trainingpb.VideosResponse, error) {
			return &trainingpb.VideosResponse{}, nil
		},
	}
	cat := &mockCategoryAPI{
		GetCategoryFunc: func(context.Context, *trainingpb.GetCategoryRequest) (*trainingpb.CategoryResponse, error) {
			return &trainingpb.CategoryResponse{Id: 1, Name: "C", Slug: "c", IconUrl: "/i.svg", Description: "d", ImageUrl: "/img.png"}, nil
		},
		GetCategoryVideosFunc: func(context.Context, *trainingpb.GetCategoryVideosRequest) (*trainingpb.VideosResponse, error) {
			return &trainingpb.VideosResponse{
				Videos:     []*trainingpb.VideoResponse{{Id: 2, Title: "V", Slug: "v"}},
				Pagination: &commonpb.PaginationMeta{CurrentPage: 2, PerPage: 5, Total: 1, LastPage: 1},
			}, nil
		},
		GetSubCategoryFunc: func(context.Context, *trainingpb.GetSubCategoryRequest) (*trainingpb.SubCategoryResponse, error) {
			return &trainingpb.SubCategoryResponse{
				Id: 3, Name: "S", Slug: "s", Description: "d", ImageUrl: "/s.png", IconUrl: "/s.svg",
				Category: &trainingpb.CategoryInfo{Id: 1, Name: "C", Slug: "c"},
			}, nil
		},
		GetCategoriesFunc: func(context.Context, *trainingpb.GetCategoriesRequest) (*trainingpb.CategoriesResponse, error) {
			return &trainingpb.CategoriesResponse{
				Categories: []*trainingpb.CategoryResponse{{Id: 1, Name: "C", Slug: "c"}},
				Pagination: &commonpb.PaginationMeta{CurrentPage: 1, PerPage: 30, Total: 1, LastPage: 1},
			}, nil
		},
	}
	h := handler.NewHTTPTrainingHandler(video, cat, &mockCommentAPI{}, &mockReplyAPI{})

	rr := httptest.NewRecorder()
	h.GetVideo(rr, httptest.NewRequest(http.MethodPost, "/api/tutorials/intro", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("get video method code=%d", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.GetVideo(rr, httptest.NewRequest(http.MethodGet, "/api/tutorials/", nil))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("empty slug code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.GetVideo(rr, httptest.NewRequest(http.MethodGet, "/api/tutorials/missing", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("video not found code=%d", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.GetVideos(rr, httptest.NewRequest(http.MethodPost, "/api/tutorials", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("list method code=%d", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.GetCategory(rr, httptest.NewRequest(http.MethodPost, "/api/tutorials/categories/c", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("category method code=%d", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.GetCategory(rr, httptest.NewRequest(http.MethodGet, "/api/tutorials/categories/", nil))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("empty category slug code=%d", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.GetCategoryVideos(rr, httptest.NewRequest(http.MethodGet, "/api/tutorials/categories/c/videos?page=2&per_page=5", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"slug":"v"`) {
		t.Fatalf("category videos code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.GetCategoryVideos(rr, httptest.NewRequest(http.MethodPost, "/api/tutorials/categories/c/videos", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("category videos method code=%d", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.GetSubCategory(rr, httptest.NewRequest(http.MethodPost, "/api/tutorials/categories/c/s", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("subcategory method code=%d", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.GetSubCategory(rr, httptest.NewRequest(http.MethodGet, "/api/tutorials/categories/", nil))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("empty subcategory slug code=%d", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.GetCategoryVideos(rr, httptest.NewRequest(http.MethodGet, "/api/tutorials/categories/", nil))
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("empty category videos slug code=%d", rr.Code)
	}

	mux := newTestMux(h, identityMW, identityMW)
	rr = doJSON(mux, http.MethodGet, "/health", "")
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"ok"`) {
		t.Fatalf("health code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = httptest.NewRecorder()
	h.GetComments(rr, httptest.NewRequest(http.MethodPost, "/api/tutorials/5/comments", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("comments method code=%d", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.GetReplies(rr, httptest.NewRequest(http.MethodPost, "/api/comments/11/replies", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("replies method code=%d", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.SearchVideos(rr, httptest.NewRequest(http.MethodGet, "/api/tutorials/search", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("search method code=%d", rr.Code)
	}

	rr = httptest.NewRecorder()
	h.GetCategories(rr, httptest.NewRequest(http.MethodPost, "/api/tutorials/categories", nil))
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("categories method code=%d", rr.Code)
	}

	plain := &mockVideoAPI{
		GetVideosFunc: func(context.Context, *trainingpb.GetVideosRequest) (*trainingpb.VideosResponse, error) {
			return nil, errors.New("not grpc")
		},
	}
	h = handler.NewHTTPTrainingHandler(plain, cat, &mockCommentAPI{}, &mockReplyAPI{})
	rr = httptest.NewRecorder()
	h.GetVideos(rr, httptest.NewRequest(http.MethodGet, "/api/tutorials", nil))
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("plain error code=%d", rr.Code)
	}
}
