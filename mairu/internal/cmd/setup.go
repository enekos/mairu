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
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Welcome to Mairu Setup!")
			fmt.Print("Please enter your Gemini API Key: ")
			reader := bufio.NewReader(os.Stdin)
			apiKey, err := reader.ReadString('\n')
			if err != nil {
				fmt.Printf("Error reading input: %v\n", err)
				os.Exit(1)
			}
			apiKey = strings.TrimSpace(apiKey)
			if apiKey == "" {
				fmt.Println("API Key cannot be empty.")
				os.Exit(1)
			}

			targetPath := config.UserConfigPath()
			fv := config.NewViper("")
			fv.SetConfigFile(targetPath)
			fv.SetConfigType("toml")
			_ = fv.ReadInConfig() // ok if missing

			fv.Set("api.gemini_api_key", apiKey)

			dir := filepath.Dir(targetPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Printf("Error creating config directory: %v\n", err)
				os.Exit(1)
			}
			if err := fv.WriteConfigAs(targetPath); err != nil {
				fmt.Printf("Error saving configuration: %v\n", err)
				os.Exit(1)
			}

			fmt.Printf("Configuration saved successfully to %s!\n", targetPath)
		},
	}
	return cmd
}
