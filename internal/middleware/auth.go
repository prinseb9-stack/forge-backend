package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"forge-backend/internal/auth"
)

type contextKey string

const (
	FirebaseUIDKey         contextKey = "firebase_uid"
	FirebaseEmailKey       contextKey = "firebase_email"
	FirebaseDisplayNameKey contextKey = "firebase_display_name"
)

// RequireAuth is a middleware that validates Firebase ID tokens
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check for development bypass
		if os.Getenv("DEV_AUTH_BYPASS") == "true" {
			ctx := context.WithValue(r.Context(), FirebaseUIDKey, "dev-user-123")
			ctx = context.WithValue(ctx, FirebaseEmailKey, "dev@forge.local")
			ctx = context.WithValue(ctx, FirebaseDisplayNameKey, "Dev User")
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			sendAuthError(w, "Authorization header is required", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			sendAuthError(w, "Authorization header must be 'Bearer <token>'", http.StatusUnauthorized)
			return
		}

		token := parts[1]
		if token == "" {
			sendAuthError(w, "Token is required", http.StatusUnauthorized)
			return
		}

		// Verify token - returns verified user info from Firebase (NOT client headers)
		verifiedUser, err := auth.VerifyIDToken(r.Context(), token)
		if err != nil {
			sendAuthError(w, "Invalid or expired authentication token", http.StatusUnauthorized)
			return
		}

		// Store verified identity in context
		ctx := context.WithValue(r.Context(), FirebaseUIDKey, verifiedUser.UID)
		ctx = context.WithValue(ctx, FirebaseEmailKey, verifiedUser.Email)
		ctx = context.WithValue(ctx, FirebaseDisplayNameKey, verifiedUser.DisplayName)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetFirebaseUID retrieves the verified Firebase UID from request context
func GetFirebaseUID(r *http.Request) (string, bool) {
	uid, ok := r.Context().Value(FirebaseUIDKey).(string)
	return uid, ok
}

// GetFirebaseEmail retrieves the verified Firebase email from request context
func GetFirebaseEmail(r *http.Request) (string, bool) {
	email, ok := r.Context().Value(FirebaseEmailKey).(string)
	return email, ok
}

// GetFirebaseDisplayName retrieves the verified display name from request context
func GetFirebaseDisplayName(r *http.Request) (string, bool) {
	name, ok := r.Context().Value(FirebaseDisplayNameKey).(string)
	return name, ok
}

func sendAuthError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   message,
	})
}
