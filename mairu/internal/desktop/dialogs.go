package desktop

import (
	"encoding/json"
	"fmt"
	"os"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"mairu/internal/contextsrv"
)

// ExportData opens a native save dialog and exports all data for a project as JSON.
func (a *App) ExportData(project string) error {
	path, err := wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{
		Title:           "Export Mairu Data",
		DefaultFilename: fmt.Sprintf("mairu-export-%s.json", project),
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "JSON Files", Pattern: "*.json"},
		},
	})
	if err != nil {
		return err
	}
	if path == "" {
		return nil // user cancelled
	}

	memories, err := a.svc.ListMemories(project, 10000)
	if err != nil {
		return fmt.Errorf("list memories: %w", err)
	}
	skills, err := a.svc.ListSkills(project, 10000)
	if err != nil {
		return fmt.Errorf("list skills: %w", err)
	}
	nodes, err := a.svc.ListContextNodes(project, nil, 10000)
	if err != nil {
		return fmt.Errorf("list context nodes: %w", err)
	}

	export := map[string]any{
		"project":       project,
		"memories":      memories,
		"skills":        skills,
		"context_nodes": nodes,
	}

	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// ImportData opens a native file dialog and imports data from a JSON file.
func (a *App) ImportData() error {
	path, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Import Mairu Data",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "JSON Files", Pattern: "*.json"},
		},
	})
	if err != nil {
		return err
	}
	if path == "" {
		return nil // user cancelled
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var data struct {
		Project      string            `json:"project"`
		Memories     []json.RawMessage `json:"memories"`
		Skills       []json.RawMessage `json:"skills"`
		ContextNodes []json.RawMessage `json:"context_nodes"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("invalid export file: %w", err)
	}

	var errs []error
	for _, raw := range data.Memories {
		var m struct {
			Content    string `json:"content"`
			Category   string `json:"category"`
			Owner      string `json:"owner"`
			Importance int    `json:"importance"`
		}
		if err := json.Unmarshal(raw, &m); err != nil {
			errs = append(errs, err)
			continue
		}
		if _, err := a.svc.CreateMemory(contextsrv.MemoryCreateInput{
			Project:    data.Project,
			Content:    m.Content,
			Category:   m.Category,
			Owner:      m.Owner,
			Importance: m.Importance,
		}); err != nil {
			errs = append(errs, err)
		}
	}

	for _, raw := range data.Skills {
		var s struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.Unmarshal(raw, &s); err != nil {
			errs = append(errs, err)
			continue
		}
		if _, err := a.svc.CreateSkill(contextsrv.SkillCreateInput{
			Project:     data.Project,
			Name:        s.Name,
			Description: s.Description,
		}); err != nil {
			errs = append(errs, err)
		}
	}

	for _, raw := range data.ContextNodes {
		var n struct {
			URI       string  `json:"uri"`
			ParentURI *string `json:"parent_uri"`
			Name      string  `json:"name"`
			Abstract  string  `json:"abstract"`
			Overview  string  `json:"overview"`
			Content   string  `json:"content"`
		}
		if err := json.Unmarshal(raw, &n); err != nil {
			errs = append(errs, err)
			continue
		}
		if _, err := a.svc.CreateContextNode(contextsrv.ContextCreateInput{
			URI:       n.URI,
			Project:   data.Project,
			ParentURI: n.ParentURI,
			Name:      n.Name,
			Abstract:  n.Abstract,
			Overview:  n.Overview,
			Content:   n.Content,
		}); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("imported with %d errors", len(errs))
	}
	return nil
}
