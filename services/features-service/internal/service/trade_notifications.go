package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"metarang/features-service/internal/client"
	"metarang/features-service/internal/constants"
	"metarang/features-service/internal/models"
	"metarang/shared/pkg/helpers"
)

func (s *MarketplaceService) notifyContext(ctx context.Context) (context.Context, context.CancelFunc) {
	// HTML email rendering + SMTP delivery can exceed a few seconds on remote providers.
	return context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
}

func (s *MarketplaceService) tradeNotifyTarget(ctx context.Context, userID uint64) (name string, sendSMS, sendEmail bool) {
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

type featureEmailMeta struct {
	Area         string
	Application  string
	Density      string
	Coordinates  string
	Address      string
	Title        string
	PropertiesID string
}

func (s *MarketplaceService) loadFeatureEmailMeta(ctx context.Context, featureID uint64, properties *models.FeatureProperties) featureEmailMeta {
	meta := featureEmailMeta{}
	if properties != nil {
		meta.PropertiesID = properties.ID
		meta.Area = formatFeatureArea(properties.Area)
		meta.Application = constants.GetKarbariTitle(properties.Karbari)
		meta.Density = strconv.Itoa(properties.Density)
		meta.Address = strings.TrimSpace(properties.Address)
		meta.Title = firstNonEmptyString(strings.TrimSpace(properties.Label), properties.ID)
	}
	if s.geometryRepo != nil {
		coords, err := s.geometryRepo.GetCoordinatesByFeatureID(ctx, featureID)
		if err == nil && len(coords) > 0 {
			meta.Coordinates = strings.Join(coords, " | ")
		}
	}
	return meta
}

func formatFeatureArea(area float64) string {
	if area == 0 {
		return "0"
	}
	return strconv.FormatFloat(area, 'f', 2, 64)
}

func (s *MarketplaceService) userCodeOrEmpty(ctx context.Context, userID uint64) string {
	if userID == 0 || s.db == nil {
		return ""
	}
	code, err := s.GetUserCode(ctx, userID)
	if err != nil {
		return ""
	}
	return code
}

func jalaliNowParts() (date, timePart string) {
	now := time.Now()
	return helpers.FormatJalaliDate(now), helpers.FormatJalaliTime(now)
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func (s *MarketplaceService) sendBuyFeatureNotification(ctx context.Context, recipientID uint64, in client.BuyFeatureNotifyInput, properties *models.FeatureProperties, sellerID uint64) {
	if s.notificationClient == nil {
		return
	}
	name, sms, email := s.tradeNotifyTarget(ctx, recipientID)
	if in.BuyerName == "" && in.UserID == recipientID {
		in.BuyerName = name
	}
	meta := s.loadFeatureEmailMeta(ctx, in.FeatureID, properties)
	in.FeatureArea = meta.Area
	in.FeatureApplication = meta.Application
	in.FeatureDensity = meta.Density
	in.FeatureCoordinates = meta.Coordinates
	in.FeatureAddress = meta.Address
	if in.BuyerCode == "" {
		in.BuyerCode = s.userCodeOrEmpty(ctx, recipientID)
	}
	if in.SellerCode == "" && sellerID > 0 {
		in.SellerCode = s.userCodeOrEmpty(ctx, sellerID)
	}
	if in.OwnerCode == "" {
		in.OwnerCode = in.BuyerCode
	}
	if in.TransactionDate == "" || in.TransactionTime == "" {
		d, t := jalaliNowParts()
		if in.TransactionDate == "" {
			in.TransactionDate = d
		}
		if in.TransactionTime == "" {
			in.TransactionTime = t
		}
	}
	in.UserID = recipientID
	in.Delivery = client.NotificationDelivery{SendSMS: sms, SendEmail: email}
	nctx, cancel := s.notifyContext(ctx)
	defer cancel()
	if err := s.notificationClient.SendBuyFeatureNotification(nctx, in); err != nil {
		s.log.Warn("Failed to send buy feature notification", "error", err, "user_id", recipientID)
	}
}

func (s *MarketplaceService) sendSellFeatureNotification(ctx context.Context, recipientID uint64, in client.SellFeatureNotifyInput, properties *models.FeatureProperties, buyerID uint64) {
	if s.notificationClient == nil {
		return
	}
	name, sms, email := s.tradeNotifyTarget(ctx, recipientID)
	if in.SellerName == "" {
		in.SellerName = name
	}
	meta := s.loadFeatureEmailMeta(ctx, in.FeatureID, properties)
	in.FeatureArea = meta.Area
	in.FeatureApplication = meta.Application
	if in.SellerCode == "" {
		in.SellerCode = s.userCodeOrEmpty(ctx, recipientID)
	}
	if in.BuyerCode == "" && buyerID > 0 {
		in.BuyerCode = s.userCodeOrEmpty(ctx, buyerID)
	}
	if in.TransactionDate == "" || in.TransactionTime == "" {
		d, t := jalaliNowParts()
		if in.TransactionDate == "" {
			in.TransactionDate = d
		}
		if in.TransactionTime == "" {
			in.TransactionTime = t
		}
	}
	in.UserID = recipientID
	in.Delivery = client.NotificationDelivery{SendSMS: sms, SendEmail: email}
	nctx, cancel := s.notifyContext(ctx)
	defer cancel()
	if err := s.notificationClient.SendSellFeatureNotification(nctx, in); err != nil {
		s.log.Warn("Failed to send sell feature notification", "error", err, "user_id", recipientID)
	}
}

func (s *MarketplaceService) sendBuyRequestNotification(ctx context.Context, in client.BuyRequestNotifyInput, properties *models.FeatureProperties, buyerID, sellerID uint64) {
	if s.notificationClient == nil {
		return
	}
	name, sms, email := s.tradeNotifyTarget(ctx, in.UserID)
	meta := s.loadFeatureEmailMeta(ctx, in.FeatureID, properties)
	in.FeatureArea = meta.Area
	in.FeatureApplication = meta.Application
	in.FeatureDensity = meta.Density
	in.FeatureCoordinates = meta.Coordinates
	in.FeatureAddress = meta.Address
	if in.BuyerName == "" {
		if in.Role == "buyer" {
			in.BuyerName = name
		} else {
			in.BuyerName = s.getUserName(ctx, buyerID)
		}
	}
	if in.OwnerName == "" {
		if in.Role == "seller" {
			in.OwnerName = name
		} else {
			in.OwnerName = s.getUserName(ctx, sellerID)
		}
	}
	if in.BuyerCode == "" {
		in.BuyerCode = s.userCodeOrEmpty(ctx, buyerID)
	}
	if in.OwnerCode == "" {
		in.OwnerCode = s.userCodeOrEmpty(ctx, sellerID)
	}
	if in.CreatedDate == "" || in.CreatedTime == "" {
		d, t := jalaliNowParts()
		if in.CreatedDate == "" {
			in.CreatedDate = d
		}
		if in.CreatedTime == "" {
			in.CreatedTime = t
		}
	}
	in.Delivery = client.NotificationDelivery{SendSMS: sms, SendEmail: email}
	nctx, cancel := s.notifyContext(ctx)
	defer cancel()
	role := in.Role
	if err := s.notificationClient.SendBuyRequestNotification(nctx, in); err != nil {
		s.log.Warn("Failed to send buy request notification", "error", err, "user_id", in.UserID, "role", role)
	}
}

func (s *MarketplaceService) sendSellRequestNotification(ctx context.Context, sellerID, featureID uint64, properties *models.FeatureProperties, offerPSC, offerIRR float64) {
	if s.notificationClient == nil {
		return
	}
	name, sms, email := s.tradeNotifyTarget(ctx, sellerID)
	meta := s.loadFeatureEmailMeta(ctx, featureID, properties)
	d, t := jalaliNowParts()
	propertiesID := ""
	if properties != nil {
		propertiesID = properties.ID
	}
	in := client.SellRequestNotifyInput{
		SellerID:      sellerID,
		FeatureID:     featureID,
		PropertiesID:  propertiesID,
		SellerName:    name,
		SellerCode:    s.userCodeOrEmpty(ctx, sellerID),
		RequesterCode: s.userCodeOrEmpty(ctx, sellerID),
		FeatureTitle:  meta.Title,
		OfferPSC:      offerPSC,
		OfferIRR:      offerIRR,
		CreatedDate:   d,
		CreatedTime:   t,
		Delivery:      client.NotificationDelivery{SendSMS: sms, SendEmail: email},
	}
	nctx, cancel := s.notifyContext(ctx)
	defer cancel()
	if err := s.notificationClient.SendSellRequestNotification(nctx, in); err != nil {
		s.log.Warn("Failed to send sell request notification", "error", err)
	}
}
