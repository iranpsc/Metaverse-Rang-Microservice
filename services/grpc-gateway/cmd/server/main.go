package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"strings"

	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"metargb/grpc-gateway/internal/config"
	"metargb/grpc-gateway/internal/handler"
	"metargb/grpc-gateway/internal/middleware"
	pb "metargb/shared/pb/auth"
)

func main() {
	// Load environment variables
	configPaths := []string{
		"config.env",
		"./config.env",
		"../config.env",
		"../../config.env",
		"services/grpc-gateway/config.env",
	}
	var configLoaded bool
	for _, configPath := range configPaths {
		if err := godotenv.Load(configPath); err == nil {
			configLoaded = true
			log.Printf("✅ Loaded config from: %s", configPath)
			break
		}
	}
	if !configLoaded {
		log.Println("⚠️  No config.env found, using environment variables")
	}

	cfg := config.Load()

	// Create gRPC connections
	authConn, err := grpc.NewClient(
		cfg.AuthServiceAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("Failed to connect to auth service: %v", err)
	}
	defer authConn.Close()
	log.Printf("✅ Created auth service client for %s (connection will be established on first RPC call)", cfg.AuthServiceAddr)

	// Create connections to other services (with fallback if not configured)
	var calendarConn, dynastyConn, featuresConn, financialConn, levelsConn, trainingConn, supportConn, notificationConn *grpc.ClientConn

	if cfg.CalendarServiceAddr != "" {
		calendarConn, err = grpc.NewClient(
			cfg.CalendarServiceAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			log.Printf("⚠️  Failed to connect to calendar service: %v", err)
		} else {
			defer calendarConn.Close()
			log.Printf("✅ Connected to calendar service at %s", cfg.CalendarServiceAddr)
		}
	}

	if cfg.DynastyServiceAddr != "" {
		dynastyConn, err = grpc.NewClient(
			cfg.DynastyServiceAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			log.Printf("⚠️  Failed to connect to dynasty service: %v", err)
		} else {
			defer dynastyConn.Close()
			log.Printf("✅ Connected to dynasty service at %s", cfg.DynastyServiceAddr)
		}
	}

	if cfg.FeaturesServiceAddr != "" {
		featuresConn, err = grpc.NewClient(
			cfg.FeaturesServiceAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			log.Printf("⚠️  Failed to connect to features service: %v", err)
		} else {
			defer featuresConn.Close()
			log.Printf("✅ Connected to features service at %s", cfg.FeaturesServiceAddr)
		}
	}

	if cfg.FinancialServiceAddr != "" {
		financialConn, err = grpc.NewClient(
			cfg.FinancialServiceAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			log.Printf("⚠️  Failed to connect to financial service: %v", err)
		} else {
			defer financialConn.Close()
			log.Printf("✅ Connected to financial service at %s", cfg.FinancialServiceAddr)
		}
	}

	if cfg.LevelsServiceAddr != "" {
		levelsConn, err = grpc.NewClient(
			cfg.LevelsServiceAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			log.Printf("⚠️  Failed to connect to levels service: %v", err)
		} else {
			defer levelsConn.Close()
			log.Printf("✅ Connected to levels service at %s", cfg.LevelsServiceAddr)
		}
	}

	if cfg.TrainingServiceAddr != "" {
		trainingConn, err = grpc.NewClient(
			cfg.TrainingServiceAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			log.Printf("⚠️  Failed to connect to training service: %v", err)
			log.Printf("⚠️  Training routes will not be available until service is running")
			trainingConn = nil
		} else {
			defer trainingConn.Close()
			log.Printf("✅ Connected to training service at %s", cfg.TrainingServiceAddr)
		}
	} else {
		log.Printf("⚠️  TRAINING_SERVICE_ADDR not set - training routes will not be available")
	}

	if cfg.SupportServiceAddr != "" {
		supportConn, err = grpc.NewClient(
			cfg.SupportServiceAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			log.Printf("⚠️  Failed to connect to support service: %v", err)
		} else {
			defer supportConn.Close()
			log.Printf("✅ Connected to support service at %s", cfg.SupportServiceAddr)
		}
	}

	if cfg.NotificationServiceAddr != "" {
		notificationConn, err = grpc.NewClient(
			cfg.NotificationServiceAddr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			log.Printf("⚠️  Failed to connect to notification service: %v", err)
		} else {
			defer notificationConn.Close()
			log.Printf("✅ Connected to notification service at %s", cfg.NotificationServiceAddr)
		}
	}

	// Create auth client for middleware
	authClient := pb.NewAuthServiceClient(authConn)

	// Create authentication middleware
	authMiddleware := middleware.AuthMiddleware(authClient)
	optionalAuthMiddleware := middleware.OptionalAuthMiddleware(authClient)
	guestMiddleware := middleware.GuestMiddleware(authClient)

	// Create handlers
	authHandler := handler.NewAuthHandler(authConn, cfg.Locale)

	var calendarHandler *handler.CalendarHandler
	if calendarConn != nil {
		calendarHandler = handler.NewCalendarHandler(calendarConn, authConn)
	}

	var dynastyHandler *handler.DynastyHandler
	if dynastyConn != nil {
		dynastyHandler = handler.NewDynastyHandler(dynastyConn, authConn)
	}

	var featuresHandler *handler.FeaturesHandler
	if featuresConn != nil {
		featuresHandler = handler.NewFeaturesHandler(featuresConn, authConn, cfg.Locale)
	}

	var financialHandler *handler.FinancialHandler
	if financialConn != nil {
		financialHandler = handler.NewFinancialHandler(financialConn, authConn, cfg.Locale)
	}

	var levelsHandler *handler.LevelsHandler
	if levelsConn != nil {
		levelsHandler = handler.NewLevelsHandler(levelsConn, cfg.AppURL)
	}

	var trainingHandler *handler.TrainingHandler
	if trainingConn != nil {
		trainingHandler = handler.NewTrainingHandler(trainingConn, authConn)
		log.Printf("✅ Training handler created")
	} else {
		log.Printf("⚠️  Training handler NOT created - trainingConn is nil (check TRAINING_SERVICE_ADDR config)")
	}

	var supportHandler *handler.SupportHandler
	if supportConn != nil {
		supportHandler = handler.NewSupportHandler(supportConn, authConn)
	}

	var notificationHandler *handler.NotificationHandler
	if notificationConn != nil {
		notificationHandler = handler.NewNotificationHandler(notificationConn, authConn)
	}

	// Create storage handler (HTTP reverse proxy, no gRPC connection needed)
	var storageHandler *handler.StorageHandler
	if cfg.StorageServiceAddr != "" {
		storageHandler = handler.NewStorageHandler(cfg.StorageServiceAddr)
		log.Printf("✅ Created storage handler for %s", cfg.StorageServiceAddr)
	} else {
		log.Printf("⚠️  STORAGE_SERVICE_ADDR not set - upload routes will not be available")
	}

	// Setup HTTP routes
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Guest-only auth routes (only accessible to unauthenticated users)
	mux.Handle("/api/auth/register", guestMiddleware(http.HandlerFunc(authHandler.Register)))
	mux.Handle("/api/auth/redirect", guestMiddleware(http.HandlerFunc(authHandler.Redirect)))

	// Public auth routes (no authentication required)
	mux.HandleFunc("/api/auth/callback", authHandler.Callback)
	mux.HandleFunc("/api/auth/validate", authHandler.ValidateToken)

	// Protected auth routes (authentication required)
	mux.Handle("/api/auth/me", authMiddleware(http.HandlerFunc(authHandler.GetMe)))
	mux.Handle("/api/auth/logout", authMiddleware(http.HandlerFunc(authHandler.Logout)))

	// Account security verification requests are rate limited in auth-service (production only).
	mux.Handle("/api/account/security", authMiddleware(http.HandlerFunc(authHandler.RequestAccountSecurity)))
	mux.Handle("/api/account/security/verify", authMiddleware(http.HandlerFunc(authHandler.VerifyAccountSecurity)))

	// User routes - register /api/users FIRST before any other user routes
	mux.Handle("/api/users", optionalAuthMiddleware(http.HandlerFunc(authHandler.ListUsers)))
	mux.Handle("/api/user", optionalAuthMiddleware(http.HandlerFunc(authHandler.GetUser)))
	mux.Handle("/api/user/profile", authMiddleware(http.HandlerFunc(authHandler.UpdateProfile)))

	// Dynamic /api/users/{user}/... routes
	// Must be registered AFTER /api/users to avoid prefix matching conflicts
	// Use a dedicated handler function similar to HandleCitizenRoutes
	mux.Handle("/api/users/", optionalAuthMiddleware(http.HandlerFunc(authHandler.HandleUsersRoutes)))

	// Citizen routes (public, no authentication required)
	mux.HandleFunc("/api/citizen/", authHandler.HandleCitizenRoutes)

	// Search routes
	mux.Handle("/api/search/users", optionalAuthMiddleware(http.HandlerFunc(authHandler.SearchUsers)))
	mux.Handle("/api/search/features", optionalAuthMiddleware(http.HandlerFunc(authHandler.SearchFeatures)))
	mux.Handle("/api/search/isic-codes", optionalAuthMiddleware(http.HandlerFunc(authHandler.SearchIsicCodes)))

	// Account security routes (already handled above, but keeping for consistency)
	// These are already registered as protected routes above

	// KYC routes - match the pattern used by /api/personal-info which works
	mux.Handle("/api/kyc", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			authHandler.GetKYC(w, r)
		} else if r.Method == http.MethodPut || r.Method == http.MethodPatch {
			authHandler.UpdateKYC(w, r)
		} else {
			http.NotFound(w, r)
		}
	})))
	mux.Handle("/api/kyc/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			authHandler.GetKYC(w, r)
		} else if r.Method == http.MethodPut || r.Method == http.MethodPatch {
			authHandler.UpdateKYC(w, r)
		} else {
			http.NotFound(w, r)
		}
	})))
	// Bank Accounts routes - registered at /api/bank-accounts per documentation
	mux.Handle("/api/bank-accounts", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			authHandler.ListBankAccounts(w, r)
		} else if r.Method == http.MethodPost {
			authHandler.CreateBankAccount(w, r)
		} else {
			http.NotFound(w, r)
		}
	})))
	mux.Handle("/api/bank-accounts/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		id := strings.TrimPrefix(path, "/api/bank-accounts/")
		if id == "" {
			http.NotFound(w, r)
			return
		}
		switch r.Method {
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

	// Personal Info routes
	mux.Handle("/api/personal-info", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			authHandler.GetPersonalInfo(w, r)
		} else if r.Method == http.MethodPut || r.Method == http.MethodPatch {
			authHandler.UpdatePersonalInfo(w, r)
		} else {
			http.NotFound(w, r)
		}
	})))

	// Profile Limitation routes
	// Register route with trailing slash first to handle /api/profile-limitations/{id}
	mux.Handle("/api/profile-limitations/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			authHandler.GetProfileLimitation(w, r)
		case http.MethodPut, http.MethodPatch:
			authHandler.UpdateProfileLimitation(w, r)
		case http.MethodDelete:
			authHandler.DeleteProfileLimitation(w, r)
		default:
			http.NotFound(w, r)
		}
	})))
	// Register exact match route for POST /api/profile-limitations (must be after prefix route)
	mux.Handle("/api/profile-limitations", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			authHandler.CreateProfileLimitation(w, r)
		} else {
			http.NotFound(w, r)
		}
	})))
	mux.Handle("/api/user/profile-limitations", authMiddleware(http.HandlerFunc(authHandler.GetProfileLimitations)))

	// Profile Photo routes
	mux.Handle("/api/profilePhotos", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			authHandler.ListProfilePhotos(w, r)
		} else if r.Method == http.MethodPost {
			authHandler.UploadProfilePhoto(w, r)
		} else {
			http.NotFound(w, r)
		}
	})))
	mux.Handle("/api/profile-photos", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			authHandler.ListProfilePhotos(w, r)
		} else if r.Method == http.MethodPost {
			authHandler.UploadProfilePhoto(w, r)
		} else {
			http.NotFound(w, r)
		}
	})))
	mux.Handle("/api/profilePhotos/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			authHandler.GetProfilePhoto(w, r)
		} else if r.Method == http.MethodDelete {
			authHandler.DeleteProfilePhoto(w, r)
		} else {
			http.NotFound(w, r)
		}
	})))
	mux.Handle("/api/profile-photos/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			authHandler.GetProfilePhoto(w, r)
		} else if r.Method == http.MethodDelete {
			authHandler.DeleteProfilePhoto(w, r)
		} else {
			http.NotFound(w, r)
		}
	})))

	// Settings routes
	mux.Handle("/api/settings", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			authHandler.GetSettings(w, r)
		} else if r.Method == http.MethodPost {
			authHandler.UpdateSettings(w, r)
		} else {
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
		if r.Method == http.MethodPut {
			authHandler.UpdateGeneralSettings(w, r)
		} else {
			log.Printf("⚠️  [DEBUG] /api/general-settings/ called with method %s (expected PUT), path: %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})))
	mux.Handle("/api/privacy", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			authHandler.GetPrivacySettings(w, r)
		} else if r.Method == http.MethodPost {
			authHandler.UpdatePrivacySettings(w, r)
		} else {
			http.NotFound(w, r)
		}
	})))

	// User Events routes
	mux.Handle("/api/events", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			authHandler.ListUserEvents(w, r)
		} else {
			http.NotFound(w, r)
		}
	})))
	mux.Handle("/api/events/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if strings.Contains(path, "/report/response/") {
			authHandler.SendReportResponse(w, r)
		} else if strings.Contains(path, "/report/close/") {
			authHandler.CloseEventReport(w, r)
		} else if strings.Contains(path, "/report/") {
			authHandler.ReportUserEvent(w, r)
		} else {
			authHandler.GetUserEvent(w, r)
		}
	})))

	// Calendar routes
	if calendarHandler != nil {
		mux.Handle("/api/calendar", optionalAuthMiddleware(http.HandlerFunc(calendarHandler.GetEvents)))
		mux.Handle("/api/calendar/", optionalAuthMiddleware(http.HandlerFunc(calendarHandler.GetEvent)))
		mux.Handle("/api/calendar/filter", optionalAuthMiddleware(http.HandlerFunc(calendarHandler.FilterByDateRange)))
		mux.Handle("/api/calendar/latest-version", optionalAuthMiddleware(http.HandlerFunc(calendarHandler.GetLatestVersion)))
		mux.Handle("/api/calendar/events/", authMiddleware(http.HandlerFunc(calendarHandler.AddInteraction)))
	}

	// Dynasty routes
	if dynastyHandler != nil {
		// GET /api/dynasty - Get user's dynasty or available features/intro prizes
		mux.Handle("/api/dynasty", authMiddleware(http.HandlerFunc(dynastyHandler.GetDynasty)))

		// POST /api/dynasty/create/{feature} - Create dynasty with feature
		mux.Handle("/api/dynasty/create/", authMiddleware(http.HandlerFunc(dynastyHandler.CreateDynasty)))

		// POST /api/dynasty/{dynasty}/update/{feature} - Update dynasty feature
		mux.Handle("/api/dynasty/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			// Handle /api/dynasty/{dynasty}/update/{feature}
			if strings.Contains(path, "/update/") {
				if r.Method == http.MethodPost {
					dynastyHandler.UpdateDynastyFeature(w, r)
				} else {
					http.NotFound(w, r)
				}
				return
			}
			// Handle /api/dynasty/{dynasty}/family/{family}
			if strings.Contains(path, "/family/") {
				if r.Method == http.MethodGet {
					dynastyHandler.GetFamily(w, r)
				} else {
					http.NotFound(w, r)
				}
				return
			}
			http.NotFound(w, r)
		})))

		// GET /api/dynasty/requests/sent - List sent join requests
		mux.Handle("/api/dynasty/requests/sent", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				dynastyHandler.GetSentRequests(w, r)
			} else {
				http.NotFound(w, r)
			}
		})))
		// GET /api/dynasty/requests/sent/{joinRequest} - View sent request
		// DELETE /api/dynasty/requests/sent/{joinRequest} - Delete sent request
		mux.Handle("/api/dynasty/requests/sent/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				dynastyHandler.GetSentRequest(w, r)
			} else if r.Method == http.MethodDelete {
				dynastyHandler.DeleteJoinRequest(w, r)
			} else {
				http.NotFound(w, r)
			}
		})))

		// GET /api/dynasty/requests/recieved - List received join requests
		mux.Handle("/api/dynasty/requests/recieved", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				dynastyHandler.GetReceivedRequests(w, r)
			} else {
				http.NotFound(w, r)
			}
		})))
		// GET /api/dynasty/requests/recieved/{joinRequest} - View received request
		// POST /api/dynasty/requests/recieved/{joinRequest} - Accept request
		// DELETE /api/dynasty/requests/recieved/{joinRequest} - Reject request
		mux.Handle("/api/dynasty/requests/recieved/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				dynastyHandler.GetReceivedRequest(w, r)
			} else if r.Method == http.MethodPost {
				dynastyHandler.AcceptJoinRequest(w, r)
			} else if r.Method == http.MethodDelete {
				dynastyHandler.RejectJoinRequest(w, r)
			} else {
				http.NotFound(w, r)
			}
		})))

		// POST /api/dynasty/add/member/get/permissions - Get default permissions
		mux.Handle("/api/dynasty/add/member/get/permissions", authMiddleware(http.HandlerFunc(dynastyHandler.GetDefaultPermissions)))

		// POST /api/dynasty/add/member - Send join request
		mux.Handle("/api/dynasty/add/member", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				dynastyHandler.SendJoinRequest(w, r)
			} else {
				http.NotFound(w, r)
			}
		})))

		// POST /api/dynasty/search - Search users
		mux.Handle("/api/dynasty/search", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				dynastyHandler.SearchUsers(w, r)
			} else {
				http.NotFound(w, r)
			}
		})))

		// GET /api/dynasty/prizes - List prizes
		// GET /api/dynasty/prizes/{recievedPrize} - View prize
		// POST /api/dynasty/prizes/{recievedPrize} - Claim prize
		mux.Handle("/api/dynasty/prizes", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				dynastyHandler.GetPrizes(w, r)
			} else {
				http.NotFound(w, r)
			}
		})))
		mux.Handle("/api/dynasty/prizes/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				// View prize handler would go here
				http.NotFound(w, r)
			} else if r.Method == http.MethodPost {
				dynastyHandler.ClaimPrize(w, r)
			} else {
				http.NotFound(w, r)
			}
		})))

		// POST /api/dynasty/children/{user} - Update child permissions
		mux.Handle("/api/dynasty/children/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				dynastyHandler.UpdateChildPermissions(w, r)
			} else {
				http.NotFound(w, r)
			}
		})))
	}

	// Features routes
	if featuresHandler != nil {
		mux.Handle("/api/features", optionalAuthMiddleware(http.HandlerFunc(featuresHandler.ListFeatures)))
		mux.Handle("/api/features/", optionalAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if strings.Contains(path, "/buy/") {
				// Buy feature handler would go here
				http.NotFound(w, r)
			} else if strings.Contains(path, "/build/package") {
				// Build package handler would go here
				http.NotFound(w, r)
			} else if strings.Contains(path, "/build/buildings") {
				// Buildings handler would go here
				http.NotFound(w, r)
			} else {
				featuresHandler.GetFeature(w, r)
			}
		})))
		mux.Handle("/api/features/my-features", authMiddleware(http.HandlerFunc(featuresHandler.ListMyFeatures)))
		mux.Handle("/api/my-features", authMiddleware(http.HandlerFunc(featuresHandler.ListMyFeatures))) // Also handle /api/my-features
		mux.Handle("/api/features/my-features/", authMiddleware(http.HandlerFunc(featuresHandler.GetMyFeature)))
		mux.Handle("/api/my-features/", authMiddleware(http.HandlerFunc(featuresHandler.GetMyFeature))) // Also handle /api/my-features/
		mux.HandleFunc("/api/maps", func(w http.ResponseWriter, r *http.Request) {
			// Maps handler would need to be created separately
			http.NotFound(w, r)
		})
		mux.HandleFunc("/api/v2/maps", func(w http.ResponseWriter, r *http.Request) {
			// Maps handler would need to be created separately
			http.NotFound(w, r)
		})
		mux.HandleFunc("/api/v2/maps/", func(w http.ResponseWriter, r *http.Request) {
			// Maps handler would need to be created separately
			http.NotFound(w, r)
		})
		// Buy/Sell requests routes
		mux.HandleFunc("/api/buy-requests", func(w http.ResponseWriter, r *http.Request) {
			// Buy requests handler would go here
			http.NotFound(w, r)
		})
		mux.HandleFunc("/api/buy-requests/", func(w http.ResponseWriter, r *http.Request) {
			// Buy requests handler would go here
			http.NotFound(w, r)
		})
		mux.HandleFunc("/api/sell-requests", func(w http.ResponseWriter, r *http.Request) {
			// Sell requests handler would go here
			http.NotFound(w, r)
		})
		mux.HandleFunc("/api/sell-requests/", func(w http.ResponseWriter, r *http.Request) {
			// Sell requests handler would go here
			http.NotFound(w, r)
		})
		// Hourly profits routes
		mux.HandleFunc("/api/hourly-profits", func(w http.ResponseWriter, r *http.Request) {
			// Hourly profits handler would go here
			http.NotFound(w, r)
		})
		mux.HandleFunc("/api/hourly-profits/", func(w http.ResponseWriter, r *http.Request) {
			// Hourly profits handler would go here
			http.NotFound(w, r)
		})
	}

	// Financial routes
	if financialHandler != nil {
		mux.Handle("/api/order", authMiddleware(http.HandlerFunc(financialHandler.CreateOrder)))
		mux.Handle("/api/parsian/callback", http.HandlerFunc(financialHandler.HandleCallback)) // Callback doesn't require auth
		mux.Handle("/api/store", optionalAuthMiddleware(http.HandlerFunc(financialHandler.GetStorePackages)))
	}

	// Levels routes - using router function to handle all nested routes
	if levelsHandler != nil {
		// Register exact match for list endpoint
		mux.Handle("/api/levels", http.HandlerFunc(levelsHandler.GetAllLevels)) // Public
		// Register catch-all router for all other routes (nested paths)
		mux.Handle("/api/levels/", http.HandlerFunc(levelsHandler.HandleLevelsRoutes)) // Public
		// v2 routes - for backward compatibility
		mux.Handle("/api/v2/levels", http.HandlerFunc(levelsHandler.GetAllLevels)) // Public
		mux.Handle("/api/v2/levels/", http.HandlerFunc(levelsHandler.HandleLevelsRoutes)) // Public
	}

	// Training routes
	if trainingHandler != nil {
		log.Printf("✅ Registering training service routes...")

		// Register more specific routes FIRST (before catch-all routes)

		// V1 modal lookup route (completely separate path) - requires auth
		mux.Handle("/api/video-tutorials", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				trainingHandler.GetVideoByFileName(w, r)
			} else {
				http.NotFound(w, r)
			}
		})))
		log.Printf("  ✅ Registered: POST /api/video-tutorials")

		// Category routes (must be before /api/tutorials/ catch-all) - public viewing
		mux.Handle("/api/tutorials/categories", optionalAuthMiddleware(http.HandlerFunc(trainingHandler.GetCategories)))
		mux.Handle("/api/tutorials/categories/", optionalAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			// Remove prefix to get the category path
			categoryPath := strings.TrimPrefix(path, "/api/tutorials/categories/")
			parts := strings.Split(categoryPath, "/")

			if len(parts) == 1 && parts[0] != "" {
				// /api/tutorials/categories/{category:slug}
				if r.Method == http.MethodGet {
					trainingHandler.GetCategory(w, r)
				} else {
					http.NotFound(w, r)
				}
			} else if len(parts) == 2 && parts[1] == "videos" {
				// /api/tutorials/categories/{category:slug}/videos
				if r.Method == http.MethodGet {
					trainingHandler.GetCategoryVideos(w, r)
				} else {
					http.NotFound(w, r)
				}
			} else if len(parts) == 2 && parts[1] != "" {
				// /api/tutorials/categories/{category:slug}/{subCategory:slug}
				if r.Method == http.MethodGet {
					trainingHandler.GetSubCategory(w, r)
				} else {
					http.NotFound(w, r)
				}
			} else {
				http.NotFound(w, r)
			}
		})))

		// Search route (must be before /api/tutorials/ catch-all) - public
		mux.Handle("/api/tutorials/search", optionalAuthMiddleware(http.HandlerFunc(trainingHandler.SearchVideos)))

		// Dynamic video routes - catch-all for /api/tutorials/{...}
		// This must be registered AFTER more specific routes like /api/tutorials/categories
		// But BEFORE /api/tutorials to handle /api/tutorials/ properly
		// Uses conditional middleware: authMiddleware for authenticated routes, optionalAuthMiddleware for others
		mux.Handle("/api/tutorials/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			videoPath := strings.TrimPrefix(path, "/api/tutorials/")
			videoPath = strings.Trim(videoPath, "/")
			parts := strings.Split(videoPath, "/")

			// Check if this is an authenticated route that requires authMiddleware
			isAuthenticatedRoute := len(parts) == 2 && parts[1] == "interactions" && r.Method == http.MethodPost

			// Apply appropriate middleware based on route
			if isAuthenticatedRoute {
				authMiddleware(http.HandlerFunc(trainingHandler.AddInteraction)).ServeHTTP(w, r)
				return
			}

			// For all other routes, use optionalAuthMiddleware
			optionalAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// If empty, this is exactly /api/tutorials/ - handle as list
				if videoPath == "" {
					if r.Method == http.MethodGet {
						trainingHandler.GetVideos(w, r)
						return
					} else {
						http.NotFound(w, r)
						return
					}
				}

			// Check for comment routes: /api/tutorials/{video}/comments/...
			if len(parts) >= 2 && parts[1] == "comments" {
					if len(parts) >= 4 {
						// /api/tutorials/{video}/comments/{comment}/{action}
						action := parts[3]
						switch action {
						case "interactions":
							if r.Method == http.MethodPost {
								trainingHandler.AddCommentInteraction(w, r)
							} else {
								http.NotFound(w, r)
							}
						case "report":
							if r.Method == http.MethodPost {
								trainingHandler.ReportComment(w, r)
							} else {
								http.NotFound(w, r)
							}
						default:
							// Update or delete: /api/tutorials/{video}/comments/{comment}
							if r.Method == http.MethodPut {
								trainingHandler.UpdateComment(w, r)
							} else if r.Method == http.MethodDelete {
								trainingHandler.DeleteComment(w, r)
							} else {
								http.NotFound(w, r)
							}
						}
					} else if len(parts) == 2 {
						// /api/tutorials/{video}/comments
						if r.Method == http.MethodGet {
							trainingHandler.GetComments(w, r)
						} else if r.Method == http.MethodPost {
							trainingHandler.AddComment(w, r)
						} else {
							http.NotFound(w, r)
						}
					} else {
						http.NotFound(w, r)
					}
				} else if len(parts) == 1 {
					// /api/tutorials/{slug} - Get video by slug
					if r.Method == http.MethodGet {
						trainingHandler.GetVideo(w, r)
					} else {
						http.NotFound(w, r)
					}
				} else {
					// Unmatched path
					http.NotFound(w, r)
				}
			})).ServeHTTP(w, r)
		}))

		// Video tutorials list route - exact match (no trailing slash)
		// This is registered AFTER /api/tutorials/ to handle exact /api/tutorials
		// Public viewing route
		mux.Handle("/api/tutorials", optionalAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodGet {
				trainingHandler.GetVideos(w, r)
			} else {
				http.NotFound(w, r)
			}
		})))

		// Comment replies routes: /api/comments/{comment}/...
		// Uses optionalAuthMiddleware - handlers will enforce auth for actions
		mux.Handle("/api/comments/", optionalAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			// Remove prefix to get the comment path
			commentPath := strings.TrimPrefix(path, "/api/comments/")
			parts := strings.Split(commentPath, "/")

			if len(parts) >= 2 && parts[1] == "replies" {
				if len(parts) == 2 {
					// /api/comments/{comment}/replies
					if r.Method == http.MethodGet {
						trainingHandler.GetReplies(w, r)
					} else {
						http.NotFound(w, r)
					}
				} else if len(parts) == 3 {
					// /api/comments/{comment}/replies/{reply}
					if r.Method == http.MethodPut {
						trainingHandler.UpdateReply(w, r)
					} else if r.Method == http.MethodDelete {
						trainingHandler.DeleteReply(w, r)
					} else {
						http.NotFound(w, r)
					}
				} else if len(parts) == 4 && parts[3] == "interactions" {
					// /api/comments/{comment}/replies/{reply}/interactions
					if r.Method == http.MethodPost {
						trainingHandler.AddReplyInteraction(w, r)
					} else {
						http.NotFound(w, r)
					}
				} else {
					http.NotFound(w, r)
				}
			} else if len(parts) == 1 && strings.HasSuffix(path, "/reply") {
				// /api/comments/{comment}/reply
				if r.Method == http.MethodPost {
					trainingHandler.AddReply(w, r)
				} else {
					http.NotFound(w, r)
				}
			} else {
				http.NotFound(w, r)
			}
		})))
		log.Printf("✅ All training service routes registered successfully")
	} else {
		log.Printf("⚠️  Training routes NOT registered - trainingHandler is nil")
		log.Printf("   Check if TRAINING_SERVICE_ADDR is set and training service is running")
		log.Printf("   Current TRAINING_SERVICE_ADDR: %s", cfg.TrainingServiceAddr)
		log.Printf("   trainingConn value: %v", trainingConn)
	}

	// Support routes
	if supportHandler != nil {
		// Support service routes (with /support prefix)
		mux.Handle("/api/support/tickets", authMiddleware(http.HandlerFunc(supportHandler.ListTickets)))
		mux.Handle("/api/support/tickets/create", authMiddleware(http.HandlerFunc(supportHandler.CreateTicket)))
		mux.Handle("/api/support/tickets/", authMiddleware(http.HandlerFunc(supportHandler.GetTicket)))
		mux.Handle("/api/support/reports", authMiddleware(http.HandlerFunc(supportHandler.ListReports)))
		mux.Handle("/api/support/reports/create", authMiddleware(http.HandlerFunc(supportHandler.CreateReport)))
		mux.Handle("/api/support/reports/", authMiddleware(http.HandlerFunc(supportHandler.GetReport)))
		// Direct routes (without /support prefix) - for Kong compatibility
		mux.Handle("/api/tickets", authMiddleware(http.HandlerFunc(supportHandler.ListTickets)))
		mux.Handle("/api/tickets/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if strings.Contains(path, "/response/") {
				// Response handler would go here
				http.NotFound(w, r)
			} else if strings.Contains(path, "/close/") {
				// Close handler would go here
				http.NotFound(w, r)
			} else {
				supportHandler.GetTicket(w, r)
			}
		})))
		mux.Handle("/api/reports", authMiddleware(http.HandlerFunc(supportHandler.ListReports)))
		mux.Handle("/api/reports/", authMiddleware(http.HandlerFunc(supportHandler.GetReport)))
		mux.Handle("/api/notes", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Notes handler would go here
			http.NotFound(w, r)
		})))
		mux.Handle("/api/notes/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Notes handler would go here
			http.NotFound(w, r)
		})))
	}

	// Notification routes
	if notificationHandler != nil {
		mux.Handle("/api/notifications", authMiddleware(http.HandlerFunc(notificationHandler.GetNotifications)))
		mux.Handle("/api/notifications/", authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if strings.Contains(path, "/read/") && !strings.Contains(path, "/read/all") {
				notificationHandler.MarkAsRead(w, r)
			} else {
				notificationHandler.GetNotification(w, r)
			}
		})))
		mux.Handle("/api/notifications/mark-read", authMiddleware(http.HandlerFunc(notificationHandler.MarkAsRead)))
		mux.Handle("/api/notifications/read/all", authMiddleware(http.HandlerFunc(notificationHandler.MarkAllAsRead)))
		mux.Handle("/api/notifications/mark-all-read", authMiddleware(http.HandlerFunc(notificationHandler.MarkAllAsRead)))
	}

	// Storage routes (public endpoint, no authentication required)
	if storageHandler != nil {
		mux.HandleFunc("/api/upload", storageHandler.HandleUpload)
		log.Printf("✅ Registered storage upload route: /api/upload")
	}

	// Note: We don't register a catch-all "/" handler because it would interfere with route matching
	// Instead, unmatched routes will naturally return 404 from ServeMux

	// Chain middleware: logging -> CORS -> mux
	handler := middleware.LoggingMiddleware(middleware.CORSMiddleware(mux))

	// Start HTTP server
	server := &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: handler,
	}

	// Graceful shutdown
	go func() {
		log.Printf("🚀 gRPC Gateway starting on port %s", cfg.HTTPPort)
		log.Printf("🏥 Health check: http://localhost:%s/health", cfg.HTTPPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
