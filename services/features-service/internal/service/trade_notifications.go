package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"metarang/features-service/internal/client"
)

func (s *MarketplaceService) notifyContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
}

func (s *MarketplaceService) tradeNotifyTarget(ctx context.Context, userID uint64) (name string, sendSMS, sendEmail bool) {
	sendEmail = true
	if s.db == nil {
		return "", false, true
	}

	var nameNS, phone sql.NullString
	var verifiedAt sql.NullTime
	var notificationsJSON sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT u.name, u.phone, u.phone_verified_at, s.notifications
		FROM users u
		LEFT JOIN settings s ON s.user_id = u.id
		WHERE u.id = ? -- trade_notification_channels
		LIMIT 1
	`, userID).Scan(&nameNS, &phone, &verifiedAt, &notificationsJSON)
	if err != nil {
		return "", false, true
	}

	name = nameNS.String
	tradesSMS, tradesEmail := true, true
	if notificationsJSON.Valid && strings.TrimSpace(notificationsJSON.String) != "" {
		tradesSMS = notificationJSONFlag(notificationsJSON.String, "trades_sms", true)
		tradesEmail = notificationJSONFlag(notificationsJSON.String, "trades_email", true)
	}
	hasVerifiedPhone := strings.TrimSpace(phone.String) != "" && verifiedAt.Valid
	return name, hasVerifiedPhone && tradesSMS, tradesEmail
}

func notificationJSONFlag(raw, key string, defaultVal bool) bool {
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return defaultVal
	}
	v, ok := m[key]
	if !ok || v == nil {
		return defaultVal
	}
	switch t := v.(type) {
	case bool:
		return t
	case float64:
		return t != 0
	case json.Number:
		n, convErr := t.Float64()
		if convErr != nil {
			return defaultVal
		}
		return n != 0
	case string:
		return t == "1" || strings.EqualFold(t, "true")
	default:
		return defaultVal
	}
}

func (s *MarketplaceService) sendBuyFeatureNotification(ctx context.Context, recipientID uint64, in client.BuyFeatureNotifyInput) {
	if s.notificationClient == nil {
		return
	}
	name, sms, email := s.tradeNotifyTarget(ctx, recipientID)
	if in.BuyerName == "" && in.UserID == recipientID {
		in.BuyerName = name
	}
	in.UserID = recipientID
	in.Delivery = client.NotificationDelivery{SendSMS: sms, SendEmail: email}
	nctx, cancel := s.notifyContext(ctx)
	defer cancel()
	if err := s.notificationClient.SendBuyFeatureNotification(nctx, in); err != nil {
		s.log.Warn("Failed to send buy feature notification", "error", err, "user_id", recipientID)
	}
}

func (s *MarketplaceService) sendSellFeatureNotification(ctx context.Context, recipientID uint64, in client.SellFeatureNotifyInput) {
	if s.notificationClient == nil {
		return
	}
	name, sms, email := s.tradeNotifyTarget(ctx, recipientID)
	if in.SellerName == "" {
		in.SellerName = name
	}
	in.UserID = recipientID
	in.Delivery = client.NotificationDelivery{SendSMS: sms, SendEmail: email}
	nctx, cancel := s.notifyContext(ctx)
	defer cancel()
	if err := s.notificationClient.SendSellFeatureNotification(nctx, in); err != nil {
		s.log.Warn("Failed to send sell feature notification", "error", err, "user_id", recipientID)
	}
}

func (s *MarketplaceService) sendBuyRequestNotification(ctx context.Context, in client.BuyRequestNotifyInput) {
	if s.notificationClient == nil {
		return
	}
	_, sms, email := s.tradeNotifyTarget(ctx, in.UserID)
	in.Delivery = client.NotificationDelivery{SendSMS: sms, SendEmail: email}
	nctx, cancel := s.notifyContext(ctx)
	defer cancel()
	role := in.Role
	if err := s.notificationClient.SendBuyRequestNotification(nctx, in); err != nil {
		s.log.Warn("Failed to send buy request notification", "error", err, "user_id", in.UserID, "role", role)
	}
}

func (s *MarketplaceService) sendSellRequestNotification(ctx context.Context, sellerID, featureID uint64, propertiesID string) {
	if s.notificationClient == nil {
		return
	}
	_, sms, email := s.tradeNotifyTarget(ctx, sellerID)
	nctx, cancel := s.notifyContext(ctx)
	defer cancel()
	if err := s.notificationClient.SendSellRequestNotification(nctx, sellerID, featureID, propertiesID, client.NotificationDelivery{
		SendSMS: sms, SendEmail: email,
	}); err != nil {
		s.log.Warn("Failed to send sell request notification", "error", err)
	}
}
