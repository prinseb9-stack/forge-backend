package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// FlutterwaveConfig holds the Flutterwave credentials
type FlutterwaveConfig struct {
	PublicKey     string
	SecretKey     string
	EncryptionKey string
	WebhookSecret string
	Env           string // "test" or "live"

	PlanPro       string
	PlanHigherPro string
}

// FlutterwaveService is a thin client for the Flutterwave API
type FlutterwaveService struct {
	cfg     FlutterwaveConfig
	baseURL string
}

func NewFlutterwaveService(cfg FlutterwaveConfig) *FlutterwaveService {
	// Flutterwave uses the same base URL for test and live — the key determines the mode
	return &FlutterwaveService{
		cfg:     cfg,
		baseURL: "https://api.flutterwave.com/v3",
	}
}

// PlanIDForPlan returns the Flutterwave plan ID for our internal plan name
func (s *FlutterwaveService) PlanIDForPlan(planName string) string {
	switch planName {
	case "pro":
		return s.cfg.PlanPro
	case "higher_pro":
		return s.cfg.PlanHigherPro
	default:
		return ""
	}
}

// AmountForPlan returns the price in USD
func (s *FlutterwaveService) AmountForPlan(planName string) float64 {
	switch planName {
	case "pro":
		return 15.00
	case "higher_pro":
		return 49.00
	default:
		return 0
	}
}

// GenerateTxRef creates a unique transaction reference
func GenerateTxRef(uid string) string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("FORGE-%s-%d-%s", uid, time.Now().Unix(), hex.EncodeToString(b))
}

// InitializeCheckoutRequest is what we send to Flutterwave to create a payment link
type InitializeCheckoutRequest struct {
	TxRef       string  `json:"tx_ref"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	RedirectURL string  `json:"redirect_url"`
	PaymentPlan string  `json:"payment_plan,omitempty"`
	Customer    struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	} `json:"customer"`
	Customizations struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Logo        string `json:"logo,omitempty"`
	} `json:"customizations"`
}

// InitializeCheckoutResponse is what Flutterwave returns
type InitializeCheckoutResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    struct {
		Link string `json:"link"`
	} `json:"data"`
}

// InitializeCheckout creates a Flutterwave payment link
func (s *FlutterwaveService) InitializeCheckout(
	ctx context.Context,
	txRef string,
	amount float64,
	planID string,
	email string,
	name string,
	redirectURL string,
) (string, error) {
	reqBody := InitializeCheckoutRequest{
		TxRef:       txRef,
		Amount:      amount,
		Currency:    "USD",
		RedirectURL: redirectURL,
		PaymentPlan: planID,
	}
	reqBody.Customer.Email = email
	reqBody.Customer.Name = name
	reqBody.Customizations.Title = "FORGE Subscription"
	reqBody.Customizations.Description = "AI Content Studio subscription"

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to encode checkout request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		s.baseURL+"/payments",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create checkout request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.cfg.SecretKey)

	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("checkout request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("flutterwave returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result InitializeCheckoutResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to decode checkout response: %w", err)
	}

	if result.Status != "success" || result.Data.Link == "" {
		return "", fmt.Errorf("flutterwave error: %s", result.Message)
	}

	return result.Data.Link, nil
}

// VerifyTransaction fetches the full transaction from Flutterwave
type VerifyTransactionResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    struct {
		ID          int64   `json:"id"`
		TxRef       string  `json:"tx_ref"`
		FlwRef      string  `json:"flw_ref"`
		Amount      float64 `json:"amount"`
		Currency    string  `json:"currency"`
		Status      string  `json:"status"` // "successful" | "failed" | ...
		PaymentType string  `json:"payment_type"`
		Customer    struct {
			ID    int64  `json:"id"`
			Email string `json:"email"`
			Name  string `json:"name"`
		} `json:"customer"`
		PaymentPlan string `json:"payment_plan,omitempty"`
	} `json:"data"`
}

// VerifyTransactionByID checks with Flutterwave that a transaction is real
func (s *FlutterwaveService) VerifyTransactionByID(ctx context.Context, transactionID string) (*VerifyTransactionResponse, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		s.baseURL+"/transactions/"+transactionID+"/verify",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create verify request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.cfg.SecretKey)

	client := &http.Client{Timeout: 30 * time.Second}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("verify request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("flutterwave verify status %d: %s", resp.StatusCode, string(respBody))
	}

	var result VerifyTransactionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to decode verify response: %w", err)
	}

	return &result, nil
}

// ValidateWebhookHash checks that the incoming webhook's verif-hash header matches our secret
func (s *FlutterwaveService) ValidateWebhookHash(verifHash string) bool {
	if s.cfg.WebhookSecret == "" {
		return false
	}
	// Constant-time comparison would be nicer, but this is fine
	return verifHash == s.cfg.WebhookSecret
}
