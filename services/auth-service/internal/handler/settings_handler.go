package handler

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"metargb/auth-service/internal/lang"
	"metargb/auth-service/internal/service"
	pb "metargb/shared/pb/auth"
)

type settingsHandler struct {
	pb.UnimplementedSettingsServiceServer
	settingsService service.SettingsService
}

func RegisterSettingsHandler(grpcServer *grpc.Server, settingsService service.SettingsService) {
	pb.RegisterSettingsServiceServer(grpcServer, &settingsHandler{
		settingsService: settingsService,
	})
}

func (h *settingsHandler) GetSettings(ctx context.Context, req *pb.GetSettingsRequest) (*pb.GetSettingsResponse, error) {
	locale := getProjectLocale()
	settings, err := h.settingsService.GetSettings(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, lang.Tf(locale, "failed to get settings: %v", err))
	}

	return &pb.GetSettingsResponse{
		Data: &pb.SettingsData{
			CheckoutDaysCount: settings.CheckoutDaysCount,
			AutomaticLogout:   settings.AutomaticLogout,
		},
	}, nil
}

func (h *settingsHandler) UpdateSettings(ctx context.Context, req *pb.UpdateSettingsRequest) (*emptypb.Empty, error) {
	locale := getProjectLocale()
	var checkoutDaysCount *uint32
	var automaticLogout *int32
	var setting *string
	var statusVal *bool

	// Check if updating checkout cadence (checkout_days_count must be >= 3 if provided)
	checkoutProvided := req.CheckoutDaysCount >= 3
	automaticProvided := req.AutomaticLogout >= 1

	if checkoutProvided || automaticProvided {
		// Both must be present
		if !checkoutProvided || !automaticProvided {
			return nil, status.Errorf(codes.InvalidArgument, lang.T(locale, "both checkout_days_count and automatic_logout must be provided when updating checkout cadence"))
		}
		checkoutDaysCountVal := req.CheckoutDaysCount
		automaticLogoutVal := req.AutomaticLogout
		checkoutDaysCount = &checkoutDaysCountVal
		automaticLogout = &automaticLogoutVal
	}

	// Check if updating profile exposure
	if req.Setting != "" {
		// Both must be present
		val := req.Status
		settingVal := req.Setting
		setting = &settingVal
		statusVal = &val
	}

	err := h.settingsService.UpdateSettings(ctx, req.UserId, checkoutDaysCount, automaticLogout, setting, statusVal)
	if err != nil {
		switch err {
		case service.ErrInvalidCheckoutDays, service.ErrInvalidAutomaticLogout, service.ErrInvalidProfileSetting, service.ErrMissingRequiredFields:
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		default:
			return nil, status.Errorf(codes.Internal, lang.Tf(locale, "failed to update settings: %v", err))
		}
	}

	return &emptypb.Empty{}, nil
}

func (h *settingsHandler) GetGeneralSettings(ctx context.Context, req *pb.GetGeneralSettingsRequest) (*pb.GetGeneralSettingsResponse, error) {
	locale := getProjectLocale()
	notifications, err := h.settingsService.GetGeneralSettings(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, lang.Tf(locale, "failed to get general settings: %v", err))
	}

	return &pb.GetGeneralSettingsResponse{
		Data: &pb.NotificationSettingsData{
			AnnouncementsSms:       notifications["announcements_sms"],
			AnnouncementsEmail:     notifications["announcements_email"],
			ReportsSms:             notifications["reports_sms"],
			ReportsEmail:           notifications["reports_email"],
			LoginVerificationSms:   notifications["login_verification_sms"],
			LoginVerificationEmail: notifications["login_verification_email"],
			TransactionsSms:        notifications["transactions_sms"],
			TransactionsEmail:      notifications["transactions_email"],
			TradesSms:              notifications["trades_sms"],
			TradesEmail:            notifications["trades_email"],
		},
	}, nil
}

func (h *settingsHandler) UpdateGeneralSettings(ctx context.Context, req *pb.UpdateGeneralSettingsRequest) (*pb.UpdateGeneralSettingsResponse, error) {
	notifications := map[string]bool{
		"announcements_sms":        req.Notifications.AnnouncementsSms,
		"announcements_email":      req.Notifications.AnnouncementsEmail,
		"reports_sms":              req.Notifications.ReportsSms,
		"reports_email":            req.Notifications.ReportsEmail,
		"login_verification_sms":   req.Notifications.LoginVerificationSms,
		"login_verification_email": req.Notifications.LoginVerificationEmail,
		"transactions_sms":         req.Notifications.TransactionsSms,
		"transactions_email":       req.Notifications.TransactionsEmail,
		"trades_sms":               req.Notifications.TradesSms,
		"trades_email":             req.Notifications.TradesEmail,
	}

	locale := getProjectLocale()
	updated, err := h.settingsService.UpdateGeneralSettings(ctx, req.UserId, req.SettingId, notifications)
	if err != nil {
		switch err {
		case service.ErrSettingsNotFound:
			return nil, status.Errorf(codes.NotFound, lang.T(locale, "settings not found"))
		default:
			if err.Error() == "settings do not belong to user" {
				return nil, status.Errorf(codes.PermissionDenied, lang.T(locale, "settings do not belong to user"))
			}
			if err.Error() == "missing required notification channel" {
				return nil, status.Errorf(codes.InvalidArgument, "%v", err)
			}
			return nil, status.Errorf(codes.Internal, lang.Tf(locale, "failed to update general settings: %v", err))
		}
	}

	return &pb.UpdateGeneralSettingsResponse{
		Data: &pb.NotificationSettingsData{
			AnnouncementsSms:       updated["announcements_sms"],
			AnnouncementsEmail:     updated["announcements_email"],
			ReportsSms:             updated["reports_sms"],
			ReportsEmail:           updated["reports_email"],
			LoginVerificationSms:   updated["login_verification_sms"],
			LoginVerificationEmail: updated["login_verification_email"],
			TransactionsSms:        updated["transactions_sms"],
			TransactionsEmail:      updated["transactions_email"],
			TradesSms:              updated["trades_sms"],
			TradesEmail:            updated["trades_email"],
		},
	}, nil
}

func (h *settingsHandler) GetPrivacySettings(ctx context.Context, req *pb.GetPrivacySettingsRequest) (*pb.GetPrivacySettingsResponse, error) {
	locale := getProjectLocale()
	privacy, err := h.settingsService.GetPrivacySettings(ctx, req.UserId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, lang.Tf(locale, "failed to get privacy settings: %v", err))
	}

	// Convert map[string]int to map[string]int32
	privacyInt32 := make(map[string]int32)
	for k, v := range privacy {
		privacyInt32[k] = int32(v)
	}

	return &pb.GetPrivacySettingsResponse{
		Data: privacyInt32,
	}, nil
}

func (h *settingsHandler) UpdatePrivacySettings(ctx context.Context, req *pb.UpdatePrivacySettingsRequest) (*emptypb.Empty, error) {
	locale := getProjectLocale()
	err := h.settingsService.UpdatePrivacySettings(ctx, req.UserId, req.Key, req.Value)
	if err != nil {
		switch err {
		case service.ErrInvalidPrivacyKey, service.ErrInvalidPrivacyValue:
			return nil, status.Errorf(codes.InvalidArgument, "%v", err)
		default:
			return nil, status.Errorf(codes.Internal, lang.Tf(locale, "failed to update privacy settings: %v", err))
		}
	}

	return &emptypb.Empty{}, nil
}
