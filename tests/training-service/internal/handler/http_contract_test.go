package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	commonpb "metarang/shared/pb/common"
	trainingpb "metarang/shared/pb/training"
	authpkg "metarang/shared/pkg/auth"
	"metarang/training-service/internal/handler"
)

type mockVideoAPI struct {
	GetVideosFunc          func(context.Context, *trainingpb.GetVideosRequest) (*trainingpb.VideosResponse, error)
	GetVideoFunc           func(context.Context, *trainingpb.GetVideoRequest) (*trainingpb.VideoResponse, error)
	GetVideoByFileNameFunc func(context.Context, *trainingpb.GetVideoByFileNameRequest) (*trainingpb.VideoResponse, error)
	SearchVideosFunc       func(context.Context, *trainingpb.SearchVideosRequest) (*trainingpb.VideosResponse, error)
	AddInteractionFunc     func(context.Context, *trainingpb.AddInteractionRequest) (*commonpb.Empty, error)
}

func (m *mockVideoAPI) GetVideos(ctx context.Context, req *trainingpb.GetVideosRequest) (*trainingpb.VideosResponse, error) {
	if m.GetVideosFunc != nil {
		return m.GetVideosFunc(ctx, req)
	}
	return &trainingpb.VideosResponse{}, nil
}
func (m *mockVideoAPI) GetVideo(ctx context.Context, req *trainingpb.GetVideoRequest) (*trainingpb.VideoResponse, error) {
	if m.GetVideoFunc != nil {
		return m.GetVideoFunc(ctx, req)
	}
	return &trainingpb.VideoResponse{}, nil
}
func (m *mockVideoAPI) GetVideoByFileName(ctx context.Context, req *trainingpb.GetVideoByFileNameRequest) (*trainingpb.VideoResponse, error) {
	if m.GetVideoByFileNameFunc != nil {
		return m.GetVideoByFileNameFunc(ctx, req)
	}
	return &trainingpb.VideoResponse{}, nil
}
func (m *mockVideoAPI) SearchVideos(ctx context.Context, req *trainingpb.SearchVideosRequest) (*trainingpb.VideosResponse, error) {
	if m.SearchVideosFunc != nil {
		return m.SearchVideosFunc(ctx, req)
	}
	return &trainingpb.VideosResponse{}, nil
}
func (m *mockVideoAPI) AddInteraction(ctx context.Context, req *trainingpb.AddInteractionRequest) (*commonpb.Empty, error) {
	if m.AddInteractionFunc != nil {
		return m.AddInteractionFunc(ctx, req)
	}
	return &commonpb.Empty{}, nil
}

type mockCategoryAPI struct {
	GetCategoriesFunc     func(context.Context, *trainingpb.GetCategoriesRequest) (*trainingpb.CategoriesResponse, error)
	GetCategoryFunc       func(context.Context, *trainingpb.GetCategoryRequest) (*trainingpb.CategoryResponse, error)
	GetSubCategoryFunc    func(context.Context, *trainingpb.GetSubCategoryRequest) (*trainingpb.SubCategoryResponse, error)
	GetCategoryVideosFunc func(context.Context, *trainingpb.GetCategoryVideosRequest) (*trainingpb.VideosResponse, error)
}

func (m *mockCategoryAPI) GetCategories(ctx context.Context, req *trainingpb.GetCategoriesRequest) (*trainingpb.CategoriesResponse, error) {
	if m.GetCategoriesFunc != nil {
		return m.GetCategoriesFunc(ctx, req)
	}
	return &trainingpb.CategoriesResponse{}, nil
}
func (m *mockCategoryAPI) GetCategory(ctx context.Context, req *trainingpb.GetCategoryRequest) (*trainingpb.CategoryResponse, error) {
	if m.GetCategoryFunc != nil {
		return m.GetCategoryFunc(ctx, req)
	}
	return &trainingpb.CategoryResponse{}, nil
}
func (m *mockCategoryAPI) GetSubCategory(ctx context.Context, req *trainingpb.GetSubCategoryRequest) (*trainingpb.SubCategoryResponse, error) {
	if m.GetSubCategoryFunc != nil {
		return m.GetSubCategoryFunc(ctx, req)
	}
	return &trainingpb.SubCategoryResponse{}, nil
}
func (m *mockCategoryAPI) GetCategoryVideos(ctx context.Context, req *trainingpb.GetCategoryVideosRequest) (*trainingpb.VideosResponse, error) {
	if m.GetCategoryVideosFunc != nil {
		return m.GetCategoryVideosFunc(ctx, req)
	}
	return &trainingpb.VideosResponse{}, nil
}

type mockCommentAPI struct {
	GetCommentsFunc           func(context.Context, *trainingpb.GetCommentsRequest) (*trainingpb.CommentsResponse, error)
	AddCommentFunc            func(context.Context, *trainingpb.AddCommentRequest) (*trainingpb.CommentResponse, error)
	UpdateCommentFunc         func(context.Context, *trainingpb.UpdateCommentRequest) (*trainingpb.CommentResponse, error)
	DeleteCommentFunc         func(context.Context, *trainingpb.DeleteCommentRequest) (*commonpb.Empty, error)
	AddCommentInteractionFunc func(context.Context, *trainingpb.AddCommentInteractionRequest) (*commonpb.Empty, error)
	ReportCommentFunc         func(context.Context, *trainingpb.ReportCommentRequest) (*commonpb.Empty, error)
}

func (m *mockCommentAPI) GetComments(ctx context.Context, req *trainingpb.GetCommentsRequest) (*trainingpb.CommentsResponse, error) {
	if m.GetCommentsFunc != nil {
		return m.GetCommentsFunc(ctx, req)
	}
	return &trainingpb.CommentsResponse{}, nil
}
func (m *mockCommentAPI) AddComment(ctx context.Context, req *trainingpb.AddCommentRequest) (*trainingpb.CommentResponse, error) {
	if m.AddCommentFunc != nil {
		return m.AddCommentFunc(ctx, req)
	}
	return &trainingpb.CommentResponse{}, nil
}
func (m *mockCommentAPI) UpdateComment(ctx context.Context, req *trainingpb.UpdateCommentRequest) (*trainingpb.CommentResponse, error) {
	if m.UpdateCommentFunc != nil {
		return m.UpdateCommentFunc(ctx, req)
	}
	return &trainingpb.CommentResponse{}, nil
}
func (m *mockCommentAPI) DeleteComment(ctx context.Context, req *trainingpb.DeleteCommentRequest) (*commonpb.Empty, error) {
	if m.DeleteCommentFunc != nil {
		return m.DeleteCommentFunc(ctx, req)
	}
	return &commonpb.Empty{}, nil
}
func (m *mockCommentAPI) AddCommentInteraction(ctx context.Context, req *trainingpb.AddCommentInteractionRequest) (*commonpb.Empty, error) {
	if m.AddCommentInteractionFunc != nil {
		return m.AddCommentInteractionFunc(ctx, req)
	}
	return &commonpb.Empty{}, nil
}
func (m *mockCommentAPI) ReportComment(ctx context.Context, req *trainingpb.ReportCommentRequest) (*commonpb.Empty, error) {
	if m.ReportCommentFunc != nil {
		return m.ReportCommentFunc(ctx, req)
	}
	return &commonpb.Empty{}, nil
}

type mockReplyAPI struct {
	GetRepliesFunc          func(context.Context, *trainingpb.GetRepliesRequest) (*trainingpb.RepliesResponse, error)
	AddReplyFunc            func(context.Context, *trainingpb.AddReplyRequest) (*trainingpb.CommentResponse, error)
	UpdateReplyFunc         func(context.Context, *trainingpb.UpdateReplyRequest) (*trainingpb.CommentResponse, error)
	DeleteReplyFunc         func(context.Context, *trainingpb.DeleteReplyRequest) (*commonpb.Empty, error)
	AddReplyInteractionFunc func(context.Context, *trainingpb.AddReplyInteractionRequest) (*commonpb.Empty, error)
}

func (m *mockReplyAPI) GetReplies(ctx context.Context, req *trainingpb.GetRepliesRequest) (*trainingpb.RepliesResponse, error) {
	if m.GetRepliesFunc != nil {
		return m.GetRepliesFunc(ctx, req)
	}
	return &trainingpb.RepliesResponse{}, nil
}
func (m *mockReplyAPI) AddReply(ctx context.Context, req *trainingpb.AddReplyRequest) (*trainingpb.CommentResponse, error) {
	if m.AddReplyFunc != nil {
		return m.AddReplyFunc(ctx, req)
	}
	return &trainingpb.CommentResponse{}, nil
}
func (m *mockReplyAPI) UpdateReply(ctx context.Context, req *trainingpb.UpdateReplyRequest) (*trainingpb.CommentResponse, error) {
	if m.UpdateReplyFunc != nil {
		return m.UpdateReplyFunc(ctx, req)
	}
	return &trainingpb.CommentResponse{}, nil
}
func (m *mockReplyAPI) DeleteReply(ctx context.Context, req *trainingpb.DeleteReplyRequest) (*commonpb.Empty, error) {
	if m.DeleteReplyFunc != nil {
		return m.DeleteReplyFunc(ctx, req)
	}
	return &commonpb.Empty{}, nil
}
func (m *mockReplyAPI) AddReplyInteraction(ctx context.Context, req *trainingpb.AddReplyInteractionRequest) (*commonpb.Empty, error) {
	if m.AddReplyInteractionFunc != nil {
		return m.AddReplyInteractionFunc(ctx, req)
	}
	return &commonpb.Empty{}, nil
}

func identityMW(next http.Handler) http.Handler { return next }

func withUser(userID uint64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), authpkg.UserContextKey{}, &authpkg.UserContext{UserID: userID})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func newTestMux(h *handler.HTTPTrainingHandler, authMW, optionalMW func(http.Handler) http.Handler) *http.ServeMux {
	mux := http.NewServeMux()
	h.RegisterHTTPRoutes(mux, authMW, optionalMW)
	return mux
}

func doJSON(mux http.Handler, method, path, body string) *httptest.ResponseRecorder {
	var rdr io.Reader
	if body != "" {
		rdr = bytes.NewBufferString(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	return rr
}

func TestHTTPContract_CategoryNestedRoutes(t *testing.T) {
	cat := &mockCategoryAPI{
		GetCategoryFunc: func(_ context.Context, req *trainingpb.GetCategoryRequest) (*trainingpb.CategoryResponse, error) {
			if req.Slug != "basics" {
				t.Fatalf("slug=%s", req.Slug)
			}
			return &trainingpb.CategoryResponse{
				Id: 1, Name: "Basics", Slug: "basics", Description: "d", ImageUrl: "/img.png",
				VideosCount: 2,
				Stats:       &trainingpb.VideoStats{ViewsCount: 5},
				SubCategories: []*trainingpb.SubCategoryInfo{
					{Id: 9, Name: "Intro", Slug: "intro", Description: "s", VideosCount: 1},
				},
			}, nil
		},
		GetSubCategoryFunc: func(_ context.Context, req *trainingpb.GetSubCategoryRequest) (*trainingpb.SubCategoryResponse, error) {
			if req.CategorySlug != "basics" || req.SubCategorySlug != "intro" {
				t.Fatalf("req=%+v", req)
			}
			return &trainingpb.SubCategoryResponse{
				Id: 9, Name: "Intro", Slug: "intro", Description: "s", VideosCount: 1,
				Category: &trainingpb.CategoryInfo{Id: 1, Name: "Basics", Slug: "basics"},
				Stats:    &trainingpb.VideoStats{ViewsCount: 3},
			}, nil
		},
		GetCategoryVideosFunc: func(_ context.Context, req *trainingpb.GetCategoryVideosRequest) (*trainingpb.VideosResponse, error) {
			return &trainingpb.VideosResponse{
				Videos:     []*trainingpb.VideoResponse{{Id: 4, Title: "V", Slug: "v"}},
				Pagination: &commonpb.PaginationMeta{CurrentPage: 1, PerPage: 18, Total: 1, LastPage: 1},
			}, nil
		},
		GetCategoriesFunc: func(_ context.Context, req *trainingpb.GetCategoriesRequest) (*trainingpb.CategoriesResponse, error) {
			if req.Pagination == nil || req.Pagination.PerPage != 5 {
				t.Fatalf("pagination=%+v", req.Pagination)
			}
			return &trainingpb.CategoriesResponse{
				Categories: []*trainingpb.CategoryResponse{{Id: 1, Name: "Basics", Slug: "basics"}},
			}, nil
		},
	}
	video := &mockVideoAPI{
		GetVideosFunc: func(_ context.Context, req *trainingpb.GetVideosRequest) (*trainingpb.VideosResponse, error) {
			if req.SubCategoryId != 9 {
				t.Fatalf("sub=%d", req.SubCategoryId)
			}
			return &trainingpb.VideosResponse{
				Videos: []*trainingpb.VideoResponse{{Id: 4, Title: "V", Slug: "v", Stats: &trainingpb.VideoStats{ViewsCount: 1}}},
			}, nil
		},
	}
	h := handler.NewHTTPTrainingHandler(video, cat, &mockCommentAPI{}, &mockReplyAPI{})
	mux := newTestMux(h, identityMW, identityMW)

	rr := doJSON(mux, http.MethodGet, "/api/tutorials/categories/basics", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("category code=%d body=%s", rr.Code, rr.Body.String())
	}
	var catBody map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &catBody); err != nil {
		t.Fatal(err)
	}
	catData, _ := catBody["data"].(map[string]interface{})
	if catData["slug"] != "basics" || catData["subcategories"] == nil {
		t.Fatalf("body=%v", catBody)
	}

	rr = doJSON(mux, http.MethodGet, "/api/tutorials/categories/basics/intro", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("sub code=%d body=%s", rr.Code, rr.Body.String())
	}
	var subBody map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &subBody); err != nil {
		t.Fatal(err)
	}
	subData, _ := subBody["data"].(map[string]interface{})
	if subData["videos"] == nil {
		t.Fatalf("expected attached videos: %v", subBody)
	}

	rr = doJSON(mux, http.MethodGet, "/api/tutorials/categories/basics/videos", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("videos code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doJSON(mux, http.MethodGet, "/api/tutorials/categories?count=5", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("list code=%d", rr.Code)
	}

	rr = doJSON(mux, http.MethodPost, "/api/tutorials/categories/basics", "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("post category code=%d", rr.Code)
	}
}

func TestHTTPContract_CommentCRUDAndInteractions(t *testing.T) {
	comment := &mockCommentAPI{
		AddCommentFunc: func(_ context.Context, req *trainingpb.AddCommentRequest) (*trainingpb.CommentResponse, error) {
			if req.VideoId != 5 || req.UserId != 3 || req.Content != "hello" {
				t.Fatalf("add=%+v", req)
			}
			return &trainingpb.CommentResponse{Id: 11, VideoId: 5, Content: "hello", User: &commonpb.UserBasic{Id: 3, Name: "U", Code: "u"}}, nil
		},
		UpdateCommentFunc: func(_ context.Context, req *trainingpb.UpdateCommentRequest) (*trainingpb.CommentResponse, error) {
			if req.CommentId != 11 || req.Content != "upd" {
				t.Fatalf("upd=%+v", req)
			}
			return &trainingpb.CommentResponse{Id: 11, Content: "upd"}, nil
		},
		DeleteCommentFunc: func(_ context.Context, req *trainingpb.DeleteCommentRequest) (*commonpb.Empty, error) {
			if req.CommentId != 11 {
				t.Fatalf("del=%+v", req)
			}
			return &commonpb.Empty{}, nil
		},
		AddCommentInteractionFunc: func(_ context.Context, req *trainingpb.AddCommentInteractionRequest) (*commonpb.Empty, error) {
			return &commonpb.Empty{}, nil
		},
		ReportCommentFunc: func(_ context.Context, req *trainingpb.ReportCommentRequest) (*commonpb.Empty, error) {
			if req.Content != "spam" {
				t.Fatalf("report=%+v", req)
			}
			return &commonpb.Empty{}, nil
		},
	}
	h := handler.NewHTTPTrainingHandler(&mockVideoAPI{}, &mockCategoryAPI{}, comment, &mockReplyAPI{})
	mux := newTestMux(h, withUser(3), identityMW)

	rr := doJSON(mux, http.MethodPost, "/api/tutorials/5/comments", `{"content":"hello"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("add code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doJSON(mux, http.MethodPut, "/api/tutorials/5/comments/11", `{"content":"upd"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("update code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doJSON(mux, http.MethodDelete, "/api/tutorials/5/comments/11", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("delete code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doJSON(mux, http.MethodPost, "/api/tutorials/5/comments/11/like", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("like code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doJSON(mux, http.MethodPost, "/api/tutorials/5/comments/11/dislike", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("dislike code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doJSON(mux, http.MethodPost, "/api/tutorials/5/comments/11/interactions?liked=1", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("interactions code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doJSON(mux, http.MethodPost, "/api/tutorials/5/comments/11/report", `{"content":"spam"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("report code=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestHTTPContract_CommentAuthAndValidation(t *testing.T) {
	h := handler.NewHTTPTrainingHandler(&mockVideoAPI{}, &mockCategoryAPI{}, &mockCommentAPI{}, &mockReplyAPI{})
	mux := newTestMux(h, identityMW, identityMW)

	rr := doJSON(mux, http.MethodPost, "/api/tutorials/5/comments", `{"content":"x"}`)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("add unauth code=%d", rr.Code)
	}

	muxAuth := newTestMux(h, withUser(1), identityMW)
	rr = doJSON(muxAuth, http.MethodPost, "/api/tutorials/5/comments", `{"content":""}`)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty content code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doJSON(muxAuth, http.MethodPost, "/api/tutorials/abc/comments", `{"content":"x"}`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("bad video id code=%d", rr.Code)
	}

	rr = doJSON(muxAuth, http.MethodPost, "/api/tutorials/5/comments/11/report", `{}`)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty report code=%d", rr.Code)
	}

	rr = doJSON(mux, http.MethodGet, "/api/tutorials/abc/comments", "")
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("get comments bad id code=%d", rr.Code)
	}
}

func TestHTTPContract_ReplyRoutes(t *testing.T) {
	reply := &mockReplyAPI{
		AddReplyFunc: func(_ context.Context, req *trainingpb.AddReplyRequest) (*trainingpb.CommentResponse, error) {
			return &trainingpb.CommentResponse{Id: 21, Content: req.Content, ParentId: req.ParentCommentId}, nil
		},
		UpdateReplyFunc: func(_ context.Context, req *trainingpb.UpdateReplyRequest) (*trainingpb.CommentResponse, error) {
			return &trainingpb.CommentResponse{Id: req.ReplyId, Content: req.Content, ParentId: 8}, nil
		},
		DeleteReplyFunc: func(context.Context, *trainingpb.DeleteReplyRequest) (*commonpb.Empty, error) {
			return &commonpb.Empty{}, nil
		},
		AddReplyInteractionFunc: func(_ context.Context, req *trainingpb.AddReplyInteractionRequest) (*commonpb.Empty, error) {
			if !req.Liked {
				t.Fatalf("liked=%v", req.Liked)
			}
			return &commonpb.Empty{}, nil
		},
		GetRepliesFunc: func(_ context.Context, req *trainingpb.GetRepliesRequest) (*trainingpb.RepliesResponse, error) {
			return nil, status.Error(codes.NotFound, "missing")
		},
	}
	h := handler.NewHTTPTrainingHandler(&mockVideoAPI{}, &mockCategoryAPI{}, &mockCommentAPI{}, reply)
	mux := newTestMux(h, withUser(4), withUser(4))

	rr := doJSON(mux, http.MethodPost, "/api/comments/8/reply", `{"content":"r"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("add reply code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doJSON(mux, http.MethodPut, "/api/comments/8/replies/21", `{"content":"nr"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("update reply code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doJSON(mux, http.MethodDelete, "/api/comments/8/replies/21", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("delete reply code=%d", rr.Code)
	}

	rr = doJSON(mux, http.MethodPost, "/api/comments/8/replies/21/interactions", `{"liked":true}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("reply interaction code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doJSON(mux, http.MethodGet, "/api/comments/8/replies", "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("get replies mapped code=%d", rr.Code)
	}

	rr = doJSON(mux, http.MethodPost, "/api/comments/8/replies", "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("post replies list code=%d", rr.Code)
	}
}

func TestHTTPContract_SearchSuccessAndGRPCMapping(t *testing.T) {
	video := &mockVideoAPI{
		SearchVideosFunc: func(_ context.Context, req *trainingpb.SearchVideosRequest) (*trainingpb.VideosResponse, error) {
			if req.Query != "go" {
				t.Fatalf("query=%s", req.Query)
			}
			liked := true
			return &trainingpb.VideosResponse{
				Videos: []*trainingpb.VideoResponse{{
					Id: 1, Title: "Go", Slug: "go", ImageUrl: "/i", VideoUrl: "/v",
					Creator:         &commonpb.UserBasic{Name: "C", Code: "c", ProfilePhoto: "p"},
					Category:        &trainingpb.CategoryInfo{Id: 1, Name: "Cat", Slug: "cat"},
					SubCategory:     &trainingpb.SubCategoryInfo{Id: 2, Name: "Sub", Slug: "sub"},
					Stats:           &trainingpb.VideoStats{ViewsCount: 1, LikesCount: 2, DislikesCount: 0, CommentsCount: 3},
					UserInteraction: &liked,
				}},
				Pagination: &commonpb.PaginationMeta{CurrentPage: 2, PerPage: 5, Total: 1, LastPage: 1},
			}, nil
		},
		GetVideosFunc: func(_ context.Context, req *trainingpb.GetVideosRequest) (*trainingpb.VideosResponse, error) {
			if req.CategoryId != 7 || req.SubCategoryId != 8 {
				t.Fatalf("filters=%+v", req)
			}
			return nil, status.Error(codes.PermissionDenied, "nope")
		},
		GetVideoByFileNameFunc: func(context.Context, *trainingpb.GetVideoByFileNameRequest) (*trainingpb.VideoResponse, error) {
			return nil, status.Error(codes.InvalidArgument, "bad file")
		},
	}
	h := handler.NewHTTPTrainingHandler(video, &mockCategoryAPI{}, &mockCommentAPI{}, &mockReplyAPI{})
	mux := newTestMux(h, identityMW, identityMW)

	rr := doJSON(mux, http.MethodPost, "/api/tutorials/search?page=2&per_page=5", `{"searchTerm":"go"}`)
	if rr.Code != http.StatusOK {
		t.Fatalf("search code=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"user_interaction":{"liked":true}`) {
		t.Fatalf("expected interaction in body %s", rr.Body.String())
	}

	rr = doJSON(mux, http.MethodGet, "/api/tutorials?category_id=7&sub_category_id=8", "")
	if rr.Code != http.StatusForbidden {
		t.Fatalf("forbidden code=%d", rr.Code)
	}

	rr = doJSON(mux, http.MethodPost, "/api/video-tutorials", `{"url":"x"}`)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid argument code=%d body=%s", rr.Code, rr.Body.String())
	}

	rr = doJSON(mux, http.MethodGet, "/api/video-tutorials", "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("get video-tutorials code=%d", rr.Code)
	}

	rr = doJSON(mux, http.MethodPost, "/api/tutorials", "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("post tutorials code=%d", rr.Code)
	}
}

func TestHTTPContract_VideoTutorialsValidationAndInternal(t *testing.T) {
	video := &mockVideoAPI{
		GetVideoByFileNameFunc: func(context.Context, *trainingpb.GetVideoByFileNameRequest) (*trainingpb.VideoResponse, error) {
			return nil, status.Error(codes.Internal, "boom")
		},
	}
	h := handler.NewHTTPTrainingHandler(video, &mockCategoryAPI{}, &mockCommentAPI{}, &mockReplyAPI{})
	mux := newTestMux(h, identityMW, identityMW)

	rr := doJSON(mux, http.MethodPost, "/api/video-tutorials", `{"url":""}`)
	if rr.Code != http.StatusUnprocessableEntity {
		t.Fatalf("empty url code=%d", rr.Code)
	}

	rr = doJSON(mux, http.MethodPost, "/api/video-tutorials", `{`)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("bad json code=%d", rr.Code)
	}

	rr = doJSON(mux, http.MethodPost, "/api/video-tutorials", `{"url":"clip"}`)
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("internal code=%d", rr.Code)
	}
}

func TestHTTPContract_UnknownCommentAction404(t *testing.T) {
	h := handler.NewHTTPTrainingHandler(&mockVideoAPI{}, &mockCategoryAPI{}, &mockCommentAPI{}, &mockReplyAPI{})
	mux := newTestMux(h, withUser(1), identityMW)
	rr := doJSON(mux, http.MethodPost, "/api/tutorials/5/comments/11/unknown", "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("code=%d", rr.Code)
	}
	rr = doJSON(mux, http.MethodGet, "/api/tutorials/5/comments/11", "")
	if rr.Code != http.StatusNotFound {
		t.Fatalf("get single comment code=%d", rr.Code)
	}
}
