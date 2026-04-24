package cmd

import (
	"fmt"
	"strings"
)

const integrationContentCap = 5000

// integrationIssue is the shared shape produced by external-tracker sync
// commands (github sync-issues, linear sync-issues, ...) before persisting
// into the context-node store.
type integrationIssue struct {
	Source   string   // e.g. "github", "linear"
	ID       string   // issue identifier, used in the URI path
	Name     string   // human-readable title for the node
	Abstract []string // joined with " | "
	Overview []string // joined with "\n"
	Content  string   // truncated at integrationContentCap
}

func syncIntegrationNode(project string, n integrationIssue) error {
	uri := fmt.Sprintf("contextfs://%s/%s/issues/%s", project, n.Source, n.ID)
	parent := fmt.Sprintf("contextfs://%s/%s", project, n.Source)
	content := n.Content
	if len(content) > integrationContentCap {
		content = content[:integrationContentCap] + "\n...(truncated)"
	}
	_, err := StoreNodeRaw(project, uri, n.Name,
		strings.Join(n.Abstract, " | "), parent,
		strings.Join(n.Overview, "\n"), content)
	return err
}

func syncIntegrationSummary(project, source, detail string, count int) {
	memContent := fmt.Sprintf("Synced %d %s into project '%s'.", count, detail, project)
	_ = RunMemoryStore(project, memContent, source+"_sync", source, 7)
}
