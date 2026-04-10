package cmd

import "mairu/internal/analyzer"

func loadLogicGraph(project string) (*analyzer.LogicGraph, error) {
	nodes, err := FetchAllNodes(project)
	if err != nil {
		return nil, err
	}

	graph := &analyzer.LogicGraph{
		Symbols: make(map[string]string),
		Edges:   []analyzer.LogicEdge{},
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
						graph.Edges = append(graph.Edges, analyzer.LogicEdge{
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
