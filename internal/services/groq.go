package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// GroqService implements AIProvider using Groq's fast inference API
// Endpoint: https://api.groq.com/openai/v1/chat/completions
// Model: llama-3.3-70b-versatile (fast + reliable)
type GroqService struct {
	APIKey  string
	BaseURL string
}

type groqRequest struct {
	Model       string        `json:"model"`
	Messages    []groqMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
}

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// NewGroqService creates a new Groq service
func NewGroqService(apiKey, baseURL string) *GroqService {
	if baseURL == "" {
		baseURL = "https://api.groq.com/openai/v1"
	}
	return &GroqService{
		APIKey:  apiKey,
		BaseURL: strings.TrimRight(baseURL, "/"),
	}
}

// Generate implements the AIProvider interface
func (s *GroqService) Generate(
	sourceContent string,
	platforms []string,
) (string, error) {
	if s.APIKey == "" {
		return "", fmt.Errorf("Groq API key is not configured")
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

	requestBody := groqRequest{
		Model: "openai/gpt-oss-120b",
		Messages: []groqMessage{
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
		return "", fmt.Errorf("failed to encode Groq request: %w", err)
	}

	url := s.BaseURL + "/chat/completions"

	// Groq is fast — simple retry on network errors only
	var lastErr error
	delays := []time.Duration{0, 2 * time.Second, 5 * time.Second}

	for attempt, delay := range delays {
		if delay > 0 {
			time.Sleep(delay)
		}

		content, err := s.doRequest(url, body)
		if err == nil {
			if attempt > 0 {
				fmt.Printf("✅ Groq succeeded on attempt %d\n", attempt+1)
			}
			return content, nil
		}
		lastErr = err

		// Don't retry on 4xx (except 429)
		errStr := err.Error()
		if strings.Contains(errStr, "HTTP status 4") && !strings.Contains(errStr, "429") {
			return "", err
		}

		fmt.Printf("⚠️  Groq attempt %d failed: %v\n", attempt+1, err)
	}

	return "", fmt.Errorf("Groq failed after %d attempts: %w", len(delays), lastErr)
}

// doRequest performs a single HTTP request to Groq
func (s *GroqService) doRequest(url string, body []byte) (string, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return "", fmt.Errorf("failed to create Groq request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.APIKey)

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Groq request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("Groq returned HTTP status %d", resp.StatusCode)
	}

	var result groqResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode Groq response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("Groq returned no choices")
	}

	return result.Choices[0].Message.Content, nil
}
