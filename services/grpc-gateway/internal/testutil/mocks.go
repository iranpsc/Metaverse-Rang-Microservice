package testutil

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	pb "metargb/shared/pb/auth"
	calendarpb "metargb/shared/pb/calendar"
	featurespb "metargb/shared/pb/features"
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

// MockUserService implements pb.UserServiceServer for tests.
type MockUserService struct {
	pb.UnimplementedUserServiceServer
	ListUsersFunc func(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error)
}

func (m *MockUserService) ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	if m.ListUsersFunc != nil {
		return m.ListUsersFunc(ctx, req)
	}
	return &pb.ListUsersResponse{}, nil
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
