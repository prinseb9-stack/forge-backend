package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"forge-backend/internal/middleware"
	"forge-backend/internal/services"
)

type MeResponse struct {
	Success bool        `json:"success"`
	User    *MeUserInfo `json:"user,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type MeUserInfo struct {
	UID             string     `json:"uid"`
	Email           string     `json:"email"`
	DisplayName     string     `json:"displayName"`
	Plan            string     `json:"plan"`
	TextGeneration  *UsageInfo `json:"textGeneration,omitempty"`
	ImageGeneration *UsageInfo `json:"imageGeneration,omitempty"`
	VideoGeneration *UsageInfo `json:"videoGeneration,omitempty"`
}

type MeHandler struct {
	firestoreService *services.FirestoreService
}

func NewMeHandler(fs *services.FirestoreService) *MeHandler {
	return &MeHandler{firestoreService: fs}
}

func (h *MeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uid, ok := middleware.GetFirebaseUID(r)
	if !ok || uid == "" {
		writeJSON(w, http.StatusUnauthorized, MeResponse{
			Success: false, Error: "Authentication required",
		})
		return
	}

	email, _ := middleware.GetFirebaseEmail(r)
	displayName, _ := middleware.GetFirebaseDisplayName(r)

	user, err := h.firestoreService.GetOrCreateUser(r.Context(), uid, email, displayName)
	if err != nil {
		log.Printf("🔥 /api/me GetOrCreateUser failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, MeResponse{
			Success: false, Error: "Failed to load user profile",
		})
		return
	}

	writeJSON(w, http.StatusOK, MeResponse{
		Success: true,
		User: &MeUserInfo{
			UID:             user.UID,
			Email:           user.Email,
			DisplayName:     user.DisplayName,
			Plan:            string(user.Plan),
			TextGeneration:  buildUsageInfo(&user.TextGeneration),
			ImageGeneration: buildUsageInfo(&user.ImageGeneration),
			VideoGeneration: buildUsageInfo(&user.VideoGeneration),
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
