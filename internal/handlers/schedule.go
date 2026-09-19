package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"forge-backend/internal/middleware"
	"forge-backend/internal/models"
	"forge-backend/internal/services"
)

type ScheduleHandler struct {
	firestoreService *services.FirestoreService
}

func NewScheduleHandler(fs *services.FirestoreService) *ScheduleHandler {
	return &ScheduleHandler{firestoreService: fs}
}

// ═══════════════════════════════════════════════════════════
// Request / Response types
// ═══════════════════════════════════════════════════════════

type CreateScheduleRequest struct {
	Platform     string `json:"platform"`
	Content      string `json:"content"`
	ScheduledFor string `json:"scheduledFor"` // RFC3339
	Notes        string `json:"notes,omitempty"`
}

type CreateScheduleResponse struct {
	Success bool                  `json:"success"`
	Post    *models.ScheduledPost `json:"post,omitempty"`
	Error   string                `json:"error,omitempty"`
	Code    string                `json:"code,omitempty"`
}

type ListSchedulesResponse struct {
	Success bool                   `json:"success"`
	Posts   []models.ScheduledPost `json:"posts"`
	Error   string                 `json:"error,omitempty"`
}

type DeleteScheduleResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// ═══════════════════════════════════════════════════════════
// POST /api/scheduled
// ═══════════════════════════════════════════════════════════

func (h *ScheduleHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uid, ok := middleware.GetFirebaseUID(r)
	if !ok || uid == "" {
		jsonResponse(w, http.StatusUnauthorized, CreateScheduleResponse{
			Success: false, Error: "Authentication required", Code: "UNAUTHENTICATED",
		})
		return
	}

	var req CreateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, CreateScheduleResponse{
			Success: false, Error: "Invalid request body", Code: "INVALID_REQUEST",
		})
		return
	}

	// Validate
	if req.Platform == "" {
		jsonResponse(w, http.StatusBadRequest, CreateScheduleResponse{
			Success: false, Error: "Platform is required", Code: "INVALID_REQUEST",
		})
		return
	}
	if req.Content == "" {
		jsonResponse(w, http.StatusBadRequest, CreateScheduleResponse{
			Success: false, Error: "Content is required", Code: "INVALID_REQUEST",
		})
		return
	}
	if len(req.Content) > 10000 {
		jsonResponse(w, http.StatusBadRequest, CreateScheduleResponse{
			Success: false, Error: "Content too long (max 10,000 chars)", Code: "INVALID_REQUEST",
		})
		return
	}

	scheduledFor, err := time.Parse(time.RFC3339, req.ScheduledFor)
	if err != nil {
		jsonResponse(w, http.StatusBadRequest, CreateScheduleResponse{
			Success: false, Error: "Invalid scheduledFor (must be RFC3339)", Code: "INVALID_REQUEST",
		})
		return
	}

	// Must be at least 1 minute in the future
	if scheduledFor.Before(time.Now().Add(1 * time.Minute)) {
		jsonResponse(w, http.StatusBadRequest, CreateScheduleResponse{
			Success: false, Error: "Scheduled time must be at least 1 minute in the future", Code: "INVALID_TIME",
		})
		return
	}

	// Max 90 days out
	if scheduledFor.After(time.Now().Add(90 * 24 * time.Hour)) {
		jsonResponse(w, http.StatusBadRequest, CreateScheduleResponse{
			Success: false, Error: "Cannot schedule more than 90 days in advance", Code: "INVALID_TIME",
		})
		return
	}

	post, err := h.firestoreService.CreateScheduledPost(
		r.Context(), uid, req.Platform, req.Content, scheduledFor, req.Notes,
	)
	if err != nil {
		log.Printf("🔥 CreateScheduledPost failed uid=%s: %v", uid, err)
		jsonResponse(w, http.StatusInternalServerError, CreateScheduleResponse{
			Success: false, Error: "Failed to schedule post", Code: "SERVER_ERROR",
		})
		return
	}

	jsonResponse(w, http.StatusOK, CreateScheduleResponse{
		Success: true,
		Post:    post,
	})
}

// ═══════════════════════════════════════════════════════════
// GET /api/scheduled
// ═══════════════════════════════════════════════════════════

func (h *ScheduleHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uid, ok := middleware.GetFirebaseUID(r)
	if !ok || uid == "" {
		jsonResponse(w, http.StatusUnauthorized, ListSchedulesResponse{
			Success: false, Error: "Authentication required",
		})
		return
	}

	posts, err := h.firestoreService.ListScheduledPosts(r.Context(), uid)
	if err != nil {
		log.Printf("🔥 ListScheduledPosts failed uid=%s: %v", uid, err)
		jsonResponse(w, http.StatusInternalServerError, ListSchedulesResponse{
			Success: false, Error: "Failed to load scheduled posts",
		})
		return
	}

	if posts == nil {
		posts = []models.ScheduledPost{}
	}

	jsonResponse(w, http.StatusOK, ListSchedulesResponse{
		Success: true,
		Posts:   posts,
	})
}

// ═══════════════════════════════════════════════════════════
// DELETE /api/scheduled/{id}
// ═══════════════════════════════════════════════════════════

func (h *ScheduleHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uid, ok := middleware.GetFirebaseUID(r)
	if !ok || uid == "" {
		jsonResponse(w, http.StatusUnauthorized, DeleteScheduleResponse{
			Success: false, Error: "Authentication required",
		})
		return
	}

	postID := chi.URLParam(r, "id")
	if strings.TrimSpace(postID) == "" {
		jsonResponse(w, http.StatusBadRequest, DeleteScheduleResponse{
			Success: false, Error: "Post ID is required",
		})
		return
	}

	err := h.firestoreService.DeleteScheduledPost(r.Context(), uid, postID)
	if err != nil {
		log.Printf("🔥 DeleteScheduledPost failed uid=%s postId=%s: %v", uid, postID, err)
		jsonResponse(w, http.StatusNotFound, DeleteScheduleResponse{
			Success: false, Error: fmt.Sprintf("Post not found or already deleted"),
		})
		return
	}

	jsonResponse(w, http.StatusOK, DeleteScheduleResponse{
		Success: true,
	})
}
