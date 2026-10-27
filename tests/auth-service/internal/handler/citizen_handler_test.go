package handler_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/auth-service/internal/handler"
	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/service"
	pb "metarang/shared/pb/auth"
)

type mockCitizenService struct {
	userInfo *models.CitizenUserInfo
	profile  *models.CitizenProfile
	level    *models.CitizenLevel
	err      error
}

func (m *mockCitizenService) GetCitizenUserInfo(context.Context, string) (*models.CitizenUserInfo, error) {
	return m.userInfo, m.err
}
func (m *mockCitizenService) GetCitizenLevel(context.Context, string) (*models.CitizenLevel, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.level, nil
}
func (m *mockCitizenService) GetCitizenProfile(context.Context, string) (*models.CitizenProfile, error) {
	return m.profile, m.err
}
func (m *mockCitizenService) GetCitizenReferrals(context.Context, string, string, int32) ([]*models.CitizenReferral, *models.PaginationMeta, error) {
	return []*models.CitizenReferral{}, &models.PaginationMeta{CurrentPage: 1}, m.err
}
func (m *mockCitizenService) GetCitizenReferralChart(context.Context, string, string) (*models.ReferralChartData, error) {
	return &models.ReferralChartData{}, m.err
}
func (m *mockCitizenService) AbsoluteURL(path string) string { return "https://app" + path }
func (m *mockCitizenService) PassionIconURL(string) string   { return "https://app/p.png" }
func (m *mockCitizenService) NationalityFlagURL() string     { return "https://app/f.png" }
func (m *mockCitizenService) CitizenPosition() string        { return "pos" }
func (m *mockCitizenService) CitizenAvatar() string          { return "https://app/a.png" }
func (m *mockCitizenService) ScorePercentageToNextLevel(context.Context, uint64, int32) float64 {
	return 42.5
}

var _ service.CitizenService = (*mockCitizenService)(nil)

func TestCitizenHandler(t *testing.T) {
	t.Run("empty code validation", func(t *testing.T) {
		h := handler.RegisterCitizenHandler(grpc.NewServer(), &mockCitizenService{})
		_, err := h.GetCitizenUserInfo(context.Background(), &pb.GetCitizenUserInfoRequest{})
		st, _ := status.FromError(err)
		if st.Code() != codes.InvalidArgument {
			t.Fatalf("code=%v", st.Code())
		}
		_, err = h.GetCitizenProfile(context.Background(), &pb.GetCitizenProfileRequest{})
		st, _ = status.FromError(err)
		if st.Code() != codes.InvalidArgument {
			t.Fatalf("code=%v", st.Code())
		}
		_, err = h.GetCitizenLevel(context.Background(), &pb.GetCitizenLevelRequest{})
		st, _ = status.FromError(err)
		if st.Code() != codes.InvalidArgument {
			t.Fatalf("code=%v", st.Code())
		}
	})

	t.Run("not found", func(t *testing.T) {
		h := handler.RegisterCitizenHandler(grpc.NewServer(), &mockCitizenService{})
		_, err := h.GetCitizenUserInfo(context.Background(), &pb.GetCitizenUserInfoRequest{Code: "x"})
		st, _ := status.FromError(err)
		if st.Code() != codes.NotFound {
			t.Fatalf("code=%v", st.Code())
		}
		_, err = h.GetCitizenLevel(context.Background(), &pb.GetCitizenLevelRequest{Code: "x"})
		st, _ = status.FromError(err)
		if st.Code() != codes.NotFound {
			t.Fatalf("code=%v", st.Code())
		}
	})

	t.Run("user info success", func(t *testing.T) {
		h := handler.RegisterCitizenHandler(grpc.NewServer(), &mockCitizenService{
			userInfo: &models.CitizenUserInfo{UserID: 9, Privacy: map[string]int32{"score": 1}},
			level:    &models.CitizenLevel{Slug: "citizen", Score: 10},
		})
		resp, err := h.GetCitizenUserInfo(context.Background(), &pb.GetCitizenUserInfoRequest{Code: "hm-1"})
		if err != nil || resp.UserId != 9 {
			t.Fatalf("resp=%v err=%v", resp, err)
		}
		lvl, err := h.GetCitizenLevel(context.Background(), &pb.GetCitizenLevelRequest{Code: "hm-1"})
		if err != nil || lvl.GetLevel().GetSlug() != "citizen" || lvl.GetLevel().GetScore() != "10" {
			t.Fatalf("level=%v err=%v", lvl, err)
		}
	})

	t.Run("profile success with privacy", func(t *testing.T) {
		h := handler.RegisterCitizenHandler(grpc.NewServer(), &mockCitizenService{
			profile: &models.CitizenProfile{
				ID: 1, Code: "hm-1", Name: "n", Score: 10,
				EmailVerifiedAt: time.Now(),
				Privacy: map[string]bool{
					"score": true, "name": true, "level": true, "details": true,
				},
				ProfilePhotos: []*models.ProfilePhoto{{ID: 1, URL: "/p.jpg"}},
				CurrentLevel:  &models.CitizenLevel{ID: 2, Name: "L", Score: 1, Slug: "l"},
				PersonalInfo: &models.CitizenPersonalInfo{
					Occupation: "o",
					Passions:   map[string]bool{"music": true},
				},
			},
		})
		resp, err := h.GetCitizenProfile(context.Background(), &pb.GetCitizenProfileRequest{Code: "hm-1"})
		if err != nil || resp == nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("profile makes missing privacy keys public", func(t *testing.T) {
		h := handler.RegisterCitizenHandler(grpc.NewServer(), &mockCitizenService{
			profile: &models.CitizenProfile{
				ID: 1, Code: "hm-1", Name: "n", Score: 10,
				Privacy: map[string]bool{"email": false},
				KYC:     &models.CitizenKYC{Fname: "first"},
				Phone:   "0912", Email: "hidden@example.test",
				PersonalInfo: &models.CitizenPersonalInfo{Occupation: "engineer"},
			},
		})

		resp, err := h.GetCitizenProfile(context.Background(), &pb.GetCitizenProfileRequest{Code: "hm-1"})
		if err != nil {
			t.Fatal(err)
		}
		if resp.GetName() != "n" || resp.GetScore() != 10 || resp.GetKyc().GetFname() != "first" || resp.GetKyc().GetPhone() != "0912" || resp.GetCustoms().GetOccupation() != "engineer" {
			t.Fatalf("missing privacy keys should be public, got %+v", resp)
		}
		if resp.GetKyc().GetEmail() != "" {
			t.Fatalf("explicitly private email was exposed: %+v", resp.GetKyc())
		}
	})

	t.Run("referrals and chart", func(t *testing.T) {
		h := handler.RegisterCitizenHandler(grpc.NewServer(), &mockCitizenService{})
		_, err := h.GetCitizenReferrals(context.Background(), &pb.GetCitizenReferralsRequest{Code: ""})
		st, _ := status.FromError(err)
		if st.Code() != codes.InvalidArgument {
			t.Fatalf("code=%v", st.Code())
		}
		resp, err := h.GetCitizenReferrals(context.Background(), &pb.GetCitizenReferralsRequest{Code: "hm-1", Page: 0})
		if err != nil || resp == nil {
			t.Fatalf("err=%v", err)
		}
		_, err = h.GetCitizenReferralChart(context.Background(), &pb.GetCitizenReferralChartRequest{Code: "hm-1"})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("service error", func(t *testing.T) {
		h := handler.RegisterCitizenHandler(grpc.NewServer(), &mockCitizenService{err: errors.New("db")})
		_, err := h.GetCitizenProfile(context.Background(), &pb.GetCitizenProfileRequest{Code: "x"})
		st, _ := status.FromError(err)
		if st.Code() != codes.Internal {
			t.Fatalf("code=%v", st.Code())
		}
		_, err = h.GetCitizenLevel(context.Background(), &pb.GetCitizenLevelRequest{Code: "x"})
		st, _ = status.FromError(err)
		if st.Code() != codes.Internal {
			t.Fatalf("code=%v", st.Code())
		}
	})
}
