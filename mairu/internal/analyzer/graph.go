package analyzer

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

func (g *LogicGraph) GetReverseDependencies() map[string][]string {
	reverseDeps := map[string][]string{}
	for _, e := range g.Edges {
		reverseDeps[e.To] = append(reverseDeps[e.To], e.SourceURI)
	}
	return reverseDeps
}

type Flow struct {
	StartSymbol string
	Trace       []string
}

type Cluster struct {
	Symbols []string
}
