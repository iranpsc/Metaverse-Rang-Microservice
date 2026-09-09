package client_test

import (
	"context"
	"errors"
	"testing"

	"metarang/features-service/internal/client"
	"metarang/features-service/tests/internal/testutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationClient_SendBuyRequestNotification(t *testing.T) {
	stub := testutil.NewNotificationStub()
	c := client.NewNotificationClientFromGRPC(stub)

	require.NoError(t, c.SendBuyRequestNotification(context.Background(), client.BuyRequestNotifyInput{
		UserID: 2, Role: "buyer", BuyRequestID: 9, FeatureID: 1, PropertiesID: "p1",
		PricePSC: 10, PriceIRR: 20, Delivery: client.NotificationDelivery{SendSMS: true, SendEmail: true},
	}))
	require.NoError(t, c.SendBuyRequestNotification(context.Background(), client.BuyRequestNotifyInput{
		UserID: 3, Role: "seller", BuyRequestID: 9, FeatureID: 1, PropertiesID: "p1",
		PricePSC: 10, PriceIRR: 20, Delivery: client.NotificationDelivery{SendSMS: true, SendEmail: true},
	}))

	require.Len(t, stub.Calls, 2)
	assert.Equal(t, "BuyRequestNotification", stub.Calls[0].Type)
	assert.Equal(t, uint64(2), stub.Calls[0].UserID)
	assert.Equal(t, "buyer", stub.Calls[0].Data["type"])
	assert.Equal(t, "transactions", stub.Calls[0].Data["related-to"])
	assert.Equal(t, "متارنگ", stub.Calls[0].Data["sender-name"])
	assert.Contains(t, stub.Calls[0].Message, "برداشت")
	assert.Contains(t, stub.Calls[0].Message, "p1")
	assert.True(t, stub.Calls[0].SendSMS)
	assert.True(t, stub.Calls[0].SendEmail)
	assert.Equal(t, "buy-land-request", stub.Calls[0].SMSTemplate)
	assert.Equal(t, "p1", stub.Calls[0].SMSTokens["token"])
	assert.Equal(t, "10", stub.Calls[0].SMSTokens["token2"])
	assert.Equal(t, "20", stub.Calls[0].SMSTokens["token3"])

	require.NoError(t, c.SendBuyRequestNotification(context.Background(), client.BuyRequestNotifyInput{
		UserID: 2, Role: "buyer", BuyRequestID: 9, FeatureID: 1, PropertiesID: "p1",
		PricePSC: 1000, PriceIRR: 0,
	}))
	assert.Equal(t, "1,000", stub.Calls[2].SMSTokens["token2"])
	assert.Equal(t, "0", stub.Calls[2].SMSTokens["token3"])

	assert.Equal(t, uint64(3), stub.Calls[1].UserID)
	assert.Equal(t, "seller", stub.Calls[1].Data["type"])
	assert.Contains(t, stub.Calls[1].Title, "دریافت")
}

func TestNotificationClient_SendBuyFeatureNotification(t *testing.T) {
	stub := testutil.NewNotificationStub()
	c := client.NewNotificationClientFromGRPC(stub)

	require.NoError(t, c.SendBuyFeatureNotification(context.Background(), client.BuyFeatureNotifyInput{
		UserID: 2, FeatureID: 1, PropertiesID: "p1", IsRGBPurchase: true, Color: "زرد", Stability: 10,
		BuyerName: "buyer", Delivery: client.NotificationDelivery{SendSMS: true, SendEmail: true},
	}))
	require.NoError(t, c.SendBuyFeatureNotification(context.Background(), client.BuyFeatureNotifyInput{
		UserID: 2, FeatureID: 1, PropertiesID: "p1", PSCAmount: 105, IRRAmount: 105,
		BuyerName: "buyer", SellerName: "seller", Delivery: client.NotificationDelivery{SendSMS: true, SendEmail: true},
	}))

	require.Len(t, stub.Calls, 2)
	assert.Equal(t, "BuyFeatureNotification", stub.Calls[0].Type)
	assert.Equal(t, "rgb", stub.Calls[0].Data["purchase_type"])
	assert.Equal(t, "زرد", stub.Calls[0].Data["color"])
	assert.Contains(t, stub.Calls[0].Message, "لیتر")
	assert.Contains(t, stub.Calls[0].Message, "p1")
	assert.Equal(t, "buy-land-metarang", stub.Calls[0].SMSTemplate)
	assert.Equal(t, "buyer", stub.Calls[0].SMSTokens["token20"])
	assert.Equal(t, "p1", stub.Calls[0].SMSTokens["token"])

	assert.Equal(t, "user", stub.Calls[1].Data["purchase_type"])
	assert.Equal(t, "105", stub.Calls[1].Data["psc_amount"])
	assert.Contains(t, stub.Calls[1].Message, "ریال")
	assert.Equal(t, "seller", stub.Calls[1].SMSTokens["token10"])
}

func TestNotificationClient_SendSellFeatureNotification(t *testing.T) {
	stub := testutil.NewNotificationStub()
	c := client.NewNotificationClientFromGRPC(stub)

	require.NoError(t, c.SendSellFeatureNotification(context.Background(), client.SellFeatureNotifyInput{
		UserID: 5, FeatureID: 1, PropertiesID: "p1", TradeID: 7, PSCAmount: 100, IRRAmount: 100,
		BuyerName: "buyer", SellerName: "seller", Delivery: client.NotificationDelivery{SendSMS: true, SendEmail: true},
	}))
	require.Len(t, stub.Calls, 1)
	assert.Equal(t, "sellFeature", stub.Calls[0].Type)
	assert.Equal(t, "transactions", stub.Calls[0].Data["related-to"])
	assert.Contains(t, stub.Calls[0].Message, "واریز شد")
	assert.Equal(t, "sell-land-metarang", stub.Calls[0].SMSTemplate)
	assert.Equal(t, "seller", stub.Calls[0].SMSTokens["token20"])
	assert.Equal(t, "buyer", stub.Calls[0].SMSTokens["token10"])
}

func TestNotificationClient_SendSellRequestNotification(t *testing.T) {
	stub := testutil.NewNotificationStub()
	c := client.NewNotificationClientFromGRPC(stub)

	require.NoError(t, c.SendSellRequestNotification(context.Background(), 3, 1, "p1", client.NotificationDelivery{SendSMS: true, SendEmail: true}))
	require.Len(t, stub.Calls, 1)
	assert.Equal(t, "SellRequestNotification", stub.Calls[0].Type)
	assert.Equal(t, "p1", stub.Calls[0].Data["properties_id"])
	assert.Equal(t, "sell-requests", stub.Calls[0].Data["related-to"])
	assert.Equal(t, "sell-land-metarang", stub.Calls[0].SMSTemplate)
	assert.Equal(t, "p1", stub.Calls[0].SMSTokens["token"])
	assert.True(t, stub.Calls[0].SendSMS)
	assert.True(t, stub.Calls[0].SendEmail)
}

func TestNotificationClient_SendFeatureHourlyProfitDeposit(t *testing.T) {
	stub := testutil.NewNotificationStub()
	c := client.NewNotificationClientFromGRPC(stub)

	require.NoError(t, c.SendFeatureHourlyProfitDeposit(context.Background(), 2, "yellow", 1.5, "m", "p1"))
	require.Len(t, stub.Calls, 1)
	assert.Equal(t, "FeatureHourlyProfitDeposit", stub.Calls[0].Type)
	assert.Equal(t, "yellow", stub.Calls[0].Data["asset"])
	assert.Equal(t, "مسکونی", stub.Calls[0].Data["karbari"])
	assert.Equal(t, "p1", stub.Calls[0].Data["id"])
	assert.Contains(t, stub.Calls[0].Message, "رنگ زرد")
	assert.Contains(t, stub.Calls[0].Message, "شناسه p1")
	assert.False(t, stub.Calls[0].SendSMS)
	assert.False(t, stub.Calls[0].SendEmail)
}

func TestNotificationClient_SendNotification_RPCError(t *testing.T) {
	stub := testutil.NewNotificationStub()
	stub.Err = errors.New("unavailable")
	c := client.NewNotificationClientFromGRPC(stub)

	err := c.SendNotification(context.Background(), 1, "t", "title", "msg", nil, client.NotificationDelivery{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to send notification")
}
