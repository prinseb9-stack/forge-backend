package models

import "time"

// GenerationKind identifies the type of AI-generated output
type GenerationKind string

const (
	GenerationKindText  GenerationKind = "text"
	GenerationKindImage GenerationKind = "image"
	GenerationKindVideo GenerationKind = "video"
)

// ImageGeneration is the stored metadata for a generated image
type ImageGeneration struct {
	ID        string    `firestore:"id" json:"id"`
	Kind      string    `firestore:"kind" json:"kind"`
	Prompt    string    `firestore:"prompt" json:"prompt"`
	Model     string    `firestore:"model" json:"model"`
	URL       string    `firestore:"url" json:"url"`
	Size      string    `firestore:"size" json:"size"`
	CreatedAt time.Time `firestore:"createdAt" json:"createdAt"`
}

// EditGeneration is the stored metadata for an edited image.
// Unlike ImageGeneration, it also records the source object key that
// was edited, so users can see "I made this from that".
type EditGeneration struct {
	ID        string    `firestore:"id" json:"id"`
	Kind      string    `firestore:"kind" json:"kind"`
	Prompt    string    `firestore:"prompt" json:"prompt"`
	Model     string    `firestore:"model" json:"model"`
	SourceKey string    `firestore:"sourceKey" json:"sourceKey"`
	ResultKey string    `firestore:"resultKey" json:"resultKey"`
	ResultURL string    `firestore:"resultUrl" json:"resultUrl"`
	Size      string    `firestore:"size" json:"size"`
	CreatedAt time.Time `firestore:"createdAt" json:"createdAt"`
}

// ValidImageSizes defines the sizes Agnes accepts
var ValidImageSizes = map[string]bool{
	"1024x1024": true,
	"1792x1024": true,
	"1024x1792": true,
}

// IsValidImageSize checks if a size is allowed
func IsValidImageSize(size string) bool {
	return ValidImageSizes[size]
}
