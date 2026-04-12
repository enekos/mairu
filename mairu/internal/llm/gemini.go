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

// Ensure GeminiProvider implements Provider interface
var _ Provider = (*GeminiProvider)(nil)

// GeminiProvider implements the Provider interface for Google Gemini
type GeminiProvider struct {
	client       *genai.Client
	model        *genai.GenerativeModel
	session      *genai.ChatSession
	isNew        bool
	modelName    string
	dynamicTools []*genai.FunctionDeclaration
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

// NewGeminiProvider creates a Gemini provider with just an API key (legacy signature)
func NewGeminiProvider(ctx context.Context, apiKey string) (*GeminiProvider, error) {
	return NewGeminiProviderFromConfig(ctx, ProviderConfig{
		Type:   ProviderGemini,
		APIKey: apiKey,
	})
}

// NewGeminiProviderFromConfig creates a Gemini provider from a ProviderConfig
func NewGeminiProviderFromConfig(ctx context.Context, cfg ProviderConfig) (*GeminiProvider, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(cfg.APIKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini client: %w", err)
	}

	modelName := cfg.Model
	if modelName == "" {
		modelName = "gemini-3.1-flash-lite-preview"
	}
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
	// Re-register dynamic tools
	if len(g.dynamicTools) > 0 {
		g.model.Tools[0].FunctionDeclarations = append(g.model.Tools[0].FunctionDeclarations, g.dynamicTools...)
	}
}

func (g *GeminiProvider) IsNewSession() bool {
	return g.isNew
}

// CacheContext implements CachingProvider interface
func (g *GeminiProvider) CacheContext(ctx context.Context, systemPrompt string, ttl time.Duration) (string, error) {
	if len(systemPrompt) < 100000 {
		return "", nil
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
		return nil
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

func (g *GeminiProvider) GetHistory() []Message {
	return genaiContentToMessages(g.session.History)
}

func (g *GeminiProvider) SetHistory(history []Message) {
	g.session.History = messagesToGenaiContent(history)
	g.isNew = false
}

// Chat implements the Provider interface
func (g *GeminiProvider) Chat(ctx context.Context, prompt string) (*ChatResponse, error) {
	g.isNew = false
	resp, err := g.session.SendMessage(ctx, genai.Text(prompt))
	if err != nil {
		return nil, err
	}
	return g.parseResponse(resp)
}

func (g *GeminiProvider) parseResponse(resp *genai.GenerateContentResponse) (*ChatResponse, error) {
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return &ChatResponse{FinishReason: "stop"}, nil
	}

	result := &ChatResponse{}
	candidate := resp.Candidates[0]

	for _, part := range candidate.Content.Parts {
		switch p := part.(type) {
		case genai.Text:
			result.Content += string(p)
		case genai.FunctionCall:
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:        p.Name,
				Name:      p.Name,
				Arguments: p.Args,
			})
		}
	}

	switch candidate.FinishReason {
	case genai.FinishReasonStop:
		result.FinishReason = "stop"
	case genai.FinishReasonMaxTokens:
		result.FinishReason = "length"
	case genai.FinishReasonSafety:
		result.FinishReason = "safety"
	case genai.FinishReasonRecitation:
		result.FinishReason = "recitation"
	default:
		result.FinishReason = "stop"
	}

	return result, nil
}

// ChatStream implements the Provider interface
func (g *GeminiProvider) ChatStream(ctx context.Context, prompt string) (ChatStreamIterator, error) {
	g.isNew = false
	iter := g.session.SendMessageStream(ctx, genai.Text(prompt))
	return newGeminiStreamIterator(iter), nil
}

// SendFunctionResponseStream implements the Provider interface
func (g *GeminiProvider) SendFunctionResponseStream(ctx context.Context, name string, result map[string]any) ChatStreamIterator {
	iter := g.session.SendMessageStream(ctx, genai.FunctionResponse{
		Name:     name,
		Response: result,
	})
	return newGeminiStreamIterator(iter)
}

// SendFunctionResponsesStream implements the Provider interface
func (g *GeminiProvider) SendFunctionResponsesStream(ctx context.Context, responses []FunctionResponsePayload) ChatStreamIterator {
	var parts []genai.Part
	for _, r := range responses {
		parts = append(parts, genai.FunctionResponse{
			Name:     r.Name,
			Response: r.Response,
		})
	}
	iter := g.session.SendMessageStream(ctx, parts...)
	return newGeminiStreamIterator(iter)
}

func (g *GeminiProvider) Close() error {
	return g.client.Close()
}

// GenerateJSON implements the Provider interface
func (g *GeminiProvider) GenerateJSON(ctx context.Context, system, user string, schema *JSONSchema, out any) error {
	model := g.client.GenerativeModel(g.modelName)
	applySafetySettings(model)
	model.ResponseMIMEType = "application/json"

	var genaiSchema *genai.Schema
	if schema == nil && out != nil {
		genaiSchema = GenerateSchema(out)
	} else if schema != nil {
		genaiSchema = FromJSONSchema(schema)
	}

	if genaiSchema != nil {
		model.ResponseSchema = genaiSchema
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

// GenerateContent implements the Provider interface
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
