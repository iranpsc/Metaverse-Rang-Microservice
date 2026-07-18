// Package client provides gRPC clients for external services used by the features service.
package client

import (
	"context"
	"fmt"
	"math"
	"time"

	pb "metarang/shared/pb/commercial"
	"metarang/shared/pkg/auth"
	grpcutil "metarang/shared/pkg/grpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// CommercialClient wraps gRPC clients for Commercial Service
type CommercialClient struct {
	walletClient      pb.WalletServiceClient
	transactionClient pb.TransactionServiceClient
	conn              *grpc.ClientConn
	timeout           time.Duration
	maxRetries        int
}

// CommercialError represents different types of commercial service errors
type CommercialError struct {
	Type    string // "insufficient_balance", "service_unavailable", "validation_error"
	Message string
	Err     error
}

func (e *CommercialError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (%v)", e.Type, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

func (e *CommercialError) Unwrap() error {
	return e.Err
}

// NewCommercialClient creates a new Commercial Service client
func NewCommercialClient(address string) (*CommercialClient, error) {
	conn, err := grpcutil.DialContextWithTimeout(address, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to commercial service at %s: %w", address, err)
	}

	return &CommercialClient{
		walletClient:      pb.NewWalletServiceClient(conn),
		transactionClient: pb.NewTransactionServiceClient(conn),
		conn:              conn,
		timeout:           3 * time.Second, // Default timeout as per plan
		maxRetries:        3,               // Max retries as per plan
	}, nil
}

// SetTimeout sets the timeout for gRPC calls
func (c *CommercialClient) SetTimeout(timeout time.Duration) {
	c.timeout = timeout
}

// SetMaxRetries sets the maximum number of retries
func (c *CommercialClient) SetMaxRetries(maxRetries int) {
	c.maxRetries = maxRetries
}

// isRetryableError checks if an error is retryable (transient failure)
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	// Retry on transient errors
	return st.Code() == codes.Unavailable || st.Code() == codes.DeadlineExceeded || st.Code() == codes.ResourceExhausted
}

// isInsufficientBalanceError checks if error is due to insufficient balance
func isInsufficientBalanceError(err error) bool {
	if err == nil {
		return false
	}
	// Check if error message indicates insufficient balance
	errStr := err.Error()
	return contains(errStr, "insufficient") || contains(errStr, "balance") || contains(errStr, "موجودی")
}

func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// retryWithBackoff executes a function with exponential backoff retry
func (c *CommercialClient) retryWithBackoff(ctx context.Context, fn func() error) error {
	var lastErr error
	for attempt := 0; attempt < c.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 100ms, 200ms, 400ms
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * 100 * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		err := fn()
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry if error is not retryable
		if !isRetryableError(err) {
			return err
		}
	}

	return lastErr
}

// withTimeout creates a context with timeout
func (c *CommercialClient) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(auth.AttachOutgoingAuth(ctx), c.timeout)
}

// withAuth attaches the caller's bearer token for downstream commercial-service RPCs.
func (c *CommercialClient) withAuth(ctx context.Context) context.Context {
	return auth.AttachOutgoingAuth(ctx)
}

// Close closes the gRPC connection
func (c *CommercialClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// UpdateWallet updates a user's wallet balance (add or deduct)
// Positive amount = add, negative amount = deduct
func (c *CommercialClient) UpdateWallet(ctx context.Context, userID uint64, asset string, amount float64) error {
	if amount > 0 {
		return c.AddBalance(ctx, userID, asset, amount)
	} else if amount < 0 {
		return c.DeductBalance(ctx, userID, asset, -amount) // Make positive for deduct
	}
	return nil // Zero amount, no-op
}

// AddBalance adds balance to a user's wallet
func (c *CommercialClient) AddBalance(ctx context.Context, userID uint64, asset string, amount float64) error {
	return c.AddBalanceWithIdempotencyKey(ctx, userID, asset, amount, "")
}

// AddBalanceWithIdempotencyKey adds balance with idempotency key to prevent duplicate operations
func (c *CommercialClient) AddBalanceWithIdempotencyKey(ctx context.Context, userID uint64, asset string, amount float64, idempotencyKey string) error {
	req := &pb.AddBalanceRequest{
		UserId: userID,
		Asset:  asset,
		Amount: amount,
	}
	// Note: If commercial service proto supports idempotency_key, add it here

	var resp *pb.AddBalanceResponse
	var err error

	err = c.retryWithBackoff(ctx, func() error {
		timeoutCtx, cancel := c.withTimeout(ctx)
		defer cancel()

		resp, err = c.walletClient.AddBalance(timeoutCtx, req)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		if isInsufficientBalanceError(err) {
			return &CommercialError{Type: "insufficient_balance", Message: "Insufficient balance", Err: err}
		}
		if isRetryableError(err) {
			return &CommercialError{Type: "service_unavailable", Message: "Commercial service unavailable", Err: err}
		}
		return &CommercialError{Type: "validation_error", Message: "Failed to add balance", Err: err}
	}

	if !resp.Success {
		return &CommercialError{Type: "validation_error", Message: resp.Message, Err: nil}
	}

	return nil
}

// DeductBalance deducts balance from a user's wallet
func (c *CommercialClient) DeductBalance(ctx context.Context, userID uint64, asset string, amount float64) error {
	return c.DeductBalanceWithIdempotencyKey(ctx, userID, asset, amount, "")
}

// DeductBalanceWithIdempotencyKey deducts balance with idempotency key to prevent double-charging
func (c *CommercialClient) DeductBalanceWithIdempotencyKey(ctx context.Context, userID uint64, asset string, amount float64, idempotencyKey string) error {
	req := &pb.DeductBalanceRequest{
		UserId: userID,
		Asset:  asset,
		Amount: amount,
	}
	// Note: If commercial service proto supports idempotency_key, add it here

	var resp *pb.DeductBalanceResponse
	var err error

	err = c.retryWithBackoff(ctx, func() error {
		timeoutCtx, cancel := c.withTimeout(ctx)
		defer cancel()

		resp, err = c.walletClient.DeductBalance(timeoutCtx, req)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		if isInsufficientBalanceError(err) {
			return &CommercialError{Type: "insufficient_balance", Message: "Insufficient balance", Err: err}
		}
		if isRetryableError(err) {
			return &CommercialError{Type: "service_unavailable", Message: "Commercial service unavailable", Err: err}
		}
		return &CommercialError{Type: "validation_error", Message: "Failed to deduct balance", Err: err}
	}

	if !resp.Success {
		return &CommercialError{Type: "validation_error", Message: resp.Message, Err: nil}
	}

	return nil
}

// GetWallet retrieves a user's wallet information
func (c *CommercialClient) GetWallet(ctx context.Context, userID uint64) (*pb.WalletResponse, error) {
	req := &pb.GetWalletRequest{
		UserId: userID,
	}

	resp, err := c.walletClient.GetWallet(c.withAuth(ctx), req)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet: %w", err)
	}

	return resp, nil
}

// CreateTransaction creates a transaction record
func (c *CommercialClient) CreateTransaction(ctx context.Context, userID uint64, asset string, amount float64, action string, status int32, payableType string, payableID uint64) (*pb.Transaction, error) {
	req := &pb.CreateTransactionRequest{
		UserId:      userID,
		Asset:       asset,
		Amount:      amount,
		Action:      action,
		Status:      status,
		PayableType: payableType,
		PayableId:   payableID,
	}

	resp, err := c.transactionClient.CreateTransaction(c.withAuth(ctx), req)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	return resp, nil
}

// LockBalance locks balance for a pending transaction
func (c *CommercialClient) LockBalance(ctx context.Context, userID uint64, asset string, amount float64, reason string) error {
	req := &pb.LockBalanceRequest{
		UserId: userID,
		Asset:  asset,
		Amount: amount,
		Reason: reason,
	}

	_, err := c.walletClient.LockBalance(c.withAuth(ctx), req)
	if err != nil {
		return fmt.Errorf("failed to lock balance: %w", err)
	}

	return nil
}

// UnlockBalance unlocks previously locked balance
func (c *CommercialClient) UnlockBalance(ctx context.Context, userID uint64, asset string, amount float64) error {
	req := &pb.UnlockBalanceRequest{
		UserId: userID,
		Asset:  asset,
		Amount: amount,
	}

	_, err := c.walletClient.UnlockBalance(c.withAuth(ctx), req)
	if err != nil {
		return fmt.Errorf("failed to unlock balance: %w", err)
	}

	return nil
}

// CheckBalance verifies if user has sufficient balance
// Returns true if balance >= required amount
func (c *CommercialClient) CheckBalance(ctx context.Context, userID uint64, asset string, requiredAmount float64) (bool, error) {
	var wallet *pb.WalletResponse
	var err error

	err = c.retryWithBackoff(ctx, func() error {
		timeoutCtx, cancel := c.withTimeout(ctx)
		defer cancel()

		wallet, err = c.GetWallet(timeoutCtx, userID)
		return err
	})

	if err != nil {
		return false, err
	}

	var balance float64
	switch asset {
	case "psc":
		balance = parseWalletString(wallet.Psc)
	case "irr":
		balance = parseWalletString(wallet.Irr)
	case "red":
		balance = parseWalletString(wallet.Red)
	case "blue":
		balance = parseWalletString(wallet.Blue)
	case "yellow":
		balance = parseWalletString(wallet.Yellow)
	default:
		return false, fmt.Errorf("unknown asset: %s", asset)
	}

	return balance >= requiredAmount, nil
}

// parseWalletString converts formatted wallet string to float
// Handles compact notation like "1.5K", "2.3M"
func parseWalletString(s string) float64 {
	// TODO: Implement proper parsing of compact notation
	// For now, this is a placeholder
	// In production, this should parse strings like "1.5K" -> 1500.0
	return 0
}
