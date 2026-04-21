package approved

import (
	"testing"
)

type testItem struct {
	Name string
	Kind string
	Desc string
}

func TestAssertQuality_AllPass(t *testing.T) {
	items := []testItem{
		{Name: "foo", Kind: "fn", Desc: "does something useful"},
		{Name: "bar", Kind: "fn", Desc: "does another thing"},
	}
	// Should not fail
	AssertQuality(t, items, QualityCheck[testItem]{
		Name:   "has description",
		Filter: func(i testItem) bool { return i.Kind == "fn" },
		Assert: func(t testing.TB, i testItem) {
			if len(i.Desc) < 5 {
				t.Errorf("%s: description too short", i.Name)
			}
		},
	})
}

func TestAssertQuality_Failure(t *testing.T) {
	items := []testItem{
		{Name: "foo", Kind: "fn", Desc: "ok desc"},
		{Name: "bar", Kind: "fn", Desc: ""},
	}

	mt := &mockT{}
	AssertQuality(mt, items, QualityCheck[testItem]{
		Name:   "has description",
		Filter: func(i testItem) bool { return i.Kind == "fn" },
		Assert: func(t testing.TB, i testItem) {
			if i.Desc == "" {
				t.Errorf("%s: missing description", i.Name)
			}
		},
	})

	if !mt.failed {
		t.Error("expected quality check to fail")
	}
}

func TestAssertQuality_FilterSkips(t *testing.T) {
	items := []testItem{
		{Name: "MyType", Kind: "type", Desc: ""},
		{Name: "doStuff", Kind: "fn", Desc: "valid description here"},
	}
	// Filter only fn — type with empty desc should be skipped
	AssertQuality(t, items, QualityCheck[testItem]{
		Name:   "fn has description",
		Filter: func(i testItem) bool { return i.Kind == "fn" },
		Assert: func(t testing.TB, i testItem) {
			if i.Desc == "" {
				t.Errorf("%s: missing description", i.Name)
			}
		},
	})
}

func TestAssertQuality_MultipleChecks(t *testing.T) {
	items := []testItem{
		{Name: "a", Kind: "fn", Desc: "short"},
		{Name: "b", Kind: "fn", Desc: "a proper long description of behavior"},
	}

	mt := &mockT{}
	AssertQuality(mt, items,
		QualityCheck[testItem]{
			Name:   "desc not empty",
			Filter: func(i testItem) bool { return i.Kind == "fn" },
			Assert: func(t testing.TB, i testItem) {
				if i.Desc == "" {
					t.Errorf("%s: empty", i.Name)
				}
			},
		},
		QualityCheck[testItem]{
			Name:   "desc min length 10",
			Filter: func(i testItem) bool { return i.Kind == "fn" },
			Assert: func(t testing.TB, i testItem) {
				if len(i.Desc) < 10 {
					t.Errorf("%s: too short (%d chars)", i.Name, len(i.Desc))
				}
			},
		},
	)

	if !mt.failed {
		t.Error("expected second check to fail for item 'a'")
	}
}
