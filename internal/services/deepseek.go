package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type DeepSeekService struct {
	APIKey string
}

type deepSeekRequest struct {
	Model       string            `json:"model"`
	Messages    []deepSeekMessage `json:"messages"`
	Temperature float64           `json:"temperature"`
}

type deepSeekMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type deepSeekResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// PlatformResult represents a single platform's generated content
type PlatformResult struct {
	Platform string `json:"platform"`
	Content  string `json:"content"`
}

// GenerateResponse wraps the results for the handler
type GenerateResponse struct {
	Results []PlatformResult `json:"results"`
}

// NewDeepSeekService creates a new DeepSeek service with explicit API key
func NewDeepSeekService(apiKey string) *DeepSeekService {
	return &DeepSeekService{
		APIKey: apiKey,
	}
}

// Generate implements the AIProvider interface
func (s *DeepSeekService) Generate(
	sourceContent string,
	platforms []string,
) (string, error) {
	if s.APIKey == "" {
		return "", fmt.Errorf("DeepSeek API key is not configured")
	}

	// Build platform instructions
	platformInstructions := buildPlatformInstructions(platforms)

	// Create a comma-separated list of platforms for the prompt
	platformList := strings.Join(platforms, ", ")

	prompt := fmt.Sprintf(`
You are FORGE, an expert content repurposing AI.

CRITICAL: You MUST generate content for ALL of these platforms: %s
You MUST return EXACTLY %d results - one for EACH platform.

Transform the source content below into high-quality, platform-specific content.

SOURCE CONTENT:
%s

%s

RETURN FORMAT - STRICT JSON:
{
  "results": [
    {"platform": "platform_name", "content": "generated content for this platform"},
    {"platform": "platform_name", "content": "generated content for this platform"},
    ...
  ]
}

RULES:
1. Return EXACTLY %d results (one for EACH platform: %s)
2. Do NOT add extra platforms or omit any
3. Do NOT wrap in markdown code blocks
4. Return ONLY valid JSON
5. Adapt writing style to each platform
6. Preserve the original meaning
`, platformList, len(platforms), sourceContent, platformInstructions, len(platforms), platformList)

	requestBody := deepSeekRequest{
		Model: "deepseek-chat",
		Messages: []deepSeekMessage{
			{
				Role:    "system",
				Content: "You are FORGE, a professional content repurposing assistant. You ALWAYS return valid JSON with exactly the number of results requested. Never omit platforms.",
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Temperature: 0.7,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to encode DeepSeek request: %w", err)
	}

	req, err := http.NewRequest(
		http.MethodPost,
		"https://api.deepseek.com/chat/completions",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create DeepSeek request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.APIKey)

	client := &http.Client{
		Timeout: 90 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("DeepSeek request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("DeepSeek returned HTTP status %d", resp.StatusCode)
	}

	var result deepSeekResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode DeepSeek response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("DeepSeek returned no choices")
	}

	return result.Choices[0].Message.Content, nil
}

// buildPlatformInstructions creates platform-specific guidance
func buildPlatformInstructions(platforms []string) string {
	instructions := "Platform-specific guidelines:\n\n"

	for _, p := range platforms {
		switch strings.ToLower(p) {
		case "x", "twitter":
			instructions += "X (Twitter): Keep under 280 characters, punchy, engaging, use relevant hashtags\n"
		case "instagram", "ig":
			instructions += "Instagram: Create an engaging caption with emojis, use line breaks, add relevant hashtags\n"
		case "facebook", "fb":
			instructions += "Facebook: Conversational tone, encourage engagement, use questions\n"
		case "linkedin", "li":
			instructions += "LinkedIn: Professional but human tone, focus on insights and value\n"
		case "blog", "blogs":
			instructions += "Blog: Structured format with headings, 300-500 words, SEO-friendly\n"
		case "newsletter", "newsletters":
			instructions += "Newsletter: Personal tone, storytelling approach, clear value proposition\n"
		default:
			instructions += p + ": Standard format\n"
		}
	}
	instructions += "\nIMPORTANT: Create ONE result for EACH platform listed above."

	return instructions
}
