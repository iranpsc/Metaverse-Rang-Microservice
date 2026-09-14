package client

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	pb "metarang/shared/pb/notifications"
	grpcutil "metarang/shared/pkg/grpc"

	"google.golang.org/grpc"
)

const (
	smsTemplateBuyFeature  = "buy-land-metarang"
	smsTemplateSellFeature = "sell-land-metarang"
	smsTemplateBuyRequest  = "buy-land-request"
	smsTemplateSellRequest = "sell-land-request"

	senderName     = "متارنگ"
	senderImageRel = "uploads/img/logo.png"
)

// NotificationDelivery controls SMS/email channels for a notification.
type NotificationDelivery struct {
	SendSMS     bool
	SendEmail   bool
	SMSTemplate string
	SMSTokens   map[string]string
}

// BuyFeatureNotifyInput is the Laravel BuyFeatureNotification payload.
type BuyFeatureNotifyInput struct {
	UserID             uint64
	FeatureID          uint64
	PropertiesID       string
	IsRGBPurchase      bool
	Color              string
	Stability          float64
	PSCAmount          float64
	IRRAmount          float64
	BuyerName          string
	SellerName         string
	BuyerCode          string
	SellerCode         string
	OwnerCode          string
	FeatureArea        string
	FeatureApplication string
	FeatureDensity     string
	FeatureCoordinates string
	FeatureAddress     string
	TransactionID      string
	TransactionDate    string
	TransactionTime    string
	Delivery           NotificationDelivery
}

// SellFeatureNotifyInput is the Laravel sellFeature payload.
type SellFeatureNotifyInput struct {
	UserID             uint64
	FeatureID          uint64
	PropertiesID       string
	TradeID            uint64
	PSCAmount          float64
	IRRAmount          float64
	BuyerName          string
	SellerName         string
	BuyerCode          string
	SellerCode         string
	FeatureArea        string
	FeatureApplication string
	TransactionDate    string
	TransactionTime    string
	Delivery           NotificationDelivery
}

// BuyRequestNotifyInput is the Laravel BuyRequestNotification payload.
type BuyRequestNotifyInput struct {
	UserID             uint64
	Role               string
	BuyRequestID       uint64
	FeatureID          uint64
	PropertiesID       string
	PricePSC           float64
	PriceIRR           float64
	BuyerName          string
	BuyerCode          string
	OwnerName          string
	OwnerCode          string
	FeatureArea        string
	FeatureApplication string
	FeatureDensity     string
	FeatureCoordinates string
	FeatureAddress     string
	CreatedDate        string
	CreatedTime        string
	Delivery           NotificationDelivery
}

// SellRequestNotifyInput is the sell-request notification payload.
type SellRequestNotifyInput struct {
	SellerID      uint64
	FeatureID     uint64
	PropertiesID  string
	SellerName    string
	SellerCode    string
	RequesterCode string
	FeatureTitle  string
	OfferPSC      float64
	OfferIRR      float64
	CreatedDate   string
	CreatedTime   string
	Delivery      NotificationDelivery
}

// NotificationClient wraps gRPC client for Notification Service
type NotificationClient struct {
	client pb.NotificationServiceClient
	conn   *grpc.ClientConn
}

// NewNotificationClient creates a new Notification Service client
func NewNotificationClient(address string) (*NotificationClient, error) {
	conn, err := grpcutil.DialContextWithTimeout(address, 5*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to notification service at %s: %w", address, err)
	}

	return &NotificationClient{
		client: pb.NewNotificationServiceClient(conn),
		conn:   conn,
	}, nil
}

// NewNotificationClientFromGRPC builds a NotificationClient from an existing gRPC stub (tests).
func NewNotificationClientFromGRPC(grpcClient pb.NotificationServiceClient) *NotificationClient {
	return &NotificationClient{client: grpcClient}
}

// Close closes the gRPC connection
func (c *NotificationClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func senderImageURL() string {
	base := strings.TrimSuffix(os.Getenv("APP_URL"), "/")
	if base == "" {
		return senderImageRel
	}
	return base + "/" + senderImageRel
}

func appPath(path string) string {
	base := strings.TrimSuffix(os.Getenv("APP_URL"), "/")
	if base == "" {
		base = "https://rgb.irpsc.com"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func colorAssetLabel(colorPersian string) string {
	color := strings.TrimSpace(colorPersian)
	if color == "" {
		return "رنگ"
	}
	if strings.HasPrefix(color, "رنگ") {
		return color
	}
	return "رنگ " + color
}

// setPaidAmount writes amount fields only when amount > 0 so email templates hide unused assets.
func setPaidAmount(data map[string]string, legacyKey, amountKey, labelKey, label string, amount float64) {
	if amount <= 0 {
		return
	}
	formatted := formatPlainAmount(amount)
	if legacyKey != "" {
		data[legacyKey] = formatted
	}
	data[amountKey] = formatted
	if labelKey != "" && label != "" {
		data[labelKey] = label
	}
}

func withSenderMeta(data map[string]string) map[string]string {
	if data == nil {
		data = map[string]string{}
	}
	data["sender-name"] = senderName
	data["sender-image"] = senderImageURL()
	if _, ok := data["ContactURL"]; !ok {
		data["ContactURL"] = "https://rgb.irpsc.com/contact"
	}
	if _, ok := data["SecurityURL"]; !ok {
		data["SecurityURL"] = "https://rgb.irpsc.com/fa/security"
	}
	if _, ok := data["FaqURL"]; !ok {
		data["FaqURL"] = "https://rgb.irpsc.com/fa/metaverse-forum"
	}
	return data
}

// SendNotification sends a notification to a user
func (c *NotificationClient) SendNotification(ctx context.Context, userID uint64, notificationType, title, message string, data map[string]string, delivery NotificationDelivery) error {
	req := &pb.SendNotificationRequest{
		UserId:      userID,
		Type:        notificationType,
		Title:       title,
		Message:     message,
		Data:        data,
		SendSms:     delivery.SendSMS,
		SendEmail:   delivery.SendEmail,
		SmsTemplate: delivery.SMSTemplate,
		SmsTokens:   delivery.SMSTokens,
	}

	_, err := c.client.SendNotification(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to send notification: %w", err)
	}

	return nil
}

// SendBuyRequestNotification sends a buy request notification to buyer or seller
func (c *NotificationClient) SendBuyRequestNotification(ctx context.Context, in BuyRequestNotifyInput) error {
	data := withSenderMeta(map[string]string{
		"buy_request_id":     fmt.Sprintf("%d", in.BuyRequestID),
		"RequestId":          fmt.Sprintf("%d", in.BuyRequestID),
		"feature_id":         fmt.Sprintf("%d", in.FeatureID),
		"properties_id":      in.PropertiesID,
		"FeatureID":          firstNonEmpty(in.PropertiesID, fmt.Sprintf("%d", in.FeatureID)),
		"type":               in.Role,
		"related-to":         "transactions",
		"BuyerName":          in.BuyerName,
		"BuyerCode":          in.BuyerCode,
		"OwnerName":          in.OwnerName,
		"OwnerCode":          in.OwnerCode,
		"FeatureArea":        in.FeatureArea,
		"FeatureApplication": in.FeatureApplication,
		"FeatureDensity":     in.FeatureDensity,
		"FeatureCoordinates": in.FeatureCoordinates,
		"FeatureAddress":     in.FeatureAddress,
		"CreatedDate":        in.CreatedDate,
		"CreatedTime":        in.CreatedTime,
		"CancelURL":          appPath("/api/buy-requests"),
		"ManageURL":          appPath("/api/buy-requests"),
	})
	setPaidAmount(data, "price_psc", "OfferPSC", "OfferPSCLabel", "PSC", in.PricePSC)
	setPaidAmount(data, "price_irr", "OfferIRR", "OfferIRRLabel", "IRR", in.PriceIRR)

	var title, message string
	if in.Role == "buyer" {
		title = "درخواست خرید ارسال شد"
		message = fmt.Sprintf("مبلغ %s psc و %s از حساب شما بابت پیشنهاد خرید ملک %s برداشت شد.",
			formatPlainAmount(in.PricePSC), formatPlainAmount(in.PriceIRR), in.PropertiesID)
	} else {
		title = "درخواست خرید دریافت شد"
		message = fmt.Sprintf("یک پیشنهاد خرید برای ملک %s دریافت شد.", in.PropertiesID)
	}

	in.Delivery.SMSTemplate = smsTemplateBuyRequest
	in.Delivery.SMSTokens = map[string]string{
		"token":  in.PropertiesID,
		"token2": formatGroupedAmount(in.PricePSC),
		"token3": formatGroupedAmount(in.PriceIRR),
	}

	return c.SendNotification(ctx, in.UserID, "BuyRequestNotification", title, message, data, in.Delivery)
}

// SendBuyFeatureNotification sends a notification when a feature is purchased
func (c *NotificationClient) SendBuyFeatureNotification(ctx context.Context, in BuyFeatureNotifyInput) error {
	data := withSenderMeta(map[string]string{
		"feature_id":         fmt.Sprintf("%d", in.FeatureID),
		"properties_id":      in.PropertiesID,
		"FeatureID":          firstNonEmpty(in.PropertiesID, fmt.Sprintf("%d", in.FeatureID)),
		"related-to":         "transactions",
		"RecipientName":      in.BuyerName,
		"BuyerName":          in.BuyerName,
		"BuyerCode":          in.BuyerCode,
		"OwnerCode":          firstNonEmpty(in.OwnerCode, in.BuyerCode),
		"SellerCode":         in.SellerCode,
		"SellerName":         in.SellerName,
		"FeatureArea":        in.FeatureArea,
		"FeatureApplication": in.FeatureApplication,
		"FeatureDensity":     in.FeatureDensity,
		"FeatureCoordinates": in.FeatureCoordinates,
		"FeatureAddress":     in.FeatureAddress,
		"TransactionId":      in.TransactionID,
		"TransactionDate":    in.TransactionDate,
		"TransactionTime":    in.TransactionTime,
		"DisputeURL":         appPath("/api/support"),
	})

	title := "خریداری ملک"
	var message string
	if in.IsRGBPurchase {
		message = fmt.Sprintf("%s لیتر رنگ %s از حساب شما بابت خرید زمین %s برداشت شد.",
			formatPlainAmount(in.Stability), in.Color, in.PropertiesID)
		data["stability"] = formatPlainAmount(in.Stability)
		data["color"] = in.Color
		data["purchase_type"] = "rgb"
		data["PaidAssetLabel"] = colorAssetLabel(in.Color)
		data["PaidAssetAmount"] = formatPlainAmount(in.Stability)
	} else {
		message = fmt.Sprintf("از حساب شما %s psc و %s ریال بابت خرید ملک %s برداشت شد.",
			formatPlainAmount(in.PSCAmount), formatPlainAmount(in.IRRAmount), in.PropertiesID)
		data["purchase_type"] = "user"
		setPaidAmount(data, "psc_amount", "PricePSC", "PricePSCLabel", "PSC", in.PSCAmount)
		setPaidAmount(data, "irr_amount", "PriceIRR", "PriceIRRLabel", "IRR", in.IRRAmount)
	}

	in.Delivery.SMSTemplate = smsTemplateBuyFeature
	in.Delivery.SMSTokens = map[string]string{
		"token":   in.PropertiesID,
		"token20": in.BuyerName,
		"token10": in.SellerName,
	}

	return c.SendNotification(ctx, in.UserID, "BuyFeatureNotification", title, message, data, in.Delivery)
}

// SendSellFeatureNotification sends a notification when a user sells a feature.
func (c *NotificationClient) SendSellFeatureNotification(ctx context.Context, in SellFeatureNotifyInput) error {
	data := withSenderMeta(map[string]string{
		"feature_id":         fmt.Sprintf("%d", in.FeatureID),
		"properties_id":      in.PropertiesID,
		"FeatureID":          firstNonEmpty(in.PropertiesID, fmt.Sprintf("%d", in.FeatureID)),
		"related-to":         "transactions",
		"RecipientName":      in.SellerName,
		"SellerName":         in.SellerName,
		"SellerCode":         in.SellerCode,
		"BuyerName":          in.BuyerName,
		"BuyerCode":          in.BuyerCode,
		"FeatureArea":        in.FeatureArea,
		"FeatureApplication": in.FeatureApplication,
		"TransactionDate":    in.TransactionDate,
		"TransactionTime":    in.TransactionTime,
		"DisputeURL":         appPath("/api/support"),
	})
	setPaidAmount(data, "psc_amount", "PricePSC", "PricePSCLabel", "PSC", in.PSCAmount)
	setPaidAmount(data, "irr_amount", "PriceIRR", "PriceIRRLabel", "IRR", in.IRRAmount)
	if in.TradeID > 0 {
		data["trade_id"] = fmt.Sprintf("%d", in.TradeID)
		data["TransactionId"] = fmt.Sprintf("%d", in.TradeID)
	}

	title := "فروش ملک"
	message := sellFeatureMessage(in.PSCAmount, in.IRRAmount, in.PropertiesID)

	in.Delivery.SMSTemplate = smsTemplateSellFeature
	in.Delivery.SMSTokens = map[string]string{
		"token":   in.PropertiesID,
		"token20": in.SellerName,
		"token10": in.BuyerName,
	}

	return c.SendNotification(ctx, in.UserID, "sellFeature", title, message, data, in.Delivery)
}

func sellFeatureMessage(pscAmount, irrAmount float64, propertiesID string) string {
	switch {
	case pscAmount > 0 && irrAmount > 0:
		return fmt.Sprintf("مبلغ %s psc و %s به حساب شما بابت فروش ملک %s واریز شد.",
			formatPlainAmount(pscAmount), formatPlainAmount(irrAmount), propertiesID)
	case pscAmount > 0:
		return fmt.Sprintf("مبلغ %s psc به حساب شما بابت فروش ملک %s واریز شد.",
			formatPlainAmount(pscAmount), propertiesID)
	case irrAmount > 0:
		return fmt.Sprintf("مبلغ %s ریال به حساب شما بابت فروش ملک %s واریز شد.",
			formatPlainAmount(irrAmount), propertiesID)
	default:
		return fmt.Sprintf("ملک %s با موفقیت فروخته شد.", propertiesID)
	}
}

// SendSellRequestNotification sends a notification when a sell request is created
func (c *NotificationClient) SendSellRequestNotification(ctx context.Context, in SellRequestNotifyInput) error {
	title := "درخواست فروش ملک"
	message := fmt.Sprintf("ملک %s با موفقیت قیمت گذاری شد.", in.PropertiesID)
	data := withSenderMeta(map[string]string{
		"feature_id":    fmt.Sprintf("%d", in.FeatureID),
		"properties_id": in.PropertiesID,
		"FeatureID":     firstNonEmpty(in.PropertiesID, fmt.Sprintf("%d", in.FeatureID)),
		"related-to":    "sell-requests",
		"SellerName":    in.SellerName,
		"SellerCode":    in.SellerCode,
		"RequesterCode": firstNonEmpty(in.RequesterCode, in.SellerCode),
		"FeatureTitle":  firstNonEmpty(in.FeatureTitle, in.PropertiesID),
		"CreatedDate":   in.CreatedDate,
		"CreatedTime":   in.CreatedTime,
		"ManageURL":     appPath("/api/sell-requests"),
		"DeclineURL":    appPath("/api/sell-requests"),
	})
	setPaidAmount(data, "offer_psc", "OfferPSC", "OfferPSCLabel", "PSC", in.OfferPSC)
	setPaidAmount(data, "offer_irr", "OfferIRR", "OfferIRRLabel", "IRR", in.OfferIRR)

	in.Delivery.SMSTemplate = smsTemplateSellRequest
	in.Delivery.SMSTokens = map[string]string{
		"token": in.PropertiesID,
	}

	return c.SendNotification(ctx, in.SellerID, "SellRequestNotification", title, message, data, in.Delivery)
}

// SendFeatureHourlyProfitDeposit sends a notification when hourly profit is withdrawn.
// Laravel uses database + broadcast only (no SMS/email).
func (c *NotificationClient) SendFeatureHourlyProfitDeposit(ctx context.Context, userID uint64, asset string, amount float64, karbari string, featurePropertiesID string) error {
	colorName := hourlyProfitAssetTitle(asset)
	karbariTitle := hourlyProfitKarbariTitle(karbari)

	title := fmt.Sprintf("سود ساعتی %s", karbariTitle)
	amountText := strconv.FormatFloat(amount, 'f', 3, 64)
	var message string
	if featurePropertiesID == "" {
		message = fmt.Sprintf("مقدار %s %s به حساب شما بابت سود ساعت شمار حاصل از ملک های %s واریز گردید.",
			amountText, colorName, karbariTitle)
	} else {
		message = fmt.Sprintf("مقدار %s %s به حساب شما بابت سود ساعت شمار حاصل از ملک به شناسه %s واریز گردید.",
			amountText, colorName, featurePropertiesID)
	}

	data := withSenderMeta(map[string]string{
		"asset":      asset,
		"amount":     amountText,
		"karbari":    karbariTitle,
		"related-to": "transactions",
	})
	if featurePropertiesID != "" {
		data["id"] = featurePropertiesID
	}

	return c.SendNotification(ctx, userID, "FeatureHourlyProfitDeposit", title, message, data, NotificationDelivery{})
}

func hourlyProfitAssetTitle(asset string) string {
	switch asset {
	case "yellow":
		return "رنگ زرد"
	case "red":
		return "رنگ قرمز"
	case "blue":
		return "رنگ آبی"
	default:
		return asset
	}
}

func hourlyProfitKarbariTitle(karbari string) string {
	switch karbari {
	case "m":
		return "مسکونی"
	case "t":
		return "تجاری"
	case "a":
		return "آموزشی"
	default:
		return karbari
	}
}

func formatPlainAmount(n float64) string {
	if n == 0 {
		return "0"
	}
	if n == math.Trunc(n) {
		return strconv.FormatInt(int64(n), 10)
	}
	return strconv.FormatFloat(n, 'f', -1, 64)
}

func formatGroupedAmount(n float64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	s := strconv.FormatInt(int64(math.Round(n)), 10)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}
