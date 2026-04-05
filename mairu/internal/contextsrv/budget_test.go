package contextsrv

import (
	"testing"
)

func TestExceedsBudget(t *testing.T) {
	b := Budget{MemoryPerProject: 2}
	if !ExceedsBudget(3, 0, 0, b) {
		t.Fatal("expected memory budget exceed")
	}
	if ExceedsBudget(2, 0, 0, b) {
		t.Fatal("did not expect exceed at limit")
	}
}
