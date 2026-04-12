package ast

type SvelteDescriber struct{}

func (d SvelteDescriber) LanguageID() string   { return "svelte" }
func (d SvelteDescriber) Extensions() []string { return []string{".svelte"} }
func (d SvelteDescriber) ExtractFileGraph(_ string, source string) FileGraph {
	g := BaseExtract(source)
	g.FileSummary = "Svelte component graph extracted."
	return g
}
