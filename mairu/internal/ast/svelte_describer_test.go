package ast

import "testing"

func TestSvelteDescriber(t *testing.T) {
	d := SvelteDescriber{}
	g := d.ExtractFileGraph("a.svelte", "<script>export function handleClick(){}</script>")
	if d.LanguageID() != "svelte" {
		t.Fatalf("unexpected language id: %s", d.LanguageID())
	}
	if len(g.Symbols) == 0 {
		t.Fatal("expected symbols")
	}
}
