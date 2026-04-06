package crawler

import (
	"context"
	"testing"
)

type mockNode struct {
	name string
	fn   func(state State) (State, error)
}

func (m *mockNode) Name() string { return m.name }
func (m *mockNode) Execute(ctx context.Context, state State) (State, error) {
	return m.fn(state)
}

func TestGraphExecution(t *testing.T) {
	n1 := &mockNode{
		name: "Node1",
		fn: func(state State) (State, error) {
			state["val1"] = "hello"
			return state, nil
		},
	}
	n2 := &mockNode{
		name: "Node2",
		fn: func(state State) (State, error) {
			val1, _ := state["val1"].(string)
			state["val2"] = val1 + " world"
			return state, nil
		},
	}

	graph := NewGraph(n1, n2)
	finalState, err := graph.Run(context.Background(), State{})

	if err != nil {
		t.Fatalf("Graph run failed: %v", err)
	}

	val2, ok := finalState["val2"].(string)
	if !ok || val2 != "hello world" {
		t.Errorf("Unexpected final state val2: %v", val2)
	}
}
