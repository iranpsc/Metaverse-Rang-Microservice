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

func StartHTTPServer(handlers HTTPServerHandlers, port string, auth func(http.Handler) http.Handler, optionalAuth func(http.Handler) http.Handler) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"}, true)
	})
	mux.Handle("GET /api/features", optionalAuth(http.HandlerFunc(handlers.Features.ListFeatures)))
	mux.Handle("GET /api/features/buildings/completed", optionalAuth(http.HandlerFunc(handlers.Features.ListCompletedBuildings)))
	mux.Handle("GET /api/features/{feature}/trade-history", http.HandlerFunc(handlers.Features.TradeHistory))
	mux.Handle("/api/features/", optionalAuth(http.HandlerFunc(handlers.Features.HandleFeaturesRoutes)))
	mux.Handle("/api/my-features", auth(http.HandlerFunc(handlers.Features.ListMyFeatures)))
	mux.Handle("/api/my-features/", auth(http.HandlerFunc(handlers.Features.HandleMyFeaturesRoutes)))
	mux.Handle("/api/buy-requests", auth(http.HandlerFunc(handlers.Features.HandleBuyRequestsRoutes)))
	mux.Handle("/api/buy-requests/", auth(http.HandlerFunc(handlers.Features.HandleBuyRequestsRoutes)))
	mux.Handle("/api/sell-requests", auth(http.HandlerFunc(handlers.Features.HandleSellRequestsRoutes)))
	mux.Handle("/api/sell-requests/", auth(http.HandlerFunc(handlers.Features.HandleSellRequestsRoutes)))
	mux.Handle("/api/hourly-profits", auth(http.HandlerFunc(handlers.Profit.Handle)))
	mux.Handle("/api/hourly-profits/", auth(http.HandlerFunc(handlers.Profit.Handle)))
	mux.Handle("/api/isic-codes", optionalAuth(http.HandlerFunc(handlers.Isic.List)))
	mux.Handle("/api/maps", optionalAuth(http.HandlerFunc(handlers.Maps.Handle)))
	mux.Handle("/api/maps/", optionalAuth(http.HandlerFunc(handlers.Maps.Handle)))
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
	return (&http.Server{Addr: ":" + port, Handler: sentry.HTTPMiddleware(mux)}).ListenAndServe()
}
