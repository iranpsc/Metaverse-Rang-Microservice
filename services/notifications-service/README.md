# Notifications Service

The Notification Service provides multi-channel delivery capabilities for metarang. It is responsible for persisting in-app notifications and dispatching outbound messages via SMS and email providers.

## Responsibilities
- Persist user notifications for in-app consumption.
- Deliver SMS messages (transactional and OTP).
- Deliver email messages with plain-text and HTML support.
- Expose gRPC endpoints defined in `shared/proto/notifications.proto`.

## Project Layout
```
notifications-service/
├── cmd/server            # Application entrypoint
├── internal/
│   ├── handler           # gRPC handlers (Notification, SMS, Email)
│   ├── models            # Domain models and payload DTOs
│   ├── repository        # Database persistence layer
│   └── service           # Business logic and provider abstractions
├── config.env.sample     # Example configuration
├── Dockerfile            # Multi-stage container build
└── go.mod                # Go module definition
```

## Getting Started
1. Copy environment configuration:
   ```bash
   cd services/notifications-service
   cp config.env.sample .env
   ```
2. Update `.env` with your database credentials and provider secrets.
3. Run the service locally:
   ```bash
   go run ./cmd/server
   ```

## Environment Variables
Key variables consumed by the service:

- `GRPC_PORT`: gRPC listener port (default `50058`).
- `DB_*`: MySQL connection settings.
- `REDIS_*`: Optional Redis connection for rate limiting and delivery tracking.
- `SMS_*`: SMS provider configuration (Kavenegar by default).
- `SMTP_*`: SMTP server credentials for email delivery.
  - Local `make dev` / `make dev-up` starts [Mailpit](https://mailpit.axllent.org/) and overrides SMTP to `mailpit:1025`.
  - Inspect captured mail at http://localhost:8025.

## Next Steps
- Implement the repository layer to match the existing notification persistence schema.
- Integrate SMS and Email providers under `internal/service`.
- Add unit tests for handlers and services.

## Email Templates

Notification emails are rendered from Go `html/template` files located in `templates/email/`.  
Each template expects the caller to provide:

- `Subject`: string used for the `<title>` tag and inbox subject.
- `ContentTemplate`: template name to render inside the base layout (e.g. `email/otp/content`).
- Data fields referenced by the specific template (see table below).
- Optional shared fields:  
  - `RecipientName`/`RecipientEmail` depending on the notification  
  - `Assets.LogoURL` to override the default metarang logo  
  - `Footer.Tagline` and `Footer.Links` (array of `{Label, URL}`) to customize footer links

| Template Name | Content Template | Expected Fields (besides `Subject`, `ContentTemplate`) |
| ------------- | ---------------- | ------------------------------------------------------ |
| `email/otp` | `email/otp/content` | `RecipientName`, `UserCode`, `OtpCode`, `ExpirationWindow`, optional `RequestIP`, `RequestedAt`, `PrimaryAction` (URL/Label) |
| `email/password_reset` | `email/password_reset/content` | `RecipientName`, `UserCode`, `ResetURL`, optional `ExpiresIn`, `RequestIP`, `RequestedAt`, `DeclineURL` |
| `email/verify_email` | `email/verify_email/content` | `RecipientEmail`, `VerifyURL`, optional `ExpiresIn`, `SignupDate`, `SignupTime`, `ReRegisterURL` |
| `email/transaction_receipt` | `email/transaction_receipt/content` | `RecipientName`, `UserCode`, `AssetTitle`, `Quantity`, `PaidAmount`, optional `PaidAmountPSC`, `PaymentId`, `TransactionDate`, `TransactionTime`, `LockURL`, `SecurityURL`, `ContactURL`, `FaqURL` |
| `email/feature_purchase` | `email/feature_purchase/content` | `RecipientName`, `OwnerCode`, `FeatureID`, `FeatureArea`, `FeatureApplication`, `FeatureDensity`, `FeatureCoordinates`, `FeatureAddress`, `SellerCode`, `PriceIRR`, `PricePSC`, optional `TransactionId`, `TransactionDate`, `TransactionTime`, `DisputeURL`, `SecurityURL`, `ContactURL`, `FaqURL` |
| `email/buy_request_sent` | `email/buy_request_sent/content` | `BuyerName`, `BuyerCode`, `FeatureID`, `FeatureArea`, `FeatureApplication`, `FeatureDensity`, `FeatureCoordinates`, `FeatureAddress`, `OfferIRR`, `OfferPSC`, optional `RequestId`, `CreatedDate`, `CreatedTime`, `CancelURL`, `SecurityURL`, `ContactURL`, `FaqURL` |
| `email/buy_request_received` | `email/buy_request_received/content` | `OwnerName`, `OwnerCode`, `BuyerCode`, `FeatureID`, `FeatureArea`, `FeatureApplication`, `FeatureDensity`, `FeatureCoordinates`, `FeatureAddress`, `OfferIRR`, `OfferPSC`, optional `RequestId`, `CreatedDate`, `CreatedTime`, `ManageURL`, `SecurityURL`, `ContactURL`, `FaqURL` |
| `email/sell_feature` | `email/sell_feature/content` | `SellerName`, `SellerCode`, `BuyerCode`, `FeatureID`, `FeatureArea`, `FeatureApplication`, `PriceIRR`, `PricePSC`, optional `TransactionId`, `TransactionDate`, `TransactionTime`, `DisputeURL`, `SecurityURL`, `ContactURL` |
| `email/sell_request` | `email/sell_request/content` | `SellerName`, `SellerCode`, `RequesterCode`, `FeatureID`, `FeatureTitle`, optional `OfferIRR`, `OfferPSC`, `CreatedDate`, `CreatedTime`, `ManageURL`, `DeclineURL`, `ContactURL`, `SecurityURL` |
| `email/login_alert` | `email/login_alert/content` | `RecipientName`, `UserCode`, optional `LoginDate`, `LoginTime`, `IPAddress`, `UserAgent`, `Location`, `SecurityURL`, `SupportURL` |
| `email/dynasty/join_request_sent` | `email/dynasty/join_request_sent/content` | `RecipientName`/`RequesterName`, `ReceiverName`, `ReceiverCode`, `DynastyName`, `DynastyCode`, optional `Role`, `Message`, `SubmittedDate`, `SubmittedTime`, `ManageURL`, `CancelURL` |
| `email/dynasty/join_request_received` | `email/dynasty/join_request_received/content` | `RecipientName`/`OwnerName`, `DynastyName`, `RequesterName`, `RequesterCode`, optional `Role`, `Message`, `SubmittedDate`, `SubmittedTime`, `ManageURL`, `SecurityURL` |
| `email/dynasty/join_request_accepted` | `email/dynasty/join_request_accepted/content` | `side` (`requester`/`receiver`), `RecipientName`, `DynastyName`, `DynastyCode`, optional `Role`, `AcceptedBy`, `AcceptedAt`, `DashboardURL`, `GuidelineURL` |
| `email/dynasty/join_request_rejected` | `email/dynasty/join_request_rejected/content` | `side` (`requester`/`receiver`), `RecipientName`, `DynastyName`, `DynastyCode`, optional `RejectionReason`, `RejectedAt`, `ExploreURL`, `ProfileURL` |

To render a template:

```go
tmpl := template.Must(template.ParseFS(templatesFS, "email/base.html.tmpl", "email/*.html.tmpl", "email/dynasty/*.html.tmpl"))
data := map[string]any{
    "Subject":         "کد تأیید ورود",
    "ContentTemplate": "email/otp/content",
    "RecipientName":   "Ali",
    "UserCode":        "RGB-1024",
    "OtpCode":         "482193",
}
tmpl.ExecuteTemplate(w, "email/otp", data)
```


