package models

import (
	"time"
)

type Plan string

const (
	PlanFree      Plan = "free"
	PlanPro       Plan = "pro"
	PlanHigherPro Plan = "higher_pro"
)

func (p Plan) IsValid() bool {
	switch p {
	case PlanFree, PlanPro, PlanHigherPro:
		return true
	default:
		return false
	}
}

// UsagePeriod holds the counters for one feature kind
type UsagePeriod struct {
	UsageCount      int       `firestore:"usageCount" json:"used"`
	MaxUsage        int       `firestore:"maxUsage" json:"limit"`
	PeriodStartedAt time.Time `firestore:"periodStartedAt" json:"periodStartedAt"`
	PeriodEndsAt    time.Time `firestore:"periodEndsAt" json:"periodEndsAt"`
}

// User represents a FORGE user stored in Firestore
type User struct {
	UID         string `firestore:"uid" json:"uid"`
	Email       string `firestore:"email" json:"email"`
	DisplayName string `firestore:"displayName" json:"displayName"`
	Plan        Plan   `firestore:"plan" json:"plan"`

	TextGeneration  UsagePeriod `firestore:"textGeneration" json:"textGeneration"`
	ImageGeneration UsagePeriod `firestore:"imageGeneration" json:"imageGeneration"`
	VideoGeneration UsagePeriod `firestore:"videoGeneration" json:"videoGeneration"`

	Subscription *Subscription `firestore:"subscription,omitempty" json:"subscription,omitempty"`

	CreatedAt time.Time `firestore:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `firestore:"updatedAt" json:"updatedAt"`
}

// ═══ Plan limit helpers ═══

// DefaultMaxUsage returns max text generations for a plan
func DefaultMaxUsage(plan Plan) int {
	switch plan {
	case PlanFree:
		return 10
	case PlanPro, PlanHigherPro:
		return -1 // unlimited
	default:
		return 10
	}
}

// DefaultImageMaxUsage returns max images for a plan
func DefaultImageMaxUsage(plan Plan) int {
	switch plan {
	case PlanFree:
		return 0
	case PlanPro:
		return 5
	case PlanHigherPro:
		return -1
	default:
		return 0
	}
}

// DefaultVideoMaxUsage returns max videos for a plan
func DefaultVideoMaxUsage(plan Plan) int {
	switch plan {
	case PlanFree:
		return 0
	case PlanPro:
		return 10
	case PlanHigherPro:
		return -1
	default:
		return 0
	}
}

// MaxInputWords returns max input words allowed for a plan
// -1 = unlimited
func MaxInputWords(plan Plan) int {
	switch plan {
	case PlanFree:
		return 500
	case PlanPro:
		return 5000
	case PlanHigherPro:
		return -1
	default:
		return 500
	}
}

// MaxPlatforms returns max platforms per generation for a plan
// -1 = unlimited
func MaxPlatforms(plan Plan) int {
	switch plan {
	case PlanFree:
		return 1
	case PlanPro, PlanHigherPro:
		return 6
	default:
		return 1
	}
}

// CanSaveHistory returns true if the plan supports history
func CanSaveHistory(plan Plan) bool {
	return plan != PlanFree
}

// MaxHistoryItems returns how many generations to keep, -1 = unlimited
func MaxHistoryItems(plan Plan) int {
	switch plan {
	case PlanFree:
		return 0
	case PlanPro:
		return 100
	case PlanHigherPro:
		return -1
	default:
		return 0
	}
}

// PeriodDuration is the length of a usage period
const PeriodDuration = 30 * 24 * time.Hour

func NewUsagePeriod(plan Plan) UsagePeriod {
	now := time.Now().UTC()
	return UsagePeriod{
		UsageCount:      0,
		MaxUsage:        DefaultMaxUsage(plan),
		PeriodStartedAt: now,
		PeriodEndsAt:    now.Add(PeriodDuration),
	}
}

func NewImageUsagePeriod(plan Plan) UsagePeriod {
	now := time.Now().UTC()
	return UsagePeriod{
		UsageCount:      0,
		MaxUsage:        DefaultImageMaxUsage(plan),
		PeriodStartedAt: now,
		PeriodEndsAt:    now.Add(PeriodDuration),
	}
}

func NewVideoUsagePeriod(plan Plan) UsagePeriod {
	now := time.Now().UTC()
	return UsagePeriod{
		UsageCount:      0,
		MaxUsage:        DefaultVideoMaxUsage(plan),
		PeriodStartedAt: now,
		PeriodEndsAt:    now.Add(PeriodDuration),
	}
}
