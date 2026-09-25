package handler

import (
	"net/http"
	"strings"

	"metarang/shared/pkg/sentry"
)

// HTTPServerHandlers groups the local RPC wrappers used by the public server.
type HTTPServerHandlers struct {
	Buildings        *HTTPBuildingsHandler
	CitizenBuildings *HTTPCitizenBuildingsHandler
}

// CorsPreflightMiddleware answers OPTIONS when Kong proxies them (defense in depth).
func CorsPreflightMiddleware(next http.Handler) http.Handler {
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

// NewPublicHTTPHandler builds the Kong-facing HTTP mux for buildings-service.
func NewPublicHTTPHandler(
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
	secureAuth := composeHTTP(auth, accountSecurity)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"}, true)
	})

	// Completed buildings (optional auth)
	mux.Handle("GET /api/features/buildings/completed", optionalAuth(http.HandlerFunc(handlers.Buildings.ListCompletedBuildings)))

	// Build package / build / buildings under /api/features/{id}/build...
	// Same middleware pattern as features-service: optional auth + account security;
	// handlers enforce auth where required.
	mux.Handle("/api/features/", optionalAuth(accountSecurity(http.HandlerFunc(handlers.Buildings.HandleFeaturesBuildRoutes))))

	_ = secureAuth // reserved for future routes that always require auth at mux level

	mux.Handle("/api/citizen/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/citizen/"), "/"), "/")
		if len(parts) < 2 || parts[0] == "" || parts[1] != "buildings" {
			http.NotFound(w, r)
			return
		}
		if handlers.CitizenBuildings == nil {
			http.NotFound(w, r)
			return
		}
		handlers.CitizenBuildings.Handle(w, r, parts[0], parts[2:])
	}))
	return CorsPreflightMiddleware(sentry.HTTPMiddleware(mux))
}

func StartHTTPServer(
	handlers HTTPServerHandlers,
	port string,
	auth func(http.Handler) http.Handler,
	optionalAuth func(http.Handler) http.Handler,
	accountSecurity func(http.Handler) http.Handler,
) error {
	return (&http.Server{
		Addr:    ":" + port,
		Handler: NewPublicHTTPHandler(handlers, auth, optionalAuth, accountSecurity),
	}).ListenAndServe()
}
