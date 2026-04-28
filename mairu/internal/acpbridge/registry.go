package acpbridge

import "sync"

// Registry tracks active bridge sessions.
type Registry struct {
	mu sync.RWMutex
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry { return &Registry{} }
