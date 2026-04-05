package tui

import (
	"fmt"
	"strings"
)

type graphListItem struct {
	title   string
	desc    string
	uri     string
	content string
	depth   int
	isLast  bool
	prefix  string
	node    *NodeItem
}

func (i graphListItem) Title() string {
	return i.prefix + i.title
}

func (i graphListItem) Description() string {
	if i.desc == "" {
		return ""
	}
	descPrefix := strings.Repeat("  ", i.depth+1)
	// Truncate desc so it fits
	d := i.desc
	if len(d) > 60 {
		d = d[:57] + "..."
	}
	return descPrefix + d
}

func (i graphListItem) FilterValue() string {
	return i.title + " " + i.desc + " " + i.uri
}

type NodeItem struct {
	URI      string      `json:"uri"`
	Project  string      `json:"project"`
	Name     string      `json:"name"`
	Abstract string      `json:"abstract"`
	Content  string      `json:"content"`
	Parent   *string     `json:"parent_uri"`
	Children []*NodeItem `json:"-"`
}

func buildGraphItems(nodes []NodeItem) []graphListItem {
	nodeMap := make(map[string]*NodeItem)
	var roots []*NodeItem

	for i := range nodes {
		nodeMap[nodes[i].URI] = &nodes[i]
	}

	for i := range nodes {
		n := &nodes[i]
		if n.Parent != nil && *n.Parent != "" {
			if parent, ok := nodeMap[*n.Parent]; ok {
				parent.Children = append(parent.Children, n)
			} else {
				roots = append(roots, n)
			}
		} else {
			roots = append(roots, n)
		}
	}

	var items []graphListItem

	var traverse func(node *NodeItem, depth int, prefix string, isLast bool)
	traverse = func(node *NodeItem, depth int, prefix string, isLast bool) {
		currentPrefix := prefix
		nextPrefix := prefix

		if depth > 0 {
			if isLast {
				currentPrefix += "└── "
				nextPrefix += "    "
			} else {
				currentPrefix += "├── "
				nextPrefix += "│   "
			}
		}

		title := fmt.Sprintf("[%s] %s", node.Project, node.Name)
		if node.Name == "" {
			title = fmt.Sprintf("[%s] %s", node.Project, node.URI)
		}

		items = append(items, graphListItem{
			title:   title,
			desc:    node.Abstract,
			uri:     node.URI,
			content: node.Content,
			depth:   depth,
			isLast:  isLast,
			prefix:  currentPrefix,
			node:    node,
		})

		for i, child := range node.Children {
			traverse(child, depth+1, nextPrefix, i == len(node.Children)-1)
		}
	}

	for i, root := range roots {
		traverse(root, 0, "", i == len(roots)-1)
	}

	return items
}

type memoryListItem struct {
	id         string
	project    string
	content    string
	category   string
	owner      string
	importance int
}

func (i memoryListItem) Title() string {
	d := i.content
	if len(d) > 40 {
		d = d[:37] + "..."
	}
	return fmt.Sprintf("[%s] %s", i.category, d)
}

func (i memoryListItem) Description() string {
	return fmt.Sprintf("ID: %s | Imp: %d", i.id, i.importance)
}

func (i memoryListItem) FilterValue() string {
	return i.content + " " + i.category + " " + i.owner + " " + i.id
}

type skillListItem struct {
	id      string
	project string
	name    string
	desc    string
}

func (i skillListItem) Title() string {
	return i.name
}

func (i skillListItem) Description() string {
	d := i.desc
	if len(d) > 60 {
		d = d[:57] + "..."
	}
	return d
}

func (i skillListItem) FilterValue() string {
	return i.name + " " + i.desc + " " + i.id
}
