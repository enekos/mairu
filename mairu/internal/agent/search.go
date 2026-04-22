package agent

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/charlievieth/fastwalk"
	ignore "github.com/sabhiram/go-gitignore"

	"mairu/internal/llm"
)

// fallbackSearch implements a concurrent, gitignore-aware search in Go.
func (a *Agent) fallbackSearch(query string) (string, error) {
	root := a.root

	var ignorer *ignore.GitIgnore
	if gi, err := ignore.CompileIgnoreFile(filepath.Join(root, ".gitignore")); err == nil {
		ignorer = gi
	}

	defaultIgnores := map[string]bool{
		".git":         true,
		"node_modules": true,
		"vendor":       true,
		"dist":         true,
		"build":        true,
		"out":          true,
		"bin":          true,
		".idea":        true,
		".vscode":      true,
		"coverage":     true,
	}

	type match struct {
		file string
		line int
		text string
	}

	var (
		matches []match
		mu      sync.Mutex
		wg      sync.WaitGroup
	)

	// Try compiling as regex first, if it fails, treat as literal
	isRegex := false
	var re *regexp.Regexp
	if compiled, err := regexp.Compile(query); err == nil {
		// Only use regex mode if it contains regex characters
		if strings.ContainsAny(query, "^$*+?()[]{}|\\") {
			isRegex = true
			re = compiled
		}
	}
	// Fallback to literal case-insensitive if regex fails or is just text
	lowerQuery := strings.ToLower(query)

	filesCh := make(chan string, 1000)
	numWorkers := runtime.NumCPU()
	if numWorkers > 16 {
		numWorkers = 16 // Cap to avoid too many open files
	}

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range filesCh {
				f, err := os.Open(path)
				if err != nil {
					continue
				}

				// Check for null bytes in the first 512 bytes to skip binary files.
				header := make([]byte, 512)
				n, err := f.Read(header)
				if err != nil || n == 0 || bytes.IndexByte(header[:n], 0) != -1 {
					f.Close()
					continue
				}
				f.Seek(0, 0)

				scanner := bufio.NewScanner(f)
				// Increase buffer size for long lines (e.g. minified JS)
				buf := make([]byte, 0, 64*1024)
				scanner.Buffer(buf, 1024*1024)

				lineNum := 1
				for scanner.Scan() {
					line := scanner.Text()

					var matched bool
					if isRegex {
						matched = re.MatchString(line)
					} else {
						// Case-insensitive literal match
						matched = strings.Contains(strings.ToLower(line), lowerQuery)
					}

					if matched {
						rel, _ := filepath.Rel(root, path)
						mu.Lock()
						matches = append(matches, match{file: rel, line: lineNum, text: line})
						mu.Unlock()
					}
					lineNum++
				}
				f.Close()
			}
		}()
	}

	err := fastwalk.Walk(nil, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		if rel == "." {
			return nil
		}

		if d.IsDir() {
			if defaultIgnores[d.Name()] {
				return filepath.SkipDir
			}
			if ignorer != nil && ignorer.MatchesPath(rel+"/") {
				return filepath.SkipDir
			}
			return nil
		}

		if ignorer != nil && ignorer.MatchesPath(rel) {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".png", ".jpg", ".jpeg", ".gif", ".ico", ".pdf", ".zip", ".tar", ".gz", ".mp4", ".mp3", ".wav", ".exe", ".dll", ".so", ".dylib", ".class", ".jar", ".woff", ".woff2", ".ttf", ".eot", ".bin", ".db", ".sqlite", ".pyc":
			return nil
		}

		if info, err := d.Info(); err == nil && info.Size() > 1024*1024 { // skip files > 1MB
			return nil
		}

		filesCh <- path
		return nil
	})

	close(filesCh)
	wg.Wait()

	if err != nil {
		return "", err
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("search failed or no results found")
	}

	// Sort matches by file name, then line number to ensure deterministic output
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].file == matches[j].file {
			return matches[i].line < matches[j].line
		}
		return matches[i].file < matches[j].file
	})

	var sb strings.Builder
	for _, m := range matches {
		sb.WriteString(fmt.Sprintf("%s:%d:%s\n", m.file, m.line, strings.TrimSpace(m.text)))
		if sb.Len() > 10000 {
			break
		}
	}

	res := sb.String()
	if len(res) > 10000 {
		res = res[:10000] + "\n...[Output truncated, too many matches]"
	}

	return res, nil
}

type findFilesTool struct{}

func (t *findFilesTool) Definition() llm.Tool {
	return llm.Tool{
		Name:        "find_files",
		Description: "Find files by glob pattern.",
		Parameters: &llm.JSONSchema{
			Type: llm.TypeObject,
			Properties: map[string]*llm.JSONSchema{
				"pattern": {Type: llm.TypeString, Description: "The glob pattern (e.g., src/**/*.ts)."},
			},
			Required: []string{"pattern"},
		},
	}
}

func (t *findFilesTool) Execute(ctx context.Context, args map[string]any, a *Agent, outChan chan<- AgentEvent) (map[string]any, error) {
	pattern, _ := args["pattern"].(string)
	outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🔍 Finding files: %s", pattern)}
	files, err := a.FindFiles(pattern)
	if err != nil {
		return map[string]any{"error": err.Error()}, nil
	}
	return map[string]any{"files": files}, nil
}

type searchCodebaseTool struct{}

func (t *searchCodebaseTool) Definition() llm.Tool {
	return llm.Tool{
		Name:        "search_codebase",
		Description: "Search the codebase by text/regex query or by symbol name (surgical read).",
		Parameters: &llm.JSONSchema{
			Type: llm.TypeObject,
			Properties: map[string]*llm.JSONSchema{
				"query":       {Type: llm.TypeString, Description: "Text or regex to search in files."},
				"symbol_name": {Type: llm.TypeString, Description: "Exact symbol name to look up (function, method, class)."},
			},
		},
	}
}

func (t *searchCodebaseTool) Execute(ctx context.Context, args map[string]any, a *Agent, outChan chan<- AgentEvent) (map[string]any, error) {
	query, _ := args["query"].(string)
	symName, _ := args["symbol_name"].(string)

	if strings.TrimSpace(symName) != "" {
		outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🔎 Searching symbol: %s", symName)}
		return a.searchBySymbolName(symName, outChan), nil
	}

	if strings.TrimSpace(query) == "" {
		return map[string]any{"error": "missing query: provide either query or symbol_name"}, nil
	}

	outChan <- AgentEvent{Type: "status", Content: fmt.Sprintf("🔎 Searching for: %s", query)}
	matches, err := a.SearchCodebase(query)
	if err != nil {
		return map[string]any{"error": err.Error()}, nil
	}
	return map[string]any{"matches": matches}, nil
}
