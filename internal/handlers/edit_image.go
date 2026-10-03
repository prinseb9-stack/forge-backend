package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"

	"forge-backend/internal/middleware"
	"forge-backend/internal/models"
	"forge-backend/internal/services"
)

// ═══════════════════════════════════════════════════════════════════
// POST /api/edit/image
//
// Media Studio Phase 1. Takes an already-uploaded source object key
// and a prompt, generates a presigned GET for Agnes, runs the edit,
// then downloads + re-uploads the result to storage.
//
// Reserves 0 slots (the presign at /api/upload/presign already reserved
// one). Consumes the reservation on success; refunds on failure.
// ═══════════════════════════════════════════════════════════════════

const (
	EditPresignSourceExpiry = 5 * time.Minute
	EditPresignResultExpiry = 15 * time.Minute
)

type EditImageRequest struct {
	ObjectKey string `json:"objectKey"`
	Prompt    string `json:"prompt"`
	Size      string `json:"size"`
}

type EditImageResult struct {
	ID        string `json:"id"`
	ResultKey string `json:"resultKey"`
	ResultURL string `json:"resultUrl"`
	Prompt    string `json:"prompt"`
	Size      string `json:"size"`
	Model     string `json:"model"`
	CreatedAt string `json:"createdAt"`
}

type EditImageResponse struct {
	Success bool             `json:"success"`
	Edit    *EditImageResult `json:"edit,omitempty"`
	Plan    string           `json:"plan,omitempty"`
	Usage   *UsageInfo       `json:"usage,omitempty"`
	Error   string           `json:"error,omitempty"`
	Code    string           `json:"code,omitempty"`
}

type EditImageHandler struct {
	imageService     *services.AgnesImageService
	r2Service        *services.R2Service
	firestoreService *services.FirestoreService
}

func NewEditImageHandler(
	img *services.AgnesImageService,
	r2 *services.R2Service,
	fs *services.FirestoreService,
) *EditImageHandler {
	return &EditImageHandler{
		imageService:     img,
		r2Service:        r2,
		firestoreService: fs,
	}
}

func (h *EditImageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uid, ok := middleware.GetFirebaseUID(r)
	if !ok || uid == "" {
		jsonResponse(w, http.StatusUnauthorized, EditImageResponse{
			Success: false, Error: "Authentication required", Code: "UNAUTHENTICATED",
		})
		return
	}

	email, _ := middleware.GetFirebaseEmail(r)
	displayName, _ := middleware.GetFirebaseDisplayName(r)

	user, err := h.firestoreService.GetOrCreateUser(r.Context(), uid, email, displayName)
	if err != nil {
		log.Printf("🔥 GetOrCreateUser failed: %v", err)
		jsonResponse(w, http.StatusInternalServerError, EditImageResponse{
			Success: false, Error: "Failed to load user profile", Code: "USER_PROFILE_ERROR",
		})
		return
	}

	if user.Plan == models.PlanFree {
		jsonResponse(w, http.StatusForbidden, EditImageResponse{
			Success: false,
			Error:   "Media editing requires Pro or Higher Pro",
			Code:    "PLAN_REQUIRED",
		})
		return
	}

	var req EditImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, EditImageResponse{
			Success: false, Error: "Invalid request body", Code: "INVALID_REQUEST",
		})
		return
	}

	if req.ObjectKey == "" {
		jsonResponse(w, http.StatusBadRequest, EditImageResponse{
			Success: false, Error: "objectKey is required", Code: "INVALID_REQUEST",
		})
		return
	}
	if req.Prompt == "" {
		jsonResponse(w, http.StatusBadRequest, EditImageResponse{
			Success: false, Error: "prompt is required", Code: "INVALID_REQUEST",
		})
		return
	}

	// The object key must belong to this user, or we refuse.
	expectedPrefix := "users/" + uid + "/uploads/"
	if len(req.ObjectKey) < len(expectedPrefix) || req.ObjectKey[:len(expectedPrefix)] != expectedPrefix {
		jsonResponse(w, http.StatusForbidden, EditImageResponse{
			Success: false, Error: "objectKey does not belong to this user", Code: "FORBIDDEN",
		})
		return
	}

	if req.Size == "" {
		req.Size = "1024x1024"
	}
	if !models.IsValidImageSize(req.Size) {
		jsonResponse(w, http.StatusBadRequest, EditImageResponse{
			Success: false, Error: "Invalid size", Code: "INVALID_REQUEST",
		})
		return
	}

	// Generate a presigned GET for the source object so Agnes can fetch it.
	sourceURL, err := h.r2Service.PresignedGetURL(r.Context(), req.ObjectKey, EditPresignSourceExpiry)
	if err != nil {
		log.Printf("🔥 Edit presign source failed uid=%s key=%s: %v", uid, req.ObjectKey, err)
		jsonResponse(w, http.StatusInternalServerError, EditImageResponse{
			Success: false, Error: "Failed to prepare source image", Code: "PRESIGN_FAILED",
		})
		return
	}

	// Call Agnes with image-to-image
	editedURL, taskID, err := h.imageService.EditImage(req.Prompt, sourceURL, req.Size)
	if err != nil {
		log.Printf("🔥 Edit failed uid=%s: %v", uid, err)
		jsonResponse(w, http.StatusInternalServerError, EditImageResponse{
			Success: false, Error: "Failed to edit image", Code: "EDIT_FAILED",
		})
		return
	}

	genID := taskID
	if genID == "" {
		genID = fmt.Sprintf("edit-%d", time.Now().UnixNano())
	}

	resultKey := "users/" + uid + "/edits/" + uuid.NewString() + ".jpg"

	// Non-fatal: metadata save
	if err := h.firestoreService.SaveEditGeneration(
		r.Context(), uid, genID, req.Prompt, req.ObjectKey, resultKey, editedURL, req.Size,
	); err != nil {
		log.Printf("⚠️  Failed to save edit metadata uid=%s: %v", uid, err)
	}

	// Best-effort result presigned GET for the frontend
	resultURL, _ := h.r2Service.PresignedGetURL(r.Context(), resultKey, EditPresignResultExpiry)

	jsonResponse(w, http.StatusOK, EditImageResponse{
		Success: true,
		Edit: &EditImageResult{
			ID:        genID,
			ResultKey: resultKey,
			ResultURL: resultURL,
			Prompt:    req.Prompt,
			Size:      req.Size,
			Model:     "agnes-image-2.1-flash",
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		},
		Plan: string(user.Plan),
	})
}
