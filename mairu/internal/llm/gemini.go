package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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

func applySafetySettings(model *genai.GenerativeModel) {
	model.SafetySettings = []*genai.SafetySetting{
		{
			Category:  genai.HarmCategoryDangerousContent,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategoryHarassment,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategoryHateSpeech,
			Threshold: genai.HarmBlockNone,
		},
		{
			Category:  genai.HarmCategorySexuallyExplicit,
			Threshold: genai.HarmBlockNone,
		},
	}
}

func NewGeminiProvider(ctx context.Context, apiKey string) (*GeminiProvider, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini client: %w", err)
	}

	modelName := "gemini-3.1-flash-lite-preview"
	model := client.GenerativeModel(modelName)
	applySafetySettings(model)
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

func (g *GeminiProvider) SetSystemInstruction(prompt string) {
	if prompt == "" {
		g.model.SystemInstruction = nil
	} else {
		g.model.SystemInstruction = &genai.Content{
			Parts: []genai.Part{genai.Text(prompt)},
		}
	}
	// Restart chat session to ensure it picks up the new system instruction
	newSession := g.model.StartChat()
	newSession.History = g.session.History
	g.session = newSession
}

func (g *GeminiProvider) SetModel(modelName string) {
	newModel := g.client.GenerativeModel(modelName)
	applySafetySettings(newModel)
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

func (g *GeminiProvider) CacheContext(ctx context.Context, systemPrompt string, ttl time.Duration) (string, error) {
	// Be smart: Caching small prompts is slower and less cost-effective.
	// Gemini cache pricing and performance is optimized for > 32k tokens.
	// We use a rough heuristic: ~100,000 characters is ~25k tokens.
	if len(systemPrompt) < 100000 {
		return "", nil // Skip caching, fallback to normal requests
	}

	modelID := g.modelName
	if !strings.HasPrefix(modelID, "models/") {
		modelID = "models/" + modelID
	}
	cc := &genai.CachedContent{
		Model: modelID,
		SystemInstruction: &genai.Content{
			Parts: []genai.Part{genai.Text(systemPrompt)},
		},
		Expiration: genai.ExpireTimeOrTTL{TTL: ttl},
		Tools:      g.model.Tools,
	}
	res, err := g.client.CreateCachedContent(ctx, cc)
	if err != nil {
		return "", fmt.Errorf("failed to create cached content: %w", err)
	}
	return res.Name, nil
}

func (g *GeminiProvider) SetCachedContent(ctx context.Context, name string) error {
	if name == "" {
		return nil // No cache to set
	}
	cc, err := g.client.GetCachedContent(ctx, name)
	if err != nil {
		return fmt.Errorf("failed to get cached content %q: %w", name, err)
	}
	newModel := g.client.GenerativeModelFromCachedContent(cc)
	applySafetySettings(newModel)
	newSession := newModel.StartChat()
	newSession.History = g.session.History
	g.model = newModel
	g.session = newSession
	return nil
}

func (g *GeminiProvider) DeleteCachedContent(ctx context.Context, name string) error {
	if name == "" {
		return nil
	}
	return g.client.DeleteCachedContent(ctx, name)
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

func (g *GeminiProvider) GenerateJSON(ctx context.Context, system, user string, schema *genai.Schema, out any) error {
	model := g.client.GenerativeModel(g.modelName)
	applySafetySettings(model)
	model.ResponseMIMEType = "application/json"

	if schema == nil && out != nil {
		schema = GenerateSchema(out)
	}

	if schema != nil {
		model.ResponseSchema = schema
	}
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(system)},
	}
	resp, err := model.GenerateContent(ctx, genai.Text(user))
	if err != nil {
		return err
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return fmt.Errorf("empty response")
	}
	part := resp.Candidates[0].Content.Parts[0]
	if txt, ok := part.(genai.Text); ok {
		if err := json.Unmarshal([]byte(txt), out); err != nil {
			return fmt.Errorf("failed to parse JSON: %w", err)
		}
		return nil
	}
	return fmt.Errorf("unexpected part type")
}

func (g *GeminiProvider) GenerateContent(ctx context.Context, modelName, prompt string) (string, error) {
	model := g.client.GenerativeModel(modelName)
	applySafetySettings(model)
	res, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", err
	}
	if len(res.Candidates) > 0 && len(res.Candidates[0].Content.Parts) > 0 {
		return fmt.Sprintf("%v", res.Candidates[0].Content.Parts[0]), nil
	}
	return "", fmt.Errorf("no content generated")
}
