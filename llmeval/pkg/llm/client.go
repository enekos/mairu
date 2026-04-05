package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

func NewClient(baseURL, apiKey string) *Client {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: time.Minute * 2,
		},
	}
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float32       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

type ChatResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *Client) Generate(ctx context.Context, model, prompt string, temperature float32) (string, error) {
	reqBody := ChatRequest{
		Model:       model,
		Temperature: temperature,
		Messages: []ChatMessage{
			{Role: "user", Content: prompt},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	var chatResp ChatResponse
	var lastErr error

	// Retry logic: up to 3 attempts
	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/chat/completions", c.BaseURL), bytes.NewBuffer(jsonData))
		if err != nil {
			return "", err
		}

		req.Header.Set("Content-Type", "application/json")
		if c.APIKey != "" {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			lastErr = err
			time.Sleep(time.Duration(attempt) * time.Second)
			continue
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == 429 || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(bodyBytes))
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(bodyBytes))
		}

		if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
			return "", fmt.Errorf("failed to decode response: %v, raw: %s", err, string(bodyBytes))
		}

		if chatResp.Error != nil {
			return "", fmt.Errorf("API error: %s", chatResp.Error.Message)
		}

		if len(chatResp.Choices) == 0 {
			return "", fmt.Errorf("no choices returned in response")
		}

		return chatResp.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("failed after 3 attempts, last error: %w", lastErr)
}
