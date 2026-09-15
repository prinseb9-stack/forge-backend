package models

import "time"

// ReservationKind identifies which usage counter a reservation belongs to
type ReservationKind string

const (
	ReservationKindText  ReservationKind = "text"
	ReservationKindImage ReservationKind = "image"
	ReservationKindVideo ReservationKind = "video"
)

// ReservationState tracks the lifecycle of a usage reservation
type ReservationState string

const (
	ReservationStateReserved ReservationState = "reserved"
	ReservationStateConsumed ReservationState = "consumed"
	ReservationStateRefunded ReservationState = "refunded"
)

// UsageReservation represents a single reserved usage slot for a request.
type UsageReservation struct {
	ReservationID   string           `firestore:"reservationId" json:"reservationId"`
	Kind            ReservationKind  `firestore:"kind" json:"kind"`
	State           ReservationState `firestore:"state" json:"state"`
	PeriodStartedAt time.Time        `firestore:"periodStartedAt" json:"periodStartedAt"`
	PeriodEndsAt    time.Time        `firestore:"periodEndsAt" json:"periodEndsAt"`
	CreatedAt       time.Time        `firestore:"createdAt" json:"createdAt"`
	UpdatedAt       time.Time        `firestore:"updatedAt" json:"updatedAt"`
}
