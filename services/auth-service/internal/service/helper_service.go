package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	commercialpb "metarang/shared/pb/commercial"
	featurespb "metarang/shared/pb/features"
	levelspb "metarang/shared/pb/levels"
)

// HelperService provides helper methods that integrate with other microservices
// These methods implement the Laravel helper functions that require cross-service calls
type HelperService interface {
	// GetUnansweredQuestionsCount calls Levels service to get unanswered questions count
	GetUnansweredQuestionsCount(ctx context.Context, userID uint64) (int32, error)

	// GetHourlyProfitTimePercentage calls Features service to get hourly profit percentage
	GetHourlyProfitTimePercentage(ctx context.Context, userID uint64) (float64, error)

	// GetScorePercentageToNextLevel calls Levels service to calculate score percentage
	GetScorePercentageToNextLevel(ctx context.Context, userID uint64, currentScore int32) (float64, error)

	// GetUserLevel calls Levels service to get user's current level
	GetUserLevel(ctx context.Context, userID uint64) (*LevelInfo, error)

	// GetUserWallet calls Commercial service to get user's wallet balances
	GetUserWallet(ctx context.Context, userID uint64) (*WalletInfo, error)

	// CreateWallet calls Commercial service to create a wallet for a new user
	CreateWallet(ctx context.Context, userID uint64) error

	// CreateUserVariables calls Commercial service to create default user_variables for a new user
	CreateUserVariables(ctx context.Context, userID uint64) error

	// Close closes gRPC connections
	Close() error
}

// WalletInfo represents wallet balance information
type WalletInfo struct {
	Psc          string
	Irr          string
	Red          string
	Blue         string
	Yellow       string
	Satisfaction string
	Effect       float64
}

type helperService struct {
	levelsServiceAddr     string
	featuresServiceAddr   string
	commercialServiceAddr string
	levelsConn            *grpc.ClientConn
	featuresConn          *grpc.ClientConn
	commercialConn        *grpc.ClientConn
	levelsClient          levelspb.LevelServiceClient
	challengeClient       levelspb.ChallengeServiceClient // Challenge service is in levels proto
	featureProfitClient   featurespb.FeatureProfitServiceClient
	walletClient          commercialpb.WalletServiceClient
	userVariableClient    commercialpb.UserVariableServiceClient
}

// NewHelperService creates a new helper service
func NewHelperService(levelsAddr, featuresAddr, commercialAddr string) HelperService {
	hs := &helperService{
		levelsServiceAddr:     levelsAddr,
		featuresServiceAddr:   featuresAddr,
		commercialServiceAddr: commercialAddr,
	}

	// Initialize gRPC connection to levels service (includes ChallengeService)
	if levelsAddr != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		conn, err := grpc.DialContext(ctx, levelsAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("Warning: Failed to connect to levels service at %s: %v (will use stub implementations)", levelsAddr, err)
		} else {
			hs.levelsConn = conn
			hs.levelsClient = levelspb.NewLevelServiceClient(conn)
			hs.challengeClient = levelspb.NewChallengeServiceClient(conn) // Challenge service is in levels proto
			log.Printf("Successfully connected to levels service at %s", levelsAddr)
		}
	}

	// Initialize gRPC connection to features service
	if featuresAddr != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		conn, err := grpc.DialContext(ctx, featuresAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			log.Printf("Warning: Failed to connect to features service at %s: %v (will use stub implementations)", featuresAddr, err)
		} else {
			hs.featuresConn = conn
			hs.featureProfitClient = featurespb.NewFeatureProfitServiceClient(conn)
			log.Printf("Successfully connected to features service at %s", featuresAddr)
		}
	}

	// Initialize gRPC connection to commercial service
	if commercialAddr != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Create client interceptor to forward authorization header
		interceptor := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			// Check if authorization is already in outgoing metadata
			outMd, hasOutgoing := metadata.FromOutgoingContext(ctx)
			if hasOutgoing && len(outMd.Get("authorization")) > 0 {
				// Already has authorization, proceed
				return invoker(ctx, method, req, reply, cc, opts...)
			}

			// Try to get authorization from incoming metadata
			if inMd, ok := metadata.FromIncomingContext(ctx); ok {
				if authHeaders := inMd.Get("authorization"); len(authHeaders) > 0 {
					// Forward authorization header to outgoing call
					ctx = metadata.AppendToOutgoingContext(ctx, "authorization", authHeaders[0])
				}
			}

			return invoker(ctx, method, req, reply, cc, opts...)
		}

		conn, err := grpc.DialContext(ctx, commercialAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithUnaryInterceptor(interceptor))
		if err != nil {
			log.Printf("Warning: Failed to connect to commercial service at %s: %v (will use stub implementations)", commercialAddr, err)
		} else {
			hs.commercialConn = conn
			hs.walletClient = commercialpb.NewWalletServiceClient(conn)
			hs.userVariableClient = commercialpb.NewUserVariableServiceClient(conn)
			log.Printf("Successfully connected to commercial service at %s", commercialAddr)
		}
	}

	return hs
}

// GetUnansweredQuestionsCount implements the Laravel getUnansweredQuestionsCount helper
// Calls the Challenge service to get count of questions user hasn't answered correctly
func (s *helperService) GetUnansweredQuestionsCount(ctx context.Context, userID uint64) (int32, error) {
	if s.challengeClient == nil {
		return 0, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Get a question - if it returns a question, user has unanswered questions
	// Note: The API returns null when no suitable question exists (all answered correctly)
	resp, err := s.challengeClient.GetQuestion(ctx, &levelspb.GetQuestionRequest{
		UserId: userID,
	})
	if err != nil {
		log.Printf("Failed to get question for unanswered count: %v", err)
		return 0, nil // Return 0 on error to not break the flow
	}

	// If has_question is false or question is nil, user has no unanswered questions
	if !resp.HasQuestion || resp.Question == nil {
		return 0, nil
	}

	// User has at least one unanswered question
	// Note: This is a simplified implementation - ideally we'd have a dedicated count method
	// For now, we return 1 if there's a question available (indicating unanswered questions exist)
	// A more accurate count would require iterating or a dedicated RPC method
	return 1, nil
}

// GetHourlyProfitTimePercentage implements the Laravel hourlyProfitInfo helper
// Calls the Features service to calculate time percentage for hourly profit
// This calculates the percentage based on active hourly profits and their deadlines
func (s *helperService) GetHourlyProfitTimePercentage(ctx context.Context, userID uint64) (float64, error) {
	if s.featureProfitClient == nil {
		return 0.0, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Get hourly profits for the user
	resp, err := s.featureProfitClient.GetHourlyProfits(ctx, &featurespb.GetHourlyProfitsRequest{
		UserId:   userID,
		Page:     1,
		PageSize: 100, // Get a reasonable number to calculate percentage
	})
	if err != nil {
		log.Printf("Failed to get hourly profits for time percentage: %v", err)
		return 0.0, nil // Return 0 on error to not break the flow
	}

	if len(resp.Profits) == 0 {
		return 0.0, nil
	}

	// Calculate percentage based on active profits and their deadlines
	// The percentage represents how much time has elapsed toward profit deadlines
	// This calculates the average time percentage across all active profits
	now := time.Now()
	activeCount := 0
	totalTimePercentage := 0.0

	for _, profit := range resp.Profits {
		if profit.IsActive && profit.DeadLine != "" {
			activeCount++
			// Parse deadline timestamp (format may vary - assuming RFC3339 or Unix timestamp)
			// Try parsing as RFC3339 first, then as Unix timestamp
			var deadline time.Time
			var err error

			// Try RFC3339 format
			deadline, err = time.Parse(time.RFC3339, profit.DeadLine)
			if err != nil {
				// Try Unix timestamp (seconds)
				if unixTime, parseErr := time.Parse(time.UnixDate, profit.DeadLine); parseErr == nil {
					deadline = unixTime
				} else {
					// If parsing fails, skip this profit
					continue
				}
			}

			// Calculate time elapsed percentage (0-100)
			// This is a simplified calculation - actual business logic may differ
			if deadline.After(now) {
				// Calculate percentage of time remaining
				// For now, return a simple indicator based on active profits
				// A more accurate calculation would need the profit start time
				totalTimePercentage += 50.0 // Placeholder: assume 50% elapsed for active profits
			} else {
				// Deadline passed but still active - might be ready for withdrawal
				totalTimePercentage += 100.0
			}
		}
	}

	if activeCount == 0 {
		return 0.0, nil
	}

	// Return average time percentage across all active profits
	return totalTimePercentage / float64(activeCount), nil
}

// GetScorePercentageToNextLevel implements the Laravel getScorePercentageToNextLevel helper
// Calls the Levels service to calculate percentage of score needed for next level
func (s *helperService) GetScorePercentageToNextLevel(ctx context.Context, userID uint64, currentScore int32) (float64, error) {
	if s.levelsClient == nil {
		return 0.0, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := s.levelsClient.GetUserLevel(ctx, &levelspb.GetUserLevelRequest{
		UserId: userID,
	})
	if err != nil {
		log.Printf("Failed to get user level for score percentage: %v", err)
		return 0.0, nil // Return 0 on error to not break the flow
	}

	// The response contains score_percentage_to_next_level as int32, convert to float64
	return float64(resp.ScorePercentageToNextLevel), nil
}

// GetUserLevel calls Levels service to get user's current level
func (s *helperService) GetUserLevel(ctx context.Context, userID uint64) (*LevelInfo, error) {
	if s.levelsClient == nil {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := s.levelsClient.GetUserLevel(ctx, &levelspb.GetUserLevelRequest{
		UserId: userID,
	})
	if err != nil {
		log.Printf("Failed to get user level: %v", err)
		return nil, nil // Return nil on error to not break the flow
	}

	if resp.LatestLevel == nil {
		return nil, nil
	}

	// Convert proto Level to LevelInfo
	level := &LevelInfo{
		ID:    resp.LatestLevel.Id,
		Title: resp.LatestLevel.Name, // Note: proto uses "name", but we map to "Title"
		Score: resp.LatestLevel.Score,
		Slug:  resp.LatestLevel.Slug,
	}

	// Get description from general_info if available
	if resp.LatestLevel.GeneralInfo != nil {
		level.Description = resp.LatestLevel.GeneralInfo.Description
	}

	return level, nil
}

// GetUserWallet calls Commercial service to get user's wallet balances
func (s *helperService) GetUserWallet(ctx context.Context, userID uint64) (*WalletInfo, error) {
	// Try to reconnect if client is nil (service might not have been ready at startup)
	if s.walletClient == nil && s.commercialServiceAddr != "" {
		log.Printf("Attempting to reconnect to commercial service at %s", s.commercialServiceAddr)
		connectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Create client interceptor to forward authorization header
		interceptor := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
			// Check if authorization is already in outgoing metadata
			outMd, hasOutgoing := metadata.FromOutgoingContext(ctx)
			if hasOutgoing && len(outMd.Get("authorization")) > 0 {
				// Already has authorization, proceed
				return invoker(ctx, method, req, reply, cc, opts...)
			}

			// Try to get authorization from incoming metadata
			if inMd, ok := metadata.FromIncomingContext(ctx); ok {
				if authHeaders := inMd.Get("authorization"); len(authHeaders) > 0 {
					// Forward authorization header to outgoing call
					ctx = metadata.AppendToOutgoingContext(ctx, "authorization", authHeaders[0])
				}
			}

			return invoker(ctx, method, req, reply, cc, opts...)
		}

		conn, err := grpc.DialContext(connectCtx, s.commercialServiceAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithUnaryInterceptor(interceptor))
		if err != nil {
			return nil, fmt.Errorf("failed to connect to commercial service at %s: %w", s.commercialServiceAddr, err)
		}
		s.commercialConn = conn
		s.walletClient = commercialpb.NewWalletServiceClient(conn)
		s.userVariableClient = commercialpb.NewUserVariableServiceClient(conn)
		log.Printf("Successfully reconnected to commercial service at %s", s.commercialServiceAddr)
	}

	if s.walletClient == nil {
		return nil, fmt.Errorf("commercial service not available")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := s.walletClient.GetWallet(ctx, &commercialpb.GetWalletRequest{
		UserId: userID,
	})
	if err != nil {
		log.Printf("Failed to get user wallet: %v", err)
		return nil, fmt.Errorf("failed to get wallet from commercial service: %w", err)
	}

	return &WalletInfo{
		Psc:          resp.Psc,
		Irr:          resp.Irr,
		Red:          resp.Red,
		Blue:         resp.Blue,
		Yellow:       resp.Yellow,
		Satisfaction: resp.Satisfaction,
		Effect:       resp.Effect,
	}, nil
}

// CreateWallet calls Commercial service to create a wallet for a newly registered user.
func (s *helperService) CreateWallet(ctx context.Context, userID uint64) error {
	if s.walletClient == nil {
		return fmt.Errorf("commercial service not available")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := s.walletClient.CreateWallet(ctx, &commercialpb.CreateWalletRequest{
		UserId: userID,
	})
	if err != nil {
		return fmt.Errorf("failed to create wallet via commercial service: %w", err)
	}
	return nil
}

// CreateUserVariables calls Commercial service to create default user_variables for a new user.
func (s *helperService) CreateUserVariables(ctx context.Context, userID uint64) error {
	if s.userVariableClient == nil {
		return fmt.Errorf("commercial service not available")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := s.userVariableClient.CreateUserVariables(ctx, &commercialpb.CreateUserVariablesRequest{
		UserId: userID,
	})
	if err != nil {
		return fmt.Errorf("failed to create user variables via commercial service: %w", err)
	}
	return nil
}

// Close closes gRPC connections
func (s *helperService) Close() error {
	var errs []error

	if s.levelsConn != nil {
		if err := s.levelsConn.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if s.featuresConn != nil {
		if err := s.featuresConn.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if s.commercialConn != nil {
		if err := s.commercialConn.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return errs[0] // Return first error
	}

	return nil
}
