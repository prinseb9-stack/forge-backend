package services

import (
	"fmt"
	"time"
)

// FallbackProvider tries a primary provider, then a fallback if the primary fails
type FallbackProvider struct {
	primary  AIProvider
	fallback AIProvider
	name     string
}

// NewFallbackProvider creates a new fallback wrapper
func NewFallbackProvider(name string, primary, fallback AIProvider) *FallbackProvider {
	return &FallbackProvider{
		name:     name,
		primary:  primary,
		fallback: fallback,
	}
}

// Generate implements the AIProvider interface
func (f *FallbackProvider) Generate(
	sourceContent string,
	platforms []string,
) (string, error) {
	start := time.Now()

	// Try primary
	content, err := f.primary.Generate(sourceContent, platforms)
	if err == nil {
		return content, nil
	}

	fmt.Printf("⚠️  %s primary failed (%v) — trying fallback\n", f.name, err)

	// Try fallback
	content, fbErr := f.fallback.Generate(sourceContent, platforms)
	if fbErr == nil {
		fmt.Printf("✅ %s fallback succeeded after %v\n", f.name, time.Since(start))
		return content, nil
	}

	return "", fmt.Errorf("%s both providers failed — primary: %v; fallback: %v", f.name, err, fbErr)
}
