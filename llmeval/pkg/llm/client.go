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
		baseURL = "https://generativelanguage.googleapis.com/v1beta"
	}
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: time.Minute * 2,
		},
	}
}

type Part struct {
	Text string `json:"text"`
}

type Content struct {
	Role  string `json:"role,omitempty"`
	Parts []Part `json:"parts"`
}

type GenerationConfig struct {
	Temperature float32 `json:"temperature"`
}

type GenerateContentRequest struct {
	SystemInstruction *Content         `json:"systemInstruction,omitempty"`
	Contents          []Content        `json:"contents"`
	GenerationConfig  GenerationConfig `json:"generationConfig"`
}

type UsageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type GenerateContentResponse struct {
	Candidates []struct {
		Content Content `json:"content"`
	} `json:"candidates"`
	UsageMetadata UsageMetadata `json:"usageMetadata"`
	Error         *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *Client) Generate(ctx context.Context, model, systemPrompt, prompt string, temperature float32) (string, Usage, error) {
	reqBody := GenerateContentRequest{
		Contents: []Content{
			{Role: "user", Parts: []Part{{Text: prompt}}},
		},
		GenerationConfig: GenerationConfig{
			Temperature: temperature,
		},
	}

	if systemPrompt != "" {
		reqBody.SystemInstruction = &Content{
			Role:  "user", // System instructions don't need a specific role or it's implicitly system based on the field
			Parts: []Part{{Text: systemPrompt}},
		}
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", Usage{}, err
	}

	var chatResp GenerateContentResponse
	var lastErr error

	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.BaseURL, model, c.APIKey)

	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return "", Usage{}, err
		}

		req.Header.Set("Content-Type", "application/json")

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

		if len(chatResp.Candidates) == 0 || len(chatResp.Candidates[0].Content.Parts) == 0 {
			// This might happen if it was blocked by safety settings
			return "", Usage{}, fmt.Errorf("no choices returned in response: %s", string(bodyBytes))
		}

		usage := Usage{
			PromptTokens:     chatResp.UsageMetadata.PromptTokenCount,
			CompletionTokens: chatResp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      chatResp.UsageMetadata.TotalTokenCount,
		}

		return chatResp.Candidates[0].Content.Parts[0].Text, usage, nil
	}

	return "", Usage{}, fmt.Errorf("failed after 3 attempts, last error: %w", lastErr)
}
