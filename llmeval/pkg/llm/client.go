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
		baseURL = "https://api.moonshot.cn/v1"
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

type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float32       `json:"temperature"`
}

type ChatCompletionResponse struct {
	Choices []struct {
		Message ChatMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

func (c *Client) Generate(ctx context.Context, model, systemPrompt, prompt string, temperature float32) (string, Usage, error) {
	messages := []ChatMessage{}
	if systemPrompt != "" {
		messages = append(messages, ChatMessage{Role: "system", Content: systemPrompt})
	}
	messages = append(messages, ChatMessage{Role: "user", Content: prompt})

	reqBody := ChatCompletionRequest{
		Model:       model,
		Messages:    messages,
		Temperature: temperature,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", Usage{}, err
	}

	var chatResp ChatCompletionResponse
	var lastErr error

	url := fmt.Sprintf("%s/chat/completions", c.BaseURL)

	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return "", Usage{}, err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.APIKey)

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
			return "", Usage{}, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(bodyBytes))
		}

		if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
			return "", Usage{}, fmt.Errorf("failed to decode response: %v, raw: %s", err, string(bodyBytes))
		}

		if chatResp.Error != nil {
			return "", Usage{}, fmt.Errorf("API error: %s", chatResp.Error.Message)
		}

		if len(chatResp.Choices) == 0 {
			return "", Usage{}, fmt.Errorf("no choices returned in response: %s", string(bodyBytes))
		}

		usage := Usage{
			PromptTokens:     chatResp.Usage.PromptTokens,
			CompletionTokens: chatResp.Usage.CompletionTokens,
			TotalTokens:      chatResp.Usage.TotalTokens,
		}

		return chatResp.Choices[0].Message.Content, usage, nil
	}

	return "", Usage{}, fmt.Errorf("failed after 3 attempts, last error: %w", lastErr)
}
