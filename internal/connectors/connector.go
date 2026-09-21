// Package connectors defines the FORGE platform connector architecture.
//
// A connector is a self-contained description of a social platform that
// FORGE may integrate with. Every connector implements the Connector
// interface and self-registers in its package's init() function.
//
// The connector registry is the single source of truth for:
//   - which platforms exist in FORGE
//   - what each platform can do (capabilities)
//   - which category each platform belongs to
//
// Platform integrations (OAuth, publishing, scheduling, analytics) are
// added later as separate files inside each connector's package. This
// package is deliberately metadata-only.
package connectors

import (
	"sort"
	"sync"
)

// ═══════════════════════════════════════════════════════════════════
// CAPABILITY STATUS
// ═══════════════════════════════════════════════════════════════════

// Status represents the honest current state of a single capability
// (connect, publish, schedule, or analytics) for a given platform.
//
// Values are never aspirational. A capability is only "available" when
// it is actually implemented and verified.
type Status string

const (
	// StatusAvailable means the capability is implemented and working.
	StatusAvailable Status = "available"

	// StatusPlanned means the capability is on the roadmap but not built.
	StatusPlanned Status = "planned"

	// StatusComingSoon means the capability is planned but far out,
	// with no committed timeline.
	StatusComingSoon Status = "coming_soon"

	// StatusUnavailable means the capability is blocked — for example,
	// the platform requires a paid developer tier we do not have, or
	// its API does not support that capability at all.
	StatusUnavailable Status = "unavailable"
)

// ═══════════════════════════════════════════════════════════════════
// CATEGORIES
// ═══════════════════════════════════════════════════════════════════

// Category groups connectors by the type of platform they represent.
// Categories are used by the frontend to organize the platform hub
// and to differentiate non-feed platforms (like messaging apps) from
// feed-based ones.
type Category string

const (
	CategorySocial       Category = "social"
	CategoryProfessional Category = "professional"
	CategoryVisual       Category = "visual"
	CategoryVideo        Category = "video"
	CategoryPublishing   Category = "publishing"
	CategoryMessaging    Category = "messaging"
)

// ═══════════════════════════════════════════════════════════════════
// CAPABILITIES
// ═══════════════════════════════════════════════════════════════════

// Capabilities describes what a connector can do right now. Every
// field is independent — a platform may support publishing without
// supporting analytics, for example.
type Capabilities struct {
	Connect   Status `json:"connect"`
	Publish   Status `json:"publish"`
	Schedule  Status `json:"schedule"`
	Analytics Status `json:"analytics"`
}

// ═══════════════════════════════════════════════════════════════════
// CONNECTOR INTERFACE
// ═══════════════════════════════════════════════════════════════════

// Connector is the metadata contract every FORGE platform integration
// must satisfy. It describes a platform's identity and honest current
// capabilities.
//
// A connector that only exposes metadata (no OAuth, no publishing) is
// fully valid. Real functionality is added later by implementing
// additional optional interfaces inside the connector's package.
type Connector interface {
	// ID returns the canonical platform identifier used everywhere in
	// FORGE (frontend, backend, prompts, requests, responses).
	// Examples: "x", "linkedin", "youtube-shorts".
	ID() string

	// Name returns the human-readable display name.
	// Examples: "X (Twitter)", "LinkedIn", "YouTube Shorts".
	Name() string

	// Icon returns a single emoji used in the UI.
	Icon() string

	// Description is a neutral, one-line description of the platform
	// itself. It must NOT imply that FORGE already supports any
	// capability. The capability matrix is the authoritative statement.
	Description() string

	// Category groups the platform by type for UI organization.
	Category() Category

	// Capabilities returns the honest current state of each capability
	// for this platform.
	Capabilities() Capabilities
}

// ═══════════════════════════════════════════════════════════════════
// REGISTRY
// ═══════════════════════════════════════════════════════════════════

var (
	regMu    sync.RWMutex
	registry = make(map[string]Connector)
)

// Register adds a connector to the global registry. It is called from
// each connector package's init() function and should not be called
// anywhere else.
//
// Registering two connectors with the same ID panics at startup, which
// is intentional: duplicate IDs are a programming error, not a
// runtime condition to handle gracefully.
func Register(c Connector) {
	if c == nil {
		panic("connectors: cannot register nil connector")
	}
	id := c.ID()
	if id == "" {
		panic("connectors: cannot register connector with empty ID")
	}

	regMu.Lock()
	defer regMu.Unlock()

	if _, exists := registry[id]; exists {
		panic("connectors: duplicate connector ID registered: " + id)
	}
	registry[id] = c
}

// All returns every registered connector, sorted deterministically by
// ID ascending. The sort guarantees /api/connectors returns platforms
// in the same order on every request.
func All() []Connector {
	regMu.RLock()
	defer regMu.RUnlock()

	out := make([]Connector, 0, len(registry))
	for _, c := range registry {
		out = append(out, c)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].ID() < out[j].ID()
	})

	return out
}

// Get returns the connector for the given ID, or (nil, false) if no
// such connector is registered.
func Get(id string) (Connector, bool) {
	regMu.RLock()
	defer regMu.RUnlock()

	c, ok := registry[id]
	return c, ok
}

// Count returns the number of registered connectors. Useful for tests
// and startup logging.
func Count() int {
	regMu.RLock()
	defer regMu.RUnlock()
	return len(registry)
}
