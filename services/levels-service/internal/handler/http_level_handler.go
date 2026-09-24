package handler

import (
	"context"
	"net/http"
	"strings"

	levelspb "metarang/shared/pb/levels"
	"metarang/shared/pkg/sentry"
)

// levelAPI is the subset of level RPCs used by the HTTP layer.
type levelAPI interface {
	GetAllLevels(context.Context, *levelspb.GetAllLevelsRequest) (*levelspb.LevelsResponse, error)
	GetLevel(context.Context, *levelspb.GetLevelRequest) (*levelspb.LevelResponse, error)
	GetLevelGeneralInfo(context.Context, *levelspb.GetLevelGeneralInfoRequest) (*levelspb.LevelGeneralInfoResponse, error)
	GetLevelGem(context.Context, *levelspb.GetLevelGemRequest) (*levelspb.LevelGemResponse, error)
	GetLevelGift(context.Context, *levelspb.GetLevelGiftRequest) (*levelspb.LevelGiftResponse, error)
	GetLevelLicenses(context.Context, *levelspb.GetLevelLicensesRequest) (*levelspb.LevelLicensesResponse, error)
	GetLevelPrizes(context.Context, *levelspb.GetLevelPrizesRequest) (*levelspb.LevelPrizesResponse, error)
}

// HTTPLevelHandler serves Kong-facing REST routes for levels-service.
type HTTPLevelHandler struct {
	api    levelAPI
	appURL string
}

// NewHTTPLevelHandler wraps the gRPC level handler for local HTTP use.
func NewHTTPLevelHandler(api levelAPI, appURL string) *HTTPLevelHandler {
	return &HTTPLevelHandler{
		api:    api,
		appURL: strings.TrimSuffix(appURL, "/"),
	}
}

// RegisterHTTPRoutes registers levels REST routes and /health.
func (h *HTTPLevelHandler) RegisterHTTPRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	mux.Handle("/api/levels", http.HandlerFunc(h.GetAllLevels))
	mux.Handle("/api/levels/", http.HandlerFunc(h.HandleLevelsRoutes))
}

// StartHTTPServer starts the public HTTP server (behind Kong).
func StartHTTPServer(httpHandler *HTTPLevelHandler, port string) error {
	mux := http.NewServeMux()
	httpHandler.RegisterHTTPRoutes(mux)

	server := &http.Server{
		Addr:    ":" + port,
		Handler: sentry.HTTPMiddleware(mux),
	}
	return server.ListenAndServe()
}

func (h *HTTPLevelHandler) prefixImageURL(url string) string {
	if url == "" {
		return url
	}
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return url
	}
	if h.appURL == "" {
		path := strings.TrimPrefix(url, "/")
		if !strings.HasPrefix(path, "uploads/") {
			return "/uploads/" + path
		}
		return "/" + path
	}
	path := strings.TrimPrefix(url, "/")
	if !strings.HasPrefix(path, "uploads/") {
		path = "uploads/" + path
	}
	return h.appURL + "/" + path
}

// GetAllLevels handles GET /api/levels
func (h *HTTPLevelHandler) GetAllLevels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	resp, err := h.api.GetAllLevels(r.Context(), &levelspb.GetAllLevelsRequest{})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	levels := make([]map[string]interface{}, 0, len(resp.Levels))
	for _, level := range resp.Levels {
		levelMap := map[string]interface{}{
			"id":   level.Id,
			"name": level.Name,
			"slug": level.Slug,
		}
		if level.ImageUrl != "" {
			levelMap["image"] = h.prefixImageURL(level.ImageUrl)
		}
		if level.BackgroundImage != "" {
			levelMap["background_image"] = h.prefixImageURL(level.BackgroundImage)
		}
		levels = append(levels, levelMap)
	}

	writeJSON(w, http.StatusOK, levels)
}

// GetLevel handles GET /api/levels/{slug}
func (h *HTTPLevelHandler) GetLevel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	slug := extractSlugFromPath(r.URL.Path, "/api/levels/")
	if slug == "" {
		writeError(w, http.StatusBadRequest, "level slug is required")
		return
	}

	resp, err := h.api.GetLevel(r.Context(), &levelspb.GetLevelRequest{LevelSlug: slug})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, h.formatLevelResponse(resp.Level))
}

// GetLevelGeneralInfo handles GET /api/levels/{slug}/general-info
func (h *HTTPLevelHandler) GetLevelGeneralInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	slug := extractSlugFromPath(r.URL.Path, "/api/levels/")
	if slug == "" {
		writeError(w, http.StatusBadRequest, "level slug is required")
		return
	}
	slug = strings.TrimSuffix(slug, "/general-info")

	resp, err := h.api.GetLevelGeneralInfo(r.Context(), &levelspb.GetLevelGeneralInfoRequest{LevelSlug: slug})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	var generalInfo map[string]interface{}
	if resp.GeneralInfo != nil {
		generalInfo = map[string]interface{}{
			"id":             resp.GeneralInfo.Id,
			"score":          resp.GeneralInfo.Score,
			"description":    resp.GeneralInfo.Description,
			"rank":           resp.GeneralInfo.Rank,
			"subcategories":  resp.GeneralInfo.Subcategories,
			"persian_font":   resp.GeneralInfo.PersianFont,
			"english_font":   resp.GeneralInfo.EnglishFont,
			"file_volume":    resp.GeneralInfo.FileVolume,
			"used_colors":    resp.GeneralInfo.UsedColors,
			"points":         resp.GeneralInfo.Points,
			"lines":          resp.GeneralInfo.Lines,
			"has_animation":  resp.GeneralInfo.HasAnimation,
			"designer":       resp.GeneralInfo.Designer,
			"model_designer": resp.GeneralInfo.ModelDesigner,
			"creation_date":  resp.GeneralInfo.CreationDate,
			"png_file":       h.prefixImageURL(resp.GeneralInfo.PngFile),
			"fbx_file":       resp.GeneralInfo.FbxFile,
			"gif_file":       h.prefixImageURL(resp.GeneralInfo.GifFile),
		}
	}

	writeJSON(w, http.StatusOK, generalInfo)
}

// GetLevelGem handles GET /api/levels/{slug}/gem
func (h *HTTPLevelHandler) GetLevelGem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	slug := extractSlugFromPath(r.URL.Path, "/api/levels/")
	if slug == "" {
		writeError(w, http.StatusBadRequest, "level slug is required")
		return
	}
	slug = strings.TrimSuffix(slug, "/gem")

	resp, err := h.api.GetLevelGem(r.Context(), &levelspb.GetLevelGemRequest{LevelSlug: slug})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	var gem map[string]interface{}
	if resp.Gem != nil {
		gem = map[string]interface{}{
			"id":            resp.Gem.Id,
			"level_id":      resp.Gem.LevelId,
			"name":          resp.Gem.Name,
			"description":   resp.Gem.Description,
			"thread":        resp.Gem.Thread,
			"points":        resp.Gem.Points,
			"volume":        resp.Gem.Volume,
			"color":         resp.Gem.Color,
			"has_animation": resp.Gem.HasAnimation,
			"lines":         resp.Gem.Lines,
			"png_file":      h.prefixImageURL(resp.Gem.PngFile),
			"fbx_file":      resp.Gem.FbxFile,
			"encryption":    resp.Gem.Encryption,
			"designer":      resp.Gem.Designer,
		}
	}

	writeJSON(w, http.StatusOK, gem)
}

// GetLevelGift handles GET /api/levels/{slug}/gift
func (h *HTTPLevelHandler) GetLevelGift(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	slug := extractSlugFromPath(r.URL.Path, "/api/levels/")
	if slug == "" {
		writeError(w, http.StatusBadRequest, "level slug is required")
		return
	}
	slug = strings.TrimSuffix(slug, "/gift")

	resp, err := h.api.GetLevelGift(r.Context(), &levelspb.GetLevelGiftRequest{LevelSlug: slug})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	var gift map[string]interface{}
	if resp.Gift != nil {
		gift = map[string]interface{}{
			"id":                        resp.Gift.Id,
			"level_id":                  resp.Gift.LevelId,
			"name":                      resp.Gift.Name,
			"description":               resp.Gift.Description,
			"monthly_capacity_count":    resp.Gift.MonthlyCapacityCount,
			"store_capacity":            resp.Gift.StoreCapacity,
			"sell_capacity":             resp.Gift.SellCapacity,
			"features":                  resp.Gift.Features,
			"sell":                      resp.Gift.Sell,
			"vod_document_registration": resp.Gift.VodDocumentRegistration,
			"seller_link":               resp.Gift.SellerLink,
			"designer":                  resp.Gift.Designer,
			"three_d_model_volume":      resp.Gift.ThreeDModelVolume,
			"three_d_model_points":      resp.Gift.ThreeDModelPoints,
			"three_d_model_lines":       resp.Gift.ThreeDModelLines,
			"has_animation":             resp.Gift.HasAnimation,
			"png_file":                  h.prefixImageURL(resp.Gift.PngFile),
			"fbx_file":                  resp.Gift.FbxFile,
			"gif_file":                  h.prefixImageURL(resp.Gift.GifFile),
			"rent":                      resp.Gift.Rent,
			"vod_count":                 resp.Gift.VodCount,
			"start_vod_id":              resp.Gift.StartVodId,
			"end_vod_id":                resp.Gift.EndVodId,
		}
	}

	writeJSON(w, http.StatusOK, gift)
}

// GetLevelLicenses handles GET /api/levels/{slug}/licenses
func (h *HTTPLevelHandler) GetLevelLicenses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	slug := extractSlugFromPath(r.URL.Path, "/api/levels/")
	if slug == "" {
		writeError(w, http.StatusBadRequest, "level slug is required")
		return
	}
	slug = strings.TrimSuffix(slug, "/licenses")

	resp, err := h.api.GetLevelLicenses(r.Context(), &levelspb.GetLevelLicensesRequest{LevelSlug: slug})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	var licenses map[string]interface{}
	if resp.Licenses != nil {
		licenses = map[string]interface{}{
			"id":                                     resp.Licenses.Id,
			"level_id":                               resp.Licenses.LevelId,
			"create_union":                           resp.Licenses.CreateUnion,
			"add_memeber_to_union":                   resp.Licenses.AddMemeberToUnion,
			"observation_license":                    resp.Licenses.ObservationLicense,
			"gate_license":                           resp.Licenses.GateLicense,
			"lawyer_license":                         resp.Licenses.LawyerLicense,
			"city_counsile_entry":                    resp.Licenses.CityCounsileEntry,
			"establish_special_residential_property": resp.Licenses.EstablishSpecialResidentialProperty,
			"establish_property_on_surface":          resp.Licenses.EstablishPropertyOnSurface,
			"judge_entry":                            resp.Licenses.JudgeEntry,
			"upload_image":                           resp.Licenses.UploadImage,
			"delete_image":                           resp.Licenses.DeleteImage,
			"inter_level_general_points":             resp.Licenses.InterLevelGeneralPoints,
			"inter_level_special_points":             resp.Licenses.InterLevelSpecialPoints,
			"rent_out_satisfaction":                  resp.Licenses.RentOutSatisfaction,
			"access_to_answer_questions_unit":        resp.Licenses.AccessToAnswerQuestionsUnit,
			"create_challenge_questions":             resp.Licenses.CreateChallengeQuestions,
			"upload_music":                           resp.Licenses.UploadMusic,
		}
	}

	writeJSON(w, http.StatusOK, licenses)
}

// GetLevelPrize handles GET /api/levels/{slug}/prize
func (h *HTTPLevelHandler) GetLevelPrize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	slug := extractSlugFromPath(r.URL.Path, "/api/levels/")
	if slug == "" {
		writeError(w, http.StatusBadRequest, "level slug is required")
		return
	}
	slug = strings.TrimSuffix(slug, "/prize")

	resp, err := h.api.GetLevelPrizes(r.Context(), &levelspb.GetLevelPrizesRequest{LevelSlug: slug})
	if err != nil {
		writeHandlerError(w, err)
		return
	}

	var prize map[string]interface{}
	if resp.Prize != nil {
		prize = map[string]interface{}{
			"id":           resp.Prize.Id,
			"level_id":     resp.Prize.LevelId,
			"psc":          resp.Prize.Psc,
			"yellow":       resp.Prize.Yellow,
			"blue":         resp.Prize.Blue,
			"red":          resp.Prize.Red,
			"effect":       resp.Prize.Effect,
			"satisfaction": resp.Prize.Satisfaction,
		}
		if resp.Prize.CreatedAt != "" {
			prize["created_at"] = resp.Prize.CreatedAt
		}
	}

	writeJSON(w, http.StatusOK, prize)
}

func (h *HTTPLevelHandler) formatLevelResponse(level *levelspb.Level) map[string]interface{} {
	if level == nil {
		return nil
	}

	levelMap := map[string]interface{}{
		"id":   level.Id,
		"name": level.Name,
		"slug": level.Slug,
	}
	if level.ImageUrl != "" {
		levelMap["image"] = h.prefixImageURL(level.ImageUrl)
	}
	if level.BackgroundImage != "" {
		levelMap["background_image"] = h.prefixImageURL(level.BackgroundImage)
	}
	if level.GeneralInfo != nil {
		levelMap["general_info"] = map[string]interface{}{
			"score":       level.GeneralInfo.Score,
			"rank":        level.GeneralInfo.Rank,
			"png_file":    h.prefixImageURL(level.GeneralInfo.PngFile),
			"fbx_file":    level.GeneralInfo.FbxFile,
			"gif_file":    h.prefixImageURL(level.GeneralInfo.GifFile),
			"description": level.GeneralInfo.Description,
		}
	}

	return levelMap
}

// HandleLevelsRoutes is the main router for levels endpoints under /api/levels
func (h *HTTPLevelHandler) HandleLevelsRoutes(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	const basePrefix = "/api/levels"

	if !strings.HasPrefix(path, basePrefix) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	if path == basePrefix {
		h.GetAllLevels(w, r)
		return
	}

	suffix := strings.TrimPrefix(path, basePrefix)
	suffix = strings.TrimPrefix(suffix, "/")
	if suffix == "" {
		h.GetAllLevels(w, r)
		return
	}

	parts := strings.Split(suffix, "/")
	slug := parts[0]

	if len(parts) == 2 {
		switch parts[1] {
		case "general-info":
			r.URL.Path = basePrefix + "/" + slug + "/general-info"
			h.GetLevelGeneralInfo(w, r)
			return
		case "gem":
			r.URL.Path = basePrefix + "/" + slug + "/gem"
			h.GetLevelGem(w, r)
			return
		case "gift":
			r.URL.Path = basePrefix + "/" + slug + "/gift"
			h.GetLevelGift(w, r)
			return
		case "licenses":
			r.URL.Path = basePrefix + "/" + slug + "/licenses"
			h.GetLevelLicenses(w, r)
			return
		case "prize":
			r.URL.Path = basePrefix + "/" + slug + "/prize"
			h.GetLevelPrize(w, r)
			return
		}
	}

	if len(parts) == 1 {
		r.URL.Path = basePrefix + "/" + slug
		h.GetLevel(w, r)
		return
	}

	writeError(w, http.StatusNotFound, "not found")
}

func extractSlugFromPath(path, prefix string) string {
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	return strings.TrimPrefix(path, prefix)
}
