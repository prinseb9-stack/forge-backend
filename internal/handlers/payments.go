package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"forge-backend/internal/middleware"
	"forge-backend/internal/models"
	"forge-backend/internal/services"
)

type PaymentsHandler struct {
	flutterwave *services.FlutterwaveService
	firestore   *services.FirestoreService
}

func NewPaymentsHandler(fw *services.FlutterwaveService, fs *services.FirestoreService) *PaymentsHandler {
	return &PaymentsHandler{flutterwave: fw, firestore: fs}
}

// ═══════════════════════════════════════════════════════════
// POST /api/payments/checkout
// Body: { "plan": "pro" | "higher_pro" }
// Returns: { "success": true, "checkoutURL": "https://..." }
// ═══════════════════════════════════════════════════════════

type CheckoutRequest struct {
	Plan string `json:"plan"`
}

type CheckoutResponse struct {
	Success     bool   `json:"success"`
	CheckoutURL string `json:"checkoutURL,omitempty"`
	Error       string `json:"error,omitempty"`
	Code        string `json:"code,omitempty"`
}

func (h *PaymentsHandler) HandleCheckout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uid, ok := middleware.GetFirebaseUID(r)
	if !ok || uid == "" {
		jsonResponse(w, http.StatusUnauthorized, CheckoutResponse{
			Success: false, Error: "Authentication required", Code: "UNAUTHENTICATED",
		})
		return
	}

	email, _ := middleware.GetFirebaseEmail(r)
	displayName, _ := middleware.GetFirebaseDisplayName(r)

	var req CheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, CheckoutResponse{
			Success: false, Error: "Invalid request body", Code: "INVALID_REQUEST",
		})
		return
	}

	planName := strings.ToLower(strings.TrimSpace(req.Plan))
	if planName != "pro" && planName != "higher_pro" {
		jsonResponse(w, http.StatusBadRequest, CheckoutResponse{
			Success: false, Error: "Plan must be 'pro' or 'higher_pro'", Code: "INVALID_PLAN",
		})
		return
	}

	planID := h.flutterwave.PlanIDForPlan(planName)
	if planID == "" {
		log.Printf("🔥 Flutterwave plan ID not configured for %s", planName)
		jsonResponse(w, http.StatusInternalServerError, CheckoutResponse{
			Success: false, Error: "Payment plan not configured", Code: "CONFIG_ERROR",
		})
		return
	}

	amount := h.flutterwave.AmountForPlan(planName)
	txRef := services.GenerateTxRef(uid)

	// Redirect back to frontend after payment
	redirectURL := "https://forge777.netlify.app/payment/callback?tx_ref=" + txRef

	checkoutURL, err := h.flutterwave.InitializeCheckout(
		r.Context(),
		txRef,
		amount,
		planID,
		email,
		displayName,
		redirectURL,
	)
	if err != nil {
		log.Printf("🔥 Checkout failed for uid=%s plan=%s: %v", uid, planName, err)
		jsonResponse(w, http.StatusInternalServerError, CheckoutResponse{
			Success: false, Error: "Failed to start checkout", Code: "CHECKOUT_ERROR",
		})
		return
	}

	// Save pending subscription record so we know what to expect on webhook
	err = h.firestore.SavePendingSubscription(r.Context(), uid, txRef, planName, planID, amount)
	if err != nil {
		log.Printf("⚠️  Failed to save pending subscription for uid=%s: %v", uid, err)
		// Non-fatal — continue with checkout
	}

	jsonResponse(w, http.StatusOK, CheckoutResponse{
		Success:     true,
		CheckoutURL: checkoutURL,
	})
}

// ═══════════════════════════════════════════════════════════
// POST /api/payments/webhook
// Flutterwave calls this on payment events
// ═══════════════════════════════════════════════════════════

func (h *PaymentsHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Verify the webhook came from Flutterwave
	verifHash := r.Header.Get("verif-hash")
	if !h.flutterwave.ValidateWebhookHash(verifHash) {
		log.Printf("⚠️  Rejected webhook with invalid verif-hash")
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("⚠️  Failed to read webhook body: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var payload struct {
		Event string `json:"event"`
		Data  struct {
			ID       int64   `json:"id"`
			TxRef    string  `json:"tx_ref"`
			Amount   float64 `json:"amount"`
			Currency string  `json:"currency"`
			Status   string  `json:"status"`
			Customer struct {
				ID    int64  `json:"id"`
				Email string `json:"email"`
				Name  string `json:"name"`
			} `json:"customer"`
			PaymentPlan string `json:"payment_plan,omitempty"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("⚠️  Failed to parse webhook payload: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	log.Printf("📩 Webhook received: event=%s tx_ref=%s status=%s",
		payload.Event, payload.Data.TxRef, payload.Data.Status)

	// Handle charge.completed
	if payload.Event == "charge.completed" && payload.Data.Status == "successful" {
		// Verify with Flutterwave that this transaction is real
		verify, err := h.flutterwave.VerifyTransactionByID(r.Context(), fmt.Sprintf("%d", payload.Data.ID))
		if err != nil {
			log.Printf("🔥 Failed to verify transaction: %v", err)
			http.Error(w, "Verification failed", http.StatusInternalServerError)
			return
		}

		if verify.Data.Status != "successful" {
			log.Printf("⚠️  Transaction not successful per Flutterwave: %s", verify.Data.Status)
			http.Error(w, "Not successful", http.StatusOK)
			return
		}

		// Find which user this subscription belongs to using txRef
		uid, planName, err := h.firestore.FindUserByTxRef(r.Context(), payload.Data.TxRef)
		if err != nil {
			log.Printf("🔥 Could not find user for txRef=%s: %v", payload.Data.TxRef, err)
			http.Error(w, "User not found", http.StatusInternalServerError)
			return
		}

		// Upgrade the user's plan
		periodEnd := time.Now().Add(30 * 24 * time.Hour)
		err = h.firestore.ActivateSubscription(r.Context(), uid, models.Subscription{
			Provider:         "flutterwave",
			PlanID:           payload.Data.PaymentPlan,
			PlanName:         planName,
			CustomerID:       fmt.Sprintf("%d", payload.Data.Customer.ID),
			SubscriptionID:   verify.Data.FlwRef,
			TxRef:            payload.Data.TxRef,
			Status:           models.SubscriptionStatusActive,
			Amount:           payload.Data.Amount,
			Currency:         payload.Data.Currency,
			CurrentPeriodEnd: periodEnd,
		})
		if err != nil {
			log.Printf("🔥 Failed to activate subscription for uid=%s: %v", uid, err)
			http.Error(w, "Activation failed", http.StatusInternalServerError)
			return
		}

		log.Printf("✅ Subscription activated: uid=%s plan=%s txRef=%s",
			uid, planName, payload.Data.TxRef)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// ═══════════════════════════════════════════════════════════
// GET /api/subscription
// Returns current user's subscription status
// ═══════════════════════════════════════════════════════════

type SubscriptionResponse struct {
	Success      bool                 `json:"success"`
	Plan         string               `json:"plan"`
	Subscription *models.Subscription `json:"subscription,omitempty"`
	Error        string               `json:"error,omitempty"`
}

func (h *PaymentsHandler) HandleGetSubscription(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	uid, ok := middleware.GetFirebaseUID(r)
	if !ok || uid == "" {
		jsonResponse(w, http.StatusUnauthorized, SubscriptionResponse{
			Success: false, Error: "Authentication required",
		})
		return
	}

	user, err := h.firestore.GetUser(r.Context(), uid)
	if err != nil {
		log.Printf("🔥 Failed to load user %s: %v", uid, err)
		jsonResponse(w, http.StatusInternalServerError, SubscriptionResponse{
			Success: false, Error: "Failed to load user",
		})
		return
	}

	jsonResponse(w, http.StatusOK, SubscriptionResponse{
		Success:      true,
		Plan:         string(user.Plan),
		Subscription: user.Subscription,
	})
}
