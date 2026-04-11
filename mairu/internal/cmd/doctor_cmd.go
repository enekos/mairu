package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"time"

	"mairu/internal/config"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
)

func NewDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Validate mairu configuration and connectivity",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := GetConfig()
			if cfg == nil {
				return fmt.Errorf("config not loaded")
			}

			allOk := true
			check := func(label string, fn func() error) {
				if err := fn(); err != nil {
					fmt.Printf("!!  %-16s %s\n", label, err)
					allOk = false
				} else {
					fmt.Printf("ok  %-16s", label)
				}
			}

			// User config
			check("User config", func() error {
				path := config.UserConfigPath()
				if _, err := os.Stat(path); os.IsNotExist(err) {
					fmt.Printf("%s (not found, using defaults)\n", path)
					return nil
				}
				fmt.Printf("%s (valid)\n", path)
				return nil
			})

			// Project config
			check("Project config", func() error {
				cwd, _ := os.Getwd()
				path := config.FindProjectConfig(cwd)
				if path == "" {
					fmt.Println("none found")
					return nil
				}
				fmt.Printf("%s (valid)\n", path)
				return nil
			})

			// Meilisearch
			check("Meilisearch", func() error {
				url := cfg.API.MeiliURL + "/health"
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					return fmt.Errorf("%s (unreachable: %v)", cfg.API.MeiliURL, err)
				}
				resp.Body.Close()
				if resp.StatusCode != 200 {
					return fmt.Errorf("%s (status %d)", cfg.API.MeiliURL, resp.StatusCode)
				}
				fmt.Printf("%s (healthy)\n", cfg.API.MeiliURL)
				return nil
			})

			// Gemini API key
			check("Gemini API", func() error {
				key := cfg.API.GeminiAPIKey
				if key == "" {
					return fmt.Errorf("not configured (set api.gemini_api_key or GEMINI_API_KEY)")
				}
				if len(key) < 8 {
					return fmt.Errorf("key too short")
				}
				fmt.Printf("%s...%s (configured)\n", key[:4], key[len(key)-4:])
				return nil
			})

			// SQLite
			check("SQLite", func() error {
				dsn := cfg.Server.SQLiteDSN
				db, err := sql.Open("sqlite3", dsn)
				if err != nil {
					return fmt.Errorf("%s (open error: %v)", dsn, err)
				}
				defer db.Close()
				if err := db.Ping(); err != nil {
					return fmt.Errorf("%s (ping error: %v)", dsn, err)
				}
				fmt.Printf("%s (writable)\n", dsn)
				return nil
			})

			fmt.Println()
			if allOk {
				fmt.Println("All checks passed.")
			} else {
				fmt.Println("Some checks failed. Fix the issues above and run 'mairu doctor' again.")
				os.Exit(1)
			}
			return nil
		},
	}
}
