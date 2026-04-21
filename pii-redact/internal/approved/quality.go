package approved

import (
	"testing"
)

// QualityCheck defines a named invariant to verify against a data set.
type QualityCheck[T any] struct {
	Name   string
	Filter func(T) bool
	Assert func(t testing.TB, item T)
}

// AssertQuality runs all checks against items.
// For each check, only items passing Filter are tested.
// Failures are reported with the check name for context.
func AssertQuality[T any](t testing.TB, items []T, checks ...QualityCheck[T]) {
	t.Helper()
	for _, check := range checks {
		for _, item := range items {
			if check.Filter != nil && !check.Filter(item) {
				continue
			}
			check.Assert(t, item)
		}
	}
}
