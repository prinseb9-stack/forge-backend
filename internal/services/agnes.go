package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// AgnesService implements AIProvider using Agnes AI's text model
type AgnesService struct {
	APIKey  string
	BaseURL string
}

type agnesRequest struct {
	Model       string         `json:"model"`
	Messages    []agnesMessage `json:"messages"`
	Temperature float64        `json:"temperature"`
}

type agnesMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type agnesResponse struct {
	Choices []struct {
		Message struct {
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"message"`
	} `json:"choices"`
}

// NewAgnesService creates a new Agnes AI text service
func NewAgnesService(apiKey, baseURL string) *AgnesService {
	if baseURL == "" {
		baseURL = "https://apihub.agnes-ai.com/v1"
	}
	return &AgnesService{
		APIKey:  apiKey,
		BaseURL: strings.TrimRight(baseURL, "/"),
	}
}

// Generate implements the AIProvider interface
func (s *AgnesService) Generate(
	sourceContent string,
	platforms []string,
) (string, error) {
	if s.APIKey == "" {
		return "", fmt.Errorf("Agnes API key is not configured")
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

RETURN FORMAT - STRICT JSON (no markdown, no code fences):
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
5. Escape newlines inside strings as \n
6. Adapt writing style to each platform
7. Preserve the original meaning
`, platformList, len(platforms), sourceContent, len(platforms), platformList)

	requestBody := agnesRequest{
		Model: "agnes-2.0-flash",
		Messages: []agnesMessage{
			{
				Role:    "system",
				Content: "You are FORGE, a professional content repurposing assistant. You ALWAYS return valid JSON with exactly the number of results requested. Never wrap JSON in markdown. Escape newlines inside strings.",
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
		return "", fmt.Errorf("failed to encode Agnes request: %w", err)
	}

	url := s.BaseURL + "/chat/completions"

	// Retry strategy that specifically handles 429 rate limits
	type attempt struct {
		delay       time.Duration
		isRateLimit bool // if true, this delay was chosen because of a 429
	}

	// Attempts: [immediate, 5s, 15s, 30s, 60s]
	// 429s trigger longer waits
	schedule := []time.Duration{0, 2 * time.Second, 5 * time.Second, 10 * time.Second}

	var lastErr error
	var lastWasRateLimit bool

	for i, delay := range schedule {
		// For attempts after a 429, use the schedule's delay (which increases)
		if delay > 0 {
			if lastWasRateLimit {
				fmt.Printf("⏳ Agnes rate limited — waiting %v before retry %d\n", delay, i+1)
			}
			time.Sleep(delay)
		}

		content, err := s.doRequest(url, body)
		if err == nil {
			if i > 0 {
				fmt.Printf("✅ Agnes succeeded on attempt %d\n", i+1)
			}
			return content, nil
		}
		lastErr = err

		// Check error type
		errStr := err.Error()
		lastWasRateLimit = strings.Contains(errStr, "HTTP status 429")

		// Don't retry on 4xx errors EXCEPT 429
		if strings.Contains(errStr, "HTTP status 4") && !lastWasRateLimit {
			return "", err
		}

		// Don't retry on 401/403 (auth) even if they're technically 4xx
		if strings.Contains(errStr, "HTTP status 401") || strings.Contains(errStr, "HTTP status 403") {
			return "", err
		}

		fmt.Printf("⚠️  Agnes attempt %d failed: %v\n", i+1, err)
	}

	return "", fmt.Errorf("Agnes failed after %d attempts (last was rate limit: %v): %w",
		len(schedule), lastWasRateLimit, lastErr)
}

// doRequest performs a single HTTP request to Agnes
func (s *AgnesService) doRequest(url string, body []byte) (string, error) {
	req, err := http.NewRequest(
		http.MethodPost,
		url,
		bytes.NewBuffer(body),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create Agnes request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.APIKey)

	client := &http.Client{
		Timeout: 120 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives:     true,
			ResponseHeaderTimeout: 90 * time.Second,
			IdleConnTimeout:       30 * time.Second,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Agnes request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Agnes returned HTTP status %d", resp.StatusCode)
	}

	var result agnesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode Agnes response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("Agnes returned no choices")
	}

	return result.Choices[0].Message.Content, nil
}
