package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"forge-backend/internal/auth"
	"forge-backend/internal/config"
	"forge-backend/internal/handlers"
	forgemiddleware "forge-backend/internal/middleware"
	"forge-backend/internal/services"
)

func main() {
	devBypass := os.Getenv("DEV_AUTH_BYPASS") == "true"

	if devBypass {
		log.Println("⚠️  DEV_AUTH_BYPASS is ENABLED - Token verification is DISABLED")
		log.Println("⚠️  Firestore will still be initialized for user/plan data")
	}

	// ALWAYS initialize Firebase Admin (Auth + Firestore)
	log.Println("🔐 Initializing Firebase Admin SDK...")
	if err := auth.InitFirebaseAdmin(); err != nil {
		log.Fatalf("🔥 Firebase Admin initialization failed: %v\n\n"+
			"Firebase Admin (Firestore) is required even in DEV_AUTH_BYPASS mode.\n"+
			"Set GOOGLE_APPLICATION_CREDENTIALS or place firebase-service-account.json in the backend directory.",
			err)
	}
	log.Println("✅ Firebase Admin SDK initialized successfully")

	if auth.GetFirestoreClient() == nil {
		log.Fatal("🔥 Firestore client is nil after initialization - cannot continue")
	}

	// Load ALL provider credentials
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create provider factory
	factory := services.NewProviderFactory(
		cfg.GroqAPIKey,
		cfg.GroqBaseURL,
		cfg.AgnesAPIKey,
		cfg.AgnesBaseURL,
		cfg.OpenRouterAPIKey,
		cfg.DeepSeekAPIKey,
	)

	// Create Firestore service
	firestoreService, err := services.NewFirestoreService(auth.GetFirestoreClient())
	if err != nil {
		log.Fatalf("Failed to create Firestore service: %v", err)
	}
	log.Println("✅ Firestore service initialized")

	// Initialize handler
	generateHandler := handlers.NewGenerateHandler(factory, firestoreService)

	r := chi.NewRouter()

	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(forgemiddleware.CORS)
	// Initialize image handler
	agnesImageService := services.NewAgnesImageService(cfg.AgnesAPIKey, cfg.AgnesBaseURL)
	generateImageHandler := handlers.NewGenerateImageHandler(agnesImageService, firestoreService)

	// Public routes
	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"service": "FORGE API",
			"status":  "online",
		})
	})

	// Protected routes
	// Initialize /api/me handler
	meHandler := handlers.NewMeHandler(firestoreService)

	r.Group(func(r chi.Router) {
		r.Use(forgemiddleware.RequireAuth)
		r.Get("/api/me", meHandler.ServeHTTP)
		r.Post("/api/generate", generateHandler.ServeHTTP)
		r.Post("/api/generate-image", generateImageHandler.ServeHTTP)
	})

	port := cfg.Port
	if port == "" {
		port = "8080"
	}

	log.Printf("🔥 FORGE API running on http://localhost:%s", port)
	log.Println("🔐 Protected routes: /api/generate")
	log.Println("🌐 Public routes: /api/health")
	log.Println("🤖 AI Provider: Groq (primary) + Agnes (fallback)")

	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}
