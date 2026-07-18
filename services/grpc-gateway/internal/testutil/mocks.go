package testutil

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "metarang/shared/pb/auth"
	calendarpb "metarang/shared/pb/calendar"
	featurespb "metarang/shared/pb/features"
	socialpb "metarang/shared/pb/social"
)

// AuthMocks groups auth-service gRPC mocks registered on one bufconn server.
type AuthMocks struct {
	Auth    *MockAuthService
	Citizen *MockCitizenService
	User    *MockUserService
	KYC     *MockKYCService
}

// DialAuthConn returns a client connection with auth-service mocks registered.
func DialAuthConn(m *AuthMocks) (*grpc.ClientConn, func()) {
	if m == nil {
		m = &AuthMocks{}
	}
	if m.Auth == nil {
		m.Auth = &MockAuthService{}
	}
	if m.Citizen == nil {
		m.Citizen = &MockCitizenService{}
	}
	if m.User == nil {
		m.User = &MockUserService{}
	}
	if m.KYC == nil {
		m.KYC = &MockKYCService{}
	}
	return DialBufConn(func(s *grpc.Server) {
		pb.RegisterAuthServiceServer(s, m.Auth)
		pb.RegisterCitizenServiceServer(s, m.Citizen)
		pb.RegisterUserServiceServer(s, m.User)
		pb.RegisterKYCServiceServer(s, m.KYC)
	})
}

// MockAuthService implements pb.AuthServiceServer for tests.
type MockAuthService struct {
	pb.UnimplementedAuthServiceServer
	RequestAccountSecurityFunc func(ctx context.Context, req *pb.RequestAccountSecurityRequest) (*emptypb.Empty, error)
	VerifyAccountSecurityFunc  func(ctx context.Context, req *pb.VerifyAccountSecurityRequest) (*emptypb.Empty, error)
}

func (m *MockAuthService) RequestAccountSecurity(ctx context.Context, req *pb.RequestAccountSecurityRequest) (*emptypb.Empty, error) {
	if m.RequestAccountSecurityFunc != nil {
		return m.RequestAccountSecurityFunc(ctx, req)
	}
	return &emptypb.Empty{}, nil
}

func (m *MockAuthService) VerifyAccountSecurity(ctx context.Context, req *pb.VerifyAccountSecurityRequest) (*emptypb.Empty, error) {
	if m.VerifyAccountSecurityFunc != nil {
		return m.VerifyAccountSecurityFunc(ctx, req)
	}
	return &emptypb.Empty{}, nil
}

// MockCitizenService implements pb.CitizenServiceServer for tests.
type MockCitizenService struct {
	pb.UnimplementedCitizenServiceServer
	GetCitizenProfileFunc       func(ctx context.Context, req *pb.GetCitizenProfileRequest) (*pb.CitizenProfileResponse, error)
	GetCitizenReferralsFunc     func(ctx context.Context, req *pb.GetCitizenReferralsRequest) (*pb.CitizenReferralsResponse, error)
	GetCitizenReferralChartFunc func(ctx context.Context, req *pb.GetCitizenReferralChartRequest) (*pb.CitizenReferralChartResponse, error)
	GetCitizenUserInfoFunc      func(ctx context.Context, req *pb.GetCitizenUserInfoRequest) (*pb.GetCitizenUserInfoResponse, error)
}

func (m *MockCitizenService) GetCitizenProfile(ctx context.Context, req *pb.GetCitizenProfileRequest) (*pb.CitizenProfileResponse, error) {
	if m.GetCitizenProfileFunc != nil {
		return m.GetCitizenProfileFunc(ctx, req)
	}
	return &pb.CitizenProfileResponse{}, nil
}

func (m *MockCitizenService) GetCitizenReferrals(ctx context.Context, req *pb.GetCitizenReferralsRequest) (*pb.CitizenReferralsResponse, error) {
	if m.GetCitizenReferralsFunc != nil {
		return m.GetCitizenReferralsFunc(ctx, req)
	}
	return &pb.CitizenReferralsResponse{}, nil
}

func (m *MockCitizenService) GetCitizenReferralChart(ctx context.Context, req *pb.GetCitizenReferralChartRequest) (*pb.CitizenReferralChartResponse, error) {
	if m.GetCitizenReferralChartFunc != nil {
		return m.GetCitizenReferralChartFunc(ctx, req)
	}
	return &pb.CitizenReferralChartResponse{}, nil
}

func (m *MockCitizenService) GetCitizenUserInfo(ctx context.Context, req *pb.GetCitizenUserInfoRequest) (*pb.GetCitizenUserInfoResponse, error) {
	if m.GetCitizenUserInfoFunc != nil {
		return m.GetCitizenUserInfoFunc(ctx, req)
	}
	return &pb.GetCitizenUserInfoResponse{}, nil
}

// MockUserService implements pb.UserServiceServer for tests.
type MockUserService struct {
	pb.UnimplementedUserServiceServer
	ListUsersFunc             func(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error)
	GetProfileLimitationsFunc func(ctx context.Context, req *pb.GetProfileLimitationsRequest) (*pb.GetProfileLimitationsResponse, error)
}

func (m *MockUserService) ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	if m.ListUsersFunc != nil {
		return m.ListUsersFunc(ctx, req)
	}
	return &pb.ListUsersResponse{}, nil
}

func (m *MockUserService) GetProfileLimitations(ctx context.Context, req *pb.GetProfileLimitationsRequest) (*pb.GetProfileLimitationsResponse, error) {
	if m.GetProfileLimitationsFunc != nil {
		return m.GetProfileLimitationsFunc(ctx, req)
	}
	return &pb.GetProfileLimitationsResponse{}, nil
}

// MockKYCService implements pb.KYCServiceServer for tests.
type MockKYCService struct {
	pb.UnimplementedKYCServiceServer
	ListBankAccountsFunc func(ctx context.Context, req *pb.ListBankAccountsRequest) (*pb.ListBankAccountsResponse, error)
	GetBankAccountFunc   func(ctx context.Context, req *pb.GetBankAccountRequest) (*pb.BankAccountResponse, error)
}

func (m *MockKYCService) ListBankAccounts(ctx context.Context, req *pb.ListBankAccountsRequest) (*pb.ListBankAccountsResponse, error) {
	if m.ListBankAccountsFunc != nil {
		return m.ListBankAccountsFunc(ctx, req)
	}
	return &pb.ListBankAccountsResponse{}, nil
}

func (m *MockKYCService) GetBankAccount(ctx context.Context, req *pb.GetBankAccountRequest) (*pb.BankAccountResponse, error) {
	if m.GetBankAccountFunc != nil {
		return m.GetBankAccountFunc(ctx, req)
	}
	return &pb.BankAccountResponse{}, nil
}

// MockCalendarService implements calendarpb.CalendarServiceServer for tests.
type MockCalendarService struct {
	calendarpb.UnimplementedCalendarServiceServer
	GetEventsFunc func(ctx context.Context, req *calendarpb.GetEventsRequest) (*calendarpb.EventsResponse, error)
	GetEventFunc  func(ctx context.Context, req *calendarpb.GetEventRequest) (*calendarpb.EventResponse, error)
}

func (m *MockCalendarService) GetEvents(ctx context.Context, req *calendarpb.GetEventsRequest) (*calendarpb.EventsResponse, error) {
	if m.GetEventsFunc != nil {
		return m.GetEventsFunc(ctx, req)
	}
	return &calendarpb.EventsResponse{}, nil
}

func (m *MockCalendarService) GetEvent(ctx context.Context, req *calendarpb.GetEventRequest) (*calendarpb.EventResponse, error) {
	if m.GetEventFunc != nil {
		return m.GetEventFunc(ctx, req)
	}
	return &calendarpb.EventResponse{}, nil
}

// DialCalendarConn returns a client connection with calendar-service mock registered.
func DialCalendarConn(calendar *MockCalendarService) (*grpc.ClientConn, func()) {
	if calendar == nil {
		calendar = &MockCalendarService{}
	}
	return DialBufConn(func(s *grpc.Server) {
		calendarpb.RegisterCalendarServiceServer(s, calendar)
	})
}

// MockFeatureService implements featurespb.FeatureServiceServer for tests.
type MockFeatureService struct {
	featurespb.UnimplementedFeatureServiceServer
	ListFeaturesFunc func(ctx context.Context, req *featurespb.ListFeaturesRequest) (*featurespb.FeaturesResponse, error)
}

func (m *MockFeatureService) ListFeatures(ctx context.Context, req *featurespb.ListFeaturesRequest) (*featurespb.FeaturesResponse, error) {
	if m.ListFeaturesFunc != nil {
		return m.ListFeaturesFunc(ctx, req)
	}
	return &featurespb.FeaturesResponse{}, nil
}

// MockFeatureProfitService implements featurespb.FeatureProfitServiceServer for tests.
type MockFeatureProfitService struct {
	featurespb.UnimplementedFeatureProfitServiceServer
	GetSingleProfitFunc func(ctx context.Context, req *featurespb.GetSingleProfitRequest) (*featurespb.HourlyProfitResponse, error)
}

func (m *MockFeatureProfitService) GetSingleProfit(ctx context.Context, req *featurespb.GetSingleProfitRequest) (*featurespb.HourlyProfitResponse, error) {
	if m.GetSingleProfitFunc != nil {
		return m.GetSingleProfitFunc(ctx, req)
	}
	return &featurespb.HourlyProfitResponse{}, nil
}

// DialFeaturesConn returns a client connection with features-service mocks registered.
func DialFeaturesConn(feature *MockFeatureService, profit *MockFeatureProfitService) (*grpc.ClientConn, func()) {
	if feature == nil {
		feature = &MockFeatureService{}
	}
	if profit == nil {
		profit = &MockFeatureProfitService{}
	}
	return DialBufConn(func(s *grpc.Server) {
		featurespb.RegisterFeatureServiceServer(s, feature)
		featurespb.RegisterFeatureProfitServiceServer(s, profit)
	})
}

// MockFollowService implements socialpb.FollowServiceServer for tests.
type MockFollowService struct {
	socialpb.UnimplementedFollowServiceServer
	GetFollowersFunc func(ctx context.Context, req *socialpb.GetFollowersRequest) (*socialpb.GetFollowersResponse, error)
	GetFollowingFunc func(ctx context.Context, req *socialpb.GetFollowingRequest) (*socialpb.GetFollowingResponse, error)
	FollowFunc       func(ctx context.Context, req *socialpb.FollowRequest) (*emptypb.Empty, error)
	UnfollowFunc     func(ctx context.Context, req *socialpb.UnfollowRequest) (*emptypb.Empty, error)
	RemoveFunc       func(ctx context.Context, req *socialpb.RemoveRequest) (*emptypb.Empty, error)
}

func (m *MockFollowService) GetFollowers(ctx context.Context, req *socialpb.GetFollowersRequest) (*socialpb.GetFollowersResponse, error) {
	if m.GetFollowersFunc != nil {
		return m.GetFollowersFunc(ctx, req)
	}
	return &socialpb.GetFollowersResponse{}, nil
}

func (m *MockFollowService) GetFollowing(ctx context.Context, req *socialpb.GetFollowingRequest) (*socialpb.GetFollowingResponse, error) {
	if m.GetFollowingFunc != nil {
		return m.GetFollowingFunc(ctx, req)
	}
	return &socialpb.GetFollowingResponse{}, nil
}

func (m *MockFollowService) Follow(ctx context.Context, req *socialpb.FollowRequest) (*emptypb.Empty, error) {
	if m.FollowFunc != nil {
		return m.FollowFunc(ctx, req)
	}
	return &emptypb.Empty{}, nil
}

func (m *MockFollowService) Unfollow(ctx context.Context, req *socialpb.UnfollowRequest) (*emptypb.Empty, error) {
	if m.UnfollowFunc != nil {
		return m.UnfollowFunc(ctx, req)
	}
	return &emptypb.Empty{}, nil
}

func (m *MockFollowService) Remove(ctx context.Context, req *socialpb.RemoveRequest) (*emptypb.Empty, error) {
	if m.RemoveFunc != nil {
		return m.RemoveFunc(ctx, req)
	}
	return &emptypb.Empty{}, nil
}

// DialSocialConn returns a client connection with FollowService + UserService mocks
// registered (same conn can be passed as both socialConn and authConn).
func DialSocialConn(follow *MockFollowService, user *MockUserService) (*grpc.ClientConn, func()) {
	if follow == nil {
		follow = &MockFollowService{}
	}
	if user == nil {
		user = &MockUserService{}
	}
	return DialBufConn(func(s *grpc.Server) {
		socialpb.RegisterFollowServiceServer(s, follow)
		pb.RegisterUserServiceServer(s, user)
	})
}
