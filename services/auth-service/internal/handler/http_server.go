package handler

import (
	"net/http"
	"strings"

	"metarang/shared/pkg/sentry"
)

// HTTPServerHandlers groups Kong-facing HTTP handlers for auth-service.
type HTTPServerHandlers struct {
	Auth   *HTTPAuthHandler
	Wallet *HTTPWalletHandler
}

// StartHTTPServer starts the public HTTP server (behind Kong).
func StartHTTPServer(
	handlers HTTPServerHandlers,
	port string,
	authMiddleware func(http.Handler) http.Handler,
	optionalAuthMiddleware func(http.Handler) http.Handler,
	guestMiddleware func(http.Handler) http.Handler,
) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"}, true)
	})

	authHandler := handlers.Auth
	walletHandler := handlers.Wallet

	mux.Handle("/api/auth/register", guestMiddleware(http.HandlerFunc(authHandler.Register)))
	mux.Handle("/api/auth/redirect", guestMiddleware(http.HandlerFunc(authHandler.Redirect)))
	mux.HandleFunc("/api/auth/callback", authHandler.Callback)
	mux.HandleFunc("/api/auth/validate", authHandler.ValidateToken)
	mux.Handle("/api/auth/me", authMiddleware(http.HandlerFunc(authHandler.GetMe)))
	mux.Handle("/api/auth/logout", authMiddleware(http.HandlerFunc(authHandler.Logout)))
	mux.Handle("/api/account/security", authMiddleware(http.HandlerFunc(authHandler.RequestAccountSecurity)))
	mux.Handle("/api/account/security/verify", authMiddleware(http.HandlerFunc(authHandler.VerifyAccountSecurity)))
	mux.Handle("/api/mobile/send", authMiddleware(http.HandlerFunc(authHandler.SendMobileChangeCode)))
	mux.Handle("/api/mobile/verify", authMiddleware(http.HandlerFunc(authHandler.VerifyMobileChange)))

	mux.Handle("/api/wallet/link/nonce", authMiddleware(http.HandlerFunc(walletHandler.GetLinkNonce)))
	mux.Handle("/api/wallet/link", authMiddleware(http.HandlerFunc(walletHandler.LinkWallet)))
	mux.Handle("/api/wallet/security/nonce", authMiddleware(http.HandlerFunc(walletHandler.GetSecurityNonce)))
	mux.Handle("/api/wallet/security/verify", authMiddleware(http.HandlerFunc(walletHandler.VerifySecuritySignature)))
	mux.HandleFunc("POST /api/wallets/registered", walletHandler.CheckRegistered)

	mux.Handle("/api/users", optionalAuthMiddleware(http.HandlerFunc(authHandler.ListUsers)))
	mux.Handle("/api/user", authMiddleware(http.HandlerFunc(authHandler.GetUser)))
	mux.Handle("/api/user/wallet", authMiddleware(http.HandlerFunc(authHandler.GetAuthenticatedUserWallet)))
	mux.Handle("/api/user/profile", authMiddleware(http.HandlerFunc(authHandler.UpdateProfile)))
	mux.HandleFunc("GET /api/user/{code}/level", authHandler.GetUserLevelByCode)
	mux.Handle("GET /api/users/{user}/profile-limitations", authMiddleware(http.HandlerFunc(authHandler.GetProfileLimitations)))
	mux.Handle("/api/users/", optionalAuthMiddleware(http.HandlerFunc(authHandler.HandleUsersRoutes)))

	mux.HandleFunc("/api/citizen/", authHandler.HandleCitizenRoutes)

	mux.Handle("/api/search/users", optionalAuthMiddleware(http.HandlerFunc(authHandler.SearchUsers)))
	mux.Handle("/api/search/features", optionalAuthMiddleware(http.HandlerFunc(authHandler.SearchFeatures)))
	mux.Handle("/api/search/isic-codes", optionalAuthMiddleware(http.HandlerFunc(authHandler.SearchIsicCodes)))

	mux.Handle("/api/kyc", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch EffectiveHTTPMethod(r) {
		case http.MethodGet:
			authHandler.GetKYC(w, r)
		case http.MethodPut, http.MethodPatch:
			authHandler.UpdateKYC(w, r)
		default:
			http.NotFound(w, r)
		}
	})))
	mux.Handle("/api/kyc/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch EffectiveHTTPMethod(r) {
		case http.MethodGet:
			authHandler.GetKYC(w, r)
		case http.MethodPut, http.MethodPatch:
			authHandler.UpdateKYC(w, r)
		default:
			http.NotFound(w, r)
		}
	})))

	bankAccountsVerified := func(next http.Handler) http.Handler {
		return authMiddleware(authHandler.RequireVerifiedEmail(next))
	}
	mux.Handle("/api/bank-accounts", bankAccountsVerified(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authHandler.ListBankAccounts(w, r)
		case http.MethodPost:
			authHandler.CreateBankAccount(w, r)
		default:
			http.NotFound(w, r)
		}
	})))
	mux.Handle("/api/bank-accounts/", bankAccountsVerified(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/bank-accounts/")
		if id == "" {
			http.NotFound(w, r)
			return
		}
		switch EffectiveHTTPMethod(r) {
		case http.MethodGet:
			authHandler.GetBankAccount(w, r)
		case http.MethodPut, http.MethodPatch:
			authHandler.UpdateBankAccount(w, r)
		case http.MethodDelete:
			authHandler.DeleteBankAccount(w, r)
		default:
			http.NotFound(w, r)
		}
	})))

	personalInfoHandler := authMiddleware(PersonalInfoRoutes(authHandler))
	mux.Handle("/api/personal-info", personalInfoHandler)
	mux.Handle("/api/personal-info/", personalInfoHandler)

	mux.Handle("/api/profile-limitations/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut, http.MethodPatch:
			authHandler.UpdateProfileLimitation(w, r)
		case http.MethodDelete:
			authHandler.DeleteProfileLimitation(w, r)
		default:
			http.NotFound(w, r)
		}
	})))
	mux.Handle("/api/profile-limitations", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			authHandler.CreateProfileLimitation(w, r)
		} else {
			http.NotFound(w, r)
		}
	})))

	registerProfilePhotoRoutes := func(prefix string) {
		mux.Handle(prefix, authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(prefix, "/") {
				switch r.Method {
				case http.MethodGet:
					authHandler.GetProfilePhoto(w, r)
				case http.MethodDelete:
					authHandler.DeleteProfilePhoto(w, r)
				default:
					http.NotFound(w, r)
				}
				return
			}
			switch r.Method {
			case http.MethodGet:
				authHandler.ListProfilePhotos(w, r)
			case http.MethodPost:
				authHandler.UploadProfilePhoto(w, r)
			default:
				http.NotFound(w, r)
			}
		})))
	}
	registerProfilePhotoRoutes("/api/profilePhotos")
	registerProfilePhotoRoutes("/api/profile-photos")
	registerProfilePhotoRoutes("/api/profilePhotos/")
	registerProfilePhotoRoutes("/api/profile-photos/")

	mux.Handle("/api/settings", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authHandler.GetSettings(w, r)
		case http.MethodPost:
			authHandler.UpdateSettings(w, r)
		default:
			http.NotFound(w, r)
		}
	})))
	mux.Handle("/api/general-settings", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			authHandler.GetGeneralSettings(w, r)
		} else {
			http.NotFound(w, r)
		}
	})))
	mux.Handle("/api/general-settings/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if EffectiveHTTPMethod(r) == http.MethodPut {
			authHandler.UpdateGeneralSettings(w, r)
		} else {
			http.NotFound(w, r)
		}
	})))
	mux.Handle("/api/privacy", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authHandler.GetPrivacySettings(w, r)
		case http.MethodPost:
			authHandler.UpdatePrivacySettings(w, r)
		default:
			http.NotFound(w, r)
		}
	})))

	mux.Handle("/api/events", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			authHandler.ListUserEvents(w, r)
		} else {
			http.NotFound(w, r)
		}
	})))
	mux.Handle("/api/events/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.Contains(path, "/report/response/"):
			authHandler.SendReportResponse(w, r)
		case strings.Contains(path, "/report/close/"):
			authHandler.CloseEventReport(w, r)
		case strings.Contains(path, "/report/"):
			authHandler.ReportUserEvent(w, r)
		default:
			authHandler.GetUserEvent(w, r)
		}
	})))

	server := &http.Server{
		Addr:    ":" + port,
		Handler: sentry.HTTPMiddleware(mux),
	}
	return server.ListenAndServe()
}
