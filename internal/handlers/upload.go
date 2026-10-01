package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"forge-backend/internal/middleware"
	"forge-backend/internal/models"
	"forge-backend/internal/services"
)

// ═══════════════════════════════════════════════════════════════════
// POST /api/upload/presign
//
// Phase 1 (media studio): returns a presigned R2 PUT URL so the client
// can upload an image directly to R2. The file never passes through
// the backend.
//
// Reserves one image-generation slot. The reservation is consumed when
// the edit completes (Stage 3) or refunded if the edit fails. If the
// upload is abandoned, the slot is released when the reservation
// expires via the existing usage-period rollover.
// ═══════════════════════════════════════════════════════════════════

const (
	// MaxUploadBytes is the Phase 1 size cap for photo uploads.
	MaxUploadBytes = 15 * 1024 * 1024 // 15 MB

	// PresignExpiry is how long the presigned PUT URL stays valid.
	PresignExpiry = 10 * time.Minute

	// AllowedImageTypes mirrors the R2 service's allow-list.
	// Kept here too so the handler can reject early with a clear error.
)

var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
}

type PresignRequest struct {
	ContentType string `json:"contentType"`
	FileSize    int64  `json:"fileSize"`
}

type PresignResponse struct {
	Success   bool       `json:"success"`
	UploadURL string     `json:"uploadUrl,omitempty"`
	ObjectKey string     `json:"objectKey,omitempty"`
	ExpiresAt string     `json:"expiresAt,omitempty"`
	Plan      string     `json:"plan,omitempty"`
	Usage     *UsageInfo `json:"usage,omitempty"`
	Error     string     `json:"error,omitempty"`
	Code      string     `json:"code,omitempty"`
}

type UploadHandler struct {
	r2Service        *services.R2Service
	firestoreService *services.FirestoreService
}

func NewUploadHandler(r2 *services.R2Service, fs *services.FirestoreService) *UploadHandler {
	return &UploadHandler{
		r2Service:        r2,
		firestoreService: fs,
	}
}

func (h *UploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uid, ok := middleware.GetFirebaseUID(r)
	if !ok || uid == "" {
		jsonResponse(w, http.StatusUnauthorized, PresignResponse{
			Success: false, Error: "Authentication required", Code: "UNAUTHENTICATED",
		})
		return
	}

	email, _ := middleware.GetFirebaseEmail(r)
	displayName, _ := middleware.GetFirebaseDisplayName(r)

	user, err := h.firestoreService.GetOrCreateUser(r.Context(), uid, email, displayName)
	if err != nil {
		log.Printf("🔥 GetOrCreateUser failed: %v", err)
		jsonResponse(w, http.StatusInternalServerError, PresignResponse{
			Success: false, Error: "Failed to load user profile", Code: "USER_PROFILE_ERROR",
		})
		return
	}

	// Plan gate — matches image-generation behavior.
	if user.Plan == models.PlanFree {
		jsonResponse(w, http.StatusForbidden, PresignResponse{
			Success: false,
			Error:   "Media editing requires Pro or Higher Pro",
			Code:    "PLAN_REQUIRED",
		})
		return
	}

	var req PresignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, PresignResponse{
			Success: false, Error: "Invalid request body", Code: "INVALID_REQUEST",
		})
		return
	}

	// Content-Type validation — normalize case and whitespace.
	normalizedCT := strings.TrimSpace(strings.ToLower(req.ContentType))
	ext, ok := allowedImageTypes[normalizedCT]
	if !ok {
		jsonResponse(w, http.StatusBadRequest, PresignResponse{
			Success: false,
			Error:   "Unsupported content type. Allowed: image/jpeg, image/png",
			Code:    "INVALID_CONTENT_TYPE",
		})
		return
	}

	// Size validation — early rejection based on the client-reported size.
	// Stage 3 will verify the actual stored size with a HEAD request.
	if req.FileSize <= 0 {
		jsonResponse(w, http.StatusBadRequest, PresignResponse{
			Success: false, Error: "fileSize must be positive", Code: "INVALID_REQUEST",
		})
		return
	}
	if req.FileSize > MaxUploadBytes {
		jsonResponse(w, http.StatusRequestEntityTooLarge, PresignResponse{
			Success: false,
			Error:   "File exceeds the 15 MB limit",
			Code:    "FILE_TOO_LARGE",
		})
		return
	}

	// Reserve one image slot. Consumed by the edit handler in Stage 3.
	// If the user abandons the upload, the reservation is naturally
	// discarded when the usage period rolls over.
	reservationID, periodAfter, err := h.firestoreService.ReserveImageGeneration(r.Context(), uid)
	if err != nil {
		if err == services.ErrUsageLimitExceeded {
			jsonResponse(w, http.StatusTooManyRequests, PresignResponse{
				Success: false, Error: "Media usage limit reached", Code: "USAGE_LIMIT_EXCEEDED",
				Plan: string(user.Plan),
			})
			return
		}
		log.Printf("🔥 ReserveImage failed (upload) uid=%s: %v", uid, err)
		jsonResponse(w, http.StatusInternalServerError, PresignResponse{
			Success: false, Error: "Failed to reserve usage", Code: "RESERVATION_ERROR",
		})
		return
	}

	// Build a namespaced object key: users/{uid}/uploads/{uuid}{ext}
	// The uid prefix ensures cross-user access is impossible even if
	// a presigned URL leaks. The uuid prevents collision and enumeration.
	objectKey := filepath.ToSlash(
		"users/" + uid + "/uploads/" + uuid.NewString() + ext,
	)

	uploadURL, err := h.r2Service.PresignedPutURL(
		r.Context(), objectKey, normalizedCT, PresignExpiry,
	)
	if err != nil {
		// Refund the reservation — no upload URL was issued.
		if refundErr := h.firestoreService.RefundImageGeneration(r.Context(), uid, reservationID); refundErr != nil {
			log.Printf("🚨 Upload presign refund failed uid=%s res=%s: %v",
				uid, reservationID, refundErr)
		}
		log.Printf("🔥 Presign PUT failed uid=%s: %v", uid, err)
		jsonResponse(w, http.StatusInternalServerError, PresignResponse{
			Success: false, Error: "Failed to generate upload URL", Code: "PRESIGN_FAILED",
		})
		return
	}

	expiresAt := time.Now().UTC().Add(PresignExpiry).Format(time.RFC3339)

	jsonResponse(w, http.StatusOK, PresignResponse{
		Success:   true,
		UploadURL: uploadURL,
		ObjectKey: objectKey,
		ExpiresAt: expiresAt,
		Plan:      string(user.Plan),
		Usage:     buildUsageInfo(periodAfter),
	})
}
