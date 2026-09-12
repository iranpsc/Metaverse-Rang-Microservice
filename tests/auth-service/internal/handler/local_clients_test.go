package handler_test

import (
	"context"
	"testing"

	"metarang/auth-service/internal/handler"
	pb "metarang/shared/pb/auth"
)

// stubServers embed Unimplemented* so LocalClients wrappers are exercised even when RPCs return Unimplemented.
type stubAuth struct {
	pb.UnimplementedAuthServiceServer
}
type stubUser struct {
	pb.UnimplementedUserServiceServer
}
type stubKYC struct {
	pb.UnimplementedKYCServiceServer
}
type stubCitizen struct {
	pb.UnimplementedCitizenServiceServer
}
type stubPersonalInfo struct {
	pb.UnimplementedPersonalInfoServiceServer
}
type stubProfileLimitation struct {
	pb.UnimplementedProfileLimitationServiceServer
}
type stubProfilePhoto struct {
	pb.UnimplementedProfilePhotoServiceServer
}
type stubSettings struct {
	pb.UnimplementedSettingsServiceServer
}
type stubUserEvents struct {
	pb.UnimplementedUserEventsServiceServer
}
type stubSearch struct {
	pb.UnimplementedSearchServiceServer
}
type stubWallet struct {
	pb.UnimplementedWalletConnectionServiceServer
}

func TestLocalClients_AllMethods(t *testing.T) {
	clients := handler.NewLocalClients(
		&stubAuth{},
		&stubUser{},
		&stubKYC{},
		&stubCitizen{},
		&stubPersonalInfo{},
		&stubProfileLimitation{},
		&stubProfilePhoto{},
		&stubSettings{},
		&stubUserEvents{},
		&stubSearch{},
		&stubWallet{},
	)
	ctx := context.Background()

	// Auth
	_, _ = clients.Auth.Register(ctx, &pb.RegisterRequest{})
	_, _ = clients.Auth.Redirect(ctx, &pb.RedirectRequest{})
	_, _ = clients.Auth.Callback(ctx, &pb.CallbackRequest{})
	_, _ = clients.Auth.GetMe(ctx, &pb.GetMeRequest{})
	_, _ = clients.Auth.Logout(ctx, &pb.LogoutRequest{})
	_, _ = clients.Auth.ValidateToken(ctx, &pb.ValidateTokenRequest{})
	_, _ = clients.Auth.RequestAccountSecurity(ctx, &pb.RequestAccountSecurityRequest{})
	_, _ = clients.Auth.VerifyAccountSecurity(ctx, &pb.VerifyAccountSecurityRequest{})
	_, _ = clients.Auth.CheckAccountSecurity(ctx, &pb.CheckAccountSecurityRequest{UserId: 1})

	// User
	_, _ = clients.User.GetUser(ctx, &pb.GetUserRequest{})
	_, _ = clients.User.UpdateProfile(ctx, &pb.UpdateProfileRequest{})
	_, _ = clients.User.ListUsers(ctx, &pb.ListUsersRequest{})
	_, _ = clients.User.GetUserLevels(ctx, &pb.GetUserLevelsRequest{})
	_, _ = clients.User.GetUserProfile(ctx, &pb.GetUserProfileRequest{})
	_, _ = clients.User.GetUserWallet(ctx, &pb.GetUserWalletRequest{})
	_, _ = clients.User.GetUserLevel(ctx, &pb.GetUserLevelRequest{})
	_, _ = clients.User.GetProfileLimitations(ctx, &pb.GetProfileLimitationsRequest{})
	_, _ = clients.User.GetUserFeaturesCount(ctx, &pb.GetUserFeaturesCountRequest{})

	// KYC
	_, _ = clients.KYC.GetKYC(ctx, &pb.GetKYCRequest{})
	_, _ = clients.KYC.UpdateKYC(ctx, &pb.UpdateKYCRequest{})
	_, _ = clients.KYC.ListBankAccounts(ctx, &pb.ListBankAccountsRequest{})
	_, _ = clients.KYC.CreateBankAccount(ctx, &pb.CreateBankAccountRequest{})
	_, _ = clients.KYC.GetBankAccount(ctx, &pb.GetBankAccountRequest{})
	_, _ = clients.KYC.UpdateBankAccount(ctx, &pb.UpdateBankAccountRequest{})
	_, _ = clients.KYC.DeleteBankAccount(ctx, &pb.DeleteBankAccountRequest{})

	// Citizen
	_, _ = clients.Citizen.GetCitizenProfile(ctx, &pb.GetCitizenProfileRequest{})
	_, _ = clients.Citizen.GetCitizenReferrals(ctx, &pb.GetCitizenReferralsRequest{})
	_, _ = clients.Citizen.GetCitizenReferralChart(ctx, &pb.GetCitizenReferralChartRequest{})
	_, _ = clients.Citizen.GetCitizenUserInfo(ctx, &pb.GetCitizenUserInfoRequest{})
	_, _ = clients.Citizen.GetCitizenLevel(ctx, &pb.GetCitizenLevelRequest{})

	// Personal info
	_, _ = clients.PersonalInfo.GetPersonalInfo(ctx, &pb.GetPersonalInfoRequest{})
	_, _ = clients.PersonalInfo.UpdatePersonalInfo(ctx, &pb.UpdatePersonalInfoRequest{})

	// Profile limitation
	_, _ = clients.ProfileLimitation.CreateProfileLimitation(ctx, &pb.CreateProfileLimitationRequest{})
	_, _ = clients.ProfileLimitation.UpdateProfileLimitation(ctx, &pb.UpdateProfileLimitationRequest{})
	_, _ = clients.ProfileLimitation.DeleteProfileLimitation(ctx, &pb.DeleteProfileLimitationRequest{})

	// Profile photo
	_, _ = clients.ProfilePhoto.ListProfilePhotos(ctx, &pb.ListProfilePhotosRequest{})
	_, _ = clients.ProfilePhoto.UploadProfilePhoto(ctx, &pb.UploadProfilePhotoRequest{})
	_, _ = clients.ProfilePhoto.GetProfilePhoto(ctx, &pb.GetProfilePhotoRequest{})
	_, _ = clients.ProfilePhoto.DeleteProfilePhoto(ctx, &pb.DeleteProfilePhotoRequest{})

	// Settings
	_, _ = clients.Settings.GetSettings(ctx, &pb.GetSettingsRequest{})
	_, _ = clients.Settings.UpdateSettings(ctx, &pb.UpdateSettingsRequest{})
	_, _ = clients.Settings.GetGeneralSettings(ctx, &pb.GetGeneralSettingsRequest{})
	_, _ = clients.Settings.UpdateGeneralSettings(ctx, &pb.UpdateGeneralSettingsRequest{})
	_, _ = clients.Settings.GetPrivacySettings(ctx, &pb.GetPrivacySettingsRequest{})
	_, _ = clients.Settings.UpdatePrivacySettings(ctx, &pb.UpdatePrivacySettingsRequest{})

	// User events
	_, _ = clients.UserEvents.ListUserEvents(ctx, &pb.ListUserEventsRequest{})
	_, _ = clients.UserEvents.GetUserEvent(ctx, &pb.GetUserEventRequest{})
	_, _ = clients.UserEvents.ReportUserEvent(ctx, &pb.ReportUserEventRequest{})
	_, _ = clients.UserEvents.SendReportResponse(ctx, &pb.SendReportResponseRequest{})
	_, _ = clients.UserEvents.CloseEventReport(ctx, &pb.CloseEventReportRequest{})

	// Search
	_, _ = clients.Search.SearchUsers(ctx, &pb.SearchUsersRequest{})
	_, _ = clients.Search.SearchFeatures(ctx, &pb.SearchFeaturesRequest{})
	_, _ = clients.Search.SearchIsicCodes(ctx, &pb.SearchIsicCodesRequest{})

	// Wallet
	_, _ = clients.WalletConnection.GetLinkNonce(ctx, &pb.GetWalletLinkNonceRequest{})
	_, _ = clients.WalletConnection.LinkWallet(ctx, &pb.LinkWalletRequest{})
	_, _ = clients.WalletConnection.GetSecurityNonce(ctx, &pb.GetWalletSecurityNonceRequest{})
	_, _ = clients.WalletConnection.VerifySecuritySignature(ctx, &pb.VerifyWalletSecuritySignatureRequest{})
	_, _ = clients.WalletConnection.CheckRegistered(ctx, &pb.CheckWalletRegisteredRequest{})

	if clients.Auth == nil || clients.WalletConnection == nil {
		t.Fatal("expected non-nil clients")
	}
}
