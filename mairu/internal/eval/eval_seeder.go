package eval

import (
	"context"

	"mairu/internal/contextsrv"
)

type MemoryFixture struct {
	ID         string `json:"id"`
	Content    string `json:"content"`
	Category   string `json:"category"`
	Owner      string `json:"owner"`
	Importance int    `json:"importance"`
	Project    string `json:"project"`
}

type SkillFixture struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Project     string `json:"project"`
}

type ContextFixture struct {
	URI       string  `json:"uri"`
	Name      string  `json:"name"`
	Abstract  string  `json:"abstract"`
	Overview  string  `json:"overview"`
	Content   string  `json:"content"`
	ParentURI *string `json:"parent_uri"`
	Project   string  `json:"project"`
}

type FixtureSpec struct {
	Memories []MemoryFixture  `json:"memories"`
	Skills   []SkillFixture   `json:"skills"`
	Context  []ContextFixture `json:"context"`
}

func SeedFixtures(ctx context.Context, svc *contextsrv.AppService, spec FixtureSpec) error {
	for _, m := range spec.Memories {
		_, err := svc.CreateMemory(contextsrv.MemoryCreateInput{
			Project:    m.Project,
			Content:    m.Content,
			Category:   m.Category,
			Owner:      m.Owner,
			Importance: m.Importance,
		})
		if err != nil {
			return err
		}
	}
	for _, s := range spec.Skills {
		_, err := svc.CreateSkill(contextsrv.SkillCreateInput{
			Project:     s.Project,
			Name:        s.Name,
			Description: s.Description,
		})
		if err != nil {
			return err
		}
	}
	for _, c := range spec.Context {
		_, err := svc.CreateContextNode(contextsrv.ContextCreateInput{
			Project:   c.Project,
			URI:       c.URI,
			ParentURI: c.ParentURI,
			Name:      c.Name,
			Abstract:  c.Abstract,
			Overview:  c.Overview,
			Content:   c.Content,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func CleanupFixtures(ctx context.Context, svc *contextsrv.AppService, spec FixtureSpec) error {
	// Revers order for context nodes (children first) is ideal, but here we just delete by ID/URI
	for i := len(spec.Context) - 1; i >= 0; i-- {
		_ = svc.DeleteContextNode(spec.Context[i].URI)
	}
	for _, s := range spec.Skills {
		_ = svc.DeleteSkill(s.ID)
	}
	for _, m := range spec.Memories {
		_ = svc.DeleteMemory(m.ID)
	}
	return nil
}
