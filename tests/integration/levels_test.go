package integration

import (
	"context"
	"testing"

	pb "metarang/shared/pb/levels"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// TestUserLevelProgression tests level-up flow
func TestUserLevelProgression(t *testing.T) {
	// TODO: Implement full test
	// 1. Create test user
	// 2. Record activities (trades, deposits, followers)
	// 3. Trigger score calculation
	// 4. Verify level-up
	// 5. Verify prize distribution
	// 6. Compare with Laravel behavior

	conn, err := grpc.Dial("localhost:50054", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := pb.NewLevelServiceClient(conn)

	req := &pb.GetUserLevelRequest{
		UserId: 1,
	}

	resp, err := client.GetUserLevel(context.Background(), req)
	if err != nil {
		t.Fatalf("GetUserLevel failed: %v", err)
	}

	if resp == nil {
		t.Error("Expected level response, got nil")
	}

	// TODO: Compare with golden JSON
	t.Logf("User score: %d", resp.UserScore)
}

// TestScoreCalculation tests the score calculation logic
func TestScoreCalculation(t *testing.T) {
	// TODO: Implement
	// Score = transactions_count + followers_count + deposit_amount + activity_hours
	// transactions_count = significant_trades * 2 (where irr > 7M or psc > equivalent)
	// followers_count = total_followers * 0.1
	// deposit_amount = deposits * 0.0001
	// activity_hours = total_hours * 0.1

	t.Skip("Full implementation pending")
}

// TestChallengeQuiz tests the challenge question system owned by social-service.
func TestChallengeQuiz(t *testing.T) {
	// TODO: Implement against social-service ChallengeService
	// 1. Get random unanswered question
	// 2. Submit correct answer
	// 3. Verify PSC reward added to wallet
	// 4. Submit wrong answer
	// 5. Verify no reward
	// 6. Verify answered questions are never retrieved again

	t.Skip("Full implementation pending")
}

// TestActivityLogging tests activity tracking
func TestActivityLogging(t *testing.T) {
	// TODO: Implement
	// 1. Log login activity
	// 2. Log logout activity
	// 3. Verify total minutes calculated
	// 4. Verify activity_hours updated in user_logs
	// 5. Verify score updated

	t.Skip("Full implementation pending")
}

// TestLevelPrizeDistribution tests prize claiming
func TestLevelPrizeDistribution(t *testing.T) {
	// TODO: Implement
	// 1. User reaches new level
	// 2. Verify prize attached to level
	// 3. Claim prize
	// 4. Verify wallet incremented (PSC, blue, red, yellow)
	// 5. Verify effect and satisfaction updated
	// 6. Verify prize marked as received

	t.Skip("Full implementation pending")
}
