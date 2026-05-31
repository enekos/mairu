package prompts

import (
	"bytes"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	// 1. Try project-local override first (most specific).
	for _, path := range getLocalOverrides(projectRoot, name) {
		content, err := os.ReadFile(path)
		if err == nil {
			return renderTemplateSource(name, data, path, content)
		}
	}

	// 2. Try active-profile override (~/.config/mairu/profiles/<name>/...).
	// The profile name comes from MAIRU_PROFILE (set by the -p root flag),
	// so this package stays independent of cmd.
	for _, path := range getProfileOverrides(name) {
		content, err := os.ReadFile(path)
		if err == nil {
			return renderTemplateSource(name, data, path, content)
		}
	}

	// 3. Try user-global override.
	if home, err := os.UserHomeDir(); err == nil {
		globalPath := filepath.Join(home, ".config", "mairu", "prompts", name+".md")
		content, err := os.ReadFile(globalPath)
		if err == nil {
			return renderTemplateSource(name, data, globalPath, content)
		}
	}

	// 4. Fallback to built-in template.
	var buf bytes.Buffer
	err := tmpl.ExecuteTemplate(&buf, name+".md", data)
	if err != nil {
		return "", fmt.Errorf("failed to execute prompt template %s: %w", name, err)
	}
	return buf.String(), nil
}

// getProfileOverrides returns the candidate paths for the active profile's
// override of the named prompt. The system prompt has a special filename
// (agent_system.md) that lives at the profile root, in line with hermes's
// SOUL.md convention; everything else is looked up under prompts/.
func getProfileOverrides(name string) []string {
	profileName := strings.TrimSpace(os.Getenv("MAIRU_PROFILE"))
	if profileName == "" {
		return nil
	}
	root := profileRoot()
	if root == "" {
		return nil
	}
	base := filepath.Join(root, "profiles", profileName)
	out := []string{filepath.Join(base, "prompts", name+".md")}
	if name == "agent_system" {
		out = append(out, filepath.Join(base, "agent_system.md"))
	}
	return out
}

// profileRoot mirrors internal/profile.Root() without the dependency.
// Keeps internal/prompts free of an upward import.
func profileRoot() string {
	if h := os.Getenv("MAIRU_HOME"); h != "" {
		return h
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "mairu")
	}
	return ""
}

// Get renders a prompt template using the process working directory as project root.
func Get(name string, data any) (string, error) {
	cwd, _ := os.Getwd()
	return GetForProject(name, data, cwd)
}

// Render is a convenience wrapper around Get.
func Render(name string, data any) (string, error) {
	return Get(name, data)
}
