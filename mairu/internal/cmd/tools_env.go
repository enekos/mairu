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
var envDiff string
var envRequired string

type envKey struct {
	Key      string `json:"key"`
	Value    string `json:"val,omitempty"`
	IsSecret bool   `json:"is_secret,omitempty"`
}

type envResult struct {
	Vars    []envKey `json:"vars"`
	Missing []string `json:"missing,omitempty"`
	OK      *bool    `json:"ok,omitempty"`
}

type envDiffChanged struct {
	Key  string `json:"key"`
	From string `json:"from"`
	To   string `json:"to"`
}

type envDiffResult struct {
	Added     []string         `json:"added"`
	Removed   []string         `json:"removed"`
	Changed   []envDiffChanged `json:"changed"`
	Unchanged int              `json:"unchanged"`
}

var secretIndicators = []string{"key", "secret", "token", "pass", "pwd", "auth", "cert", "hash"}

func isSecretKey(key string) bool {
	keyLower := strings.ToLower(key)
	for _, indicator := range secretIndicators {
		if strings.Contains(keyLower, indicator) {
			return true
		}
	}
	return false
}

func isSafeValue(val string) bool {
	val = strings.ToLower(strings.TrimSpace(val))
	if val == "true" || val == "false" || val == "1" || val == "0" {
		return true
	}
	if strings.HasPrefix(val, "http://localhost") || strings.HasPrefix(val, "http://127.0.0.1") {
		return true
	}
	return false
}

func parseEnvFile(path string) (map[string]string, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	vars := make(map[string]string)
	var order []string

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
			val := ""
			if len(parts) > 1 {
				val = strings.TrimSpace(parts[1])
			}
			val = strings.Trim(val, `"'`)
			vars[key] = val
			order = append(order, key)
		}
	}
	return vars, order, nil
}

func NewEnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env [file...]",
		Short: "AI-optimized safe environment reader (JSON)",
		Args:  cobra.ArbitraryArgs,
		Run: func(cmd *cobra.Command, args []string) {
			files := args
			if len(files) == 0 {
				files = []string{".env"}
			}

			// Diff mode
			if envDiff != "" {
				vars1, _, err1 := parseEnvFile(files[0])
				vars2, _, err2 := parseEnvFile(envDiff)
				if err1 != nil {
					fmt.Fprintf(os.Stderr, "error reading %s: %v\n", files[0], err1)
					os.Exit(1)
				}
				if err2 != nil {
					fmt.Fprintf(os.Stderr, "error reading %s: %v\n", envDiff, err2)
					os.Exit(1)
				}

				diff := envDiffResult{
					Added:   []string{},
					Removed: []string{},
					Changed: []envDiffChanged{},
				}

				for k := range vars2 {
					if _, ok := vars1[k]; !ok {
						diff.Added = append(diff.Added, k)
					}
				}
				for k := range vars1 {
					if _, ok := vars2[k]; !ok {
						diff.Removed = append(diff.Removed, k)
					}
				}
				for k, v1 := range vars1 {
					if v2, ok := vars2[k]; ok {
						if v1 != v2 {
							from, to := v1, v2
							if isSecretKey(k) {
								from = "***"
								to = "***"
							}
							diff.Changed = append(diff.Changed, envDiffChanged{Key: k, From: from, To: to})
						} else {
							diff.Unchanged++
						}
					}
				}

				out, _ := json.Marshal(diff)
				fmt.Println(string(out))
				return
			}

			// Normal mode: parse and merge all files
			merged := make(map[string]string)
			var order []string
			for _, file := range files {
				vars, fileOrder, err := parseEnvFile(file)
				if err != nil {
					if len(files) == 1 && files[0] == ".env" && os.IsNotExist(err) {
						res := envResult{Vars: []envKey{}}
						out, _ := json.Marshal(res)
						fmt.Println(string(out))
						return
					}
					fmt.Fprintf(os.Stderr, "error reading env file %s: %v\n", file, err)
					os.Exit(1)
				}
				for _, k := range fileOrder {
					if _, exists := merged[k]; !exists {
						order = append(order, k)
					}
					merged[k] = vars[k]
				}
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

			for _, key := range order {
				if re != nil && !re.MatchString(key) {
					continue
				}

				val := merged[key]
				isSecret := isSecretKey(key)
				entry := envKey{Key: key}
				if isSecret {
					entry.IsSecret = true
				}

				if envReveal {
					if !isSecret && (isSafeValue(val) || len(val) < 8) {
						entry.Value = val
					} else {
						entry.Value = "***HIDDEN***"
					}
				}

				res.Vars = append(res.Vars, entry)
			}

			// Required keys check
			if envRequired != "" {
				var missing []string
				for _, k := range strings.Split(envRequired, ",") {
					k = strings.TrimSpace(k)
					if _, ok := merged[k]; !ok {
						missing = append(missing, k)
					}
				}
				ok := len(missing) == 0
				res.OK = &ok
				if len(missing) > 0 {
					res.Missing = missing
				}

				out, _ := json.Marshal(res)
				fmt.Println(string(out))
				if !ok {
					os.Exit(1)
				}
				return
			}

			out, _ := json.Marshal(res)
			fmt.Println(string(out))
		},
	}
	cmd.Flags().BoolVarP(&envReveal, "reveal", "r", false, "Reveal non-sensitive values (like booleans, numbers, or non-credential strings)")
	cmd.Flags().StringVarP(&envPattern, "match", "m", "", "Only return keys matching regex pattern")
	cmd.Flags().StringVar(&envDiff, "diff", "", "Compare with another env file, showing added/removed/changed keys")
	cmd.Flags().StringVar(&envRequired, "required", "", "Comma-separated keys that must exist (exit 1 if missing)")
	return cmd
}
