package agent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/universal-tool-calling-protocol/go-utcp/src/providers/http"
	"github.com/universal-tool-calling-protocol/go-utcp/src/tools"
	transports "github.com/universal-tool-calling-protocol/go-utcp/src/transports/http"
	"mairu/internal/llm"
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

// Initialize fetches tools from all configured UTCP providers and converts them to llm.Tool format
func (m *UTCPManager) Initialize(ctx context.Context) []llm.Tool {
	if len(m.providers) == 0 {
		return nil
	}

	var allTools []llm.Tool
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, providerURL := range m.providers {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()

			if cachedTools, ok := m.schemaCache.Get(url); ok {
				var tools []llm.Tool
				for _, t := range cachedTools {
					tools = append(tools, m.convertToLLMTool(t))
				}
				mu.Lock()
				allTools = append(allTools, tools...)
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

			var providerTools []llm.Tool
			for _, t := range tools {
				// We also store the target provider info for this specific tool
				// Assumes UTCP server uses POST /tools/{name}/call for invocations by standard
				callProvider := &http.HttpProvider{
					URL:        fmt.Sprintf("%s/%s/call", url, t.Name),
					HTTPMethod: "POST",
					Headers:    map[string]string{"Content-Type": "application/json"},
				}
				m.providerCache.Add(t.Name, callProvider)
				providerTools = append(providerTools, m.convertToLLMTool(t))
			}

			mu.Lock()
			allTools = append(allTools, providerTools...)
			mu.Unlock()
		}(providerURL)
	}

	wg.Wait()
	return allTools
}

// ExecuteTool invokes a tool via UTCP protocol
func (m *UTCPManager) ExecuteTool(ctx context.Context, toolName string, args map[string]interface{}) (interface{}, error) {
	provider, ok := m.providerCache.Get(toolName)
	if !ok {
		return nil, fmt.Errorf("tool %s not found in UTCP cache", toolName)
	}

	return m.transport.CallTool(ctx, toolName, args, provider, nil)
}

// IsUTCPTool checks if a tool name is a UTCP tool
func (m *UTCPManager) IsUTCPTool(name string) bool {
	return m.providerCache.Contains(name)
}

// convertToLLMTool converts a UTCP tool to llm.Tool format
func (m *UTCPManager) convertToLLMTool(t tools.Tool) llm.Tool {
	return llm.Tool{
		Name:        t.Name,
		Description: t.Description,
		Parameters:  m.convertSchemaToLLM(t.Inputs),
	}
}

func (m *UTCPManager) convertSchemaToLLM(schema tools.ToolInputOutputSchema) *llm.JSONSchema {
	llmSchema := &llm.JSONSchema{
		Type:        m.mapTypeToLLM(schema.Type),
		Description: schema.Description,
		Required:    schema.Required,
	}

	if schema.Properties != nil {
		llmSchema.Properties = make(map[string]*llm.JSONSchema)
		for k, v := range schema.Properties {
			if propMap, ok := v.(map[string]interface{}); ok {
				llmSchema.Properties[k] = m.convertMapSchemaToLLM(propMap)
			}
		}
	}

	return llmSchema
}

func (m *UTCPManager) convertMapSchemaToLLM(schema map[string]interface{}) *llm.JSONSchema {
	llmSchema := &llm.JSONSchema{
		Type: llm.TypeObject,
	}

	if t, ok := schema["type"].(string); ok {
		llmSchema.Type = m.mapTypeToLLM(t)
	}

	if desc, ok := schema["description"].(string); ok {
		llmSchema.Description = desc
	}

	if props, ok := schema["properties"].(map[string]interface{}); ok {
		llmSchema.Properties = make(map[string]*llm.JSONSchema)
		for k, v := range props {
			if propMap, ok := v.(map[string]interface{}); ok {
				llmSchema.Properties[k] = m.convertMapSchemaToLLM(propMap)
			}
		}
	}

	if req, ok := schema["required"].([]interface{}); ok {
		for _, r := range req {
			if rStr, ok := r.(string); ok {
				llmSchema.Required = append(llmSchema.Required, rStr)
			}
		}
	} else if reqStr, ok := schema["required"].([]string); ok {
		llmSchema.Required = reqStr
	}

	return llmSchema
}

func (m *UTCPManager) mapTypeToLLM(t string) llm.JSONSchemaType {
	switch t {
	case "string":
		return llm.TypeString
	case "number":
		return llm.TypeNumber
	case "integer":
		return llm.TypeInteger
	case "boolean":
		return llm.TypeBoolean
	case "array":
		return llm.TypeArray
	case "object":
		return llm.TypeObject
	default:
		return llm.TypeString
	}
}
