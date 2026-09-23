package handlers

import (
	"log"
	"net/http"

	"forge-backend/internal/middleware"
	"forge-backend/internal/models"
	"forge-backend/internal/services"
)

// ═══════════════════════════════════════════════════════════════════
// GET /api/connections
// ═══════════════════════════════════════════════════════════════════

type ConnectionsResponse struct {
	Success     bool                      `json:"success"`
	Connections []models.PublicConnection `json:"connections"`
	Count       int                       `json:"count"`
	Error       string                    `json:"error,omitempty"`
}

type ConnectionsHandler struct {
	firestoreService *services.FirestoreService
}

func NewConnectionsHandler(fs *services.FirestoreService) *ConnectionsHandler {
	return &ConnectionsHandler{firestoreService: fs}
}

func (h *ConnectionsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uid, ok := middleware.GetFirebaseUID(r)
	if !ok || uid == "" {
		jsonResponse(w, http.StatusUnauthorized, ConnectionsResponse{
			Success: false, Error: "Authentication required",
		})
		return
	}

	connections, err := h.firestoreService.ListConnections(r.Context(), uid)
	if err != nil {
		log.Printf("🔥 ListConnections failed uid=%s: %v", uid, err)
		jsonResponse(w, http.StatusInternalServerError, ConnectionsResponse{
			Success: false, Error: "Failed to load connections",
		})
		return
	}

	if connections == nil {
		connections = []models.PublicConnection{}
	}

	jsonResponse(w, http.StatusOK, ConnectionsResponse{
		Success:     true,
		Connections: connections,
		Count:       len(connections),
	})
}
