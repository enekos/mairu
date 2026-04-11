package admincmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func NewInitCmd() *cobra.Command {
	var defaults bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a project configuration file (.mairu.toml)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			target := filepath.Join(cwd, ".mairu.toml")

			if _, err := os.Stat(target); err == nil {
				return fmt.Errorf(".mairu.toml already exists in %s", cwd)
			}

			if defaults {
				return writeMinimalConfig(target, filepath.Base(cwd))
			}

			return runInitWizard(target, cwd)
		},
	}
	cmd.Flags().BoolVar(&defaults, "defaults", false, "Create minimal .mairu.toml without prompting")
	return cmd
}

func runInitWizard(target, cwd string) error {
	reader := bufio.NewReader(os.Stdin)
	dirName := filepath.Base(cwd)

	fmt.Printf("Project name [%s]: ", dirName)
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)
	if name == "" {
		name = dirName
	}

	fmt.Print("Customize search weights? [y/N]: ")
	ans, _ := reader.ReadString('\n')
	ans = strings.TrimSpace(strings.ToLower(ans))

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Mairu project config for %s\n\n", name))

	if ans == "y" || ans == "yes" {
		b.WriteString("# Uncomment and adjust search weights (must sum to ~1.0, auto-normalized)\n")
		b.WriteString("# [search.memories]\n")
		b.WriteString("# vector = 0.6\n")
		b.WriteString("# keyword = 0.2\n")
		b.WriteString("# recency = 0.05\n")
		b.WriteString("# importance = 0.15\n\n")
	}

	fmt.Print("Customize daemon settings? [y/N]: ")
	ans, _ = reader.ReadString('\n')
	ans = strings.TrimSpace(strings.ToLower(ans))

	if ans == "y" || ans == "yes" {
		b.WriteString("[daemon]\n")
		b.WriteString("# concurrency = 8\n")
		b.WriteString("# max_file_size = \"512KB\"\n")
		b.WriteString("# debounce = \"200ms\"\n\n")
	}

	if err := os.WriteFile(target, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write .mairu.toml: %w", err)
	}

	fmt.Printf("Created %s\n", target)
	fmt.Println("Tip: add .mairu.toml to git so your team shares the same config")
	return nil
}

func writeMinimalConfig(target, projectName string) error {
	content := fmt.Sprintf("# Mairu project config for %s\n", projectName)
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write .mairu.toml: %w", err)
	}
	fmt.Printf("Created %s\n", target)
	return nil
}
