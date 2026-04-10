package analyzer

func AnalyzeFlows(graph *LogicGraph) []Flow {
	incomingCounts := map[string]int{}
	for _, e := range graph.Edges {
		incomingCounts[e.To]++
	}

	var flows []Flow

	for sym := range graph.Symbols {
		if incomingCounts[sym] == 0 {
			visited := map[string]bool{sym: true}
			queue := []string{sym}
			trace := []string{sym}

			for len(queue) > 0 {
				curr := queue[0]
				queue = queue[1:]

				for _, e := range graph.Edges {
					if e.From == curr && !visited[e.To] {
						visited[e.To] = true
						trace = append(trace, e.To)
						queue = append(queue, e.To)
					}
				}
			}

			if len(trace) > 1 {
				flows = append(flows, Flow{
					StartSymbol: sym,
					Trace:       trace,
				})
			}
		}
	}

	return flows
}

func AnalyzeClusters(graph *LogicGraph) []Cluster {
	visitedClusters := map[string]bool{}
	var clusters []Cluster

	adj := map[string][]string{}
	for _, e := range graph.Edges {
		adj[e.From] = append(adj[e.From], e.To)
		adj[e.To] = append(adj[e.To], e.From)
	}

	for sym := range graph.Symbols {
		if !visitedClusters[sym] {
			clusterSyms := []string{}
			queue := []string{sym}
			visitedClusters[sym] = true

			for len(queue) > 0 {
				curr := queue[0]
				queue = queue[1:]
				clusterSyms = append(clusterSyms, curr)

				for _, neighbor := range adj[curr] {
					if !visitedClusters[neighbor] {
						visitedClusters[neighbor] = true
						queue = append(queue, neighbor)
					}
				}
			}

			if len(clusterSyms) > 1 {
				clusters = append(clusters, Cluster{Symbols: clusterSyms})
			}
		}
	}

	return clusters
}
