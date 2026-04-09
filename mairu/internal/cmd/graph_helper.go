package cmd

type LogicEdge struct {
	From      string
	To        string
	Kind      string
	SourceURI string
}

type LogicGraph struct {
	Symbols map[string]string // ID -> URI
	Edges   []LogicEdge
}

func loadLogicGraph(project string) (*LogicGraph, error) {
	nodes, err := fetchAllNodes(project)
	if err != nil {
		return nil, err
	}

	graph := &LogicGraph{
		Symbols: make(map[string]string),
		Edges:   []LogicEdge{},
	}

	for _, n := range nodes {
		uri, _ := n["uri"].(string)
		meta, ok := n["metadata"].(map[string]any)
		if !ok {
			continue
		}
		lg, ok := meta["logic_graph"].(map[string]any)
		if !ok {
			continue
		}

		if syms, ok := lg["symbols"].([]any); ok {
			for _, sRaw := range syms {
				s, ok := sRaw.(map[string]any)
				if ok {
					id, _ := s["ID"].(string)
					if id == "" {
						id, _ = s["id"].(string)
					}
					if id != "" {
						graph.Symbols[id] = uri
					}
				}
			}
		}

		if edgs, ok := lg["edges"].([]any); ok {
			for _, eRaw := range edgs {
				e, ok := eRaw.(map[string]any)
				if ok {
					from, _ := e["From"].(string)
					if from == "" {
						from, _ = e["from"].(string)
					}
					to, _ := e["To"].(string)
					if to == "" {
						to, _ = e["to"].(string)
					}
					kind, _ := e["Kind"].(string)
					if kind == "" {
						kind, _ = e["kind"].(string)
					}

					if from != "" && to != "" {
						graph.Edges = append(graph.Edges, LogicEdge{
							From:      from,
							To:        to,
							Kind:      kind,
							SourceURI: uri,
						})
					}
				}
			}
		}
	}

	return graph, nil
}

func (g *LogicGraph) GetReverseDependencies() map[string][]string {
	reverseDeps := map[string][]string{}
	for _, e := range g.Edges {
		reverseDeps[e.To] = append(reverseDeps[e.To], e.SourceURI)
	}
	return reverseDeps
}
