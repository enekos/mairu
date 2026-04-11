package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"mairu/internal/config"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage mairu configuration",
	}
	cmd.AddCommand(NewConfigListCmd())
	cmd.AddCommand(NewConfigGetCmd())
	cmd.AddCommand(NewConfigSetCmd())
	cmd.AddCommand(NewConfigEditCmd())
	return cmd
}

func NewConfigListCmd() *cobra.Command {
	var showOrigin bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Show all resolved configuration values",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := GetConfig()
			if cfg == nil {
				return fmt.Errorf("config not loaded")
			}

			f := GetFormatter()

			if showOrigin {
				return printConfigWithOrigin(f)
			}

			// Flatten config to key=value pairs
			v := loadViperForDisplay()
			keys := v.AllKeys()
			sort.Strings(keys)

			rows := make([]map[string]string, 0, len(keys))
			for _, k := range keys {
				rows = append(rows, map[string]string{
					"key":   k,
					"value": fmt.Sprintf("%v", v.Get(k)),
				})
			}
			f.PrintTable([]string{"key", "value"}, rows)
			return nil
		},
	}
	cmd.Flags().BoolVar(&showOrigin, "show-origin", false, "Show which layer each value comes from")
	return cmd
}

func NewConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Get a configuration value (dot-notation)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			v := loadViperForDisplay()
			key := args[0]
			val := v.Get(key)
			if val == nil {
				return fmt.Errorf("unknown config key: %s", key)
			}
			GetFormatter().PrintRaw(val)
			return nil
		},
	}
}

func NewConfigSetCmd() *cobra.Command {
	var project bool
	cmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]

			var targetPath string
			if project {
				cwd, _ := os.Getwd()
				targetPath = config.FindProjectConfig(cwd)
				if targetPath == "" {
					targetPath = filepath.Join(cwd, ".mairu.toml")
				}
			} else {
				targetPath = config.UserConfigPath()
			}

			// Load existing file into a fresh viper, set value, write back
			fv := viper.New()
			fv.SetConfigFile(targetPath)
			fv.SetConfigType("toml")
			_ = fv.ReadInConfig() // ok if missing

			fv.Set(key, value)

			dir := filepath.Dir(targetPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("create config directory: %w", err)
			}
			if err := fv.WriteConfigAs(targetPath); err != nil {
				return fmt.Errorf("write config: %w", err)
			}

			fmt.Printf("Set %s = %s in %s\n", key, value, targetPath)
			return nil
		},
	}
	cmd.Flags().BoolVar(&project, "project", false, "Write to project config (.mairu.toml) instead of user config")
	return cmd
}

func NewConfigEditCmd() *cobra.Command {
	var project bool
	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Open configuration file in $EDITOR",
		RunE: func(cmd *cobra.Command, args []string) error {
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}

			var targetPath string
			if project {
				cwd, _ := os.Getwd()
				targetPath = config.FindProjectConfig(cwd)
				if targetPath == "" {
					targetPath = filepath.Join(cwd, ".mairu.toml")
				}
			} else {
				targetPath = config.UserConfigPath()
			}

			// Ensure file exists
			dir := filepath.Dir(targetPath)
			os.MkdirAll(dir, 0755)
			if _, err := os.Stat(targetPath); os.IsNotExist(err) {
				os.WriteFile(targetPath, []byte("# Mairu configuration\n# See: https://github.com/enekos/mairu\n\n"), 0644)
			}

			c := exec.Command(editor, targetPath)
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			return c.Run()
		},
	}
	cmd.Flags().BoolVar(&project, "project", false, "Edit project config (.mairu.toml)")
	return cmd
}

// loadViperForDisplay creates a fresh Viper instance that mirrors Load() for display purposes.
// It reuses config.NewViper() to avoid duplicating defaults.
func loadViperForDisplay() *viper.Viper {
	cwd, _ := os.Getwd()
	return config.NewViper(cwd)
}

func printConfigWithOrigin(f *Formatter) error {
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()

	// Load each layer separately to determine origin
	defaults := viper.New()
	defaults.SetDefault("api.meili_url", "http://localhost:7700") // ... abbreviated, same defaults
	// We'll compare against the resolved viper to determine source.

	resolved := loadViperForDisplay()
	keys := resolved.AllKeys()
	sort.Strings(keys)

	// Load user-only and project-only vipers
	userV := viper.New()
	userV.SetConfigType("toml")
	if home != "" {
		userV.SetConfigFile(filepath.Join(home, ".config", "mairu", "config.toml"))
		_ = userV.ReadInConfig()
	}

	projectV := viper.New()
	projectV.SetConfigType("toml")
	if p := config.FindProjectConfig(cwd); p != "" {
		projectV.SetConfigFile(p)
		_ = projectV.ReadInConfig()
	}

	rows := make([]map[string]string, 0, len(keys))
	for _, k := range keys {
		origin := "default"
		envKey := "MAIRU_" + strings.ToUpper(strings.ReplaceAll(k, ".", "_"))
		if _, ok := os.LookupEnv(envKey); ok {
			origin = fmt.Sprintf("env: %s", envKey)
		} else if projectV.IsSet(k) {
			origin = fmt.Sprintf("project: %s", config.FindProjectConfig(cwd))
		} else if userV.IsSet(k) {
			origin = fmt.Sprintf("user: %s", filepath.Join(home, ".config", "mairu", "config.toml"))
		}

		rows = append(rows, map[string]string{
			"key":    k,
			"value":  fmt.Sprintf("%v", resolved.Get(k)),
			"origin": origin,
		})
	}
	f.PrintTable([]string{"key", "value", "origin"}, rows)
	return nil
}
