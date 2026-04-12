package llm

import (
	"github.com/google/generative-ai-go/genai"
)

// ToJSONSchema converts a genai.Schema to our provider-agnostic JSONSchema
func ToJSONSchema(s *genai.Schema) *JSONSchema {
	if s == nil {
		return nil
	}

	schema := &JSONSchema{
		Type:        genaiTypeToJSONSchemaType(s.Type),
		Description: s.Description,
		Required:    s.Required,
		Enum:        s.Enum,
	}

	if s.Properties != nil {
		schema.Properties = make(map[string]*JSONSchema)
		for name, prop := range s.Properties {
			schema.Properties[name] = ToJSONSchema(prop)
		}
	}

	if s.Items != nil {
		schema.Items = ToJSONSchema(s.Items)
	}

	return schema
}

// FromJSONSchema converts our JSONSchema to genai.Schema
func FromJSONSchema(s *JSONSchema) *genai.Schema {
	if s == nil {
		return nil
	}

	schema := &genai.Schema{
		Type:        jsonSchemaTypeToGenaiType(s.Type),
		Description: s.Description,
		Required:    s.Required,
		Enum:        s.Enum,
	}

	if s.Properties != nil {
		schema.Properties = make(map[string]*genai.Schema)
		for name, prop := range s.Properties {
			schema.Properties[name] = FromJSONSchema(prop)
		}
	}

	if s.Items != nil {
		schema.Items = FromJSONSchema(s.Items)
	}

	return schema
}

func genaiTypeToJSONSchemaType(t genai.Type) JSONSchemaType {
	switch t {
	case genai.TypeObject:
		return TypeObject
	case genai.TypeArray:
		return TypeArray
	case genai.TypeString:
		return TypeString
	case genai.TypeInteger:
		return TypeInteger
	case genai.TypeNumber:
		return TypeNumber
	case genai.TypeBoolean:
		return TypeBoolean
	default:
		return TypeString
	}
}

func jsonSchemaTypeToGenaiType(t JSONSchemaType) genai.Type {
	switch t {
	case TypeObject:
		return genai.TypeObject
	case TypeArray:
		return genai.TypeArray
	case TypeString:
		return genai.TypeString
	case TypeInteger:
		return genai.TypeInteger
	case TypeNumber:
		return genai.TypeNumber
	case TypeBoolean:
		return genai.TypeBoolean
	default:
		return genai.TypeString
	}
}

// genaiContentToMessages converts genai.Content history to our Message format
func genaiContentToMessages(contents []*genai.Content) []Message {
	messages := make([]Message, 0, len(contents))
	for _, c := range contents {
		msg := Message{
			Role: genaiRoleToMessageRole(c.Role),
		}
		// Concatenate all parts into content
		for _, part := range c.Parts {
			if text, ok := part.(genai.Text); ok {
				msg.Content += string(text)
			}
			// TODO: Handle FunctionCall parts if needed
		}
		messages = append(messages, msg)
	}
	return messages
}

// messagesToGenaiContent converts our Message history to genai.Content format
func messagesToGenaiContent(messages []Message) []*genai.Content {
	contents := make([]*genai.Content, 0, len(messages))
	for _, m := range messages {
		content := &genai.Content{
			Role:  messageRoleToGenaiRole(m.Role),
			Parts: []genai.Part{genai.Text(m.Content)},
		}
		contents = append(contents, content)
	}
	return contents
}

func genaiRoleToMessageRole(role string) string {
	switch role {
	case "user":
		return "user"
	case "model":
		return "assistant"
	default:
		return role
	}
}

func messageRoleToGenaiRole(role string) string {
	switch role {
	case "user":
		return "user"
	case "assistant":
		return "model"
	case "system":
		// Gemini doesn't have system role in history, it's set separately
		return "user"
	default:
		return role
	}
}

// genaiToolsToTools converts genai function declarations to our Tool format
func genaiToolsToTools(funcDecls []*genai.FunctionDeclaration) []Tool {
	tools := make([]Tool, 0, len(funcDecls))
	for _, fd := range funcDecls {
		tools = append(tools, Tool{
			Name:        fd.Name,
			Description: fd.Description,
			Parameters:  ToJSONSchema(fd.Parameters),
		})
	}
	return tools
}

// toolsToGenaiFunctionDeclarations converts our Tools to genai format
func toolsToGenaiFunctionDeclarations(tools []Tool) []*genai.FunctionDeclaration {
	funcDecls := make([]*genai.FunctionDeclaration, 0, len(tools))
	for _, t := range tools {
		funcDecls = append(funcDecls, &genai.FunctionDeclaration{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  FromJSONSchema(t.Parameters),
		})
	}
	return funcDecls
}
