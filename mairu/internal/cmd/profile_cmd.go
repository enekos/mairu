package cmd

import (
	"bufio"
	"fmt"
	"os"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"mairu/internal/profile"
)

// NewProfileCmd returns the `mairu profile` command group: install, update,
// info, and list. Mirrors hermes-agent's `hermes profile` surface.
func NewProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Install, update, and inspect mairu profile distributions",
		Long: `Profile distributions package a prompt + config bundle as a git repo
that anyone can install with one command and update in place.

A profile lives under ~/.config/mairu/profiles/<name>/ and overlays its
config.toml and agent_system.md on top of the global config.`,
	}
	cmd.AddCommand(
		newProfileInstallCmd(),
		newProfileUpdateCmd(),
		newProfileInfoCmd(),
		newProfileListCmd(),
	)
	return cmd
}

// mairuVersion returns the running binary's semver, or "" for unversioned
// dev builds (`go run`, `go test`, `go build` without ldflags). An empty
// string disables the manifest's mairu_requires gate — that's deliberate,
// because dev builds have no meaningful version to compare against.
func mairuVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return strings.TrimPrefix(info.Main.Version, "v")
	}
	return ""
}

func newProfileInstallCmd() *cobra.Command {
	var (
		name  string
		force bool
		yes   bool
	)
	cmd := &cobra.Command{
		Use:   "install <source>",
		Short: "Install a profile distribution from a git URL or local directory",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			source := args[0]
			if !yes {
				fmt.Printf("Installing distribution from %q\n", source)
				if !confirm("Continue? [y/N]: ") {
					return fmt.Errorf("aborted")
				}
			}
			plan, err := profile.Install(source, profile.InstallOptions{
				Name:         name,
				MairuVersion: mairuVersion(),
				Force:        force,
			})
			if err != nil {
				return err
			}
			fmt.Printf("Installed profile %q (version %s) at %s\n",
				plan.Manifest.Name, plan.Manifest.Version, plan.TargetDir)
			if len(plan.Manifest.EnvRequires) > 0 {
				fmt.Printf("Fill in environment variables: cp %s/.env.EXAMPLE %s/.env\n",
					plan.TargetDir, plan.TargetDir)
			}
			fmt.Printf("Activate with: mairu -p %s ...\n", plan.Manifest.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Override the manifest's name (install the same distribution under a different local name)")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing profile of the same name")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the confirmation prompt")
	return cmd
}

func newProfileUpdateCmd() *cobra.Command {
	var (
		forceConfig bool
		yes         bool
	)
	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Re-pull a profile's distribution and apply updates",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if !yes {
				fmt.Printf("Updating profile %q from its recorded source\n", name)
				if !confirm("Continue? [y/N]: ") {
					return fmt.Errorf("aborted")
				}
			}
			plan, err := profile.Update(name, profile.UpdateOptions{
				MairuVersion: mairuVersion(),
				ForceConfig:  forceConfig,
			})
			if err != nil {
				return err
			}
			suffix := ""
			if plan.PreservesConfig {
				suffix = " (config.toml preserved)"
			}
			fmt.Printf("Updated profile %q to version %s%s\n",
				plan.Manifest.Name, plan.Manifest.Version, suffix)
			return nil
		},
	}
	cmd.Flags().BoolVar(&forceConfig, "force-config", false, "Replace config.toml from the new source (default: preserve user edits)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the confirmation prompt")
	return cmd
}

func newProfileInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info <name>",
		Short: "Show distribution metadata for an installed profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, dir, err := profile.Describe(args[0])
			if err != nil {
				return err
			}
			if m == nil {
				fmt.Printf("Profile %q exists at %s but is not a distribution (no %s).\n",
					args[0], dir, profile.ManifestFilename)
				return nil
			}
			fmt.Printf("Profile:      %s\n", m.Name)
			fmt.Printf("Version:      %s\n", m.Version)
			if m.Description != "" {
				fmt.Printf("Description:  %s\n", m.Description)
			}
			if m.Author != "" {
				fmt.Printf("Author:       %s\n", m.Author)
			}
			if m.License != "" {
				fmt.Printf("License:      %s\n", m.License)
			}
			if m.MairuRequires != "" {
				fmt.Printf("Requires:     mairu %s\n", m.MairuRequires)
			}
			if m.Source != "" {
				fmt.Printf("Source:       %s\n", m.Source)
			}
			if m.InstalledAt != "" {
				fmt.Printf("Installed:    %s\n", m.InstalledAt)
			}
			fmt.Printf("Path:         %s\n", dir)
			if len(m.EnvRequires) > 0 {
				fmt.Println("\nEnvironment variables:")
				for _, r := range m.EnvRequires {
					status := "required"
					if !r.Required {
						status = "optional"
					}
					line := fmt.Sprintf("  %s (%s)", r.Name, status)
					if r.Description != "" {
						line += " — " + r.Description
					}
					fmt.Println(line)
				}
			}
			return nil
		},
	}
}

func newProfileListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed profile distributions",
		RunE: func(cmd *cobra.Command, args []string) error {
			names, err := profile.List()
			if err != nil {
				return err
			}
			sort.Strings(names)
			if len(names) == 0 {
				fmt.Println("No profiles installed.")
				return nil
			}
			f := GetFormatter()
			rows := make([]map[string]string, 0, len(names))
			for _, n := range names {
				m, dir, err := profile.Describe(n)
				if err != nil {
					continue
				}
				row := map[string]string{
					"profile": n,
					"path":    dir,
				}
				if m != nil {
					row["version"] = m.Version
					row["source"] = m.Source
				}
				rows = append(rows, row)
			}
			f.PrintTable([]string{"profile", "version", "source", "path"}, rows)
			return nil
		},
	}
}

func confirm(prompt string) bool {
	fmt.Print(prompt)
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil {
		return false
	}
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}
