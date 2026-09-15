package services

import (
	"fmt"

	"forge-backend/internal/models"
)

// ProviderFactory creates AI providers based on a plan
// CURRENT: Groq primary + Agnes fallback for all tiers
type ProviderFactory struct {
	groqAPIKey   string
	groqBaseURL  string
	agnesAPIKey  string
	agnesBaseURL string

	// Reserved for later
	openRouterAPIKey string
	deepSeekAPIKey   string
}

func NewProviderFactory(
	groqAPIKey, groqBaseURL string,
	agnesAPIKey, agnesBaseURL string,
	openRouterAPIKey, deepSeekAPIKey string,
) *ProviderFactory {
	return &ProviderFactory{
		groqAPIKey:       groqAPIKey,
		groqBaseURL:      groqBaseURL,
		agnesAPIKey:      agnesAPIKey,
		agnesBaseURL:     agnesBaseURL,
		openRouterAPIKey: openRouterAPIKey,
		deepSeekAPIKey:   deepSeekAPIKey,
	}
}

func (f *ProviderFactory) GetProvider(plan models.Plan) (AIProvider, error) {
	switch plan {
	case models.PlanFree:
		return f.createFreeProvider()
	case models.PlanPro:
		return f.createProProvider()
	case models.PlanHigherPro:
		return f.createHigherProProvider()
	default:
		return nil, fmt.Errorf("unknown plan: %s", plan)
	}
}

// createChainedProvider builds Groq primary + Agnes fallback
func (f *ProviderFactory) createChainedProvider(planName string) (AIProvider, error) {
	if f.groqAPIKey == "" {
		return nil, fmt.Errorf("GROQ_API_KEY is required for %s plan", planName)
	}

	groq := NewGroqService(f.groqAPIKey, f.groqBaseURL)

	// If Agnes key exists, use it as fallback
	if f.agnesAPIKey != "" {
		agnes := NewAgnesService(f.agnesAPIKey, f.agnesBaseURL)
		return NewFallbackProvider(planName, groq, agnes), nil
	}

	// Otherwise just Groq
	return groq, nil
}

func (f *ProviderFactory) createFreeProvider() (AIProvider, error) {
	return f.createChainedProvider("FREE")
}

func (f *ProviderFactory) createProProvider() (AIProvider, error) {
	return f.createChainedProvider("PRO")
}

func (f *ProviderFactory) createHigherProProvider() (AIProvider, error) {
	return f.createChainedProvider("HIGHER PRO")
}

func GetPlanName(plan models.Plan) string {
	switch plan {
	case models.PlanFree:
		return "Free (Groq)"
	case models.PlanPro:
		return "Pro (Groq)"
	case models.PlanHigherPro:
		return "Higher Pro (Groq)"
	default:
		return "Unknown"
	}
}
