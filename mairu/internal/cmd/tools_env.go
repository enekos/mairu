package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(envCmd)
}

type envResult struct {
	Keys []string `json:"keys"`
}

var envCmd = &cobra.Command{
	Use:   "env [file]",
	Short: "AI-optimized safe environment reader (JSON keys only)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		file := ".env"
		if len(args) > 0 {
			file = args[0]
		}

		res := envResult{Keys: []string{}}

		f, err := os.Open(file)
		if err != nil {
			// If default .env is not found, just return empty
			if len(args) == 0 && os.IsNotExist(err) {
				out, _ := json.Marshal(res)
				fmt.Println(string(out))
				return
			}
			fmt.Fprintf(os.Stderr, "error reading env file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) > 0 {
				key := strings.TrimSpace(parts[0])
				key = strings.TrimPrefix(key, "export ")
				key = strings.TrimSpace(key)
				if key != "" {
					res.Keys = append(res.Keys, key)
				}
			}
		}

		out, _ := json.Marshal(res)
		fmt.Println(string(out))
	},
}
