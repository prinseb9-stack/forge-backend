package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"forge-backend/internal/middleware"
	"forge-backend/internal/models"
	"forge-backend/internal/oauth"
	"forge-backend/internal/services"
)

// ═══════════════════════════════════════════════════════════════════
// Bluesky OAuth handlers
// ═══════════════════════════════════════════════════════════════════

type BlueskyConnectRequest struct {
	Handle      string `json:"handle"`
	AppPassword string `json:"appPassword"`
}

type BlueskyConnectResponse struct {
	Success     bool   `json:"success"`
	Handle      string `json:"handle,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Error       string `json:"error,omitempty"`
	Code        string `json:"code,omitempty"`
}

type BlueskyDisconnectResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type BlueskyHandler struct {
	bluesky          *oauth.BlueskyClient
	firestoreService *services.FirestoreService
}

func NewBlueskyHandler(
	bluesky *oauth.BlueskyClient,
	fs *services.FirestoreService,
) *BlueskyHandler {
	return &BlueskyHandler{
		bluesky:          bluesky,
		firestoreService: fs,
	}
}

// ─────────────────────────────────────────────────────────────────
// POST /api/oauth/bluesky/connect
// ─────────────────────────────────────────────────────────────────

func (h *BlueskyHandler) HandleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uid, ok := middleware.GetFirebaseUID(r)
	if !ok || uid == "" {
		jsonResponse(w, http.StatusUnauthorized, BlueskyConnectResponse{
			Success: false, Error: "Authentication required", Code: "UNAUTHENTICATED",
		})
		return
	}

	var req BlueskyConnectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, BlueskyConnectResponse{
			Success: false, Error: "Invalid request body", Code: "INVALID_REQUEST",
		})
		return
	}

	req.Handle = strings.TrimSpace(req.Handle)
	req.AppPassword = strings.TrimSpace(req.AppPassword)

	if req.Handle == "" {
		jsonResponse(w, http.StatusBadRequest, BlueskyConnectResponse{
			Success: false, Error: "Bluesky handle is required", Code: "INVALID_REQUEST",
		})
		return
	}
	if req.AppPassword == "" {
		jsonResponse(w, http.StatusBadRequest, BlueskyConnectResponse{
			Success: false, Error: "App password is required", Code: "INVALID_REQUEST",
		})
		return
	}

	// Call Bluesky to create a session
	session, err := h.bluesky.CreateSession(r.Context(), req.Handle, req.AppPassword)
	if err != nil {
		// Do NOT log the app password or handle
		log.Printf("🔥 Bluesky CreateSession failed uid=%s: %v", uid, err)
		jsonResponse(w, http.StatusUnauthorized, BlueskyConnectResponse{
			Success: false,
			Error:   "Could not authenticate with Bluesky. Check your handle and app password.",
			Code:    "BLUESKY_AUTH_FAILED",
		})
		return
	}

	// Look up the user profile to get display name and avatar
	var displayName, avatar string
	if profile, err := h.bluesky.GetProfile(r.Context(), session.AccessJwt, session.Handle); err == nil {
		displayName = profile.DisplayName
		avatar = profile.Avatar
	}
	// Failure here is non-fatal — we just don't have a display name

	now := time.Now().UTC()
	// Bluesky access JWTs expire in ~2 hours; refresh JWTs in ~2 months.
	// We don't strictly enforce expiry yet; refresh happens on publish (future feature).
	accessExpiry := now.Add(2 * time.Hour)

	conn := &models.Connection{
		PlatformID:          "bluesky",
		DID:                 session.DID,
		Handle:              session.Handle,
		DisplayName:         displayName,
		Avatar:              avatar,
		EncryptedAccessJwt:  session.AccessJwt,
		EncryptedRefreshJwt: session.RefreshJwt,
		AccessExpiresAt:     accessExpiry,
		Status:              models.ConnectionStatusActive,
		ConnectedAt:         now,
		UpdatedAt:           now,
	}

	if err := h.firestoreService.SaveConnection(r.Context(), uid, conn); err != nil {
		log.Printf("🔥 SaveConnection failed uid=%s: %v", uid, err)
		jsonResponse(w, http.StatusInternalServerError, BlueskyConnectResponse{
			Success: false, Error: "Failed to save connection", Code: "SERVER_ERROR",
		})
		return
	}

	log.Printf("✅ Bluesky connected uid=%s handle=%s", uid, session.Handle)

	jsonResponse(w, http.StatusOK, BlueskyConnectResponse{
		Success:     true,
		Handle:      session.Handle,
		DisplayName: displayName,
	})
}

// ─────────────────────────────────────────────────────────────────
// POST /api/oauth/bluesky/disconnect
// ─────────────────────────────────────────────────────────────────

func (h *BlueskyHandler) HandleDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uid, ok := middleware.GetFirebaseUID(r)
	if !ok || uid == "" {
		jsonResponse(w, http.StatusUnauthorized, BlueskyDisconnectResponse{
			Success: false, Error: "Authentication required",
		})
		return
	}

	// Try to load the connection so we can revoke it with Bluesky.
	// Failure here is non-fatal — we still delete the local record.
	if conn, err := h.firestoreService.GetConnection(r.Context(), uid, "bluesky"); err == nil {
		if err := h.bluesky.DeleteSession(r.Context(), conn.EncryptedAccessJwt); err != nil {
			log.Printf("⚠️  Bluesky DeleteSession failed uid=%s: %v", uid, err)
		}
	}

	if err := h.firestoreService.DeleteConnection(r.Context(), uid, "bluesky"); err != nil {
		log.Printf("🔥 DeleteConnection failed uid=%s: %v", uid, err)
		jsonResponse(w, http.StatusInternalServerError, BlueskyDisconnectResponse{
			Success: false, Error: "Failed to disconnect",
		})
		return
	}

	log.Printf("✅ Bluesky disconnected uid=%s", uid)

	jsonResponse(w, http.StatusOK, BlueskyDisconnectResponse{Success: true})
}
