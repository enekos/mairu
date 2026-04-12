package agent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/generative-ai-go/genai"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/universal-tool-calling-protocol/go-utcp/src/providers/http"
	"github.com/universal-tool-calling-protocol/go-utcp/src/tools"
	transports "github.com/universal-tool-calling-protocol/go-utcp/src/transports/http"
)

type UTCPManager struct {
	providers     []string
	transport     *transports.HttpClientTransport
	schemaCache   *lru.Cache[string, []tools.Tool]
	providerCache *lru.Cache[string, *http.HttpProvider]
}

func NewUTCPManager(providersList []string) (*UTCPManager, error) {
	// Initialize LRU cache (max 100 providers/schemas)
	sCache, err := lru.New[string, []tools.Tool](100)
	if err != nil {
		return nil, err
	}
	pCache, err := lru.New[string, *http.HttpProvider](100)
	if err != nil {
		return nil, err
	}

	transport := transports.NewHttpClientTransport(func(format string, args ...interface{}) {
		// Suppress verbose UTCP logs by default, or connect to agent logger
	})

	manager := &UTCPManager{
		providers:     providersList,
		transport:     transport,
		schemaCache:   sCache,
		providerCache: pCache,
	}

	return manager, nil
}

func (m *UTCPManager) Initialize(ctx context.Context) []*genai.FunctionDeclaration {
	if len(m.providers) == 0 {
		return nil
	}

	var allDeclarations []*genai.FunctionDeclaration
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, providerURL := range m.providers {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()

			if cachedTools, ok := m.schemaCache.Get(url); ok {
				var decs []*genai.FunctionDeclaration
				for _, t := range cachedTools {
					decs = append(decs, m.convertToGenaiDeclaration(t))
				}
				mu.Lock()
				allDeclarations = append(allDeclarations, decs...)
				mu.Unlock()
				return
			}

			provider := &http.HttpProvider{
				URL:        url,
				HTTPMethod: "GET",
				Headers:    map[string]string{"Accept": "application/json"},
			}

			// Add a timeout to prevent hanging on unreachable providers
			fetchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			tools, err := m.transport.RegisterToolProvider(fetchCtx, provider)
			if err != nil {
				log.Printf("Warning: Failed to load UTCP tools from %s: %v", url, err)
				return
			}

			m.schemaCache.Add(url, tools)

			var decs []*genai.FunctionDeclaration
			for _, t := range tools {
				// We also store the target provider info for this specific tool
				// Assumes UTCP server uses POST /tools/{name}/call for invocations by standard
				callProvider := &http.HttpProvider{
					URL:        fmt.Sprintf("%s/%s/call", url, t.Name),
					HTTPMethod: "POST",
					Headers:    map[string]string{"Content-Type": "application/json"},
				}
				// Format URL properly if the base URL already ends with /tools or something
				// Wait, the standard says "URL: base_url". The example used `baseURL + "/tools"` for discovery
				// Let's assume the provided URL is the exact discovery endpoint (e.g. `http://localhost:8080/tools`)
				// Then the call endpoint is `http://localhost:8080/tools/{name}/call`
				m.providerCache.Add(t.Name, callProvider)

				decs = append(decs, m.convertToGenaiDeclaration(t))
			}

			mu.Lock()
			allDeclarations = append(allDeclarations, decs...)
			mu.Unlock()
		}(providerURL)
	}

	wg.Wait()
	return allDeclarations
}

func (m *UTCPManager) ExecuteTool(ctx context.Context, name string, args map[string]interface{}) (interface{}, error) {
	provider, ok := m.providerCache.Get(name)
	if !ok {
		return nil, fmt.Errorf("UTCP tool '%s' not found or provider unknown", name)
	}

	result, err := m.transport.CallTool(ctx, name, args, provider, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call UTCP tool '%s': %w", name, err)
	}

	return result, nil
}

func (m *UTCPManager) IsUTCPTool(name string) bool {
	return m.providerCache.Contains(name)
}

func (m *UTCPManager) convertToGenaiDeclaration(t tools.Tool) *genai.FunctionDeclaration {
	decl := &genai.FunctionDeclaration{
		Name:        t.Name,
		Description: t.Description,
	}

	// Inputs in UTCP model is ToolInputOutputSchema, let's map it via JSON serialization
	decl.Parameters = m.convertSchema(t.Inputs)

	return decl
}

func (m *UTCPManager) convertSchema(schema tools.ToolInputOutputSchema) *genai.Schema {
	genSchema := &genai.Schema{
		Type: m.mapType(schema.Type),
	}

	if schema.Properties != nil {
		genSchema.Properties = make(map[string]*genai.Schema)
		for k, v := range schema.Properties {
			if propMap, ok := v.(map[string]interface{}); ok {
				genSchema.Properties[k] = m.convertMapSchema(propMap)
			}
		}
	}

	if len(schema.Required) > 0 {
		genSchema.Required = schema.Required
	}

	if schema.Description != "" {
		genSchema.Description = schema.Description
	}

	return genSchema
}

func (m *UTCPManager) convertMapSchema(schema map[string]interface{}) *genai.Schema {
	genSchema := &genai.Schema{
		Type: genai.TypeObject,
	}

	if t, ok := schema["type"].(string); ok {
		genSchema.Type = m.mapType(t)
	}

	if props, ok := schema["properties"].(map[string]interface{}); ok {
		genSchema.Properties = make(map[string]*genai.Schema)
		for k, v := range props {
			if propMap, ok := v.(map[string]interface{}); ok {
				genSchema.Properties[k] = m.convertMapSchema(propMap)
			}
		}
	}

	if req, ok := schema["required"].([]interface{}); ok {
		for _, r := range req {
			if rStr, ok := r.(string); ok {
				genSchema.Required = append(genSchema.Required, rStr)
			}
		}
	} else if reqStr, ok := schema["required"].([]string); ok {
		genSchema.Required = reqStr
	}

	if desc, ok := schema["description"].(string); ok {
		genSchema.Description = desc
	}

	return genSchema
}

func (m *UTCPManager) mapType(t string) genai.Type {
	switch t {
	case "string":
		return genai.TypeString
	case "number", "integer":
		return genai.TypeNumber
	case "boolean":
		return genai.TypeBoolean
	case "array":
		return genai.TypeArray
	case "object":
		return genai.TypeObject
	default:
		return genai.TypeString
	}
}
