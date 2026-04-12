package llm

import (
	"testing"

	"github.com/google/generative-ai-go/genai"
	"github.com/stretchr/testify/assert"
)

type ComplexAddress struct {
	City string `json:"city" desc:"City name"`
	Zip  string `json:"zip,omitempty"` // omitempty -> not required
}

type ComplexStruct struct {
	ID       int               `json:"id" desc:"The ID"`
	Score    float64           `json:"score"`
	IsActive bool              `json:"is_active"`
	Tags     []string          `json:"tags"`
	Address  *ComplexAddress   `json:"address"`
	Secret   string            `json:"-"`
	Data     map[string]string `json:"data"`
	History  []ComplexAddress  `json:"history"`
}

func TestGenerateSchema_Comprehensive(t *testing.T) {
	s := GenerateSchema(ComplexStruct{})

	assert.Equal(t, genai.TypeObject, s.Type)
	assert.NotNil(t, s.Properties)

	// ID
	assert.Equal(t, genai.TypeInteger, s.Properties["id"].Type)
	assert.Equal(t, "The ID", s.Properties["id"].Description)
	assert.Contains(t, s.Required, "id")

	// Score
	assert.Equal(t, genai.TypeNumber, s.Properties["score"].Type)
	assert.Contains(t, s.Required, "score")

	// IsActive
	assert.Equal(t, genai.TypeBoolean, s.Properties["is_active"].Type)
	assert.Contains(t, s.Required, "is_active")

	// Tags
	assert.Equal(t, genai.TypeArray, s.Properties["tags"].Type)
	assert.Equal(t, genai.TypeString, s.Properties["tags"].Items.Type)
	assert.Contains(t, s.Required, "tags")

	// Address (nested object pointer)
	addrSchema := s.Properties["address"]
	assert.NotNil(t, addrSchema)
	assert.Equal(t, genai.TypeObject, addrSchema.Type)
	assert.Equal(t, genai.TypeString, addrSchema.Properties["city"].Type)
	assert.Equal(t, "City name", addrSchema.Properties["city"].Description)
	assert.Contains(t, addrSchema.Required, "city")
	assert.NotContains(t, addrSchema.Required, "zip") // omitempty check

	// History (array of structs)
	histSchema := s.Properties["history"]
	assert.NotNil(t, histSchema)
	assert.Equal(t, genai.TypeArray, histSchema.Type)
	assert.Equal(t, genai.TypeObject, histSchema.Items.Type)
	assert.Equal(t, genai.TypeString, histSchema.Items.Properties["city"].Type)
	assert.Contains(t, histSchema.Items.Required, "city")

	// Data (map)
	dataSchema := s.Properties["data"]
	assert.NotNil(t, dataSchema)
	assert.Equal(t, genai.TypeObject, dataSchema.Type)

	// Ignored fields
	assert.Nil(t, s.Properties["Secret"])
	assert.Nil(t, s.Properties["unexported"])
	assert.Nil(t, s.Properties["-"])
}

func TestGenerateSchema_Fallback(t *testing.T) {
	s1 := GenerateSchema("just a string")
	assert.Equal(t, genai.TypeString, s1.Type)

	s2 := GenerateSchema(42)
	assert.Equal(t, genai.TypeInteger, s2.Type)

	s3 := GenerateSchema([]int{1, 2})
	assert.Equal(t, genai.TypeArray, s3.Type)
	assert.Equal(t, genai.TypeInteger, s3.Items.Type)
}
