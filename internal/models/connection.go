package models

import "time"

// ConnectionStatus tracks the lifecycle of a stored platform connection.
type ConnectionStatus string

const (
	ConnectionStatusActive  ConnectionStatus = "active"
	ConnectionStatusRevoked ConnectionStatus = "revoked"
)

// Connection represents a user's linked account on a platform.
//
// Stored at: users/{uid}/connections/{platformId}
//
// Tokens (access + refresh JWTs) are stored encrypted at rest using
// the crypto.EncryptionService. They are never returned to the
// frontend and never logged.
type Connection struct {
	// Platform identity
	PlatformID string `firestore:"platformId" json:"platformId"`

	// Platform-specific user identity
	DID         string `firestore:"did" json:"did"`
	Handle      string `firestore:"handle" json:"handle"`
	DisplayName string `firestore:"displayName,omitempty" json:"displayName,omitempty"`
	Avatar      string `firestore:"avatar,omitempty" json:"avatar,omitempty"`

	// Encrypted credentials
	EncryptedAccessJwt  string    `firestore:"encryptedAccessJwt" json:"-"`
	EncryptedRefreshJwt string    `firestore:"encryptedRefreshJwt" json:"-"`
	AccessExpiresAt     time.Time `firestore:"accessExpiresAt" json:"-"`

	// Lifecycle
	Status      ConnectionStatus `firestore:"status" json:"status"`
	ConnectedAt time.Time        `firestore:"connectedAt" json:"connectedAt"`
	UpdatedAt   time.Time        `firestore:"updatedAt" json:"updatedAt"`
}

// PublicConnection is what the API returns. It excludes all sensitive fields.
type PublicConnection struct {
	PlatformID  string           `json:"platformId"`
	Handle      string           `json:"handle"`
	DisplayName string           `json:"displayName,omitempty"`
	Avatar      string           `json:"avatar,omitempty"`
	Status      ConnectionStatus `json:"status"`
	ConnectedAt time.Time        `json:"connectedAt"`
}

// ToPublic converts a Connection to its safe, frontend-facing form.
func (c *Connection) ToPublic() PublicConnection {
	return PublicConnection{
		PlatformID:  c.PlatformID,
		Handle:      c.Handle,
		DisplayName: c.DisplayName,
		Avatar:      c.Avatar,
		Status:      c.Status,
		ConnectedAt: c.ConnectedAt,
	}
}
