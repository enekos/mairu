package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type Provider interface {
	Chat(ctx context.Context, prompt string) (*genai.GenerateContentResponse, error)
	SetupTools()
}

type GeminiProvider struct {
	client    *genai.Client
	model     *genai.GenerativeModel
	session   *genai.ChatSession
	isNew     bool
	modelName string
}

func NewGeminiProvider(ctx context.Context, apiKey string) (*GeminiProvider, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini client: %w", err)
	}

	modelName := "gemini-3.1-flash-lite-preview"
	model := client.GenerativeModel(modelName)
	session := model.StartChat()

	provider := &GeminiProvider{
		client:    client,
		model:     model,
		session:   session,
		isNew:     true,
		modelName: modelName,
	}
	provider.SetupTools()
	return provider, nil
}

func (g *GeminiProvider) GetModelName() string {
	return g.modelName
}

func (g *GeminiProvider) SetModel(modelName string) {
	newModel := g.client.GenerativeModel(modelName)
	newSession := newModel.StartChat()
	newSession.History = g.session.History
	g.modelName = modelName
	g.model = newModel
	g.session = newSession
	g.SetupTools()
}

func (g *GeminiProvider) IsNewSession() bool {
	return g.isNew
}

func (g *GeminiProvider) GetHistory() []*genai.Content {
	return g.session.History
}

func (g *GeminiProvider) SetHistory(history []*genai.Content) {
	g.session.History = history
	g.isNew = false
}

func (g *GeminiProvider) ChatStream(ctx context.Context, prompt string) *genai.GenerateContentResponseIterator {
	g.isNew = false
	return g.session.SendMessageStream(ctx, genai.Text(prompt))
}

func (g *GeminiProvider) SendFunctionResponseStream(ctx context.Context, name string, result map[string]any) *genai.GenerateContentResponseIterator {
	return g.session.SendMessageStream(ctx, genai.FunctionResponse{
		Name:     name,
		Response: result,
	})
}

func (g *GeminiProvider) Close() error {
	return g.client.Close()
}

type FunctionResponsePayload struct {
	Name     string
	Response map[string]any
}

func (g *GeminiProvider) SendFunctionResponsesStream(ctx context.Context, responses []FunctionResponsePayload) *genai.GenerateContentResponseIterator {
	var parts []genai.Part
	for _, r := range responses {
		parts = append(parts, genai.FunctionResponse{
			Name:     r.Name,
			Response: r.Response,
		})
	}
	return g.session.SendMessageStream(ctx, parts...)
}

func (g *GeminiProvider) GenerateJSON(ctx context.Context, system, user string) (map[string]any, error) {
	model := g.client.GenerativeModel(g.modelName)
	model.ResponseMIMEType = "application/json"
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(system)},
	}
	resp, err := model.GenerateContent(ctx, genai.Text(user))
	if err != nil {
		return nil, err
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response")
	}
	part := resp.Candidates[0].Content.Parts[0]
	if txt, ok := part.(genai.Text); ok {
		var out map[string]any
		if err := json.Unmarshal([]byte(txt), &out); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
		return out, nil
	}
	return nil, fmt.Errorf("unexpected part type")
}

func (g *GeminiProvider) GenerateContent(ctx context.Context, modelName, prompt string) (string, error) {
	model := g.client.GenerativeModel(modelName)
	res, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", err
	}
	if len(res.Candidates) > 0 && len(res.Candidates[0].Content.Parts) > 0 {
		return fmt.Sprintf("%v", res.Candidates[0].Content.Parts[0]), nil
	}
	return "", fmt.Errorf("no content generated")
}
