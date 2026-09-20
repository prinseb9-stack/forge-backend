package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"

	"forge-backend/internal/middleware"
	"forge-backend/internal/models"
	"forge-backend/internal/services"
)

type GenerateRequest struct {
	Content   string   `json:"content"`
	Platforms []string `json:"platforms"`
}

type UsageInfo struct {
	Used            int    `json:"used"`
	Limit           int    `json:"limit"`
	Remaining       int    `json:"remaining"`
	PeriodStartedAt string `json:"periodStartedAt,omitempty"`
	PeriodEndsAt    string `json:"periodEndsAt,omitempty"`
}

type GenerateResponse struct {
	Success bool                      `json:"success"`
	Results []services.PlatformResult `json:"results,omitempty"`
	Plan    string                    `json:"plan,omitempty"`
	Usage   *UsageInfo                `json:"usage,omitempty"`
	Error   string                    `json:"error,omitempty"`
	Code    string                    `json:"code,omitempty"`
}

type GenerateHandler struct {
	factory          *services.ProviderFactory
	firestoreService *services.FirestoreService
}

func NewGenerateHandler(factory *services.ProviderFactory, fs *services.FirestoreService) *GenerateHandler {
	return &GenerateHandler{
		factory:          factory,
		firestoreService: fs,
	}
}

func buildUsageInfo(period *models.UsagePeriod) *UsageInfo {
	if period == nil {
		return nil
	}
	remaining := -1
	if period.MaxUsage != -1 {
		remaining = period.MaxUsage - period.UsageCount
		if remaining < 0 {
			remaining = 0
		}
	}
	return &UsageInfo{
		Used:            period.UsageCount,
		Limit:           period.MaxUsage,
		Remaining:       remaining,
		PeriodStartedAt: period.PeriodStartedAt.UTC().Format("2006-01-02T15:04:05Z"),
		PeriodEndsAt:    period.PeriodEndsAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
}

// countWords counts whitespace-separated words
func countWords(s string) int {
	return len(strings.Fields(s))
}

func (h *GenerateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uid, ok := middleware.GetFirebaseUID(r)
	if !ok || uid == "" {
		jsonResponse(w, http.StatusUnauthorized, GenerateResponse{
			Success: false, Error: "Authentication required", Code: "UNAUTHENTICATED",
		})
		return
	}

	email, _ := middleware.GetFirebaseEmail(r)
	displayName, _ := middleware.GetFirebaseDisplayName(r)

	user, err := h.firestoreService.GetOrCreateUser(r.Context(), uid, email, displayName)
	if err != nil {
		log.Printf("🔥 GetOrCreateUser failed: %v", err)
		jsonResponse(w, http.StatusInternalServerError, GenerateResponse{
			Success: false, Error: "Failed to load user profile", Code: "USER_PROFILE_ERROR",
		})
		return
	}

	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, GenerateResponse{
			Success: false, Error: "Invalid request body", Code: "INVALID_REQUEST",
		})
		return
	}

	if req.Content == "" {
		jsonResponse(w, http.StatusBadRequest, GenerateResponse{
			Success: false, Error: "Content is required", Code: "INVALID_REQUEST",
		})
		return
	}
	if len(req.Platforms) == 0 {
		jsonResponse(w, http.StatusBadRequest, GenerateResponse{
			Success: false, Error: "At least one platform is required", Code: "INVALID_REQUEST",
		})
		return
	}

	// ═══ Plan-based validation ═══

	// 1. Input word count
	wordCount := countWords(req.Content)
	maxWords := models.MaxInputWords(user.Plan)
	if maxWords != -1 && wordCount > maxWords {
		jsonResponse(w, http.StatusForbidden, GenerateResponse{
			Success: false,
			Error:   fmt.Sprintf("Input too long for %s plan: %d words (max %d). Upgrade to Pro for longer inputs.", user.Plan, wordCount, maxWords),
			Code:    "INPUT_TOO_LONG",
			Plan:    string(user.Plan),
		})
		return
	}

	// 2. Platform count
	maxPlatforms := models.MaxPlatforms(user.Plan)
	if maxPlatforms != -1 && len(req.Platforms) > maxPlatforms {
		jsonResponse(w, http.StatusForbidden, GenerateResponse{
			Success: false,
			Error:   fmt.Sprintf("Your %s plan allows %d platform(s) per generation. Upgrade to Pro to use all platforms.", user.Plan, maxPlatforms),
			Code:    "PLATFORM_LIMIT_EXCEEDED",
			Plan:    string(user.Plan),
		})
		return
	}

	// ═══ Reservation ═══
	reservationID, periodAfter, err := h.firestoreService.ReserveTextGeneration(r.Context(), uid)
	if err != nil {
		if err == services.ErrUsageLimitExceeded {
			jsonResponse(w, http.StatusTooManyRequests, GenerateResponse{
				Success: false, Error: "Usage limit reached", Code: "USAGE_LIMIT_EXCEEDED",
				Plan: string(user.Plan),
			})
			return
		}
		log.Printf("🔥 Reserve failed: %v", err)
		jsonResponse(w, http.StatusInternalServerError, GenerateResponse{
			Success: false, Error: "Failed to reserve usage", Code: "RESERVATION_ERROR",
		})
		return
	}

	refund := func(reason string) {
		if err := h.firestoreService.RefundTextGeneration(r.Context(), uid, reservationID); err != nil {
			log.Printf("🚨 Refund failed uid=%s res=%s reason=%s: %v", uid, reservationID, reason, err)
		}
	}

	provider, err := h.factory.GetProvider(user.Plan)
	if err != nil {
		refund("provider_init_failed")
		jsonResponse(w, http.StatusInternalServerError, GenerateResponse{
			Success: false, Error: "Failed to initialize AI provider", Code: "PROVIDER_ERROR",
		})
		return
	}

	response, err := provider.Generate(req.Content, req.Platforms)
	if err != nil {
		refund("ai_generation_failed")
		log.Printf("🔥 AI generation failed uid=%s: %v", uid, err)
		jsonResponse(w, http.StatusInternalServerError, GenerateResponse{
			Success: false, Error: "Failed to generate content", Code: "GENERATION_FAILED",
		})
		return
	}

	cleaned := cleanAIResponse(response)

	var result services.GenerateResponse
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		refund("malformed_json")
		jsonResponse(w, http.StatusInternalServerError, GenerateResponse{
			Success: false, Error: "Failed to parse AI response", Code: "AI_RESPONSE_INVALID",
		})
		return
	}

	if err := validatePlatforms(req.Platforms, result.Results); err != nil {
		// ─── TEMPORARY DIAGNOSTIC (remove after capture) ───
		{
			normalizedRequested := make([]string, len(req.Platforms))
			for i, p := range req.Platforms {
				normalizedRequested[i] = normalizePlatformName(p)
			}
			normalizedReturned := make([]string, len(result.Results))
			for i, r := range result.Results {
				normalizedReturned[i] = normalizePlatformName(r.Platform)
			}
			sort.Strings(normalizedRequested)
			sort.Strings(normalizedReturned)

			log.Printf(
				"🔬 PLATFORM_VALIDATION_FAILED uid=%s err=%q requested_raw=%v requested_norm=%v returned_raw=%v returned_norm=%v",
				uid,
				err.Error(),
				req.Platforms,
				normalizedRequested,
				platformsOf(result.Results),
				normalizedReturned,
			)
		}
		// ─── END TEMPORARY DIAGNOSTIC ───

		refund("platform_validation_failed")
		jsonResponse(w, http.StatusInternalServerError, GenerateResponse{
			Success: false, Error: "AI response did not match requested platforms", Code: "AI_RESPONSE_INVALID",
		})
		return
	}

	for i, r := range result.Results {
		result.Results[i].Platform = normalizePlatformName(r.Platform)
	}

	// Save history (Pro+ only, non-fatal)
	if models.CanSaveHistory(user.Plan) {
		if err := h.firestoreService.SaveTextGeneration(r.Context(), uid, req.Content, result.Results); err != nil {
			log.Printf("⚠️  Failed to save text history uid=%s: %v", uid, err)
		}
	}

	if err := h.firestoreService.ConsumeTextGeneration(r.Context(), uid, reservationID); err != nil {
		log.Printf("⚠️  Consume failed uid=%s res=%s: %v", uid, reservationID, err)
	}

	jsonResponse(w, http.StatusOK, GenerateResponse{
		Success: true,
		Results: result.Results,
		Plan:    string(user.Plan),
		Usage:   buildUsageInfo(periodAfter),
	})
}

func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func cleanAIResponse(response string) string {
	response = strings.TrimPrefix(response, "```json\n")
	response = strings.TrimPrefix(response, "```\n")
	response = strings.TrimSuffix(response, "\n```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start >= 0 && end > start {
		response = response[start : end+1]
	}

	return sanitizeJSON(response)
}

func sanitizeJSON(jsonStr string) string {
	if jsonStr == "" {
		return jsonStr
	}

	var result strings.Builder
	inString := false
	escapeNext := false

	for i := 0; i < len(jsonStr); i++ {
		ch := jsonStr[i]

		if escapeNext {
			result.WriteByte(ch)
			escapeNext = false
			continue
		}

		if ch == '\\' && inString {
			result.WriteByte(ch)
			escapeNext = true
			continue
		}

		if ch == '"' && !escapeNext {
			inString = !inString
			result.WriteByte(ch)
			continue
		}

		if inString {
			switch ch {
			case '\n':
				result.WriteString("\\n")
			case '\r':
				result.WriteString("\\r")
			case '\t':
				result.WriteString("\\t")
			default:
				result.WriteByte(ch)
			}
		} else {
			result.WriteByte(ch)
		}
	}

	return result.String()
}

func validatePlatforms(requested []string, results []services.PlatformResult) error {
	normalizedRequested := make([]string, len(requested))
	for i, p := range requested {
		normalizedRequested[i] = normalizePlatformName(p)
	}

	normalizedReturned := make([]string, len(results))
	returnedMap := make(map[string]int)
	for i, r := range results {
		normalized := normalizePlatformName(r.Platform)
		normalizedReturned[i] = normalized
		returnedMap[normalized]++
	}

	if len(normalizedRequested) != len(normalizedReturned) {
		return fmt.Errorf("count mismatch: requested %d, got %d",
			len(normalizedRequested), len(normalizedReturned))
	}

	for platform, count := range returnedMap {
		if count > 1 {
			return fmt.Errorf("duplicate platform: '%s'", platform)
		}
	}

	requestedSet := make(map[string]bool)
	for _, p := range normalizedRequested {
		requestedSet[p] = true
	}

	for _, p := range normalizedRequested {
		if _, exists := returnedMap[p]; !exists {
			return fmt.Errorf("missing platform: '%s'", p)
		}
	}

	for platform := range returnedMap {
		if !requestedSet[platform] {
			return fmt.Errorf("unexpected platform: '%s'", platform)
		}
	}

	return nil
}

func normalizePlatformName(platform string) string {
	platform = strings.ToLower(strings.TrimSpace(platform))

	switch platform {
	case "x", "twitter":
		return "x"
	case "instagram", "ig":
		return "instagram"
	case "facebook", "fb":
		return "facebook"
	case "linkedin", "li":
		return "linkedin"
	case "blog", "blogs":
		return "blog"
	case "newsletter", "newsletters":
		return "newsletter"
	default:
		return platform
	}
}

// ─── TEMPORARY DIAGNOSTIC HELPER (remove after capture) ───
func platformsOf(results []services.PlatformResult) []string {
	out := make([]string, len(results))
	for i, r := range results {
		out[i] = r.Platform
	}
	return out
}
