package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/dynasty-service/internal/handler"
	commonpb "metarang/shared/pb/common"
	dynastypb "metarang/shared/pb/dynasty"
	authpkg "metarang/shared/pkg/auth"
)

type fakeDynastyAPI struct {
	GetUserDynastyFunc       func(context.Context, *dynastypb.GetUserDynastyRequest) (*dynastypb.DynastyResponse, error)
	CreateDynastyFunc        func(context.Context, *dynastypb.CreateDynastyRequest) (*dynastypb.DynastyResponse, error)
	UpdateDynastyFeatureFunc func(context.Context, *dynastypb.UpdateDynastyFeatureRequest) (*dynastypb.DynastyResponse, error)
}

func (f *fakeDynastyAPI) GetUserDynasty(ctx context.Context, req *dynastypb.GetUserDynastyRequest) (*dynastypb.DynastyResponse, error) {
	if f.GetUserDynastyFunc != nil {
		return f.GetUserDynastyFunc(ctx, req)
	}
	return &dynastypb.DynastyResponse{UserHasDynasty: false}, nil
}
func (f *fakeDynastyAPI) CreateDynasty(ctx context.Context, req *dynastypb.CreateDynastyRequest) (*dynastypb.DynastyResponse, error) {
	if f.CreateDynastyFunc != nil {
		return f.CreateDynastyFunc(ctx, req)
	}
	return &dynastypb.DynastyResponse{UserHasDynasty: true, Id: 1, FamilyId: 2}, nil
}
func (f *fakeDynastyAPI) UpdateDynastyFeature(ctx context.Context, req *dynastypb.UpdateDynastyFeatureRequest) (*dynastypb.DynastyResponse, error) {
	if f.UpdateDynastyFeatureFunc != nil {
		return f.UpdateDynastyFeatureFunc(ctx, req)
	}
	return &dynastypb.DynastyResponse{UserHasDynasty: true, Id: req.DynastyId}, nil
}

type fakeJoinRequestAPI struct {
	SendJoinRequestFunc       func(context.Context, *dynastypb.SendJoinRequestRequest) (*dynastypb.JoinRequestResponse, error)
	GetSentRequestsFunc       func(context.Context, *dynastypb.GetSentRequestsRequest) (*dynastypb.JoinRequestsResponse, error)
	GetReceivedRequestsFunc   func(context.Context, *dynastypb.GetReceivedRequestsRequest) (*dynastypb.JoinRequestsResponse, error)
	GetJoinRequestFunc        func(context.Context, *dynastypb.GetJoinRequestRequest) (*dynastypb.JoinRequestResponse, error)
	AcceptJoinRequestFunc     func(context.Context, *dynastypb.AcceptJoinRequestRequest) (*commonpb.Empty, error)
	RejectJoinRequestFunc     func(context.Context, *dynastypb.RejectJoinRequestRequest) (*commonpb.Empty, error)
	DeleteJoinRequestFunc     func(context.Context, *dynastypb.DeleteJoinRequestRequest) (*commonpb.Empty, error)
	GetDefaultPermissionsFunc func(context.Context, *dynastypb.GetDefaultPermissionsRequest) (*dynastypb.DefaultPermissionsResponse, error)
	SearchUsersFunc           func(context.Context, *dynastypb.SearchUsersRequest) (*dynastypb.SearchUsersResponse, error)
}

func (f *fakeJoinRequestAPI) SendJoinRequest(ctx context.Context, req *dynastypb.SendJoinRequestRequest) (*dynastypb.JoinRequestResponse, error) {
	if f.SendJoinRequestFunc != nil {
		return f.SendJoinRequestFunc(ctx, req)
	}
	return &dynastypb.JoinRequestResponse{Id: 1, Status: 0, Relationship: req.Relationship}, nil
}
func (f *fakeJoinRequestAPI) GetSentRequests(ctx context.Context, req *dynastypb.GetSentRequestsRequest) (*dynastypb.JoinRequestsResponse, error) {
	if f.GetSentRequestsFunc != nil {
		return f.GetSentRequestsFunc(ctx, req)
	}
	return &dynastypb.JoinRequestsResponse{}, nil
}
func (f *fakeJoinRequestAPI) GetReceivedRequests(ctx context.Context, req *dynastypb.GetReceivedRequestsRequest) (*dynastypb.JoinRequestsResponse, error) {
	if f.GetReceivedRequestsFunc != nil {
		return f.GetReceivedRequestsFunc(ctx, req)
	}
	return &dynastypb.JoinRequestsResponse{}, nil
}
func (f *fakeJoinRequestAPI) GetJoinRequest(ctx context.Context, req *dynastypb.GetJoinRequestRequest) (*dynastypb.JoinRequestResponse, error) {
	if f.GetJoinRequestFunc != nil {
		return f.GetJoinRequestFunc(ctx, req)
	}
	return &dynastypb.JoinRequestResponse{Id: req.RequestId, Status: 0, Relationship: "brother", CreatedAt: "1403/01/01 12:00"}, nil
}
func (f *fakeJoinRequestAPI) AcceptJoinRequest(ctx context.Context, req *dynastypb.AcceptJoinRequestRequest) (*commonpb.Empty, error) {
	if f.AcceptJoinRequestFunc != nil {
		return f.AcceptJoinRequestFunc(ctx, req)
	}
	return &commonpb.Empty{}, nil
}
func (f *fakeJoinRequestAPI) RejectJoinRequest(ctx context.Context, req *dynastypb.RejectJoinRequestRequest) (*commonpb.Empty, error) {
	if f.RejectJoinRequestFunc != nil {
		return f.RejectJoinRequestFunc(ctx, req)
	}
	return &commonpb.Empty{}, nil
}
func (f *fakeJoinRequestAPI) DeleteJoinRequest(ctx context.Context, req *dynastypb.DeleteJoinRequestRequest) (*commonpb.Empty, error) {
	if f.DeleteJoinRequestFunc != nil {
		return f.DeleteJoinRequestFunc(ctx, req)
	}
	return &commonpb.Empty{}, nil
}
func (f *fakeJoinRequestAPI) GetDefaultPermissions(ctx context.Context, req *dynastypb.GetDefaultPermissionsRequest) (*dynastypb.DefaultPermissionsResponse, error) {
	if f.GetDefaultPermissionsFunc != nil {
		return f.GetDefaultPermissionsFunc(ctx, req)
	}
	return &dynastypb.DefaultPermissionsResponse{Permissions: &dynastypb.ChildPermissions{BFR: true}}, nil
}
func (f *fakeJoinRequestAPI) SearchUsers(ctx context.Context, req *dynastypb.SearchUsersRequest) (*dynastypb.SearchUsersResponse, error) {
	if f.SearchUsersFunc != nil {
		return f.SearchUsersFunc(ctx, req)
	}
	return &dynastypb.SearchUsersResponse{}, nil
}

type fakeFamilyAPI struct {
	GetFamilyFunc           func(context.Context, *dynastypb.GetFamilyRequest) (*dynastypb.FamilyResponse, error)
	SetChildPermissionsFunc func(context.Context, *dynastypb.SetChildPermissionsRequest) (*commonpb.Empty, error)
}

func (f *fakeFamilyAPI) GetFamily(ctx context.Context, req *dynastypb.GetFamilyRequest) (*dynastypb.FamilyResponse, error) {
	if f.GetFamilyFunc != nil {
		return f.GetFamilyFunc(ctx, req)
	}
	return &dynastypb.FamilyResponse{Members: []*dynastypb.FamilyMember{
		{UserId: 1, Relationship: "owner", UserInfo: &commonpb.UserBasic{Id: 1, Code: "O1", ProfilePhoto: "p.jpg", Level: "Gold"}},
	}}, nil
}
func (f *fakeFamilyAPI) SetChildPermissions(ctx context.Context, req *dynastypb.SetChildPermissionsRequest) (*commonpb.Empty, error) {
	if f.SetChildPermissionsFunc != nil {
		return f.SetChildPermissionsFunc(ctx, req)
	}
	return &commonpb.Empty{}, nil
}

type fakePrizeAPI struct {
	GetPrizesFunc  func(context.Context, *dynastypb.GetPrizesRequest) (*dynastypb.PrizesResponse, error)
	ClaimPrizeFunc func(context.Context, *dynastypb.ClaimPrizeRequest) (*commonpb.Empty, error)
}

func (f *fakePrizeAPI) GetPrizes(ctx context.Context, req *dynastypb.GetPrizesRequest) (*dynastypb.PrizesResponse, error) {
	if f.GetPrizesFunc != nil {
		return f.GetPrizesFunc(ctx, req)
	}
	return &dynastypb.PrizesResponse{}, nil
}
func (f *fakePrizeAPI) ClaimPrize(ctx context.Context, req *dynastypb.ClaimPrizeRequest) (*commonpb.Empty, error) {
	if f.ClaimPrizeFunc != nil {
		return f.ClaimPrizeFunc(ctx, req)
	}
	return &commonpb.Empty{}, nil
}

func identityAuth(next http.Handler) http.Handler { return next }

func withUser(userID uint64, r *http.Request) *http.Request {
	userCtx := &authpkg.UserContext{UserID: userID, Email: "u@example.com", Token: "tok"}
	ctx := context.WithValue(r.Context(), authpkg.UserContextKey{}, userCtx)
	return r.WithContext(ctx)
}

func newHTTPHandler(d *fakeDynastyAPI, j *fakeJoinRequestAPI, f *fakeFamilyAPI, p *fakePrizeAPI) *handler.HTTPDynastyHandler {
	if d == nil {
		d = &fakeDynastyAPI{}
	}
	if j == nil {
		j = &fakeJoinRequestAPI{}
	}
	if f == nil {
		f = &fakeFamilyAPI{}
	}
	if p == nil {
		p = &fakePrizeAPI{}
	}
	return handler.NewHTTPDynastyHandler(d, j, f, p)
}

func TestHTTPDynastyHandler_StartHTTPServer_InvalidPort(t *testing.T) {
	h := newHTTPHandler(nil, nil, nil, nil)
	err := handler.StartHTTPServer(h, "not-a-valid-port", identityAuth)
	require.Error(t, err)
}

func TestHTTPDynastyHandler_HealthAndRegisterRoutes(t *testing.T) {
	h := newHTTPHandler(nil, nil, nil, nil)
	mux := http.NewServeMux()
	h.RegisterHTTPRoutes(mux, identityAuth)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/health", nil))
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"status":"ok"`)
}

func TestHTTPDynastyHandler_GetDynasty(t *testing.T) {
	t.Run("success with full response builders", func(t *testing.T) {
		photo := "img.png"
		api := &fakeDynastyAPI{
			GetUserDynastyFunc: func(_ context.Context, req *dynastypb.GetUserDynastyRequest) (*dynastypb.DynastyResponse, error) {
				assert.Equal(t, uint64(7), req.UserId)
				return &dynastypb.DynastyResponse{
					UserHasDynasty: true,
					Id:             11,
					FamilyId:       22,
					CreatedAt:      "1403/01/01",
					ProfileImage:   photo,
					DynastyFeature: &dynastypb.DynastyFeature{
						Id: 100, PropertiesId: "p1", Area: "a", Density: "d",
						FeatureProfitIncrease: "0.5", FamilyMembersCount: 3, LastUpdated: "1403/01/02",
					},
					Features: []*dynastypb.AvailableFeature{
						{Id: 100, PropertiesId: "p1", Density: "d", Stability: "15000", Area: "a"},
						{Id: 1, PropertiesId: "x", Density: "1", Stability: "2", Area: "3"},
					},
					Prizes: []*dynastypb.IntroductionPrize{{
						Member: "brother", Satisfaction: 1, IntroductionProfitIncrease: 2,
						AccumulatedCapitalReserve: 3, DataStorage: 4, Psc: "5",
					}},
				}, nil
			},
		}
		h := newHTTPHandler(api, nil, nil, nil)
		req := withUser(7, httptest.NewRequest(http.MethodGet, "/api/dynasty", nil))
		rr := httptest.NewRecorder()
		h.GetDynasty(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)

		var payload map[string]interface{}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &payload))
		data, ok := payload["data"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, true, data["user-has-dynasty"])
		assert.Equal(t, photo, data["profile-image"])
		assert.Equal(t, map[string]interface{}{
			"id": float64(100), "properties_id": "p1", "density": "d", "area": "a", "stability": "15000",
		}, data["features"])
		_, prizesOK := data["prizes"].([]interface{})
		assert.True(t, prizesOK)
	})

	t.Run("success without dynasty returns available features list", func(t *testing.T) {
		api := &fakeDynastyAPI{
			GetUserDynastyFunc: func(context.Context, *dynastypb.GetUserDynastyRequest) (*dynastypb.DynastyResponse, error) {
				return &dynastypb.DynastyResponse{
					UserHasDynasty: false,
					Features: []*dynastypb.AvailableFeature{
						{Id: 1, PropertiesId: "x", Density: "1", Stability: "2", Area: "3"},
					},
					Prizes: []*dynastypb.IntroductionPrize{{
						Member: "brother", Satisfaction: 1, IntroductionProfitIncrease: 2,
						AccumulatedCapitalReserve: 3, DataStorage: 4, Psc: "5",
					}},
				}, nil
			},
		}
		h := newHTTPHandler(api, nil, nil, nil)
		rr := httptest.NewRecorder()
		h.GetDynasty(rr, withUser(1, httptest.NewRequest(http.MethodGet, "/api/dynasty", nil)))
		assert.Equal(t, http.StatusOK, rr.Code)

		var payload map[string]interface{}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &payload))
		data, ok := payload["data"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, false, data["user-has-dynasty"])
		features, ok := data["features"].([]interface{})
		require.True(t, ok)
		require.Len(t, features, 1)
		assert.Equal(t, map[string]interface{}{
			"id": float64(1), "properties_id": "x", "density": "1", "stability": "2", "area": "3",
		}, features[0])
		_, hasID := data["id"]
		assert.False(t, hasID)
		_, hasDynastyFeature := data["dynasty-feature"]
		assert.False(t, hasDynastyFeature)
	})

	t.Run("success with dynasty and missing feature details", func(t *testing.T) {
		api := &fakeDynastyAPI{
			GetUserDynastyFunc: func(context.Context, *dynastypb.GetUserDynastyRequest) (*dynastypb.DynastyResponse, error) {
				return &dynastypb.DynastyResponse{UserHasDynasty: true, Id: 3, FamilyId: 4}, nil
			},
		}
		h := newHTTPHandler(api, nil, nil, nil)
		rr := httptest.NewRecorder()
		h.GetDynasty(rr, withUser(1, httptest.NewRequest(http.MethodGet, "/api/dynasty", nil)))
		assert.Equal(t, http.StatusOK, rr.Code)

		var payload map[string]interface{}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &payload))
		data, ok := payload["data"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, map[string]interface{}{
			"id": "", "properties_id": "", "density": "", "area": "", "stability": "",
		}, data["features"])
	})

	t.Run("unauthenticated", func(t *testing.T) {
		h := newHTTPHandler(nil, nil, nil, nil)
		rr := httptest.NewRecorder()
		h.GetDynasty(rr, httptest.NewRequest(http.MethodGet, "/api/dynasty", nil))
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("method not allowed", func(t *testing.T) {
		h := newHTTPHandler(nil, nil, nil, nil)
		rr := httptest.NewRecorder()
		h.GetDynasty(rr, withUser(1, httptest.NewRequest(http.MethodPost, "/api/dynasty", nil)))
		assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
	})

	t.Run("grpc error mapping", func(t *testing.T) {
		api := &fakeDynastyAPI{
			GetUserDynastyFunc: func(context.Context, *dynastypb.GetUserDynastyRequest) (*dynastypb.DynastyResponse, error) {
				return nil, status.Error(codes.NotFound, "missing")
			},
		}
		h := newHTTPHandler(api, nil, nil, nil)
		rr := httptest.NewRecorder()
		h.GetDynasty(rr, withUser(1, httptest.NewRequest(http.MethodGet, "/api/dynasty", nil)))
		assert.Equal(t, http.StatusNotFound, rr.Code)
	})
}

func TestHTTPDynastyHandler_CreateDynasty(t *testing.T) {
	api := &fakeDynastyAPI{
		CreateDynastyFunc: func(_ context.Context, req *dynastypb.CreateDynastyRequest) (*dynastypb.DynastyResponse, error) {
			assert.Equal(t, uint64(1), req.UserId)
			assert.Equal(t, uint64(55), req.FeatureId)
			return &dynastypb.DynastyResponse{UserHasDynasty: true, Id: 9, FamilyId: 8}, nil
		},
	}
	h := newHTTPHandler(api, nil, nil, nil)

	rr := httptest.NewRecorder()
	h.CreateDynasty(rr, withUser(1, httptest.NewRequest(http.MethodPost, "/api/dynasty/create/55", nil)))
	assert.Equal(t, http.StatusCreated, rr.Code)

	rr = httptest.NewRecorder()
	h.CreateDynasty(rr, withUser(1, httptest.NewRequest(http.MethodPost, "/api/dynasty/create/", nil)))
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = httptest.NewRecorder()
	h.CreateDynasty(rr, withUser(1, httptest.NewRequest(http.MethodPost, "/api/dynasty/create/abc", nil)))
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = httptest.NewRecorder()
	h.CreateDynasty(rr, withUser(1, httptest.NewRequest(http.MethodGet, "/api/dynasty/create/1", nil)))
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)

	rr = httptest.NewRecorder()
	h.CreateDynasty(rr, httptest.NewRequest(http.MethodPost, "/api/dynasty/create/1", nil))
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestHTTPDynastyHandler_UpdateAndFamily(t *testing.T) {
	h := newHTTPHandler(nil, nil, nil, nil)

	rr := httptest.NewRecorder()
	h.UpdateDynastyFeature(rr, withUser(1, httptest.NewRequest(http.MethodPost, "/api/dynasty/10/update/20", nil)))
	assert.Equal(t, http.StatusOK, rr.Code)

	rr = httptest.NewRecorder()
	h.UpdateDynastyFeature(rr, withUser(1, httptest.NewRequest(http.MethodPost, "/api/dynasty/bad/update/20", nil)))
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = httptest.NewRecorder()
	h.GetFamily(rr, withUser(1, httptest.NewRequest(http.MethodGet, "/api/dynasty/10/family/3", nil)))
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"relationship"`)
	assert.Contains(t, rr.Body.String(), `"level":"Gold"`)

	rr = httptest.NewRecorder()
	h.GetFamily(rr, withUser(1, httptest.NewRequest(http.MethodGet, "/api/dynasty/x/family/3", nil)))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHTTPDynastyHandler_JoinRequestFlows(t *testing.T) {
	join := &fakeJoinRequestAPI{
		GetSentRequestsFunc: func(_ context.Context, req *dynastypb.GetSentRequestsRequest) (*dynastypb.JoinRequestsResponse, error) {
			assert.Equal(t, int32(2), req.Pagination.Page)
			return &dynastypb.JoinRequestsResponse{Requests: []*dynastypb.JoinRequestResponse{{
				Id: 1, Status: 0, Relationship: "brother", CreatedAt: "1403/01/01 10:00",
				Message:      "please join",
				ToUserInfo:   &commonpb.UserBasic{Id: 2, Code: "C2", Name: "Bob", ProfilePhoto: "x"},
				RequestPrize: &dynastypb.DynastyPrize{Id: 9, Psc: 1, Member: "brother", Satisfaction: "s"},
			}}}, nil
		},
		GetReceivedRequestsFunc: func(context.Context, *dynastypb.GetReceivedRequestsRequest) (*dynastypb.JoinRequestsResponse, error) {
			return &dynastypb.JoinRequestsResponse{Requests: []*dynastypb.JoinRequestResponse{{
				Id: 2, Status: 0, Relationship: "sister", CreatedAt: "1403/01/02",
				Message:    "hello there",
				ToUserInfo: &commonpb.UserBasic{Id: 3, Code: "C3", Name: "Ann"},
			}}}, nil
		},
	}
	h := newHTTPHandler(nil, join, nil, nil)

	rr := httptest.NewRecorder()
	req := withUser(1, httptest.NewRequest(http.MethodGet, "/api/dynasty/requests/sent?page=2", nil))
	h.GetSentRequests(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"to_user"`)
	assert.Contains(t, rr.Body.String(), `"prize"`)
	assert.Contains(t, rr.Body.String(), "برادر")
	assert.Contains(t, rr.Body.String(), `"message":"please join"`)

	rr = httptest.NewRecorder()
	h.GetReceivedRequests(rr, withUser(1, httptest.NewRequest(http.MethodGet, "/api/dynasty/requests/recieved?page=1", nil)))
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"from_user"`)
	assert.Contains(t, rr.Body.String(), `"message":"hello there"`)

	body := `{"user":5,"relationship":"offspring","message":"hi","permissions":{"BFR":true,"SF":false,"W":true,"JU":false,"DM":false,"PIUP":false,"PITC":false,"PIC":false,"ESOO":false,"COTB":false}}`
	rr = httptest.NewRecorder()
	h.SendJoinRequest(rr, withUser(1, httptest.NewRequest(http.MethodPost, "/api/dynasty/add/member", strings.NewReader(body))))
	assert.Equal(t, http.StatusCreated, rr.Code)

	rr = httptest.NewRecorder()
	h.SendJoinRequest(rr, withUser(1, httptest.NewRequest(http.MethodPost, "/api/dynasty/add/member", nil)))
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = httptest.NewRecorder()
	h.AcceptJoinRequest(rr, withUser(1, httptest.NewRequest(http.MethodPost, "/api/dynasty/requests/recieved/9", nil)))
	assert.Equal(t, http.StatusNoContent, rr.Code)

	rr = httptest.NewRecorder()
	h.RejectJoinRequest(rr, withUser(1, httptest.NewRequest(http.MethodDelete, "/api/dynasty/requests/recieved/9", nil)))
	assert.Equal(t, http.StatusNoContent, rr.Code)

	rr = httptest.NewRecorder()
	h.GetSentRequest(rr, withUser(1, httptest.NewRequest(http.MethodGet, "/api/dynasty/requests/sent/9", nil)))
	assert.Equal(t, http.StatusOK, rr.Code)

	rr = httptest.NewRecorder()
	h.GetReceivedRequest(rr, withUser(1, httptest.NewRequest(http.MethodGet, "/api/dynasty/requests/recieved/9", nil)))
	assert.Equal(t, http.StatusOK, rr.Code)

	rr = httptest.NewRecorder()
	h.DeleteJoinRequest(rr, withUser(1, httptest.NewRequest(http.MethodDelete, "/api/dynasty/requests/sent/9", nil)))
	assert.Equal(t, http.StatusNoContent, rr.Code)

	rr = httptest.NewRecorder()
	h.AcceptJoinRequest(rr, withUser(1, httptest.NewRequest(http.MethodPost, "/api/dynasty/requests/recieved/", nil)))
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = httptest.NewRecorder()
	h.AcceptJoinRequest(rr, withUser(1, httptest.NewRequest(http.MethodPost, "/api/dynasty/requests/recieved/abc", nil)))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHTTPDynastyHandler_SearchUsers_IncludesVerifiedAgeLevels(t *testing.T) {
	join := &fakeJoinRequestAPI{
		SearchUsersFunc: func(context.Context, *dynastypb.SearchUsersRequest) (*dynastypb.SearchUsersResponse, error) {
			return &dynastypb.SearchUsersResponse{
				Data: []*dynastypb.UserSearchResult{
					{
						Id:       7,
						Code:     "U7",
						Name:     "Sara",
						Image:    "https://img/s.png",
						Level:    "Gold",
						Verified: true,
						Age:      22,
						Levels: []*dynastypb.UserSearchLevelItem{
							{
								Id:   3,
								Slug: "3",
								Gem: &dynastypb.UserSearchLevelGem{
									Id: 13, Name: "Gold Gem", Image: "https://gem/gold.png",
								},
							},
							{
								Id:   1,
								Slug: "1",
								Gem: &dynastypb.UserSearchLevelGem{
									Id: 11, Name: "Bronze Gem", Image: "https://gem/bronze.png",
								},
							},
						},
					},
				},
			}, nil
		},
	}
	h := newHTTPHandler(nil, join, nil, nil)
	rr := httptest.NewRecorder()
	h.SearchUsers(rr, withUser(1, httptest.NewRequest(
		http.MethodPost,
		"/api/dynasty/search",
		strings.NewReader(`{"searchTerm":"sara"}`),
	)))
	require.Equal(t, http.StatusOK, rr.Code)

	var body map[string]interface{}
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	data, ok := body["data"].([]interface{})
	require.True(t, ok)
	require.Len(t, data, 1)
	item := data[0].(map[string]interface{})
	assert.Equal(t, true, item["verified"])
	assert.Equal(t, float64(22), item["age"])
	assert.Equal(t, "Gold", item["level"])
	levels := item["levels"].([]interface{})
	require.Len(t, levels, 2)
	first := levels[0].(map[string]interface{})
	assert.Equal(t, float64(3), first["id"])
	assert.Equal(t, "3", first["slug"])
	gem := first["gem"].(map[string]interface{})
	assert.Equal(t, "Gold Gem", gem["name"])
	assert.Equal(t, "https://gem/gold.png", gem["image"])
}

func TestHTTPDynastyHandler_PrizesSearchPermissionsChildren(t *testing.T) {
	h := newHTTPHandler(nil, nil, nil, nil)

	rr := httptest.NewRecorder()
	h.GetPrizes(rr, withUser(1, httptest.NewRequest(http.MethodGet, "/api/dynasty/prizes?page=3", nil)))
	assert.Equal(t, http.StatusOK, rr.Code)

	rr = httptest.NewRecorder()
	h.ClaimPrize(rr, withUser(1, httptest.NewRequest(http.MethodPost, "/api/dynasty/prizes/12", nil)))
	assert.Equal(t, http.StatusNoContent, rr.Code)

	rr = httptest.NewRecorder()
	h.ClaimPrize(rr, withUser(1, httptest.NewRequest(http.MethodPost, "/api/dynasty/prizes/", nil)))
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = httptest.NewRecorder()
	h.SearchUsers(rr, withUser(1, httptest.NewRequest(http.MethodPost, "/api/dynasty/search", strings.NewReader(`{"searchTerm":"ali"}`))))
	assert.Equal(t, http.StatusOK, rr.Code)

	rr = httptest.NewRecorder()
	h.SearchUsers(rr, withUser(1, httptest.NewRequest(http.MethodPost, "/api/dynasty/search", strings.NewReader(`{"searchTerm":""}`))))
	assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)

	rr = httptest.NewRecorder()
	h.GetDefaultPermissions(rr, withUser(1, httptest.NewRequest(http.MethodPost, "/api/dynasty/add/member/get/permissions", strings.NewReader(`{"relationship":"offspring"}`))))
	assert.Equal(t, http.StatusOK, rr.Code)

	rr = httptest.NewRecorder()
	h.GetDefaultPermissions(rr, withUser(1, httptest.NewRequest(http.MethodPost, "/api/dynasty/add/member/get/permissions", strings.NewReader(`{"relationship":"brother"}`))))
	assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)

	rr = httptest.NewRecorder()
	h.GetDefaultPermissions(rr, withUser(1, httptest.NewRequest(http.MethodGet, "/api/dynasty/add/member/get/permissions", nil)))
	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)

	perms := []string{"BFR", "SF", "W", "JU", "DM", "PIUP", "PITC", "PIC", "ESOO", "COTB"}
	for _, code := range perms {
		body := `{"permission":"` + code + `","status":true}`
		rr = httptest.NewRecorder()
		h.UpdateChildPermissions(rr, withUser(1, httptest.NewRequest(http.MethodPost, "/api/dynasty/children/5", strings.NewReader(body))))
		assert.Equal(t, http.StatusOK, rr.Code, code)
	}

	rr = httptest.NewRecorder()
	h.UpdateChildPermissions(rr, withUser(1, httptest.NewRequest(http.MethodPost, "/api/dynasty/children/5", strings.NewReader(`{"permission":"NOPE","status":true}`))))
	assert.Equal(t, http.StatusUnprocessableEntity, rr.Code)

	rr = httptest.NewRecorder()
	h.UpdateChildPermissions(rr, withUser(1, httptest.NewRequest(http.MethodPost, "/api/dynasty/children/", strings.NewReader(`{}`))))
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHTTPDynastyHandler_ErrorMappingAndInvalidBody(t *testing.T) {
	api := &fakeDynastyAPI{
		GetUserDynastyFunc: func(context.Context, *dynastypb.GetUserDynastyRequest) (*dynastypb.DynastyResponse, error) {
			return nil, errors.New("plain")
		},
	}
	h := newHTTPHandler(api, nil, nil, nil)
	rr := httptest.NewRecorder()
	h.GetDynasty(rr, withUser(1, httptest.NewRequest(http.MethodGet, "/api/dynasty", nil)))
	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	cases := []struct {
		code codes.Code
		want int
	}{
		{codes.Unauthenticated, http.StatusUnauthorized},
		{codes.PermissionDenied, http.StatusForbidden},
		{codes.AlreadyExists, http.StatusConflict},
		{codes.FailedPrecondition, http.StatusPreconditionFailed},
		{codes.Unavailable, http.StatusServiceUnavailable},
		{codes.InvalidArgument, http.StatusUnprocessableEntity},
		{codes.Internal, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		api.GetUserDynastyFunc = func(context.Context, *dynastypb.GetUserDynastyRequest) (*dynastypb.DynastyResponse, error) {
			return nil, status.Error(tc.code, "x")
		}
		rr = httptest.NewRecorder()
		h.GetDynasty(rr, withUser(1, httptest.NewRequest(http.MethodGet, "/api/dynasty", nil)))
		assert.Equal(t, tc.want, rr.Code, tc.code.String())
	}

	join := &fakeJoinRequestAPI{}
	h = newHTTPHandler(nil, join, nil, nil)
	rr = httptest.NewRecorder()
	req := withUser(1, httptest.NewRequest(http.MethodPost, "/api/dynasty/add/member", bytes.NewReader([]byte("not-json"))))
	req.ContentLength = int64(len("not-json"))
	h.SendJoinRequest(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	rr = httptest.NewRecorder()
	empty := withUser(1, httptest.NewRequest(http.MethodPost, "/api/dynasty/add/member", http.NoBody))
	empty.Body = io.NopCloser(bytes.NewReader(nil))
	empty.ContentLength = 0
	h.SendJoinRequest(rr, empty)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHTTPDynastyHandler_CatchAllRoutes(t *testing.T) {
	h := newHTTPHandler(nil, nil, nil, nil)
	mux := http.NewServeMux()
	h.RegisterHTTPRoutes(mux, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, withUser(1, r))
		})
	})

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/dynasty/1/update/2", nil))
	assert.Equal(t, http.StatusOK, rr.Code)

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/dynasty/1/family/2", nil))
	assert.Equal(t, http.StatusOK, rr.Code)

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/dynasty/1/unknown", nil))
	assert.Equal(t, http.StatusNotFound, rr.Code)

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/dynasty/requests/sent", nil))
	assert.Equal(t, http.StatusOK, rr.Code)

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/dynasty/requests/sent", nil))
	assert.Equal(t, http.StatusNotFound, rr.Code)

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/dynasty/requests/sent/3", nil))
	assert.Equal(t, http.StatusOK, rr.Code)

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/api/dynasty/requests/sent/3", nil))
	assert.Equal(t, http.StatusNoContent, rr.Code)

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/dynasty/requests/recieved", nil))
	assert.Equal(t, http.StatusOK, rr.Code)

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/dynasty/requests/recieved/3", nil))
	assert.Equal(t, http.StatusNoContent, rr.Code)

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/api/dynasty/requests/recieved/3", nil))
	assert.Equal(t, http.StatusNoContent, rr.Code)

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/dynasty/prizes", nil))
	assert.Equal(t, http.StatusOK, rr.Code)

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/dynasty/prizes/1", nil))
	assert.Equal(t, http.StatusNoContent, rr.Code)

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/dynasty/prizes/1", nil))
	assert.Equal(t, http.StatusNotFound, rr.Code)

	body, _ := json.Marshal(map[string]interface{}{"user": 2, "relationship": "brother"})
	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/dynasty/add/member", bytes.NewReader(body)))
	assert.Equal(t, http.StatusCreated, rr.Code)

	rr = httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/dynasty/children/9", strings.NewReader(`{"permission":"BFR","status":true}`)))
	assert.Equal(t, http.StatusOK, rr.Code)
}
