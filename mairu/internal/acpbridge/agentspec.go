package acpbridge

// AgentSpec describes how to launch a local ACP agent process.
type AgentSpec struct {
	Command string
	Args    []string
}

// DefaultAgentSpecs returns the built-in agent specifications.
func DefaultAgentSpecs() map[string]AgentSpec {
	return map[string]AgentSpec{}
}
