package ast

import "strings"

func EnrichDescriptions(descriptions map[string]string, edges []LogicEdge) map[string]string {
	out := map[string]string{}
	for k, v := range descriptions {
		out[k] = v
	}

	edgesByFrom := map[string][]string{}
	for _, e := range edges {
		edgesByFrom[e.From] = append(edgesByFrom[e.From], e.To)
	}

	for from, tos := range edgesByFrom {
		existing := out[from]
		if existing == "" {
			existing = "Describes symbol."
		}
		out[from] = existing + " Calls: " + strings.Join(tos, ", ") + "."
	}

	for k, v := range out {
		out[k] = strings.TrimSpace(v)
	}
	return out
}
