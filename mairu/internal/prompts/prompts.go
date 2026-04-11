package prompts

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
)

//go:embed *.md
var promptFiles embed.FS

var tmpl *template.Template

func init() {
	tmpl = template.Must(template.ParseFS(promptFiles, "*.md"))
}

func renderTemplateSource(name string, data any, sourcePath string, source []byte) (string, error) {
	t, err := template.New(name).Parse(string(source))
	if err != nil {
		return "", fmt.Errorf("failed to parse %s: %w", sourcePath, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute %s: %w", sourcePath, err)
	}
	return buf.String(), nil
}

func getLocalOverrides(projectRoot, name string) []string {
	if projectRoot == "" {
		return nil
	}
	return []string{
		filepath.Join(projectRoot, ".mairu", "prompts", name+".md"),
		filepath.Join(projectRoot, "prompts", name+".md"),
	}
}

// GetForProject renders a prompt template with project-root anchored overrides.
func GetForProject(name string, data any, projectRoot string) (string, error) {
	// 1. Try project-local override first
	for _, path := range getLocalOverrides(projectRoot, name) {
		content, err := os.ReadFile(path)
		if err == nil {
			return renderTemplateSource(name, data, path, content)
		}
	}

	// 2. Try user-global override
	if home, err := os.UserHomeDir(); err == nil {
		globalPath := filepath.Join(home, ".config", "mairu", "prompts", name+".md")
		content, err := os.ReadFile(globalPath)
		if err == nil {
			return renderTemplateSource(name, data, globalPath, content)
		}
	}

	// 3. Fallback to built-in template
	var buf bytes.Buffer
	err := tmpl.ExecuteTemplate(&buf, name+".md", data)
	if err != nil {
		return "", fmt.Errorf("failed to execute prompt template %s: %w", name, err)
	}
	return buf.String(), nil
}

// Get renders a prompt template using the process working directory as project root.
func Get(name string, data any) (string, error) {
	cwd, _ := os.Getwd()
	return GetForProject(name, data, cwd)
}

// Render is a convenience function that panics on error, useful for static prompts or when you know the template is valid.
func Render(name string, data any) string {
	res, err := Get(name, data)
	if err != nil {
		panic(err)
	}
	return res
}
