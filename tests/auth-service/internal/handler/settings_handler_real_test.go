package handler_test

import (
	"context"
	"errors"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"metarang/auth-service/internal/handler"
	"metarang/auth-service/internal/models"
	"metarang/auth-service/internal/service"
	pb "metarang/shared/pb/auth"
)

type settingsSvcMock struct {
	getSettingsFunc           func(context.Context, uint64) (*models.Settings, error)
	updateSettingsFunc        func(context.Context, uint64, *uint32, *int32, *string, *bool) error
	getGeneralSettingsFunc    func(context.Context, uint64) (*service.GeneralSettings, error)
	updateGeneralSettingsFunc func(context.Context, uint64, uint64, map[string]bool) (map[string]bool, error)
	getPrivacySettingsFunc    func(context.Context, uint64) (map[string]int, error)
	updatePrivacySettingsFunc func(context.Context, uint64, string, int32) error
}

func (m *settingsSvcMock) GetSettings(ctx context.Context, userID uint64) (*models.Settings, error) {
	if m.getSettingsFunc != nil {
		return m.getSettingsFunc(ctx, userID)
	}
	return &models.Settings{CheckoutDaysCount: 3, AutomaticLogout: 55}, nil
}
func (m *settingsSvcMock) UpdateSettings(ctx context.Context, userID uint64, c *uint32, l *int32, s *string, st *bool) error {
	if m.updateSettingsFunc != nil {
		return m.updateSettingsFunc(ctx, userID, c, l, s, st)
	}
	return nil
}
func (m *settingsSvcMock) GetGeneralSettings(ctx context.Context, userID uint64) (*service.GeneralSettings, error) {
	if m.getGeneralSettingsFunc != nil {
		return m.getGeneralSettingsFunc(ctx, userID)
	}
	return &service.GeneralSettings{ID: 1, Notifications: models.DefaultNotificationSettings()}, nil
}
func (m *settingsSvcMock) UpdateGeneralSettings(ctx context.Context, userID, settingID uint64, n map[string]bool) (map[string]bool, error) {
	if m.updateGeneralSettingsFunc != nil {
		return m.updateGeneralSettingsFunc(ctx, userID, settingID, n)
	}
	return n, nil
}
func (m *settingsSvcMock) GetPrivacySettings(ctx context.Context, userID uint64) (map[string]int, error) {
	if m.getPrivacySettingsFunc != nil {
		return m.getPrivacySettingsFunc(ctx, userID)
	}
	return models.DefaultPrivacySettings(), nil
}
func (m *settingsSvcMock) UpdatePrivacySettings(ctx context.Context, userID uint64, key string, value int32) error {
	if m.updatePrivacySettingsFunc != nil {
		return m.updatePrivacySettingsFunc(ctx, userID, key, value)
	}
	return nil
}

var _ service.SettingsService = (*settingsSvcMock)(nil)

func TestSettingsHandler_Real(t *testing.T) {
	ctx := authenticatedContext(1)
	h := handler.RegisterSettingsHandler(grpc.NewServer(), &settingsSvcMock{})

	t.Run("get settings", func(t *testing.T) {
		resp, err := h.GetSettings(ctx, &pb.GetSettingsRequest{})
		if err != nil || resp.Data.AutomaticLogout != 55 {
			t.Fatalf("resp=%v err=%v", resp, err)
		}
	})

	t.Run("get settings includes available mobile resets", func(t *testing.T) {
		h := handler.RegisterSettingsHandler(grpc.NewServer(), &settingsSvcMock{
			getSettingsFunc: func(context.Context, uint64) (*models.Settings, error) {
				return &models.Settings{CheckoutDaysCount: 3, AutomaticLogout: 55, AvailableResetMobileResets: 2}, nil
			},
		})
		resp, err := h.GetSettings(ctx, &pb.GetSettingsRequest{})
		if err != nil {
			t.Fatalf("GetSettings failed: %v", err)
		}
		if resp.Data.AvailableResetMobileResets != 2 {
			t.Fatalf("expected available_reset_mobile_resets 2, got %d", resp.Data.AvailableResetMobileResets)
		}
	})

	t.Run("update settings", func(t *testing.T) {
		_, err := h.UpdateSettings(ctx, &pb.UpdateSettingsRequest{CheckoutDaysCount: 5})
		st, _ := status.FromError(err)
		if st.Code() != codes.InvalidArgument {
			t.Fatalf("code=%v", st.Code())
		}
		_, err = h.UpdateSettings(ctx, &pb.UpdateSettingsRequest{CheckoutDaysCount: 5, AutomaticLogout: 30})
		if err != nil {
			t.Fatal(err)
		}
		_, err = h.UpdateSettings(ctx, &pb.UpdateSettingsRequest{Setting: "status", Status: true})
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("update error mapping", func(t *testing.T) {
		m := &settingsSvcMock{}
		m.updateSettingsFunc = func(context.Context, uint64, *uint32, *int32, *string, *bool) error {
			return service.ErrInvalidCheckoutDays
		}
		h := handler.RegisterSettingsHandler(grpc.NewServer(), m)
		_, err := h.UpdateSettings(ctx, &pb.UpdateSettingsRequest{CheckoutDaysCount: 5, AutomaticLogout: 30})
		st, _ := status.FromError(err)
		if st.Code() != codes.InvalidArgument {
			t.Fatalf("code=%v", st.Code())
		}
		m.updateSettingsFunc = func(context.Context, uint64, *uint32, *int32, *string, *bool) error {
			return errors.New("db")
		}
		_, err = h.UpdateSettings(ctx, &pb.UpdateSettingsRequest{CheckoutDaysCount: 5, AutomaticLogout: 30})
		st, _ = status.FromError(err)
		if st.Code() != codes.Internal {
			t.Fatalf("code=%v", st.Code())
		}
	})

	t.Run("general and privacy", func(t *testing.T) {
		resp, err := h.GetGeneralSettings(ctx, &pb.GetGeneralSettingsRequest{})
		if err != nil || resp.Data == nil {
			t.Fatalf("err=%v", err)
		}
		_, err = h.UpdateGeneralSettings(ctx, &pb.UpdateGeneralSettingsRequest{
			SettingId: 1,
			Notifications: &pb.NotificationSettingsData{
				AnnouncementsSms: true,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		pr, err := h.GetPrivacySettings(ctx, &pb.GetPrivacySettingsRequest{})
		if err != nil || pr.Data == nil {
			t.Fatalf("err=%v", err)
		}
		_, err = h.UpdatePrivacySettings(ctx, &pb.UpdatePrivacySettingsRequest{Key: "score", Value: 1})
		if err != nil {
			t.Fatal(err)
		}
	})
}
