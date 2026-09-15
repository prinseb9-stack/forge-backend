package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"forge-backend/internal/middleware"
	"forge-backend/internal/models"
	"forge-backend/internal/services"
)

type GenerateImageRequest struct {
	Prompt string `json:"prompt"`
	Size   string `json:"size"`
}

type ImageResult struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	Prompt    string `json:"prompt"`
	Size      string `json:"size"`
	Model     string `json:"model"`
	CreatedAt string `json:"createdAt"`
}

type GenerateImageResponse struct {
	Success bool         `json:"success"`
	Image   *ImageResult `json:"image,omitempty"`
	Plan    string       `json:"plan,omitempty"`
	Usage   *UsageInfo   `json:"usage,omitempty"`
	Error   string       `json:"error,omitempty"`
	Code    string       `json:"code,omitempty"`
}

type GenerateImageHandler struct {
	imageService     *services.AgnesImageService
	firestoreService *services.FirestoreService
}

func NewGenerateImageHandler(img *services.AgnesImageService, fs *services.FirestoreService) *GenerateImageHandler {
	return &GenerateImageHandler{
		imageService:     img,
		firestoreService: fs,
	}
}

func (h *GenerateImageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uid, ok := middleware.GetFirebaseUID(r)
	if !ok || uid == "" {
		jsonResponse(w, http.StatusUnauthorized, GenerateImageResponse{
			Success: false, Error: "Authentication required", Code: "UNAUTHENTICATED",
		})
		return
	}

	email, _ := middleware.GetFirebaseEmail(r)
	displayName, _ := middleware.GetFirebaseDisplayName(r)

	user, err := h.firestoreService.GetOrCreateUser(r.Context(), uid, email, displayName)
	if err != nil {
		log.Printf("🔥 GetOrCreateUser failed: %v", err)
		jsonResponse(w, http.StatusInternalServerError, GenerateImageResponse{
			Success: false, Error: "Failed to load user profile", Code: "USER_PROFILE_ERROR",
		})
		return
	}

	// Plan gating — FREE users blocked
	if user.Plan == models.PlanFree {
		jsonResponse(w, http.StatusForbidden, GenerateImageResponse{
			Success: false,
			Error:   "Image generation requires Pro or Higher Pro",
			Code:    "PLAN_REQUIRED",
		})
		return
	}

	var req GenerateImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, GenerateImageResponse{
			Success: false, Error: "Invalid request body", Code: "INVALID_REQUEST",
		})
		return
	}

	if req.Prompt == "" {
		jsonResponse(w, http.StatusBadRequest, GenerateImageResponse{
			Success: false, Error: "Prompt is required", Code: "INVALID_REQUEST",
		})
		return
	}

	if req.Size == "" {
		req.Size = "1024x1024"
	}

	if !models.IsValidImageSize(req.Size) {
		jsonResponse(w, http.StatusBadRequest, GenerateImageResponse{
			Success: false,
			Error:   "Invalid size. Allowed: 1024x1024, 1792x1024, 1024x1792",
			Code:    "INVALID_REQUEST",
		})
		return
	}

	// Reserve image usage
	reservationID, periodAfter, err := h.firestoreService.ReserveImageGeneration(r.Context(), uid)
	if err != nil {
		if err == services.ErrUsageLimitExceeded {
			jsonResponse(w, http.StatusTooManyRequests, GenerateImageResponse{
				Success: false, Error: "Image usage limit reached", Code: "USAGE_LIMIT_EXCEEDED",
				Plan: string(user.Plan),
			})
			return
		}
		log.Printf("🔥 ReserveImage failed: %v", err)
		jsonResponse(w, http.StatusInternalServerError, GenerateImageResponse{
			Success: false, Error: "Failed to reserve usage", Code: "RESERVATION_ERROR",
		})
		return
	}

	refund := func(reason string) {
		if err := h.firestoreService.RefundImageGeneration(r.Context(), uid, reservationID); err != nil {
			log.Printf("🚨 Image refund failed uid=%s res=%s reason=%s: %v", uid, reservationID, reason, err)
		}
	}

	// Generate image
	imgURL, taskID, err := h.imageService.GenerateImage(req.Prompt, req.Size)
	if err != nil {
		refund("generation_failed")
		log.Printf("🔥 Image generation failed uid=%s: %v", uid, err)
		jsonResponse(w, http.StatusInternalServerError, GenerateImageResponse{
			Success: false, Error: "Failed to generate image", Code: "GENERATION_FAILED",
		})
		return
	}

	genID := taskID
	if genID == "" {
		genID = fmt.Sprintf("img-%d", time.Now().UnixNano())
	}

	// Non-fatal if metadata save fails
	if err := h.firestoreService.SaveImageGeneration(r.Context(), uid, genID, req.Prompt, req.Size, imgURL); err != nil {
		log.Printf("⚠️  Failed to save image metadata uid=%s: %v", uid, err)
	}

	if err := h.firestoreService.ConsumeImageGeneration(r.Context(), uid, reservationID); err != nil {
		log.Printf("⚠️  Consume image failed uid=%s res=%s: %v", uid, reservationID, err)
	}

	jsonResponse(w, http.StatusOK, GenerateImageResponse{
		Success: true,
		Image: &ImageResult{
			ID:        genID,
			URL:       imgURL,
			Prompt:    req.Prompt,
			Size:      req.Size,
			Model:     "agnes-image-2.1-flash",
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		},
		Plan:  string(user.Plan),
		Usage: buildUsageInfo(periodAfter),
	})
}
