package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"mairu/internal/httptracer"
)

const (
	kimiDefaultBaseURL = "https://api.moonshot.ai/v1"
	kimiCodeBaseURL    = "https://api.kimi.com/coding/v1"
	kimiDefaultModel   = "kimi-k2.5"
)

// KimiClient is an HTTP client for the Kimi API
type KimiClient struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// NewKimiClient creates a new Kimi API client
func NewKimiClient(apiKey string, baseURL string) *KimiClient {
	if baseURL == "" {
		if strings.HasPrefix(apiKey, "sk-kimi-") {
			baseURL = kimiCodeBaseURL
		} else {
			baseURL = kimiDefaultBaseURL
		}
	}
	return &KimiClient{
		apiKey:  apiKey,
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: httptracer.Wrap(&http.Client{
			Timeout: 120 * time.Second,
		}, httptracer.Options{Component: "kimi"}),
	}
}

// ChatCompletion sends a chat completion request
func (c *KimiClient) ChatCompletion(ctx context.Context, req KimiChatRequest) (*KimiChatResponse, error) {
	url := fmt.Sprintf("%s/chat/completions", c.baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	if strings.Contains(strings.ToLower(c.baseURL), "api.kimi.com") {
		httpReq.Header.Set("User-Agent", "KimiCLI/1.34.0")
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp KimiErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			return nil, &KimiAPIError{
				HTTPStatus: resp.StatusCode,
				Message:    errResp.Error.Message,
				Type:       errResp.Error.Type,
				Code:       errResp.Error.Code,
			}
		}
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var chatResp KimiChatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &chatResp, nil
}

// ChatCompletionStream sends a streaming chat completion request
func (c *KimiClient) ChatCompletionStream(ctx context.Context, req KimiChatRequest) (*KimiStreamIterator, error) {
	url := fmt.Sprintf("%s/chat/completions", c.baseURL)

	req.Stream = true
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")
	if strings.Contains(strings.ToLower(c.baseURL), "api.kimi.com") {
		httpReq.Header.Set("User-Agent", "KimiCLI/1.30.0")
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		respBody, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("API error (status %d): failed to read error body: %w", resp.StatusCode, readErr)
		}
		var errResp KimiErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Error.Message != "" {
			return nil, &KimiAPIError{
				HTTPStatus: resp.StatusCode,
				Message:    errResp.Error.Message,
				Type:       errResp.Error.Type,
				Code:       errResp.Error.Code,
			}
		}
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return &KimiStreamIterator{
		reader: bufio.NewReader(resp.Body),
		body:   resp.Body,
	}, nil
}

// kimiToolCallAcc accumulates partial tool-call deltas from a Kimi stream.
type kimiToolCallAcc struct {
	id   string
	name string
	args strings.Builder
}

// KimiStreamIterator implements ChatStreamIterator for Kimi
type KimiStreamIterator struct {
	reader           *bufio.Reader
	body             io.ReadCloser
	done             bool
	toolCallAccs     map[int]*kimiToolCallAcc
	reasoningContent strings.Builder
}

func (k *KimiStreamIterator) Next() (ChatStreamChunk, error) {
	if k.done {
		return ChatStreamChunk{}, io.EOF
	}

	for {
		line, err := k.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				k.done = true
				chunk := ChatStreamChunk{FinishReason: "stop"}
				if len(k.toolCallAccs) > 0 {
					chunk.ToolCalls = k.finalizeToolCalls()
				}
				return chunk, nil
			}
			return ChatStreamChunk{}, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// SSE format: data: {...} or data:{...}
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))

		// Check for stream end
		if data == "[DONE]" {
			k.done = true
			chunk := ChatStreamChunk{FinishReason: "stop"}
			if len(k.toolCallAccs) > 0 {
				chunk.ToolCalls = k.finalizeToolCalls()
			}
			return chunk, nil
		}

		var streamResp KimiStreamResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			slog.Error("failed to unmarshal stream chunk", "error", err)
			continue // Skip malformed chunks
		}

		if len(streamResp.Choices) == 0 {
			continue
		}

		choice := streamResp.Choices[0]
		chunk := ChatStreamChunk{}

		if choice.Delta != nil {
			chunk.Content = choice.Delta.Content

			// Accumulate reasoning content internally and attach it to the first
			// content-bearing or finish chunk so callers can record it in history.
			if choice.Delta.ReasoningContent != "" {
				k.reasoningContent.WriteString(choice.Delta.ReasoningContent)
			}

			// Accumulate tool-call deltas; do not emit them until the stream ends.
			if len(choice.Delta.ToolCalls) > 0 {
				if k.toolCallAccs == nil {
					k.toolCallAccs = make(map[int]*kimiToolCallAcc)
				}
				for _, tc := range choice.Delta.ToolCalls {
					acc, ok := k.toolCallAccs[tc.Index]
					if !ok {
						acc = &kimiToolCallAcc{}
						k.toolCallAccs[tc.Index] = acc
					}
					if tc.ID != "" {
						acc.id = tc.ID
					}
					if tc.Function.Name != "" {
						acc.name = tc.Function.Name
					}
					if tc.Function.Arguments != "" {
						acc.args.WriteString(tc.Function.Arguments)
					}
				}
			}
		}

		// Set finish reason
		if choice.FinishReason != "" {
			chunk.FinishReason = choice.FinishReason
			k.done = (choice.FinishReason == "stop" || choice.FinishReason == "length")
			if len(k.toolCallAccs) > 0 {
				chunk.ToolCalls = k.finalizeToolCalls()
			}
		}

		if k.reasoningContent.Len() > 0 {
			chunk.ReasoningContent = k.reasoningContent.String()
			k.reasoningContent.Reset()
		}

		if chunk.Content != "" || chunk.ReasoningContent != "" || len(chunk.ToolCalls) > 0 || chunk.FinishReason != "" {
			return chunk, nil
		}
	}
}

func (k *KimiStreamIterator) finalizeToolCalls() []ToolCall {
	result := make([]ToolCall, 0, len(k.toolCallAccs))
	for i := 0; i < len(k.toolCallAccs); i++ {
		acc := k.toolCallAccs[i]
		var args map[string]any
		if acc.args.Len() > 0 {
			if err := json.Unmarshal([]byte(acc.args.String()), &args); err != nil {
				slog.Error("failed to unmarshal accumulated Kimi tool call arguments", "error", err, "args", acc.args.String())
			}
		}
		result = append(result, ToolCall{
			ID:        acc.id,
			Name:      acc.name,
			Arguments: args,
		})
	}
	k.toolCallAccs = nil
	return result
}

func (k *KimiStreamIterator) Done() bool {
	return k.done
}

// Close closes the response body
func (k *KimiStreamIterator) Close() error {
	if k.body != nil {
		return k.body.Close()
	}
	return nil
}

// KimiAPIError represents an error from the Kimi API
type KimiAPIError struct {
	HTTPStatus int
	Message    string
	Type       string
	Code       string
}

func (e *KimiAPIError) Error() string {
	return fmt.Sprintf("Kimi API error: %s (type: %s, code: %s, status: %d)",
		e.Message, e.Type, e.Code, e.HTTPStatus)
}

// StatusCode returns the HTTP status code for StatusError interface
func (e *KimiAPIError) StatusCode() int {
	return e.HTTPStatus
}
