package handler_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	commonpb "metarang/shared/pb/common"
	trainingpb "metarang/shared/pb/training"
	"metarang/training-service/internal/handler"
	"metarang/training-service/internal/models"
	"metarang/training-service/internal/repository"
	"metarang/training-service/internal/service"
)

func TestHTTPContract_ErrorMappingAndEmptyBodies(t *testing.T) {
	video := &mockVideoAPI{
		GetVideosFunc: func(context.Context, *trainingpb.GetVideosRequest) (*trainingpb.VideosResponse, error) {
			return nil, status.Error(codes.Unauthenticated, "token")
		},
		AddInteractionFunc: func(context.Context, *trainingpb.AddInteractionRequest) (*commonpb.Empty, error) {
			return nil, status.Error(codes.Internal, "db")
		},
		SearchVideosFunc: func(context.Context, *trainingpb.SearchVideosRequest) (*trainingpb.VideosResponse, error) {
			return nil, status.Error(codes.Internal, "search")
		},
	}
	cat := &mockCategoryAPI{
		GetCategoriesFunc: func(context.Context, *trainingpb.GetCategoriesRequest) (*trainingpb.CategoriesResponse, error) {
			return nil, status.Error(codes.Internal, "cats")
		},
		GetCategoryFunc: func(context.Context, *trainingpb.GetCategoryRequest) (*trainingpb.CategoryResponse, error) {
			return nil, status.Error(codes.NotFound, "missing")
		},
		GetCategoryVideosFunc: func(context.Context, *trainingpb.GetCategoryVideosRequest) (*trainingpb.VideosResponse, error) {
			return nil, status.Error(codes.Internal, "vids")
		},
		GetSubCategoryFunc: func(context.Context, *trainingpb.GetSubCategoryRequest) (*trainingpb.SubCategoryResponse, error) {
			return nil, status.Error(codes.NotFound, "sub")
		},
	}
	h := handler.NewHTTPTrainingHandler(video, cat, &mockCommentAPI{}, &mockReplyAPI{})
	mux := newTestMux(h, withUser(1), identityMW)

	if doJSON(mux, http.MethodGet, "/api/tutorials", "").Code != http.StatusUnauthorized {
		t.Fatal("unauthenticated mapping")
	}
	if doJSON(mux, http.MethodPost, "/api/tutorials/search", "").Code != http.StatusBadRequest {
		t.Fatal("search empty body")
	}
	if doJSON(mux, http.MethodPost, "/api/tutorials/search", `{`).Code != http.StatusBadRequest {
		t.Fatal("search bad json")
	}
	if doJSON(mux, http.MethodPost, "/api/tutorials/search", `{"searchTerm":"x"}`).Code != http.StatusInternalServerError {
		t.Fatal("search internal")
	}
	if doJSON(mux, http.MethodGet, "/api/tutorials/categories", "").Code != http.StatusInternalServerError {
		t.Fatal("categories internal")
	}
	if doJSON(mux, http.MethodGet, "/api/tutorials/categories/missing", "").Code != http.StatusNotFound {
		t.Fatal("category not found")
	}
	if doJSON(mux, http.MethodGet, "/api/tutorials/categories/x/videos", "").Code != http.StatusInternalServerError {
		t.Fatal("category videos internal")
	}
	if doJSON(mux, http.MethodGet, "/api/tutorials/categories/a/b", "").Code != http.StatusNotFound {
		t.Fatal("subcategory not found")
	}
	if doJSON(mux, http.MethodPost, "/api/tutorials/9/interactions?liked=1", "").Code != http.StatusInternalServerError {
		t.Fatal("interaction internal")
	}
	if doJSON(mux, http.MethodPost, "/api/tutorials/abc/interactions?liked=1", "").Code != http.StatusBadRequest {
		t.Fatal("invalid video id")
	}
	if doJSON(mux, http.MethodPost, "/api/tutorials/9/interactions", "").Code != http.StatusBadRequest {
		t.Fatal("interaction missing liked")
	}
	if doJSON(mux, http.MethodPost, "/api/tutorials/9/interactions", `{`).Code != http.StatusBadRequest {
		t.Fatal("interaction bad json")
	}
	if doJSON(mux, http.MethodPost, "/api/video-tutorials", "").Code != http.StatusBadRequest {
		t.Fatal("modal empty body")
	}
}

func TestHTTPContract_SubCategoryVideosFallbackAndZeroID(t *testing.T) {
	cat := &mockCategoryAPI{
		GetSubCategoryFunc: func(_ context.Context, req *trainingpb.GetSubCategoryRequest) (*trainingpb.SubCategoryResponse, error) {
			if req.SubCategorySlug == "empty" {
				return &trainingpb.SubCategoryResponse{Id: 0, Name: "E", Slug: "empty"}, nil
			}
			return &trainingpb.SubCategoryResponse{Id: 3, Name: "S", Slug: "s", VideosCount: 2}, nil
		},
	}
	video := &mockVideoAPI{
		GetVideosFunc: func(context.Context, *trainingpb.GetVideosRequest) (*trainingpb.VideosResponse, error) {
			return nil, status.Error(codes.Internal, "nope")
		},
	}
	h := handler.NewHTTPTrainingHandler(video, cat, &mockCommentAPI{}, &mockReplyAPI{})
	mux := newTestMux(h, identityMW, identityMW)

	rr := doJSON(mux, http.MethodGet, "/api/tutorials/categories/c/s", "")
	if rr.Code != http.StatusOK || !bytes.Contains(rr.Body.Bytes(), []byte(`"videos"`)) {
		t.Fatalf("expected empty videos on error code=%d body=%s", rr.Code, rr.Body.String())
	}
	rr = doJSON(mux, http.MethodGet, "/api/tutorials/categories/c/empty", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("zero id code=%d", rr.Code)
	}
}

func TestHTTPContract_CommentAndReplyValidationBranches(t *testing.T) {
	comment := &mockCommentAPI{
		GetCommentsFunc: func(context.Context, *trainingpb.GetCommentsRequest) (*trainingpb.CommentsResponse, error) {
			return nil, status.Error(codes.Internal, "c")
		},
		AddCommentFunc: func(context.Context, *trainingpb.AddCommentRequest) (*trainingpb.CommentResponse, error) {
			return nil, status.Error(codes.PermissionDenied, "no")
		},
		UpdateCommentFunc: func(context.Context, *trainingpb.UpdateCommentRequest) (*trainingpb.CommentResponse, error) {
			return &trainingpb.CommentResponse{Id: 11, Content: "upd"}, nil
		},
		DeleteCommentFunc: func(context.Context, *trainingpb.DeleteCommentRequest) (*commonpb.Empty, error) {
			return nil, status.Error(codes.Internal, "del")
		},
		AddCommentInteractionFunc: func(context.Context, *trainingpb.AddCommentInteractionRequest) (*commonpb.Empty, error) {
			return nil, status.Error(codes.PermissionDenied, "own")
		},
		ReportCommentFunc: func(context.Context, *trainingpb.ReportCommentRequest) (*commonpb.Empty, error) {
			return nil, status.Error(codes.NotFound, "gone")
		},
	}
	reply := &mockReplyAPI{
		GetRepliesFunc: func(context.Context, *trainingpb.GetRepliesRequest) (*trainingpb.RepliesResponse, error) {
			return nil, status.Error(codes.Internal, "r")
		},
		AddReplyFunc: func(context.Context, *trainingpb.AddReplyRequest) (*trainingpb.CommentResponse, error) {
			return nil, status.Error(codes.PermissionDenied, "own")
		},
		UpdateReplyFunc: func(context.Context, *trainingpb.UpdateReplyRequest) (*trainingpb.CommentResponse, error) {
			return nil, status.Error(codes.PermissionDenied, "own")
		},
		DeleteReplyFunc: func(context.Context, *trainingpb.DeleteReplyRequest) (*commonpb.Empty, error) {
			return nil, status.Error(codes.Internal, "del")
		},
		AddReplyInteractionFunc: func(context.Context, *trainingpb.AddReplyInteractionRequest) (*commonpb.Empty, error) {
			return nil, status.Error(codes.PermissionDenied, "own")
		},
	}
	h := handler.NewHTTPTrainingHandler(&mockVideoAPI{}, &mockCategoryAPI{}, comment, reply)
	mux := newTestMux(h, withUser(2), withUser(2))

	if doJSON(mux, http.MethodGet, "/api/tutorials/5/comments", "").Code != http.StatusInternalServerError {
		t.Fatal("get comments internal")
	}
	if doJSON(mux, http.MethodPost, "/api/tutorials/5/comments", "").Code != http.StatusBadRequest {
		t.Fatal("add comment empty body")
	}
	if doJSON(mux, http.MethodPost, "/api/tutorials/5/comments", `{`).Code != http.StatusBadRequest {
		t.Fatal("add comment bad json")
	}
	if doJSON(mux, http.MethodPost, "/api/tutorials/5/comments", `{"content":"x"}`).Code != http.StatusForbidden {
		t.Fatal("add comment forbidden")
	}
	if doJSON(mux, http.MethodPut, "/api/tutorials/5/comments/abc", `{"content":"x"}`).Code != http.StatusBadRequest {
		t.Fatal("update invalid comment id")
	}
	if doJSON(mux, http.MethodPut, "/api/tutorials/5/comments/11", "").Code != http.StatusBadRequest {
		t.Fatal("update empty body")
	}
	if doJSON(mux, http.MethodPost, "/api/tutorials/5/comments/11", `{"content":"upd","_method":"put"}`).Code != http.StatusOK {
		t.Fatal("update via post _method")
	}
	if doJSON(mux, http.MethodPost, "/api/tutorials/5/comments/11", `{"content":"upd","_method":"patch"}`).Code != http.StatusMethodNotAllowed {
		t.Fatal("update via post wrong _method")
	}
	if doJSON(mux, http.MethodDelete, "/api/tutorials/5/comments/abc", "").Code != http.StatusBadRequest {
		t.Fatal("delete invalid id")
	}
	if doJSON(mux, http.MethodDelete, "/api/tutorials/5/comments/11", "").Code != http.StatusInternalServerError {
		t.Fatal("delete internal")
	}
	if doJSON(mux, http.MethodPost, "/api/tutorials/5/comments/11/interactions", "").Code != http.StatusBadRequest {
		t.Fatal("interaction missing liked")
	}
	if doJSON(mux, http.MethodPost, "/api/tutorials/5/comments/11/interactions", `{`).Code != http.StatusBadRequest {
		t.Fatal("interaction bad json")
	}
	if doJSON(mux, http.MethodPost, "/api/tutorials/5/comments/11/interactions", `{"liked":true}`).Code != http.StatusForbidden {
		t.Fatal("interaction forbidden")
	}
	if doJSON(mux, http.MethodPost, "/api/tutorials/abc/comments/11/report", `{"content":"x"}`).Code != http.StatusBadRequest {
		t.Fatal("report invalid ids")
	}
	if doJSON(mux, http.MethodPost, "/api/tutorials/5/comments/11/report", "").Code != http.StatusBadRequest {
		t.Fatal("report empty body")
	}
	if doJSON(mux, http.MethodPost, "/api/tutorials/5/comments/11/report", `{`).Code != http.StatusBadRequest {
		t.Fatal("report bad json")
	}
	if doJSON(mux, http.MethodPost, "/api/tutorials/5/comments/11/report", `{"content":"x"}`).Code != http.StatusNotFound {
		t.Fatal("report not found")
	}

	if doJSON(mux, http.MethodGet, "/api/comments/abc/replies", "").Code != http.StatusBadRequest {
		t.Fatal("replies invalid id")
	}
	if doJSON(mux, http.MethodGet, "/api/comments/8/replies", "").Code != http.StatusInternalServerError {
		t.Fatal("get replies internal")
	}
	if doJSON(mux, http.MethodPost, "/api/comments/abc/reply", `{"content":"r"}`).Code != http.StatusBadRequest {
		t.Fatal("add reply invalid id")
	}
	if doJSON(mux, http.MethodPost, "/api/comments/8/reply", "").Code != http.StatusBadRequest {
		t.Fatal("add reply empty body")
	}
	if doJSON(mux, http.MethodPost, "/api/comments/8/reply", `{"content":""}`).Code != http.StatusUnprocessableEntity {
		t.Fatal("add reply empty content")
	}
	if doJSON(mux, http.MethodPost, "/api/comments/8/reply", `{"content":"r"}`).Code != http.StatusForbidden {
		t.Fatal("add reply forbidden")
	}
	if doJSON(mux, http.MethodPut, "/api/comments/8/replies/abc", `{"content":"r"}`).Code != http.StatusBadRequest {
		t.Fatal("update reply invalid id")
	}
	if doJSON(mux, http.MethodPut, "/api/comments/8/replies/21", "").Code != http.StatusBadRequest {
		t.Fatal("update reply empty body")
	}
	if doJSON(mux, http.MethodPut, "/api/comments/8/replies/21", `{"content":""}`).Code != http.StatusUnprocessableEntity {
		t.Fatal("update reply empty content")
	}
	if doJSON(mux, http.MethodPut, "/api/comments/8/replies/21", `{"content":"r"}`).Code != http.StatusForbidden {
		t.Fatal("update reply forbidden")
	}
	if doJSON(mux, http.MethodDelete, "/api/comments/8/replies/abc", "").Code != http.StatusBadRequest {
		t.Fatal("delete reply invalid id")
	}
	if doJSON(mux, http.MethodDelete, "/api/comments/8/replies/21", "").Code != http.StatusInternalServerError {
		t.Fatal("delete reply internal")
	}
	if doJSON(mux, http.MethodPost, "/api/comments/8/replies/abc/interactions", `{"liked":true}`).Code != http.StatusBadRequest {
		t.Fatal("reply interaction invalid id")
	}
	if doJSON(mux, http.MethodPost, "/api/comments/8/replies/21/interactions", "").Code != http.StatusBadRequest {
		t.Fatal("reply interaction empty body")
	}
	if doJSON(mux, http.MethodPost, "/api/comments/8/replies/21/interactions", `{`).Code != http.StatusBadRequest {
		t.Fatal("reply interaction bad json")
	}
	if doJSON(mux, http.MethodPost, "/api/comments/8/replies/21/interactions", `{"liked":true}`).Code != http.StatusForbidden {
		t.Fatal("reply interaction forbidden")
	}
}

func TestHTTPContract_Route404sAndPagination(t *testing.T) {
	video := &mockVideoAPI{
		GetVideosFunc: func(_ context.Context, req *trainingpb.GetVideosRequest) (*trainingpb.VideosResponse, error) {
			if req.Pagination == nil || req.Pagination.Page != 3 || req.Pagination.PerPage != 7 {
				t.Fatalf("pagination=%+v", req.Pagination)
			}
			return &trainingpb.VideosResponse{Videos: []*trainingpb.VideoResponse{}}, nil
		},
		GetVideoFunc: func(_ context.Context, req *trainingpb.GetVideoRequest) (*trainingpb.VideoResponse, error) {
			if req.IpAddress == "" {
				t.Fatal("expected ip")
			}
			return &trainingpb.VideoResponse{Id: 1, Title: "T", Slug: "s", Creator: &commonpb.UserBasic{Name: "N", Code: "c"}}, nil
		},
	}
	h := handler.NewHTTPTrainingHandler(video, &mockCategoryAPI{}, &mockCommentAPI{}, &mockReplyAPI{})
	mux := newTestMux(h, identityMW, identityMW)

	req := httptest.NewRequest(http.MethodGet, "/api/tutorials?page=3&per_page=7", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.1")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("paginated videos code=%d", rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/tutorials/s", nil)
	req.Header.Set("X-Real-IP", "198.51.100.9")
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("get video code=%d", rr.Code)
	}

	if doJSON(mux, http.MethodPut, "/api/tutorials/categories", "").Code != http.StatusNotFound &&
		doJSON(mux, http.MethodGet, "/api/tutorials/categories/a/b/c", "").Code != http.StatusNotFound {
		t.Fatal("expected nested category 404")
	}
	if doJSON(mux, http.MethodGet, "/api/tutorials/categories/a/b/c", "").Code != http.StatusNotFound {
		t.Fatal("deep category 404")
	}
	if doJSON(mux, http.MethodPost, "/api/tutorials/categories/a/videos", "").Code != http.StatusNotFound {
		t.Fatal("post category videos 404")
	}
	if doJSON(mux, http.MethodGet, "/api/tutorials/5/comments/11/like", "").Code != http.StatusNotFound {
		t.Fatal("get like 404")
	}
	if doJSON(mux, http.MethodGet, "/api/comments/8/replies/21/interactions", "").Code != http.StatusNotFound {
		t.Fatal("get reply interactions 404")
	}
	if doJSON(mux, http.MethodGet, "/api/comments/8/reply", "").Code != http.StatusNotFound {
		t.Fatal("get reply 404")
	}
	if doJSON(mux, http.MethodGet, "/api/comments/only", "").Code != http.StatusNotFound {
		t.Fatal("comments only 404")
	}
}

func TestVideoDetailsToProto_URLVariants(t *testing.T) {
	t.Setenv("APP_URL", "")
	d := &service.VideoDetails{
		Video: &models.Video{
			ID: 1, Title: "T", Image: "https://cdn.example/img.jpg", FileName: "",
		},
	}
	p, err := handler.VideoDetailsToProto(d)
	if err != nil || p.ImageUrl != "https://cdn.example/img.jpg" || p.VideoUrl != "" {
		t.Fatalf("https image p=%+v err=%v", p, err)
	}

	t.Setenv("APP_URL", "http://app.local/")
	d.Video.Image = "thumb.jpg"
	d.Video.FileName = "v.mp4"
	d.Creator = &repository.UserBasic{ID: 2, ProfilePhoto: "p.jpg"}
	p, err = handler.VideoDetailsToProto(d)
	if err != nil || p.ImageUrl == "" || p.VideoUrl == "" || p.Creator.ProfilePhoto != "p.jpg" {
		t.Fatalf("app url p=%+v err=%v", p, err)
	}

	d.Video.Image = "/uploads/already.jpg"
	p, err = handler.VideoDetailsToProto(d)
	if err != nil || p.ImageUrl == "" {
		t.Fatalf("uploads prefix p=%+v err=%v", p, err)
	}

	if _, err := handler.VideoDetailsToProto(&service.VideoDetails{}); err == nil {
		t.Fatal("expected invalid video")
	}
	if _, err := handler.VideoDetailsToProto(nil); err == nil {
		t.Fatal("expected nil video error")
	}
}
