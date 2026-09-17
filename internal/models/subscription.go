package models

import "time"

// SubscriptionStatus tracks the lifecycle of a Flutterwave subscription
type SubscriptionStatus string

const (
	SubscriptionStatusActive    SubscriptionStatus = "active"
	SubscriptionStatusCancelled SubscriptionStatus = "cancelled"
	SubscriptionStatusExpired   SubscriptionStatus = "expired"
	SubscriptionStatusPending   SubscriptionStatus = "pending"
)

// Subscription stores the user's paid subscription details.
type Subscription struct {
	Provider         string             `firestore:"provider" json:"provider"`             // "flutterwave"
	PlanID           string             `firestore:"planId" json:"planId"`                 // Flutterwave plan ID
	PlanName         string             `firestore:"planName" json:"planName"`             // "pro" | "higher_pro"
	CustomerID       string             `firestore:"customerId" json:"customerId"`         // Flutterwave customer ID
	SubscriptionID   string             `firestore:"subscriptionId" json:"subscriptionId"` // Flutterwave subscription ID
	TxRef            string             `firestore:"txRef" json:"txRef"`                   // Our unique reference
	Status           SubscriptionStatus `firestore:"status" json:"status"`
	Amount           float64            `firestore:"amount" json:"amount"`
	Currency         string             `firestore:"currency" json:"currency"`
	CurrentPeriodEnd time.Time          `firestore:"currentPeriodEnd" json:"currentPeriodEnd"`
	CreatedAt        time.Time          `firestore:"createdAt" json:"createdAt"`
	UpdatedAt        time.Time          `firestore:"updatedAt" json:"updatedAt"`
}

// PlanIDToName maps Flutterwave plan IDs to internal plan names.
// Set at startup from config.
var planIDToName = map[string]Plan{}

// RegisterPlanMapping sets up the plan ID → name map
func RegisterPlanMapping(planIDs map[string]Plan) {
	planIDToName = planIDs
}

// PlanFromFlutterwaveID returns the internal Plan for a Flutterwave plan ID
func PlanFromFlutterwaveID(id string) (Plan, bool) {
	p, ok := planIDToName[id]
	return p, ok
}
