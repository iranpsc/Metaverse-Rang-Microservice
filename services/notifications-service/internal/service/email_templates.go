package service

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"strings"
	"sync"

	apptemplates "metarang/notifications-service/templates"
)

// EmailView is the data bag passed to HTML email templates.
// Fields mirror placeholders documented in notifications-service README.
type EmailView struct {
	Subject         string
	ContentTemplate string
	RecipientName   string
	RecipientEmail  string

	// Feature / marketplace
	OwnerCode          string
	OwnerName          string
	FeatureID          string
	FeatureArea        string
	FeatureApplication string
	FeatureDensity     string
	FeatureCoordinates string
	FeatureAddress     string
	FeatureTitle       string
	SellerCode         string
	SellerName         string
	BuyerCode          string
	BuyerName          string
	RequesterCode      string
	PriceIRR           string
	PricePSC           string
	PriceIRRLabel      string
	PricePSCLabel      string
	PaidAssetLabel     string
	PaidAssetAmount    string
	OfferIRR           string
	OfferPSC           string
	OfferIRRLabel      string
	OfferPSCLabel      string
	TransactionID      string
	TransactionDate    string
	TransactionTime    string
	RequestID          string
	CreatedDate        string
	CreatedTime        string
	DisputeURL         string
	SecurityURL        string
	ContactURL         string
	FaqURL             string
	CancelURL          string
	ManageURL          string
	DeclineURL         string

	// Dynasty
	RequesterName   string
	DynastyName     string
	DynastyCode     string
	Message         string
	SubmittedDate   string
	SubmittedTime   string
	Role            string
	AcceptedBy      string
	AcceptedAt      string
	DashboardURL    string
	GuidelineURL    string
	RejectionReason string
	RejectedAt      string
	ExploreURL      string
	ProfileURL      string

	// Auth-style / shared
	UserCode         string
	OtpCode          string
	ExpirationWindow string
	RequestIP        string
	RequestedAt      string
	ResetURL         string
	ExpiresIn        string
	VerifyURL        string
	SignupDate       string
	SignupTime       string
	ReRegisterURL    string
	LoginDate        string
	LoginTime        string
	IPAddress        string
	UserAgent        string
	Location         string
	SupportURL       string
	PaidAmount       string
	PaidAmountPSC    string
	PaymentID        string
	AssetTitle       string
	Quantity         string
	LockURL          string
	PrimaryAction    *EmailLink

	Assets EmailAssets
	Footer EmailFooter
}

// EmailAssets holds shared branding assets for email layouts.
type EmailAssets struct {
	LogoURL string
}

// EmailFooter holds footer copy and links.
type EmailFooter struct {
	Tagline string
	Links   []EmailLink
}

// EmailLink is a labeled URL used in email footers/buttons.
type EmailLink struct {
	Label string
	URL   string
}

type emailTemplateRenderer struct {
	tmpl *template.Template
}

var (
	emailRendererOnce sync.Once
	emailRenderer     *emailTemplateRenderer
	emailRendererErr  error
)

func getEmailTemplateRenderer() (*emailTemplateRenderer, error) {
	emailRendererOnce.Do(func() {
		var tmpl *template.Template
		funcs := template.FuncMap{
			"exec": func(name string, data any) (template.HTML, error) {
				if tmpl == nil {
					return "", fmt.Errorf("email templates not initialized")
				}
				var buf bytes.Buffer
				if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
					return "", err
				}
				return template.HTML(buf.String()), nil
			},
		}
		parsed, err := template.New("").Funcs(funcs).ParseFS(
			apptemplates.EmailFS,
			"email/*.html.tmpl",
			"email/dynasty/*.html.tmpl",
		)
		if err != nil {
			emailRendererErr = fmt.Errorf("parse email templates: %w", err)
			return
		}
		tmpl = parsed
		emailRenderer = &emailTemplateRenderer{tmpl: tmpl}
	})
	return emailRenderer, emailRendererErr
}

// ResolveEmailTemplate maps a notification type (+ optional data) to a root template name.
// Returns empty string when no dedicated HTML template exists (caller keeps plain fallback).
func ResolveEmailTemplate(notificationType string, data map[string]string) string {
	switch notificationType {
	case "BuyFeatureNotification":
		return "email/feature_purchase"
	case "BuyRequestNotification":
		switch strings.ToLower(strings.TrimSpace(dataValue(data, "type", "side", "role"))) {
		case "seller", "received", "owner":
			return "email/buy_request_received"
		default:
			return "email/buy_request_sent"
		}
	case "sellFeature":
		return "email/sell_feature"
	case "SellRequestNotification":
		return "email/sell_request"
	case "dynasty_join_request":
		switch strings.ToLower(strings.TrimSpace(dataValue(data, "side", "email_side"))) {
		case "received", "receiver", "owner":
			return "email/dynasty/join_request_received"
		default:
			return "email/dynasty/join_request_sent"
		}
	case "dynasty_join_request_accept":
		return "email/dynasty/join_request_accepted"
	case "dynasty_join_request_reject":
		return "email/dynasty/join_request_rejected"
	case "login":
		return "email/login_alert"
	default:
		return ""
	}
}

func dataValue(data map[string]string, keys ...string) string {
	if data == nil {
		return ""
	}
	for _, key := range keys {
		if v := strings.TrimSpace(data[key]); v != "" {
			return v
		}
	}
	return ""
}

// RenderNotificationEmail renders the HTML body for a notification type using email templates.
// Returns ("", nil) when no template mapping exists.
func RenderNotificationEmail(notificationType, subject string, data map[string]string, recipientName string) (string, error) {
	root := ResolveEmailTemplate(notificationType, data)
	if root == "" {
		return "", nil
	}

	renderer, err := getEmailTemplateRenderer()
	if err != nil {
		return "", err
	}

	view := buildEmailView(root, subject, data, recipientName)
	var buf bytes.Buffer
	if err := renderer.tmpl.ExecuteTemplate(&buf, root, view); err != nil {
		return "", fmt.Errorf("execute email template %s: %w", root, err)
	}
	return buf.String(), nil
}

func buildEmailView(root, subject string, data map[string]string, recipientName string) EmailView {
	if data == nil {
		data = map[string]string{}
	}
	get := func(keys ...string) string {
		return dataValue(data, keys...)
	}

	view := EmailView{
		Subject:         firstNonEmpty(subject, get("subject")),
		ContentTemplate: root + "/content",
		RecipientName:   firstNonEmpty(recipientName, get("RecipientName", "recipient_name", "BuyerName", "SellerName", "OwnerName", "RequesterName")),
		RecipientEmail:  get("RecipientEmail", "recipient_email"),

		OwnerCode:          get("OwnerCode", "owner_code"),
		OwnerName:          get("OwnerName", "owner_name"),
		FeatureID:          get("FeatureID", "properties_id", "feature_id"),
		FeatureArea:        get("FeatureArea", "feature_area", "area"),
		FeatureApplication: get("FeatureApplication", "feature_application", "karbari", "karbari_title"),
		FeatureDensity:     get("FeatureDensity", "feature_density", "density"),
		FeatureCoordinates: get("FeatureCoordinates", "feature_coordinates", "coordinates"),
		FeatureAddress:     get("FeatureAddress", "feature_address", "address"),
		FeatureTitle:       get("FeatureTitle", "feature_title", "label"),
		SellerCode:         get("SellerCode", "seller_code"),
		SellerName:         get("SellerName", "seller_name"),
		BuyerCode:          get("BuyerCode", "buyer_code"),
		BuyerName:          get("BuyerName", "buyer_name"),
		RequesterCode:      get("RequesterCode", "requester_code"),
		PriceIRR:           get("PriceIRR", "price_irr", "irr_amount"),
		PricePSC:           get("PricePSC", "price_psc", "psc_amount"),
		PriceIRRLabel:      get("PriceIRRLabel", "price_irr_label"),
		PricePSCLabel:      get("PricePSCLabel", "price_psc_label"),
		PaidAssetLabel:     get("PaidAssetLabel", "paid_asset_label"),
		PaidAssetAmount:    get("PaidAssetAmount", "paid_asset_amount"),
		OfferIRR:           get("OfferIRR", "offer_irr"),
		OfferPSC:           get("OfferPSC", "offer_psc"),
		OfferIRRLabel:      get("OfferIRRLabel", "offer_irr_label"),
		OfferPSCLabel:      get("OfferPSCLabel", "offer_psc_label"),
		TransactionID:      get("TransactionId", "TransactionID", "transaction_id", "trade_id"),
		TransactionDate:    get("TransactionDate", "transaction_date"),
		TransactionTime:    get("TransactionTime", "transaction_time"),
		RequestID:          get("RequestId", "RequestID", "request_id", "buy_request_id"),
		CreatedDate:        get("CreatedDate", "created_date"),
		CreatedTime:        get("CreatedTime", "created_time"),
		DisputeURL:         get("DisputeURL", "dispute_url"),
		SecurityURL:        get("SecurityURL", "security_url"),
		ContactURL:         get("ContactURL", "contact_url"),
		FaqURL:             get("FaqURL", "faq_url"),
		CancelURL:          get("CancelURL", "cancel_url"),
		ManageURL:          get("ManageURL", "manage_url"),
		DeclineURL:         get("DeclineURL", "decline_url"),

		RequesterName:   get("RequesterName", "requester_name"),
		DynastyName:     get("DynastyName", "dynasty_name"),
		DynastyCode:     get("DynastyCode", "dynasty_code"),
		Message:         get("Message", "message"),
		SubmittedDate:   get("SubmittedDate", "submitted_date", "created_date"),
		SubmittedTime:   get("SubmittedTime", "submitted_time", "created_time"),
		Role:            get("Role", "role", "relationship_title", "relationship"),
		AcceptedBy:      get("AcceptedBy", "accepted_by"),
		AcceptedAt:      get("AcceptedAt", "accepted_at"),
		DashboardURL:    get("DashboardURL", "dashboard_url"),
		GuidelineURL:    get("GuidelineURL", "guideline_url"),
		RejectionReason: get("RejectionReason", "rejection_reason"),
		RejectedAt:      get("RejectedAt", "rejected_at"),
		ExploreURL:      get("ExploreURL", "explore_url"),
		ProfileURL:      get("ProfileURL", "profile_url"),

		UserCode:         get("UserCode", "user_code"),
		OtpCode:          get("OtpCode", "otp_code"),
		ExpirationWindow: get("ExpirationWindow", "expiration_window"),
		RequestIP:        get("RequestIP", "request_ip", "ip"),
		RequestedAt:      get("RequestedAt", "requested_at"),
		ResetURL:         get("ResetURL", "reset_url"),
		ExpiresIn:        get("ExpiresIn", "expires_in"),
		VerifyURL:        get("VerifyURL", "verify_url"),
		SignupDate:       get("SignupDate", "signup_date"),
		SignupTime:       get("SignupTime", "signup_time"),
		ReRegisterURL:    get("ReRegisterURL", "re_register_url"),
		LoginDate:        get("LoginDate", "login_date"),
		LoginTime:        get("LoginTime", "login_time"),
		IPAddress:        get("IPAddress", "ip_address", "ip"),
		UserAgent:        get("UserAgent", "user_agent"),
		Location:         get("Location", "location"),
		SupportURL:       get("SupportURL", "support_url"),
		PaidAmount:       get("PaidAmount", "paid_amount"),
		PaidAmountPSC:    get("PaidAmountPSC", "paid_amount_psc"),
		PaymentID:        get("PaymentId", "PaymentID", "payment_id"),
		AssetTitle:       get("AssetTitle", "asset_title"),
		Quantity:         get("Quantity", "quantity"),
		LockURL:          get("LockURL", "lock_url"),

		Assets: EmailAssets{LogoURL: get("LogoURL", "logo_url", "sender-image")},
		Footer: EmailFooter{Tagline: get("FooterTagline", "footer_tagline")},
	}

	if view.ContactURL == "" {
		view.ContactURL = "https://rgb.irpsc.com/contact"
	}
	if view.SecurityURL == "" {
		view.SecurityURL = "https://rgb.irpsc.com/fa/security"
	}
	if view.FaqURL == "" {
		view.FaqURL = "https://rgb.irpsc.com/fa/metaverse-forum"
	}

	return view
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func init() {
	if _, err := getEmailTemplateRenderer(); err != nil {
		log.Printf("Warning: email templates unavailable: %v", err)
	}
}
