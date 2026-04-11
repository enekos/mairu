package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func GenerateAgentCLIRef(cmd *cobra.Command) string {
	var sb strings.Builder
	sb.WriteString("Mairu CLI Utilities:\n")

	var walk func(c *cobra.Command, path string)
	walk = func(c *cobra.Command, path string) {
		if c.Hidden {
			return
		}

		fullPath := c.Name()
		if path != "" {
			fullPath = path + " " + c.Name()
		}

		if c.Runnable() {
			sb.WriteString(fmt.Sprintf("\n### `mairu %s`\n", fullPath))
			if c.Short != "" {
				sb.WriteString(fmt.Sprintf("%s\n", c.Short))
			}

			flags := c.LocalFlags()
			if flags.HasFlags() {
				sb.WriteString("Flags:\n")
				flags.VisitAll(func(f *pflag.Flag) {
					if f.Hidden {
						return
					}
					name := "--" + f.Name
					if f.Shorthand != "" {
						name = "-" + f.Shorthand + ", " + name
					}
					sb.WriteString(fmt.Sprintf("  %s : %s\n", name, f.Usage))
				})
			}
		}

		for _, sub := range c.Commands() {
			walk(sub, fullPath)
		}
	}

	for _, sub := range cmd.Commands() {
		walk(sub, "")
	}

	return sb.String()
}
