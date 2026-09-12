package handler

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "metarang/shared/pb/auth"
)

// LocalClients wraps in-process gRPC handlers as client interfaces for the HTTP layer.
type LocalClients struct {
	Auth              pb.AuthServiceClient
	User              pb.UserServiceClient
	KYC               pb.KYCServiceClient
	Citizen           pb.CitizenServiceClient
	PersonalInfo      pb.PersonalInfoServiceClient
	ProfileLimitation pb.ProfileLimitationServiceClient
	ProfilePhoto      pb.ProfilePhotoServiceClient
	Settings          pb.SettingsServiceClient
	UserEvents        pb.UserEventsServiceClient
	Search            pb.SearchServiceClient
	WalletConnection  pb.WalletConnectionServiceClient
}

// NewLocalClients builds client facades from registered gRPC server implementations.
func NewLocalClients(
	auth pb.AuthServiceServer,
	user pb.UserServiceServer,
	kyc pb.KYCServiceServer,
	citizen pb.CitizenServiceServer,
	personalInfo pb.PersonalInfoServiceServer,
	profileLimitation pb.ProfileLimitationServiceServer,
	profilePhoto pb.ProfilePhotoServiceServer,
	settings pb.SettingsServiceServer,
	userEvents pb.UserEventsServiceServer,
	search pb.SearchServiceServer,
	wallet pb.WalletConnectionServiceServer,
) LocalClients {
	return LocalClients{
		Auth:              &localAuthClient{auth},
		User:              &localUserClient{user},
		KYC:               &localKYCClient{kyc},
		Citizen:           &localCitizenClient{citizen},
		PersonalInfo:      &localPersonalInfoClient{personalInfo},
		ProfileLimitation: &localProfileLimitationClient{profileLimitation},
		ProfilePhoto:      &localProfilePhotoClient{profilePhoto},
		Settings:          &localSettingsClient{settings},
		UserEvents:        &localUserEventsClient{userEvents},
		Search:            &localSearchClient{search},
		WalletConnection:  &localWalletConnectionClient{wallet},
	}
}

type localAuthClient struct{ pb.AuthServiceServer }

func (c *localAuthClient) Register(ctx context.Context, in *pb.RegisterRequest, _ ...grpc.CallOption) (*pb.RegisterResponse, error) {
	return c.AuthServiceServer.Register(ctx, in)
}
func (c *localAuthClient) Redirect(ctx context.Context, in *pb.RedirectRequest, _ ...grpc.CallOption) (*pb.RedirectResponse, error) {
	return c.AuthServiceServer.Redirect(ctx, in)
}
func (c *localAuthClient) Callback(ctx context.Context, in *pb.CallbackRequest, _ ...grpc.CallOption) (*pb.CallbackResponse, error) {
	return c.AuthServiceServer.Callback(ctx, in)
}
func (c *localAuthClient) GetMe(ctx context.Context, in *pb.GetMeRequest, _ ...grpc.CallOption) (*pb.UserResponse, error) {
	return c.AuthServiceServer.GetMe(ctx, in)
}
func (c *localAuthClient) Logout(ctx context.Context, in *pb.LogoutRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return c.AuthServiceServer.Logout(ctx, in)
}
func (c *localAuthClient) ValidateToken(ctx context.Context, in *pb.ValidateTokenRequest, _ ...grpc.CallOption) (*pb.ValidateTokenResponse, error) {
	return c.AuthServiceServer.ValidateToken(ctx, in)
}
func (c *localAuthClient) RequestAccountSecurity(ctx context.Context, in *pb.RequestAccountSecurityRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return c.AuthServiceServer.RequestAccountSecurity(ctx, in)
}
func (c *localAuthClient) VerifyAccountSecurity(ctx context.Context, in *pb.VerifyAccountSecurityRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return c.AuthServiceServer.VerifyAccountSecurity(ctx, in)
}
func (c *localAuthClient) CheckAccountSecurity(ctx context.Context, in *pb.CheckAccountSecurityRequest, _ ...grpc.CallOption) (*pb.CheckAccountSecurityResponse, error) {
	return c.AuthServiceServer.CheckAccountSecurity(ctx, in)
}

type localUserClient struct{ pb.UserServiceServer }

func (c *localUserClient) GetUser(ctx context.Context, in *pb.GetUserRequest, _ ...grpc.CallOption) (*pb.User, error) {
	return c.UserServiceServer.GetUser(ctx, in)
}
func (c *localUserClient) UpdateProfile(ctx context.Context, in *pb.UpdateProfileRequest, _ ...grpc.CallOption) (*pb.User, error) {
	return c.UserServiceServer.UpdateProfile(ctx, in)
}
func (c *localUserClient) ListUsers(ctx context.Context, in *pb.ListUsersRequest, _ ...grpc.CallOption) (*pb.ListUsersResponse, error) {
	return c.UserServiceServer.ListUsers(ctx, in)
}
func (c *localUserClient) GetUserLevels(ctx context.Context, in *pb.GetUserLevelsRequest, _ ...grpc.CallOption) (*pb.GetUserLevelsResponse, error) {
	return c.UserServiceServer.GetUserLevels(ctx, in)
}
func (c *localUserClient) GetUserProfile(ctx context.Context, in *pb.GetUserProfileRequest, _ ...grpc.CallOption) (*pb.GetUserProfileResponse, error) {
	return c.UserServiceServer.GetUserProfile(ctx, in)
}
func (c *localUserClient) GetUserWallet(ctx context.Context, in *pb.GetUserWalletRequest, _ ...grpc.CallOption) (*pb.UserWalletResponse, error) {
	return c.UserServiceServer.GetUserWallet(ctx, in)
}
func (c *localUserClient) GetUserLevel(ctx context.Context, in *pb.GetUserLevelRequest, _ ...grpc.CallOption) (*pb.UserLevelResponse, error) {
	return c.UserServiceServer.GetUserLevel(ctx, in)
}
func (c *localUserClient) GetProfileLimitations(ctx context.Context, in *pb.GetProfileLimitationsRequest, _ ...grpc.CallOption) (*pb.GetProfileLimitationsResponse, error) {
	return c.UserServiceServer.GetProfileLimitations(ctx, in)
}
func (c *localUserClient) GetUserFeaturesCount(ctx context.Context, in *pb.GetUserFeaturesCountRequest, _ ...grpc.CallOption) (*pb.GetUserFeaturesCountResponse, error) {
	return c.UserServiceServer.GetUserFeaturesCount(ctx, in)
}

type localProfileLimitationClient struct {
	pb.ProfileLimitationServiceServer
}

func (c *localProfileLimitationClient) CreateProfileLimitation(ctx context.Context, in *pb.CreateProfileLimitationRequest, _ ...grpc.CallOption) (*pb.ProfileLimitationResponse, error) {
	return c.ProfileLimitationServiceServer.CreateProfileLimitation(ctx, in)
}
func (c *localProfileLimitationClient) UpdateProfileLimitation(ctx context.Context, in *pb.UpdateProfileLimitationRequest, _ ...grpc.CallOption) (*pb.ProfileLimitationResponse, error) {
	return c.ProfileLimitationServiceServer.UpdateProfileLimitation(ctx, in)
}
func (c *localProfileLimitationClient) DeleteProfileLimitation(ctx context.Context, in *pb.DeleteProfileLimitationRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return c.ProfileLimitationServiceServer.DeleteProfileLimitation(ctx, in)
}

type localKYCClient struct{ pb.KYCServiceServer }

func (c *localKYCClient) GetKYC(ctx context.Context, in *pb.GetKYCRequest, _ ...grpc.CallOption) (*pb.KYCResponse, error) {
	return c.KYCServiceServer.GetKYC(ctx, in)
}
func (c *localKYCClient) UpdateKYC(ctx context.Context, in *pb.UpdateKYCRequest, _ ...grpc.CallOption) (*pb.KYCResponse, error) {
	return c.KYCServiceServer.UpdateKYC(ctx, in)
}
func (c *localKYCClient) ListBankAccounts(ctx context.Context, in *pb.ListBankAccountsRequest, _ ...grpc.CallOption) (*pb.ListBankAccountsResponse, error) {
	return c.KYCServiceServer.ListBankAccounts(ctx, in)
}
func (c *localKYCClient) CreateBankAccount(ctx context.Context, in *pb.CreateBankAccountRequest, _ ...grpc.CallOption) (*pb.BankAccountResponse, error) {
	return c.KYCServiceServer.CreateBankAccount(ctx, in)
}
func (c *localKYCClient) GetBankAccount(ctx context.Context, in *pb.GetBankAccountRequest, _ ...grpc.CallOption) (*pb.BankAccountResponse, error) {
	return c.KYCServiceServer.GetBankAccount(ctx, in)
}
func (c *localKYCClient) UpdateBankAccount(ctx context.Context, in *pb.UpdateBankAccountRequest, _ ...grpc.CallOption) (*pb.BankAccountResponse, error) {
	return c.KYCServiceServer.UpdateBankAccount(ctx, in)
}
func (c *localKYCClient) DeleteBankAccount(ctx context.Context, in *pb.DeleteBankAccountRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return c.KYCServiceServer.DeleteBankAccount(ctx, in)
}

type localCitizenClient struct{ pb.CitizenServiceServer }

func (c *localCitizenClient) GetCitizenProfile(ctx context.Context, in *pb.GetCitizenProfileRequest, _ ...grpc.CallOption) (*pb.CitizenProfileResponse, error) {
	return c.CitizenServiceServer.GetCitizenProfile(ctx, in)
}
func (c *localCitizenClient) GetCitizenReferrals(ctx context.Context, in *pb.GetCitizenReferralsRequest, _ ...grpc.CallOption) (*pb.CitizenReferralsResponse, error) {
	return c.CitizenServiceServer.GetCitizenReferrals(ctx, in)
}
func (c *localCitizenClient) GetCitizenReferralChart(ctx context.Context, in *pb.GetCitizenReferralChartRequest, _ ...grpc.CallOption) (*pb.CitizenReferralChartResponse, error) {
	return c.CitizenServiceServer.GetCitizenReferralChart(ctx, in)
}
func (c *localCitizenClient) GetCitizenUserInfo(ctx context.Context, in *pb.GetCitizenUserInfoRequest, _ ...grpc.CallOption) (*pb.GetCitizenUserInfoResponse, error) {
	return c.CitizenServiceServer.GetCitizenUserInfo(ctx, in)
}
func (c *localCitizenClient) GetCitizenLevel(ctx context.Context, in *pb.GetCitizenLevelRequest, _ ...grpc.CallOption) (*pb.GetCitizenLevelResponse, error) {
	return c.CitizenServiceServer.GetCitizenLevel(ctx, in)
}

type localPersonalInfoClient struct{ pb.PersonalInfoServiceServer }

func (c *localPersonalInfoClient) GetPersonalInfo(ctx context.Context, in *pb.GetPersonalInfoRequest, _ ...grpc.CallOption) (*pb.GetPersonalInfoResponse, error) {
	return c.PersonalInfoServiceServer.GetPersonalInfo(ctx, in)
}
func (c *localPersonalInfoClient) UpdatePersonalInfo(ctx context.Context, in *pb.UpdatePersonalInfoRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return c.PersonalInfoServiceServer.UpdatePersonalInfo(ctx, in)
}

type localProfilePhotoClient struct{ pb.ProfilePhotoServiceServer }

func (c *localProfilePhotoClient) ListProfilePhotos(ctx context.Context, in *pb.ListProfilePhotosRequest, _ ...grpc.CallOption) (*pb.ListProfilePhotosResponse, error) {
	return c.ProfilePhotoServiceServer.ListProfilePhotos(ctx, in)
}
func (c *localProfilePhotoClient) UploadProfilePhoto(ctx context.Context, in *pb.UploadProfilePhotoRequest, _ ...grpc.CallOption) (*pb.ProfilePhotoResponse, error) {
	return c.ProfilePhotoServiceServer.UploadProfilePhoto(ctx, in)
}
func (c *localProfilePhotoClient) GetProfilePhoto(ctx context.Context, in *pb.GetProfilePhotoRequest, _ ...grpc.CallOption) (*pb.ProfilePhotoResponse, error) {
	return c.ProfilePhotoServiceServer.GetProfilePhoto(ctx, in)
}
func (c *localProfilePhotoClient) DeleteProfilePhoto(ctx context.Context, in *pb.DeleteProfilePhotoRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return c.ProfilePhotoServiceServer.DeleteProfilePhoto(ctx, in)
}

type localSettingsClient struct{ pb.SettingsServiceServer }

func (c *localSettingsClient) GetSettings(ctx context.Context, in *pb.GetSettingsRequest, _ ...grpc.CallOption) (*pb.GetSettingsResponse, error) {
	return c.SettingsServiceServer.GetSettings(ctx, in)
}
func (c *localSettingsClient) UpdateSettings(ctx context.Context, in *pb.UpdateSettingsRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return c.SettingsServiceServer.UpdateSettings(ctx, in)
}
func (c *localSettingsClient) GetGeneralSettings(ctx context.Context, in *pb.GetGeneralSettingsRequest, _ ...grpc.CallOption) (*pb.GetGeneralSettingsResponse, error) {
	return c.SettingsServiceServer.GetGeneralSettings(ctx, in)
}
func (c *localSettingsClient) UpdateGeneralSettings(ctx context.Context, in *pb.UpdateGeneralSettingsRequest, _ ...grpc.CallOption) (*pb.UpdateGeneralSettingsResponse, error) {
	return c.SettingsServiceServer.UpdateGeneralSettings(ctx, in)
}
func (c *localSettingsClient) GetPrivacySettings(ctx context.Context, in *pb.GetPrivacySettingsRequest, _ ...grpc.CallOption) (*pb.GetPrivacySettingsResponse, error) {
	return c.SettingsServiceServer.GetPrivacySettings(ctx, in)
}
func (c *localSettingsClient) UpdatePrivacySettings(ctx context.Context, in *pb.UpdatePrivacySettingsRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return c.SettingsServiceServer.UpdatePrivacySettings(ctx, in)
}

type localUserEventsClient struct{ pb.UserEventsServiceServer }

func (c *localUserEventsClient) ListUserEvents(ctx context.Context, in *pb.ListUserEventsRequest, _ ...grpc.CallOption) (*pb.ListUserEventsResponse, error) {
	return c.UserEventsServiceServer.ListUserEvents(ctx, in)
}
func (c *localUserEventsClient) GetUserEvent(ctx context.Context, in *pb.GetUserEventRequest, _ ...grpc.CallOption) (*pb.GetUserEventResponse, error) {
	return c.UserEventsServiceServer.GetUserEvent(ctx, in)
}
func (c *localUserEventsClient) ReportUserEvent(ctx context.Context, in *pb.ReportUserEventRequest, _ ...grpc.CallOption) (*pb.UserEventReportResponse, error) {
	return c.UserEventsServiceServer.ReportUserEvent(ctx, in)
}
func (c *localUserEventsClient) SendReportResponse(ctx context.Context, in *pb.SendReportResponseRequest, _ ...grpc.CallOption) (*pb.UserEventReportResponseResponse, error) {
	return c.UserEventsServiceServer.SendReportResponse(ctx, in)
}
func (c *localUserEventsClient) CloseEventReport(ctx context.Context, in *pb.CloseEventReportRequest, _ ...grpc.CallOption) (*emptypb.Empty, error) {
	return c.UserEventsServiceServer.CloseEventReport(ctx, in)
}

type localSearchClient struct{ pb.SearchServiceServer }

func (c *localSearchClient) SearchUsers(ctx context.Context, in *pb.SearchUsersRequest, _ ...grpc.CallOption) (*pb.SearchUsersResponse, error) {
	return c.SearchServiceServer.SearchUsers(ctx, in)
}
func (c *localSearchClient) SearchFeatures(ctx context.Context, in *pb.SearchFeaturesRequest, _ ...grpc.CallOption) (*pb.SearchFeaturesResponse, error) {
	return c.SearchServiceServer.SearchFeatures(ctx, in)
}
func (c *localSearchClient) SearchIsicCodes(ctx context.Context, in *pb.SearchIsicCodesRequest, _ ...grpc.CallOption) (*pb.SearchIsicCodesResponse, error) {
	return c.SearchServiceServer.SearchIsicCodes(ctx, in)
}

type localWalletConnectionClient struct {
	pb.WalletConnectionServiceServer
}

func (c *localWalletConnectionClient) GetLinkNonce(ctx context.Context, in *pb.GetWalletLinkNonceRequest, _ ...grpc.CallOption) (*pb.GetWalletNonceResponse, error) {
	return c.WalletConnectionServiceServer.GetLinkNonce(ctx, in)
}
func (c *localWalletConnectionClient) LinkWallet(ctx context.Context, in *pb.LinkWalletRequest, _ ...grpc.CallOption) (*pb.LinkWalletResponse, error) {
	return c.WalletConnectionServiceServer.LinkWallet(ctx, in)
}
func (c *localWalletConnectionClient) GetSecurityNonce(ctx context.Context, in *pb.GetWalletSecurityNonceRequest, _ ...grpc.CallOption) (*pb.GetWalletNonceResponse, error) {
	return c.WalletConnectionServiceServer.GetSecurityNonce(ctx, in)
}
func (c *localWalletConnectionClient) VerifySecuritySignature(ctx context.Context, in *pb.VerifyWalletSecuritySignatureRequest, _ ...grpc.CallOption) (*pb.VerifyWalletSecuritySignatureResponse, error) {
	return c.WalletConnectionServiceServer.VerifySecuritySignature(ctx, in)
}
func (c *localWalletConnectionClient) CheckRegistered(ctx context.Context, in *pb.CheckWalletRegisteredRequest, _ ...grpc.CallOption) (*pb.CheckWalletRegisteredResponse, error) {
	return c.WalletConnectionServiceServer.CheckRegistered(ctx, in)
}
