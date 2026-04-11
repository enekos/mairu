package contextsrv

import (
	"encoding/json"
	"time"
)

const (
	StoreMemories     = "memories"
	StoreMemory       = "memory"
	StoreSkill        = "skill"
	StoreSkills       = "skills"
	StoreContextNodes = "contextNodes"
	StoreContext      = "context"
	StoreAll          = "all"
	StoreNode         = "node"

	IndexMemories    = "contextfs_memories"
	IndexSkills      = "contextfs_skills"
	IndexNodes       = "contextfs_context_nodes"
	IndexSymbols     = "contextfs_symbols"
	IndexBashHistory = "contextfs_bash_history"

	ModerationStatusClean       = "clean"
	ModerationStatusFlaggedSoft = "flagged_soft"
	ModerationStatusRejectHard  = "reject_hard"
)

// BashHistory represents a recorded bash command execution.
type BashHistory struct {
	ID            string    `json:"id"`
	Project       string    `json:"project"`
	Command       string    `json:"command"`
	ExitCode      int       `json:"exit_code"`
	DurationMs    int       `json:"duration_ms"`
	Output        string    `json:"output"`
	Importance    int       `json:"importance"`
	FeedbackCount int       `json:"feedback_count"`
	CreatedAt     time.Time `json:"created_at"`
}

// Memory represents an atomic piece of knowledge stored by an agent or user.
type Memory struct {
	ID                string     `json:"id"`
	Project           string     `json:"project"`
	Content           string     `json:"content"`
	Category          string     `json:"category"`
	Owner             string     `json:"owner"`
	Importance        int        `json:"importance"`
	RetrievalCount    int        `json:"retrieval_count"`
	FeedbackCount     int        `json:"feedback_count"`
	LastRetrievedAt   *time.Time `json:"last_retrieved_at,omitempty"`
	ModerationStatus  string     `json:"moderation_status"`
	ModerationReasons []string   `json:"moderation_reasons"`
	ReviewRequired    bool       `json:"review_required"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// Skill represents a capability or tool available to agents.
type Skill struct {
	ID                string    `json:"id"`
	Project           string    `json:"project"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	ModerationStatus  string    `json:"moderation_status"`
	ModerationReasons []string  `json:"moderation_reasons"`
	ReviewRequired    bool      `json:"review_required"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// ContextNode represents a hierarchical piece of knowledge or code context.
type ContextNode struct {
	URI               string         `json:"uri"`
	Project           string         `json:"project"`
	ParentURI         *string        `json:"parent_uri,omitempty"`
	Name              string         `json:"name"`
	Abstract          string         `json:"abstract"`
	Overview          string         `json:"overview,omitempty"`
	Content           string         `json:"content,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
	ModerationStatus  string         `json:"moderation_status"`
	ModerationReasons []string       `json:"moderation_reasons"`
	ReviewRequired    bool           `json:"review_required"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

// MemoryCreateInput holds the data required to create a new Memory.
type MemoryCreateInput struct {
	Project           string
	Content           string
	Category          string
	Owner             string
	Importance        int
	Metadata          json.RawMessage
	ModerationStatus  string
	ModerationReasons []string
	ReviewRequired    bool
}

// MemoryUpdateInput holds the data required to update an existing Memory.
type MemoryUpdateInput struct {
	ID         string
	Content    string
	Category   string
	Owner      string
	Importance int
}

// SkillCreateInput holds the data required to create a new Skill.
type SkillCreateInput struct {
	Project           string
	Name              string
	Description       string
	Metadata          json.RawMessage
	ModerationStatus  string
	ModerationReasons []string
	ReviewRequired    bool
}

// SkillUpdateInput holds the data required to update an existing Skill.
type SkillUpdateInput struct {
	ID          string
	Name        string
	Description string
}

// ContextCreateInput holds the data required to create a new ContextNode.
type ContextCreateInput struct {
	URI               string
	Project           string
	ParentURI         *string
	Name              string
	Abstract          string
	Overview          string
	Content           string
	Metadata          json.RawMessage
	ModerationStatus  string
	ModerationReasons []string
	ReviewRequired    bool
}

// ContextUpdateInput holds the data required to update an existing ContextNode.
type ContextUpdateInput struct {
	URI      string
	Name     string
	Abstract string
	Overview string
	Content  string
}

// SearchOptions defines the configuration and weights used when performing a search.
type SearchOptions struct {
	Query         string             `json:"query"`
	Project       string             `json:"project"`
	Store         string             `json:"store"`
	TopK          int                `json:"topK"`
	MinScore      float64            `json:"minScore"`
	Highlight     bool               `json:"highlight"`
	FieldBoosts   map[string]float64 `json:"fieldBoosts"`
	Fuzziness     string             `json:"fuzziness"`
	PhraseBoost   float64            `json:"phraseBoost"`
	WeightVector  float64            `json:"weightVector"`
	WeightKeyword float64            `json:"weightKeyword"`
	WeightRecency float64            `json:"weightRecency"`
	WeightImp     float64            `json:"weightImportance"`
	RecencyScale  string             `json:"recencyScale"`
	RecencyDecay  float64            `json:"recencyDecay"`
}

// VibeQueryResult represents the output of a natural language read operation (vibe query).
type VibeQueryResult struct {
	Reasoning string            `json:"reasoning"`
	Results   []VibeSearchGroup `json:"results"`
}

// VibeSearchGroup groups search results for a specific internal tool execution.
type VibeSearchGroup struct {
	Store string           `json:"store"`
	Query string           `json:"query"`
	Items []map[string]any `json:"items"`
}

// VibeMutationPlan represents the LLM-generated plan for mutating the context space.
type VibeMutationPlan struct {
	Reasoning  string           `json:"reasoning"`
	Operations []VibeMutationOp `json:"operations"`
}

// VibeMutationOp defines a single operation within a vibe mutation plan.
type VibeMutationOp struct {
	Op          string         `json:"op"`
	Target      string         `json:"target,omitempty"`
	Description string         `json:"description"`
	Data        map[string]any `json:"data"`
}

// ModerationEvent records the outcome of evaluating content against safety and moderation policies.
type ModerationEvent struct {
	ID               int64     `json:"id"`
	EntityType       string    `json:"entity_type"`
	EntityID         string    `json:"entity_id"`
	Project          string    `json:"project"`
	Decision         string    `json:"decision"`
	Reasons          []string  `json:"reasons"`
	ReviewStatus     string    `json:"review_status"`
	ReviewerDecision string    `json:"reviewer_decision,omitempty"`
	ReviewRequired   bool      `json:"review_required"`
	PolicyVersion    string    `json:"policy_version"`
	CreatedAt        time.Time `json:"created_at"`
	ReviewedAt       time.Time `json:"reviewed_at,omitempty"`
	Reviewer         string    `json:"reviewer,omitempty"`
}

// ModerationReviewInput holds the required fields to manually review a flagged moderation event.
type ModerationReviewInput struct {
	EventID   int64
	Decision  string
	Reviewer  string
	Notes     string
	UpdatedBy string
}
