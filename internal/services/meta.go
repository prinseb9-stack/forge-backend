package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// MetaService implements AIProvider using Meta's Llama models via OpenRouter
type MetaService struct {
	APIKey string
	URL    string
}

type metaRequest struct {
	Model       string        `json:"model"`
	Messages    []metaMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type metaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type metaResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// NewMetaService creates a new Meta Llama service using OpenRouter
func NewMetaService(apiKey, url string) *MetaService {
	if url == "" {
		url = "https://openrouter.ai/api/v1/chat/completions"
	}
	return &MetaService{
		APIKey: apiKey,
		URL:    url,
	}
}

// Generate implements the AIProvider interface
func (s *MetaService) Generate(
	sourceContent string,
	platforms []string,
) (string, error) {
	if s.APIKey == "" {
		return "", fmt.Errorf("OpenRouter API key is not configured for Meta Llama")
	}

	platformList := strings.Join(platforms, ", ")

	prompt := fmt.Sprintf(`
You are FORGE, an expert content repurposing AI using Meta's Llama 4 model.

CRITICAL: You MUST generate content for ALL of these platforms: %s
You MUST return EXACTLY %d results - one for EACH platform.

Transform the source content below into high-quality, platform-specific content.

SOURCE CONTENT:
%s

Platform-specific guidelines:
- X/Twitter: Keep under 280 characters, punchy, engaging, use relevant hashtags
- Instagram: Create an engaging caption with emojis, use line breaks, add relevant hashtags
- Facebook: Conversational tone, encourage engagement, use questions
- LinkedIn: Professional but human tone, focus on insights and value
- Blog: Structured format with headings, 300-500 words, SEO-friendly
- Newsletter: Personal tone, storytelling approach, clear value proposition

RETURN FORMAT - STRICT JSON (do not wrap in markdown, do not add extra text):
{
  "results": [
    {"platform": "platform_name", "content": "generated content for this platform"},
    {"platform": "platform_name", "content": "generated content for this platform"}
  ]
}

RULES:
1. Return EXACTLY %d results (one for EACH platform: %s)
2. Do NOT add extra platforms or omit any
3. Do NOT wrap in markdown code blocks (NO backticks or json)
4. Return ONLY valid JSON
5. Escape newlines inside strings using backslash n
6. Adapt writing style to each platform
7. Preserve the original meaning
8. Generate high-quality, insightful content
`, platformList, len(platforms), sourceContent, len(platforms), platformList)

	requestBody := metaRequest{
		Model: "meta-llama/llama-4-scout",
		Messages: []metaMessage{
			{
				Role:    "system",
				Content: "You are FORGE, a professional content repurposing assistant using Meta Llama 4. You ALWAYS return valid JSON with exactly the number of results requested. Never omit platforms. Never wrap JSON in markdown code blocks. Always escape newlines inside string values. Generate high-quality, insightful content.",
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
		return "", fmt.Errorf("failed to encode Meta Llama request: %w", err)
	}

	req, err := http.NewRequest(
		http.MethodPost,
		s.URL,
		bytes.NewBuffer(body),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create Meta Llama request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.APIKey)

	client := &http.Client{
		Timeout: 120 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Meta Llama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Meta Llama returned HTTP status %d", resp.StatusCode)
	}

	var result metaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode Meta Llama response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("Meta Llama returned no choices")
	}

	return result.Choices[0].Message.Content, nil
}
