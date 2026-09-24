package client_test

import (
	"context"
	"testing"
	"time"

	"metarang/features-service/internal/client"
	"metarang/features-service/tests/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestNewCommercialClient_DialAndClose(t *testing.T) {
	c, err := client.NewCommercialClient("127.0.0.1:1")
	if err != nil {
		return
	}
	require.NoError(t, c.Close())
}

func TestCommercialClient_CloseNilConn(t *testing.T) {
	c := client.NewCommercialClientFromGRPC(nil, nil)
	require.NoError(t, c.Close())
}

func TestCommercialClient_SettersAndUpdateWallet(t *testing.T) {
	stub := testutil.NewCommercialStub()
	c := client.NewCommercialClientFromGRPC(stub, stub)
	c.SetTimeout(50 * time.Millisecond)
	c.SetMaxRetries(2)

	require.NoError(t, c.UpdateWallet(context.Background(), 2, "psc", 0))
	require.NoError(t, c.UpdateWallet(context.Background(), 2, "psc", 5))
	require.NoError(t, c.UpdateWallet(context.Background(), 2, "psc", -3))
	require.Len(t, stub.AddCalls, 1)
	require.Len(t, stub.DeductCalls, 1)
}

func TestCommercialClient_GetWalletLockUnlockTransaction(t *testing.T) {
	stub := testutil.NewCommercialStub()
	c := client.NewCommercialClientFromGRPC(stub, stub)

	w, err := c.GetWallet(context.Background(), 2)
	require.NoError(t, err)
	assert.Equal(t, "1000000", w.Psc)

	require.NoError(t, c.LockBalance(context.Background(), 2, "psc", 1, "hold"))
	require.NoError(t, c.UnlockBalance(context.Background(), 2, "psc", 1))

	tx, err := c.CreateTransaction(context.Background(), 2, "psc", 1, "withdraw", 0, "App\\Models\\Trade", 9)
	require.NoError(t, err)
	assert.Equal(t, "1", tx.Id)
}

func TestCommercialClient_GetWalletError(t *testing.T) {
	stub := testutil.NewCommercialStub()
	stub.GetWalletErr = status.Error(codes.Unavailable, "down")
	c := client.NewCommercialClientFromGRPC(stub, stub)
	_, err := c.GetWallet(context.Background(), 2)
	require.Error(t, err)
}

func TestCommercialClient_DeductRetryableThenSuccess(t *testing.T) {
	stub := testutil.NewCommercialStub()
	stub.FailDeductGRPC = status.Error(codes.Unavailable, "down")
	c := client.NewCommercialClientFromGRPC(stub, stub)
	c.SetMaxRetries(2)
	c.SetTimeout(50 * time.Millisecond)

	err := c.DeductBalance(context.Background(), 2, "psc", 1)
	require.Error(t, err)
	var ce *client.CommercialError
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, "service_unavailable", ce.Type)
}

func TestCommercialClient_AddInsufficient(t *testing.T) {
	stub := testutil.NewCommercialStub()
	stub.FailAddGRPC = status.Error(codes.InvalidArgument, "insufficient balance")
	c := client.NewCommercialClientFromGRPC(stub, stub)

	err := c.AddBalance(context.Background(), 2, "psc", 1)
	require.Error(t, err)
	var ce *client.CommercialError
	require.ErrorAs(t, err, &ce)
	assert.Equal(t, "insufficient_balance", ce.Type)
}

func TestCommercialClient_CheckBalanceParseAndRetry(t *testing.T) {
	stub := testutil.NewCommercialStub()
	stub.Psc = "not-a-number"
	c := client.NewCommercialClientFromGRPC(stub, stub)
	_, err := c.CheckBalance(context.Background(), 2, "psc", 1)
	require.Error(t, err)

	stub = testutil.NewCommercialStub()
	stub.Red = "10"
	stub.Blue = "10"
	c = client.NewCommercialClientFromGRPC(stub, stub)
	ok, err := c.CheckBalance(context.Background(), 2, "red", 5)
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = c.CheckBalance(context.Background(), 2, "blue", 11)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestCommercialClient_AddBalanceSuccessFalse(t *testing.T) {
	stub := testutil.NewCommercialStub()
	stub.FailNthAdd = 1
	c := client.NewCommercialClientFromGRPC(stub, stub)
	err := c.AddBalance(context.Background(), 2, "psc", 1)
	require.Error(t, err)
}

func TestCommercialError_Unwrap(t *testing.T) {
	inner := status.Error(codes.Internal, "x")
	ce := &client.CommercialError{Type: "validation_error", Message: "m", Err: inner}
	assert.Contains(t, ce.Error(), "validation_error")
	assert.Equal(t, inner, ce.Unwrap())
	ce = &client.CommercialError{Type: "validation_error", Message: "m"}
	assert.Equal(t, "validation_error: m", ce.Error())
}

func TestNewNotificationClient_DialAndClose(t *testing.T) {
	c, err := client.NewNotificationClient("127.0.0.1:1")
	if err != nil {
		c = client.NewNotificationClientFromGRPC(testutil.NewNotificationStub())
	}
	require.NoError(t, c.Close())
}

func TestNotificationClient_HourlyProfitColorVariants(t *testing.T) {
	stub := testutil.NewNotificationStub()
	c := client.NewNotificationClientFromGRPC(stub)
	require.NoError(t, c.SendFeatureHourlyProfitDeposit(context.Background(), 2, "red", 1, "t", ""))
	require.NoError(t, c.SendFeatureHourlyProfitDeposit(context.Background(), 2, "blue", 1, "a", ""))
	require.NoError(t, c.SendFeatureHourlyProfitDeposit(context.Background(), 2, "green", 1, "x", ""))
	require.Len(t, stub.Calls, 3)
	assert.Contains(t, stub.Calls[0].Message, "رنگ قرمز")
	assert.Contains(t, stub.Calls[1].Data["karbari"], "آموزشی")
	assert.Equal(t, "green", stub.Calls[2].Data["asset"])
}
