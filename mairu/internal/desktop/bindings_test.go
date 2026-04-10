package desktop

import (
	"testing"

	"mairu/internal/contextsrv"
)

// Compile-time check: App must have the expected binding methods.
// This doesn't test behavior (needs a real service), but ensures the signatures exist.
var _ interface {
	ListMemories(project string, limit int) ([]contextsrv.Memory, error)
	CreateMemory(input contextsrv.MemoryCreateInput) (contextsrv.Memory, error)
	UpdateMemory(input contextsrv.MemoryUpdateInput) (contextsrv.Memory, error)
	DeleteMemory(id string) error
	ApplyMemoryFeedback(id string, reward int) (contextsrv.Memory, error)

	ListSkills(project string, limit int) ([]contextsrv.Skill, error)
	CreateSkill(input contextsrv.SkillCreateInput) (contextsrv.Skill, error)
	UpdateSkill(input contextsrv.SkillUpdateInput) (contextsrv.Skill, error)
	DeleteSkill(id string) error

	ListContextNodes(project string, parentURI *string, limit int) ([]contextsrv.ContextNode, error)
	CreateContextNode(input contextsrv.ContextCreateInput) (contextsrv.ContextNode, error)
	UpdateContextNode(input contextsrv.ContextUpdateInput) (contextsrv.ContextNode, error)
	DeleteContextNode(uri string) error

	Search(opts contextsrv.SearchOptions) (map[string]any, error)
	Dashboard(limit int, project string) (map[string]any, error)
	Health() map[string]any
	ClusterStats() map[string]any

	VibeQuery(prompt, project string, topK int) (contextsrv.VibeQueryResult, error)
	VibeMutationPlan(prompt, project string, topK int) (contextsrv.VibeMutationPlan, error)
	VibeMutationExecute(ops []contextsrv.VibeMutationOp, project string) ([]map[string]any, error)

	ListModerationQueue(limit int) ([]contextsrv.ModerationEvent, error)
	ReviewModeration(input contextsrv.ModerationReviewInput) error
} = (*App)(nil)

func TestBindingsCompile(t *testing.T) {
	// If this file compiles, the bindings exist with correct signatures.
	t.Log("all CRUD binding signatures verified")
}
