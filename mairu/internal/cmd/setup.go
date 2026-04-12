package cmd

import (
	"bufio"
	"fmt"
	"mairu/internal/config"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func NewSetupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Setup Mairu configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Welcome to Mairu Setup!")
			fmt.Print("Please enter your Gemini API Key: ")
			reader := bufio.NewReader(os.Stdin)
			apiKey, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("error reading input: %w", err)
			}
			apiKey = strings.TrimSpace(apiKey)
			if apiKey == "" {
				return fmt.Errorf("API key cannot be empty")
			}

			targetPath := config.UserConfigPath()
			fv := config.NewViper("")
			fv.SetConfigFile(targetPath)
			fv.SetConfigType("toml")
			_ = fv.ReadInConfig() // ok if missing

			fv.Set("api.gemini_api_key", apiKey)

			dir := filepath.Dir(targetPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("error creating config directory: %w", err)
			}
			if err := fv.WriteConfigAs(targetPath); err != nil {
				return fmt.Errorf("error saving configuration: %w", err)
			}

			fmt.Printf("Configuration saved successfully to %s!\n", targetPath)
			return nil
		},
	}
	return cmd
}
