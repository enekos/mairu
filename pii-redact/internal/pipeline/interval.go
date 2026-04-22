package pipeline

import "strings"

// interval marks a region [start:end) in a single layer's input string that
// must be replaced with text.
type interval struct {
	start int
	end   int
	kind  string
	text  string
}

// overlaps reports whether [s,e) overlaps any accepted interval.
func overlapsInterval(ivs []interval, s, e int) bool {
	for _, iv := range ivs {
		if s < iv.end && e > iv.start {
			return true
		}
	}
	return false
}

// applyIntervals builds a new string from input, replacing the regions described
// by non-overlapping intervals. The intervals must be sorted by start ascending;
// earlier intervals win when overlaps occur.
func applyIntervals(input string, ivs []interval) string {
	if len(ivs) == 0 {
		return input
	}
	var b strings.Builder
	b.Grow(len(input))
	last := 0
	for _, iv := range ivs {
		if iv.start < last {
			// Overlaps with a previous interval — skip.
			continue
		}
		b.WriteString(input[last:iv.start])
		b.WriteString(iv.text)
		last = iv.end
	}
	b.WriteString(input[last:])
	return b.String()
}
