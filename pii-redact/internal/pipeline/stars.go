package pipeline

import "strings"

// starCache holds pre-built star strings for the most common lengths to avoid
// repeated allocations from strings.Repeat.
var starCache [65]string

func init() {
	for i := 1; i < len(starCache); i++ {
		starCache[i] = strings.Repeat("*", i)
	}
}

// stars returns a string of n asterisks, using a cached value when possible.
func stars(n int) string {
	switch {
	case n <= 0:
		return ""
	case n < len(starCache):
		return starCache[n]
	default:
		return strings.Repeat("*", n)
	}
}
