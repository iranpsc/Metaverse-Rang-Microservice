package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/levels-service/internal/handler"
	"metarang/levels-service/internal/service"
	"metarang/levels-service/tests/internal/testutil"
	pb "metarang/shared/pb/levels"
)

// levelAPIAdapter adapts LevelHandler to the unexported levelAPI interface
// expected by HTTPLevelHandler via the public constructor.
type levelAPIAdapter struct {
	h *handler.LevelHandler
}

func (a *levelAPIAdapter) GetAllLevels(ctx context.Context, req *pb.GetAllLevelsRequest) (*pb.LevelsResponse, error) {
	return a.h.GetAllLevels(ctx, req)
}
func (a *levelAPIAdapter) GetLevel(ctx context.Context, req *pb.GetLevelRequest) (*pb.LevelResponse, error) {
	return a.h.GetLevel(ctx, req)
}
func (a *levelAPIAdapter) GetLevelGeneralInfo(ctx context.Context, req *pb.GetLevelGeneralInfoRequest) (*pb.LevelGeneralInfoResponse, error) {
	return a.h.GetLevelGeneralInfo(ctx, req)
}
func (a *levelAPIAdapter) GetLevelGem(ctx context.Context, req *pb.GetLevelGemRequest) (*pb.LevelGemResponse, error) {
	return a.h.GetLevelGem(ctx, req)
}
func (a *levelAPIAdapter) GetLevelGift(ctx context.Context, req *pb.GetLevelGiftRequest) (*pb.LevelGiftResponse, error) {
	return a.h.GetLevelGift(ctx, req)
}
func (a *levelAPIAdapter) GetLevelLicenses(ctx context.Context, req *pb.GetLevelLicensesRequest) (*pb.LevelLicensesResponse, error) {
	return a.h.GetLevelLicenses(ctx, req)
}
func (a *levelAPIAdapter) GetLevelPrizes(ctx context.Context, req *pb.GetLevelPrizesRequest) (*pb.LevelPrizesResponse, error) {
	return a.h.GetLevelPrizes(ctx, req)
}

func newHTTPHandler(repo *testutil.MockLevelRepository, appURL string) *handler.HTTPLevelHandler {
	svc := service.NewLevelService(repo, &testutil.MockUserLogRepository{}, &testutil.MockCommercialClient{})
	grpcH := handler.NewLevelHandler(svc)
	return handler.NewHTTPLevelHandler(&levelAPIAdapter{grpcH}, appURL)
}

func doRequest(h *handler.HTTPLevelHandler, method, path string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	h.RegisterHTTPRoutes(mux)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(method, path, nil))
	return rr
}

func TestHTTP_Health(t *testing.T) {
	h := newHTTPHandler(&testutil.MockLevelRepository{}, "")
	rr := doRequest(h, http.MethodGet, "/health")
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHTTP_GetAllLevels_Success(t *testing.T) {
	h := newHTTPHandler(&testutil.MockLevelRepository{
		GetAllLevelsFunc: func(ctx context.Context) ([]*pb.Level, error) {
			return []*pb.Level{{Id: 1, Name: "bronze", Slug: "bronze"}}, nil
		},
	}, "https://cdn.example.com")
	rr := doRequest(h, http.MethodGet, "/api/levels")
	assert.Equal(t, http.StatusOK, rr.Code)
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	data, ok := body["data"].([]interface{})
	require.True(t, ok)
	assert.Len(t, data, 1)
	assert.Equal(t, "bronze", data[0].(map[string]interface{})["slug"])
}

func TestHTTP_GetAllLevels_MethodNotAllowed(t *testing.T) {
	h := newHTTPHandler(&testutil.MockLevelRepository{}, "")
	rr := doRequest(h, http.MethodPost, "/api/levels")
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHTTP_GetAllLevels_Error(t *testing.T) {
	h := newHTTPHandler(&testutil.MockLevelRepository{
		GetAllLevelsFunc: func(ctx context.Context) ([]*pb.Level, error) { return nil, errHandler },
	}, "")
	rr := doRequest(h, http.MethodGet, "/api/levels")
	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestHTTP_GetLevel_Success(t *testing.T) {
	h := newHTTPHandler(&testutil.MockLevelRepository{
		FindBySlugFunc: func(ctx context.Context, slug string) (*pb.Level, error) {
			return &pb.Level{
				Id:              2,
				Name:            "silver",
				Slug:            slug,
				ImageUrl:        "level-image.png",
				BackgroundImage: "level-background.png",
			}, nil
		},
	}, "https://admin.example.com")
	rr := doRequest(h, http.MethodGet, "/api/levels/silver")
	assert.Equal(t, http.StatusOK, rr.Code)
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	data := body["data"].(map[string]interface{})
	assert.Equal(t, "https://admin.example.com/uploads/level-image.png", data["image"])
	assert.Equal(t, "https://admin.example.com/uploads/level-background.png", data["background_image"])
}

func TestHTTP_GetLevel_NotFound(t *testing.T) {
	h := newHTTPHandler(&testutil.MockLevelRepository{
		FindBySlugFunc: func(ctx context.Context, slug string) (*pb.Level, error) {
			return nil, status.Errorf(codes.NotFound, "not found")
		},
	}, "")
	rr := doRequest(h, http.MethodGet, "/api/levels/unknown")
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHTTP_GetLevelGeneralInfo_Success(t *testing.T) {
	h := newHTTPHandler(&testutil.MockLevelRepository{
		FindBySlugFunc: func(ctx context.Context, slug string) (*pb.Level, error) { return &pb.Level{Id: 3}, nil },
		GetLevelGeneralInfoFunc: func(ctx context.Context, levelID uint64) (*pb.LevelGeneralInfo, error) {
			return &pb.LevelGeneralInfo{LevelId: levelID, Score: 100}, nil
		},
	}, "")
	rr := doRequest(h, http.MethodGet, "/api/levels/gold/general-info")
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHTTP_GetLevelGeneralInfo_Nil(t *testing.T) {
	h := newHTTPHandler(&testutil.MockLevelRepository{
		FindBySlugFunc: func(ctx context.Context, slug string) (*pb.Level, error) { return &pb.Level{Id: 3}, nil },
		GetLevelGeneralInfoFunc: func(ctx context.Context, levelID uint64) (*pb.LevelGeneralInfo, error) {
			return nil, nil
		},
	}, "")
	rr := doRequest(h, http.MethodGet, "/api/levels/gold/general-info")
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHTTP_GetLevelGem_Success(t *testing.T) {
	h := newHTTPHandler(&testutil.MockLevelRepository{
		FindBySlugFunc: func(ctx context.Context, slug string) (*pb.Level, error) { return &pb.Level{Id: 3}, nil },
		GetLevelGemFunc: func(ctx context.Context, levelID uint64) (*pb.LevelGem, error) {
			return &pb.LevelGem{LevelId: levelID}, nil
		},
	}, "")
	rr := doRequest(h, http.MethodGet, "/api/levels/gold/gem")
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHTTP_GetLevelGift_Success(t *testing.T) {
	h := newHTTPHandler(&testutil.MockLevelRepository{
		FindBySlugFunc: func(ctx context.Context, slug string) (*pb.Level, error) { return &pb.Level{Id: 4}, nil },
		GetLevelGiftFunc: func(ctx context.Context, levelID uint64) (*pb.LevelGift, error) {
			return &pb.LevelGift{LevelId: levelID}, nil
		},
	}, "")
	rr := doRequest(h, http.MethodGet, "/api/levels/gold/gift")
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHTTP_GetLevelLicenses_Success(t *testing.T) {
	h := newHTTPHandler(&testutil.MockLevelRepository{
		FindBySlugFunc: func(ctx context.Context, slug string) (*pb.Level, error) { return &pb.Level{Id: 5}, nil },
		GetLevelLicensesFunc: func(ctx context.Context, levelID uint64) (*pb.LevelLicense, error) {
			return &pb.LevelLicense{LevelId: levelID}, nil
		},
	}, "")
	rr := doRequest(h, http.MethodGet, "/api/levels/gold/licenses")
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHTTP_GetLevelPrize_Success(t *testing.T) {
	h := newHTTPHandler(&testutil.MockLevelRepository{
		FindBySlugFunc: func(ctx context.Context, slug string) (*pb.Level, error) { return &pb.Level{Id: 6}, nil },
		GetLevelPrizeFunc: func(ctx context.Context, levelID uint64) (*pb.LevelPrize, error) {
			return &pb.LevelPrize{LevelId: levelID, Psc: "1000"}, nil
		},
	}, "")
	rr := doRequest(h, http.MethodGet, "/api/levels/gold/prize")
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHTTP_GetLevelPrize_Nil(t *testing.T) {
	h := newHTTPHandler(&testutil.MockLevelRepository{
		FindBySlugFunc: func(ctx context.Context, slug string) (*pb.Level, error) { return &pb.Level{Id: 6}, nil },
		GetLevelPrizeFunc: func(ctx context.Context, levelID uint64) (*pb.LevelPrize, error) {
			return nil, nil
		},
	}, "")
	rr := doRequest(h, http.MethodGet, "/api/levels/gold/prize")
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHTTP_UnknownRoute(t *testing.T) {
	h := newHTTPHandler(&testutil.MockLevelRepository{}, "")
	rr := doRequest(h, http.MethodGet, "/api/levels/gold/unknown-sub")
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestHTTP_PrefixImageURL_AbsoluteURL(t *testing.T) {
	h := newHTTPHandler(&testutil.MockLevelRepository{
		GetAllLevelsFunc: func(ctx context.Context) ([]*pb.Level, error) {
			return []*pb.Level{{Id: 1, ImageUrl: "https://cdn.example.com/img.png"}}, nil
		},
	}, "https://app.example.com")
	rr := doRequest(h, http.MethodGet, "/api/levels")
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	data := body["data"].([]interface{})
	assert.Equal(t, "https://cdn.example.com/img.png", data[0].(map[string]interface{})["image"])
}

func TestHTTP_PrefixImageURL_RelativeWithAppURL(t *testing.T) {
	h := newHTTPHandler(&testutil.MockLevelRepository{
		GetAllLevelsFunc: func(ctx context.Context) ([]*pb.Level, error) {
			return []*pb.Level{{Id: 1, ImageUrl: "img.png"}}, nil
		},
	}, "https://app.example.com")
	rr := doRequest(h, http.MethodGet, "/api/levels")
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	data := body["data"].([]interface{})
	assert.Equal(t, "https://app.example.com/uploads/img.png", data[0].(map[string]interface{})["image"])
}

func TestHTTP_PrefixImageURL_RelativeNoAppURL(t *testing.T) {
	h := newHTTPHandler(&testutil.MockLevelRepository{
		GetAllLevelsFunc: func(ctx context.Context) ([]*pb.Level, error) {
			return []*pb.Level{{Id: 1, ImageUrl: "img.png"}}, nil
		},
	}, "")
	rr := doRequest(h, http.MethodGet, "/api/levels")
	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	data := body["data"].([]interface{})
	assert.Equal(t, "/uploads/img.png", data[0].(map[string]interface{})["image"])
}
