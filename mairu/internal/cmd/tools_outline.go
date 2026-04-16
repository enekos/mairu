package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"mairu/internal/codetools"

	"github.com/spf13/cobra"
)

var outlineExports bool
var outlineTree bool
var outlineFull bool

func NewOutlineCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "outline <file>...",
		Short: "AI-optimized file skeleton (JSON)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := codetools.OutlineOptions{
				ExportsOnly: outlineExports,
				TreeMode:    outlineTree,
				FullMode:    outlineFull,
			}

			var results []*codetools.OutlineResult
			for _, file := range args {
				res, err := codetools.OutlineFile(file, opts)
				if err != nil {
					if err == os.ErrInvalid {
						return fmt.Errorf("error: unsupported file type for %s", file)
					}
					return fmt.Errorf("error: %w", err)
				}
				results = append(results, res)
			}

			if outputFormat == "json" {
				if len(results) == 1 {
					out, _ := json.Marshal(results[0])
					fmt.Println(string(out))
				} else {
					out, _ := json.Marshal(results)
					fmt.Println(string(out))
				}
			} else {
				f := GetFormatter()
				for i, res := range results {
					if i > 0 {
						fmt.Println()
					}
					fmt.Printf("File: %s\n", res.File)
					if len(res.Imports) > 0 {
						fmt.Printf("Imports: %d\n", len(res.Imports))
					}

					if len(res.Symbols) > 0 {
						fmt.Println("\nSymbols:")
						var processSymbols func([]codetools.OutlineSymbol, string) []map[string]any
						processSymbols = func(syms []codetools.OutlineSymbol, prefix string) []map[string]any {
							var flat []map[string]any
							for _, s := range syms {
								flat = append(flat, map[string]any{
									"kind": s.Kind,
									"name": prefix + s.Name,
									"line": s.Line,
									"sig":  s.Signature,
									"desc": s.Description,
								})
								if len(s.Children) > 0 {
									flat = append(flat, processSymbols(s.Children, prefix+"  ")...)
								}
							}
							return flat
						}

						items := processSymbols(res.Symbols, "")
						f.PrintItems(
							[]string{"line", "kind", "name", "signature", "description"},
							items,
							func(item map[string]any) map[string]string {
								desc := fmt.Sprintf("%v", item["desc"])
								desc = strings.ReplaceAll(desc, "\n", " ")
								if desc == "<nil>" || desc == "" {
									desc = ""
								} else if len(desc) > 100 {
									desc = desc[:97] + "..."
								}

								return map[string]string{
									"line":        fmt.Sprintf("%v", item["line"]),
									"kind":        fmt.Sprintf("%v", item["kind"]),
									"name":        fmt.Sprintf("%v", item["name"]),
									"signature":   fmt.Sprintf("%v", item["sig"]),
									"description": desc,
								}
							},
						)
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&outlineExports, "exports", false, "Only show exported/public symbols")
	cmd.Flags().BoolVar(&outlineTree, "tree", false, "Nest methods under their parent class")
	cmd.Flags().BoolVar(&outlineFull, "full", false, "Include variables, fields, and properties in output")
	return cmd
}
