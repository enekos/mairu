package approved

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// jsonFieldDiff compares two unmarshaled JSON trees and returns a human-readable
// field-level diff. Returns empty string if trees are identical.
// Both a and b should be map[string]any (from json.Unmarshal into any).
func jsonFieldDiff(a, b any) string {
	var diffs []string
	walkDiff("", a, b, &diffs)
	if len(diffs) == 0 {
		return ""
	}
	return strings.Join(diffs, "\n")
}

func walkDiff(path string, a, b any, diffs *[]string) {
	// Both maps
	aMap, aIsMap := a.(map[string]any)
	bMap, bIsMap := b.(map[string]any)
	if aIsMap && bIsMap {
		allKeys := mergeKeys(aMap, bMap)
		for _, k := range allKeys {
			childPath := joinPath(path, k)
			aVal, aHas := aMap[k]
			bVal, bHas := bMap[k]
			if aHas && !bHas {
				*diffs = append(*diffs, fmt.Sprintf("  %s: %s (removed)", childPath, brief(aVal)))
			} else if !aHas && bHas {
				*diffs = append(*diffs, fmt.Sprintf("  %s: %s (added)", childPath, brief(bVal)))
			} else {
				walkDiff(childPath, aVal, bVal, diffs)
			}
		}
		return
	}

	// Both arrays
	aArr, aIsArr := a.([]any)
	bArr, bIsArr := b.([]any)
	if aIsArr && bIsArr {
		minLen := len(aArr)
		if len(bArr) < minLen {
			minLen = len(bArr)
		}
		for i := 0; i < minLen; i++ {
			walkDiff(fmt.Sprintf("%s[%d]", path, i), aArr[i], bArr[i], diffs)
		}
		for i := minLen; i < len(aArr); i++ {
			*diffs = append(*diffs, fmt.Sprintf("  %s[%d]: %s (removed)", path, i, brief(aArr[i])))
		}
		for i := minLen; i < len(bArr); i++ {
			*diffs = append(*diffs, fmt.Sprintf("  %s[%d]: %s (added)", path, i, brief(bArr[i])))
		}
		return
	}

	// Leaf comparison
	if !jsonEqual(a, b) {
		*diffs = append(*diffs, fmt.Sprintf("  %s: %s -> %s", path, brief(a), brief(b)))
	}
}

func joinPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

func mergeKeys(a, b map[string]any) []string {
	seen := make(map[string]bool)
	var keys []string
	for k := range a {
		if !seen[k] {
			keys = append(keys, k)
			seen[k] = true
		}
	}
	for k := range b {
		if !seen[k] {
			keys = append(keys, k)
			seen[k] = true
		}
	}
	sort.Strings(keys)
	return keys
}

func jsonEqual(a, b any) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}

// brief returns a short JSON representation of v, truncated if long.
func brief(v any) string {
	b, _ := json.Marshal(v)
	s := string(b)
	if len(s) > 80 {
		return s[:77] + "..."
	}
	return s
}
