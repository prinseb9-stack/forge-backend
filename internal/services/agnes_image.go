package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// AgnesImageService generates images via Agnes AI
type AgnesImageService struct {
	APIKey  string
	BaseURL string
}

type agnesImageRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	N      int    `json:"n"`
	Size   string `json:"size"`
}

type agnesImageResponse struct {
	Data []struct {
		URL           string `json:"url"`
		B64JSON       string `json:"b64_json"`
		RevisedPrompt string `json:"revised_prompt"`
	} `json:"data"`
	Created int64  `json:"created"`
	TaskID  string `json:"task_id"`
}

func NewAgnesImageService(apiKey, baseURL string) *AgnesImageService {
	if baseURL == "" {
		baseURL = "https://apihub.agnes-ai.com/v1"
	}
	return &AgnesImageService{
		APIKey:  apiKey,
		BaseURL: strings.TrimRight(baseURL, "/"),
	}
}

// GenerateImage generates one image and returns (url, taskID, error)
func (s *AgnesImageService) GenerateImage(prompt, size string) (string, string, error) {
	if s.APIKey == "" {
		return "", "", fmt.Errorf("Agnes API key is not configured")
	}

	reqBody := agnesImageRequest{
		Model:  "agnes-image-2.1-flash",
		Prompt: prompt,
		N:      1,
		Size:   size,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", "", fmt.Errorf("failed to encode image request: %w", err)
	}

	url := s.BaseURL + "/images/generations"

	delays := []time.Duration{0, 5 * time.Second, 15 * time.Second}
	var lastErr error

	for _, delay := range delays {
		if delay > 0 {
			time.Sleep(delay)
		}

		imgURL, taskID, err := s.doImageRequest(url, body)
		if err == nil {
			return imgURL, taskID, nil
		}
		lastErr = err

		errStr := err.Error()
		if strings.Contains(errStr, "HTTP status 4") && !strings.Contains(errStr, "429") {
			return "", "", err
		}
	}

	return "", "", fmt.Errorf("image generation failed after %d attempts: %w", len(delays), lastErr)
}

func (s *AgnesImageService) doImageRequest(url string, body []byte) (string, string, error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return "", "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.APIKey)
	req.Header.Set("Accept", "application/json")

	transport := &http.Transport{
		ForceAttemptHTTP2:     false,
		DisableKeepAlives:     true,
		ResponseHeaderTimeout: 60 * time.Second,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
	}

	client := &http.Client{
		Timeout:   90 * time.Second,
		Transport: transport,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("image request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("Agnes image returned HTTP status %d", resp.StatusCode)
	}

	var result agnesImageResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("failed to decode image response: %w", err)
	}

	if len(result.Data) == 0 || result.Data[0].URL == "" {
		return "", "", fmt.Errorf("Agnes returned no image URL")
	}

	return result.Data[0].URL, result.TaskID, nil
}
