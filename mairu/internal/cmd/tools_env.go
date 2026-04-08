package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var envReveal bool
var envPattern string

func init() {
	envCmd.Flags().BoolVarP(&envReveal, "reveal", "r", false, "Reveal non-sensitive values (like booleans, numbers, or non-credential strings)")
	envCmd.Flags().StringVarP(&envPattern, "match", "m", "", "Only return keys matching regex pattern")
}

type envKey struct {
	Key      string `json:"key"`
	Value    string `json:"val,omitempty"` // Only populated if reveal=true AND value is safe
	IsSecret bool   `json:"is_secret,omitempty"`
}

type envResult struct {
	Vars []envKey `json:"vars"`
}

var envCmd = &cobra.Command{
	Use:   "env [file]",
	Short: "AI-optimized safe environment reader (JSON)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		file := ".env"
		if len(args) > 0 {
			file = args[0]
		}

		var re *regexp.Regexp
		if envPattern != "" {
			var err error
			re, err = regexp.Compile("(?i)" + envPattern)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error compiling regex: %v\n", err)
				os.Exit(1)
			}
		}

		res := envResult{Vars: []envKey{}}

		f, err := os.Open(file)
		if err != nil {
			if len(args) == 0 && os.IsNotExist(err) {
				out, _ := json.Marshal(res)
				fmt.Println(string(out))
				return
			}
			fmt.Fprintf(os.Stderr, "error reading env file: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()

		secretIndicators := []string{"key", "secret", "token", "pass", "pwd", "auth", "cert", "hash"}
		isSafeValue := func(val string) bool {
			val = strings.ToLower(strings.TrimSpace(val))
			if val == "true" || val == "false" || val == "1" || val == "0" {
				return true
			}
			// Safe primitives like simple ports or typical local hosts
			if strings.HasPrefix(val, "http://localhost") || strings.HasPrefix(val, "http://127.0.0.1") {
				return true
			}
			return false
		}

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

				if key == "" {
					continue
				}

				if re != nil && !re.MatchString(key) {
					continue
				}

				val := ""
				if len(parts) > 1 {
					val = strings.TrimSpace(parts[1])
				}

				// strip quotes from value for analysis
				val = strings.Trim(val, `"'`)

				// Detect if key sounds like a secret
				isSecret := false
				keyLower := strings.ToLower(key)
				for _, indicator := range secretIndicators {
					if strings.Contains(keyLower, indicator) {
						isSecret = true
						break
					}
				}

				entry := envKey{Key: key}
				if isSecret {
					entry.IsSecret = true
				}

				// Only reveal value if requested AND it's demonstrably safe
				if envReveal {
					if !isSecret && (isSafeValue(val) || len(val) < 8) {
						// Small strings or obvious booleans/numbers are usually configuration flags, not secrets
						entry.Value = val
					} else {
						entry.Value = "***HIDDEN***"
					}
				}

				res.Vars = append(res.Vars, entry)
			}
		}

		out, _ := json.Marshal(res)
		fmt.Println(string(out))
	},
}
