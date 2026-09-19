package models

import "time"

// ScheduledPostStatus tracks the lifecycle of a scheduled post
type ScheduledPostStatus string

const (
	ScheduleStatusPending   ScheduledPostStatus = "pending"
	ScheduleStatusPosted    ScheduledPostStatus = "posted"
	ScheduleStatusFailed    ScheduledPostStatus = "failed"
	ScheduleStatusCancelled ScheduledPostStatus = "cancelled"
)

// ScheduledPost represents a post scheduled to go out at a future time.
// NOTE: Auto-posting requires OAuth (Feature #3) + connection to be built.
// Until then, scheduled posts stay in "pending" status indefinitely.
type ScheduledPost struct {
	ID           string              `firestore:"id" json:"id"`
	UID          string              `firestore:"uid" json:"uid"`
	Platform     string              `firestore:"platform" json:"platform"`
	Content      string              `firestore:"content" json:"content"`
	ScheduledFor time.Time           `firestore:"scheduledFor" json:"scheduledFor"`
	Status       ScheduledPostStatus `firestore:"status" json:"status"`
	Notes        string              `firestore:"notes,omitempty" json:"notes,omitempty"`
	CreatedAt    time.Time           `firestore:"createdAt" json:"createdAt"`
	UpdatedAt    time.Time           `firestore:"updatedAt" json:"updatedAt"`
}
