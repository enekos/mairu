package crawler

import "context"

// State holds the intermediate data for the graph execution
type State map[string]any

// Node represents a single step in the scraping pipeline
type Node interface {
	Name() string
	Execute(ctx context.Context, state State) (State, error)
}

// Graph represents a sequence of nodes to execute
type Graph struct {
	Nodes []Node
}

// NewGraph creates a new execution graph
func NewGraph(nodes ...Node) *Graph {
	return &Graph{
		Nodes: nodes,
	}
}

// Run executes all nodes in sequence, passing the state between them
func (g *Graph) Run(ctx context.Context, initialState State) (State, error) {
	state := initialState
	var err error

	for _, node := range g.Nodes {
		state, err = node.Execute(ctx, state)
		if err != nil {
			return state, err
		}
	}

	return state, nil
}
