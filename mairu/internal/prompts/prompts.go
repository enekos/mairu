package prompts

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed *.md
var promptFiles embed.FS

var tmpl *template.Template

func init() {
	tmpl = template.Must(template.ParseFS(promptFiles, "*.md"))
}

// Get renders a prompt template with the given data.
func Get(name string, data any) (string, error) {
	var buf bytes.Buffer
	err := tmpl.ExecuteTemplate(&buf, name+".md", data)
	if err != nil {
		return "", fmt.Errorf("failed to execute prompt template %s: %w", name, err)
	}
	return buf.String(), nil
}

// Render is a convenience function that panics on error, useful for static prompts or when you know the template is valid.
func Render(name string, data any) string {
	res, err := Get(name, data)
	if err != nil {
		panic(err)
	}
	return res
}
