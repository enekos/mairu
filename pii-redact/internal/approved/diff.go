package approved

import (
	"fmt"
	"strings"
)

// unifiedDiff produces a unified diff between two strings.
// Returns empty string if a and b are identical.
func unifiedDiff(a, b, labelA, labelB string, context int) string {
	if a == b {
		return ""
	}
	aLines := splitLines(a)
	bLines := splitLines(b)

	ops := computeOps(aLines, bLines)
	hunks := groupHunks(ops, aLines, bLines, context)
	if len(hunks) == 0 {
		return ""
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "--- %s\n+++ %s\n", labelA, labelB)
	for _, h := range hunks {
		fmt.Fprintf(&sb, "@@ -%d,%d +%d,%d @@\n", h.aStart+1, h.aCount, h.bStart+1, h.bCount)
		for _, l := range h.lines {
			sb.WriteString(l)
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	// Remove trailing empty element from final newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

type opKind int

const (
	opEqual opKind = iota
	opDelete
	opInsert
)

type editOp struct {
	kind  opKind
	aLine int // index in a (for equal/delete)
	bLine int // index in b (for equal/insert)
}

func computeOps(a, b []string) []editOp {
	m, n := len(a), len(b)
	// Build LCS table
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	var ops []editOp
	i, j := 0, 0
	for i < m && j < n {
		if a[i] == b[j] {
			ops = append(ops, editOp{kind: opEqual, aLine: i, bLine: j})
			i++
			j++
		} else if dp[i+1][j] >= dp[i][j+1] {
			ops = append(ops, editOp{kind: opDelete, aLine: i, bLine: j})
			i++
		} else {
			ops = append(ops, editOp{kind: opInsert, aLine: i, bLine: j})
			j++
		}
	}
	for ; i < m; i++ {
		ops = append(ops, editOp{kind: opDelete, aLine: i, bLine: j})
	}
	for ; j < n; j++ {
		ops = append(ops, editOp{kind: opInsert, aLine: i, bLine: j})
	}
	return ops
}

type hunk struct {
	aStart, aCount int
	bStart, bCount int
	lines          []string
}

func groupHunks(ops []editOp, aLines, bLines []string, context int) []hunk {
	type changeRange struct{ start, end int }
	var changes []changeRange

	for i, op := range ops {
		if op.kind != opEqual {
			if len(changes) > 0 && i-changes[len(changes)-1].end <= 2*context {
				changes[len(changes)-1].end = i + 1
			} else {
				changes = append(changes, changeRange{i, i + 1})
			}
		}
	}

	if len(changes) == 0 {
		return nil
	}

	var hunks []hunk
	for _, cr := range changes {
		start := cr.start - context
		if start < 0 {
			start = 0
		}
		end := cr.end + context
		if end > len(ops) {
			end = len(ops)
		}

		var h hunk
		h.aStart = ops[start].aLine
		h.bStart = ops[start].bLine
		for idx := start; idx < end; idx++ {
			op := ops[idx]
			switch op.kind {
			case opEqual:
				h.aCount++
				h.bCount++
				h.lines = append(h.lines, " "+aLines[op.aLine])
			case opDelete:
				h.aCount++
				h.lines = append(h.lines, "-"+aLines[op.aLine])
			case opInsert:
				h.bCount++
				h.lines = append(h.lines, "+"+bLines[op.bLine])
			}
		}

		hunks = append(hunks, h)
	}
	return hunks
}
