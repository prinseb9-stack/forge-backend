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
	"forge-backend/internal/models"
	"forge-backend/internal/services"
)

func main() {
	devBypass := os.Getenv("DEV_AUTH_BYPASS") == "true"

	if devBypass {
		log.Println("⚠️  DEV_AUTH_BYPASS is ENABLED - Token verification is DISABLED")
	}

	// Initialize Firebase Admin
	log.Println("🔐 Initializing Firebase Admin SDK...")
	if err := auth.InitFirebaseAdmin(); err != nil {
		log.Fatalf("🔥 Firebase Admin initialization failed: %v", err)
	}
	log.Println("✅ Firebase Admin SDK initialized successfully")

	if auth.GetFirestoreClient() == nil {
		log.Fatal("🔥 Firestore client is nil after initialization")
	}

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create provider factory (text)
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

	// Create handlers
	generateHandler := handlers.NewGenerateHandler(factory, firestoreService)

	agnesImageService := services.NewAgnesImageService(cfg.AgnesAPIKey, cfg.AgnesBaseURL)
	generateImageHandler := handlers.NewGenerateImageHandler(agnesImageService, firestoreService)

	meHandler := handlers.NewMeHandler(firestoreService)
	scheduleHandler := handlers.NewScheduleHandler(firestoreService)

	// ─── Flutterwave payments ───
	flutterwaveService := services.NewFlutterwaveService(services.FlutterwaveConfig{
		PublicKey:     cfg.FlutterwavePublicKey,
		SecretKey:     cfg.FlutterwaveSecretKey,
		EncryptionKey: cfg.FlutterwaveEncryptionKey,
		WebhookSecret: cfg.FlutterwaveWebhookSecret,
		Env:           cfg.FlutterwaveEnv,
		PlanPro:       cfg.FlutterwavePlanPro,
		PlanHigherPro: cfg.FlutterwavePlanHigherPro,
	})

	// Register Flutterwave plan ID → internal plan mapping
	models.RegisterPlanMapping(map[string]models.Plan{
		cfg.FlutterwavePlanPro:       models.PlanPro,
		cfg.FlutterwavePlanHigherPro: models.PlanHigherPro,
	})

	paymentsHandler := handlers.NewPaymentsHandler(flutterwaveService, firestoreService)

	// ─── Routes ───
	r := chi.NewRouter()

	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(forgemiddleware.CORS)

	// Public routes
	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"service": "FORGE API",
			"status":  "online",
		})
	})

	// Flutterwave webhook — public, verified by signature header
	r.Post("/api/payments/webhook", paymentsHandler.HandleWebhook)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(forgemiddleware.RequireAuth)

		// User + generation
		r.Get("/api/me", meHandler.ServeHTTP)
		r.Post("/api/generate", generateHandler.ServeHTTP)
		r.Post("/api/generate-image", generateImageHandler.ServeHTTP)

		// Payments
		r.Post("/api/payments/checkout", paymentsHandler.HandleCheckout)
		r.Get("/api/subscription", paymentsHandler.HandleGetSubscription)

		// Scheduled posts
		r.Post("/api/scheduled", scheduleHandler.HandleCreate)
		r.Get("/api/scheduled", scheduleHandler.HandleList)
		r.Delete("/api/scheduled/{id}", scheduleHandler.HandleDelete)
	})

	port := cfg.Port
	if port == "" {
		port = "8080"
	}

	log.Printf("🔥 FORGE API running on http://localhost:%s", port)
	log.Println("🔐 Protected: /api/me, /api/generate, /api/generate-image, /api/payments/*, /api/scheduled/*")
	log.Println("🌐 Public: /api/health, /api/payments/webhook")
	log.Println("🤖 AI: Groq primary + Agnes fallback")
	log.Println("💳 Payments: Flutterwave")

	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatal(err)
	}
}
