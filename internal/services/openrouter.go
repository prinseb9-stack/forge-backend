package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type OpenRouterService struct {
	APIKey string
	URL    string
}

type openRouterRequest struct {
	Model       string              `json:"model"`
	Messages    []openRouterMessage `json:"messages"`
	Temperature float64             `json:"temperature"`
}

type openRouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openRouterResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// NewOpenRouterService creates a new OpenRouter service with explicit API key
func NewOpenRouterService(apiKey, url string) *OpenRouterService {
	if url == "" {
		url = "https://openrouter.ai/api/v1/chat/completions"
	}
	return &OpenRouterService{
		APIKey: apiKey,
		URL:    url,
	}
}

// Generate implements the AIProvider interface
func (s *OpenRouterService) Generate(
	sourceContent string,
	platforms []string,
) (string, error) {
	if s.APIKey == "" {
		return "", fmt.Errorf("OpenRouter API key is not configured")
	}

	platformList := strings.Join(platforms, ", ")

	prompt := fmt.Sprintf(`
You are FORGE, an expert content repurposing AI.

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

RETURN FORMAT - STRICT JSON:
{
  "results": [
    {"platform": "platform_name", "content": "generated content for this platform"},
    {"platform": "platform_name", "content": "generated content for this platform"}
  ]
}

RULES:
1. Return EXACTLY %d results (one for EACH platform: %s)
2. Do NOT add extra platforms or omit any
3. Do NOT wrap in markdown code blocks
4. Return ONLY valid JSON
5. Adapt writing style to each platform
6. Preserve the original meaning
`, platformList, len(platforms), sourceContent, len(platforms), platformList)

	requestBody := openRouterRequest{
		Model: "google/gemma-4-31b-it:free",
		Messages: []openRouterMessage{
			{
				Role:    "system",
				Content: "You are FORGE, a professional content repurposing assistant. You ALWAYS return valid JSON with exactly the number of results requested.",
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
		return "", fmt.Errorf("failed to encode OpenRouter request: %w", err)
	}

	// Retry with backoff for network flakiness
	var lastErr error
	delays := []time.Duration{0, 3 * time.Second, 8 * time.Second}

	for attempt, delay := range delays {
		if delay > 0 {
			time.Sleep(delay)
		}

		content, err := s.doRequest(body)
		if err == nil {
			return content, nil
		}
		lastErr = err

		// Don't retry on 4xx errors (bad request, auth, etc.)
		if strings.Contains(err.Error(), "HTTP status 4") {
			return "", err
		}

		fmt.Printf("⚠️  OpenRouter attempt %d failed: %v\n", attempt+1, err)
	}

	return "", fmt.Errorf("OpenRouter failed after %d attempts: %w", len(delays), lastErr)
}

// doRequest performs a single HTTP request to OpenRouter
func (s *OpenRouterService) doRequest(body []byte) (string, error) {
	req, err := http.NewRequest(
		http.MethodPost,
		s.URL,
		bytes.NewBuffer(body),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create OpenRouter request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.APIKey)

	// Increased timeout + transport tuning for flaky mobile networks
	client := &http.Client{
		Timeout: 120 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives:     true, // avoid stale connections
			ResponseHeaderTimeout: 90 * time.Second,
			IdleConnTimeout:       30 * time.Second,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("OpenRouter request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("OpenRouter returned HTTP status %d", resp.StatusCode)
	}

	var result openRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode OpenRouter response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("OpenRouter returned no choices")
	}

	return result.Choices[0].Message.Content, nil
}
