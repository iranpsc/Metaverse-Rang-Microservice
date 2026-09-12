package handler

import (
	"net/http"
	"strings"

	"metarang/shared/pkg/sentry"
)

// HTTPServerHandlers groups the local RPC wrappers used by the public server.
type HTTPServerHandlers struct {
	Features         *HTTPFeaturesHandler
	Profit           *HTTPProfitHandler
	Maps             *HTTPMapsHandler
	Isic             *HTTPIsicCodesHandler
	CitizenFeatures  *HTTPCitizenFeaturesHandler
	CitizenBuildings *HTTPCitizenBuildingsHandler
}

// corsPreflightMiddleware answers OPTIONS when Kong proxies them (defense in depth).
// ACAO stays Kong-owned; this only supplies Allow-Methods/Headers so PATCH preflights succeed.
func corsPreflightMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Accept-Language, Content-Language, Content-Type, Authorization, X-Requested-With, Origin, X-Request-Id, X-CSRF-TOKEN, X-XSRF-TOKEN, X-Locale, sentry-trace, baggage, traceparent, tracestate")
			w.Header().Set("Access-Control-Max-Age", "60")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func passthroughHTTP(next http.Handler) http.Handler { return next }

func composeHTTP(middlewares ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		h := next
		for i := len(middlewares) - 1; i >= 0; i-- {
			if middlewares[i] == nil {
				continue
			}
			h = middlewares[i](h)
		}
		return h
	}
}

func newPublicHTTPHandler(
	handlers HTTPServerHandlers,
	auth func(http.Handler) http.Handler,
	optionalAuth func(http.Handler) http.Handler,
	accountSecurity func(http.Handler) http.Handler,
) http.Handler {
	if auth == nil {
		auth = passthroughHTTP
	}
	if optionalAuth == nil {
		optionalAuth = passthroughHTTP
	}
	if accountSecurity == nil {
		accountSecurity = passthroughHTTP
	}
	// Mutating authenticated routes require auth + unlocked account security.
	secureAuth := composeHTTP(auth, accountSecurity)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"}, true)
	})
	mux.Handle("GET /api/features", optionalAuth(http.HandlerFunc(handlers.Features.ListFeatures)))
	mux.Handle("GET /api/features/buildings/completed", optionalAuth(http.HandlerFunc(handlers.Features.ListCompletedBuildings)))
	mux.Handle("GET /api/features/{feature}/trade-history", http.HandlerFunc(handlers.Features.TradeHistory))
	mux.Handle("/api/features/", optionalAuth(accountSecurity(http.HandlerFunc(handlers.Features.HandleFeaturesRoutes))))
	mux.Handle("/api/my-features", secureAuth(http.HandlerFunc(handlers.Features.ListMyFeatures)))
	mux.Handle("/api/my-features/", secureAuth(http.HandlerFunc(handlers.Features.HandleMyFeaturesRoutes)))
	mux.Handle("/api/buy-requests", secureAuth(http.HandlerFunc(handlers.Features.HandleBuyRequestsRoutes)))
	mux.Handle("/api/buy-requests/", secureAuth(http.HandlerFunc(handlers.Features.HandleBuyRequestsRoutes)))
	mux.Handle("/api/sell-requests", secureAuth(http.HandlerFunc(handlers.Features.HandleSellRequestsRoutes)))
	mux.Handle("/api/sell-requests/", secureAuth(http.HandlerFunc(handlers.Features.HandleSellRequestsRoutes)))
	mux.Handle("/api/hourly-profits", secureAuth(http.HandlerFunc(handlers.Profit.Handle)))
	mux.Handle("/api/hourly-profits/", secureAuth(http.HandlerFunc(handlers.Profit.Handle)))
	mux.Handle("/api/isic-codes", optionalAuth(accountSecurity(http.HandlerFunc(handlers.Isic.List))))
	mux.Handle("/api/maps", optionalAuth(accountSecurity(http.HandlerFunc(handlers.Maps.Handle))))
	mux.Handle("/api/maps/", optionalAuth(accountSecurity(http.HandlerFunc(handlers.Maps.Handle))))
	mux.Handle("/api/citizen/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/citizen/"), "/"), "/")
		if len(parts) < 2 || parts[0] == "" {
			http.NotFound(w, r)
			return
		}
		switch parts[1] {
		case "features":
			handlers.CitizenFeatures.Handle(w, r, parts[0], parts[2:])
		case "buildings":
			if handlers.CitizenBuildings == nil {
				http.NotFound(w, r)
				return
			}
			handlers.CitizenBuildings.Handle(w, r, parts[0], parts[2:])
		default:
			http.NotFound(w, r)
		}
	}))
	return corsPreflightMiddleware(sentry.HTTPMiddleware(mux))
}

func StartHTTPServer(
	handlers HTTPServerHandlers,
	port string,
	auth func(http.Handler) http.Handler,
	optionalAuth func(http.Handler) http.Handler,
	accountSecurity func(http.Handler) http.Handler,
) error {
	return (&http.Server{Addr: ":" + port, Handler: newPublicHTTPHandler(handlers, auth, optionalAuth, accountSecurity)}).ListenAndServe()
}
