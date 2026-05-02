package acpbridge

import "os"

// AgentSpec describes how to launch an ACP-speaking agent as a subprocess.
type AgentSpec struct {
	Command string   // executable name (looked up in PATH) or absolute path
	Args    []string // arguments after Command
}

// DefaultAgentSpecs returns the built-in registry of supported agents.
//
// "mairu" re-execs the current binary with `acp` so the bridge always pairs
// with the same mairu version it shipped with.
func DefaultAgentSpecs() map[string]AgentSpec {
	self, err := os.Executable()
	if err != nil {
		self = "mairu"
	}
	return map[string]AgentSpec{
		"mairu":       {Command: self, Args: []string{"acp"}},
		"claude-code": {Command: "claude-code", Args: []string{"--acp"}},
		"gemini":      {Command: "gemini", Args: []string{"--acp"}},
	}
}
