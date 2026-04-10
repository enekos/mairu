package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"mairu/internal/contextsrv"
	"mairu/internal/crawler"
	"mairu/internal/llm"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newScrapeWebCmd() *cobra.Command {
	var project string
	var maxPages int
	var maxDepth int
	var concurrency int

	cmd := &cobra.Command{
		Use:   "web <url>",
		Short: "Fetch a URL, extract content, summarize, and store as a context node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			urlStr := args[0]
			fmt.Printf("Crawling %s...\n", urlStr)

			opts := crawler.ScrapeOptions{
				Project: project,
				CrawlOptions: crawler.CrawlOptions{
					SeedURL:     urlStr,
					MaxPages:    maxPages,
					MaxDepth:    maxDepth,
					Concurrency: concurrency,
				},
			}

			storeFn := func(ctx context.Context, input contextsrv.ContextCreateInput) error {
				fmt.Printf("Storing node %s...\n", input.URI)
				var parent string
				if input.ParentURI != nil {
					parent = *input.ParentURI
				}
				return RunNodeStore(input.Project, input.URI, input.Name, input.Abstract, parent, input.Overview, input.Content)
			}

			apiKey := GetAPIKey()
			res, err := crawler.ScrapeAndIngest(cmd.Context(), opts, storeFn, apiKey)
			if err != nil {
				return err
			}

			fmt.Printf("Scraping complete. Total pages: %d, Stored: %d, Skipped: %d\n", res.PagesTotal, res.PagesStored, res.PagesSkipped)
			if len(res.Errors) > 0 {
				fmt.Printf("Errors encountered: %d\n", len(res.Errors))
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "P", "default", "Project namespace")
	cmd.Flags().IntVar(&maxPages, "max-pages", 50, "Max pages to crawl")
	cmd.Flags().IntVar(&maxDepth, "max-depth", 3, "Max depth to crawl")
	cmd.Flags().IntVar(&concurrency, "concurrency", 3, "Concurrent requests")
	return cmd
}
func newSmartScrapeCmd() *cobra.Command {
	var project string
	var prompt string
	var useRAG bool
	var refinePrompt bool

	cmd := &cobra.Command{
		Use:   "smart <url>",
		Short: "Fetch a URL and extract structured data using LLM based on prompt",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetURL := args[0]
			if prompt == "" {
				return fmt.Errorf("prompt is required")
			}

			apiKey := GetAPIKey()
			if apiKey == "" {
				return fmt.Errorf("gemini api key required for smart-scrape")
			}

			provider, err := llm.NewGeminiProvider(cmd.Context(), apiKey)
			if err != nil {
				return fmt.Errorf("failed to init LLM: %w", err)
			}

			fmt.Printf("Running smart scrape on %s...\n", targetURL)

			graph := crawler.NewSmartScraperGraph(provider)
			data, err := graph.Run(cmd.Context(), targetURL, prompt)
			if err != nil {
				return fmt.Errorf("scrape failed: %w", err)
			}

			if data == nil {
				fmt.Println("No data extracted.")
				return nil
			}

			jsonBytes, _ := json.MarshalIndent(data, "", "  ")
			fmt.Printf("\nExtracted Data:\n%s\n\n", string(jsonBytes))

			// Store as context node
			uri := fmt.Sprintf("contextfs://scrape/%s", targetURL)
			// basic clean up of URL for URI
			uri = strings.ReplaceAll(uri, "https://", "")
			uri = strings.ReplaceAll(uri, "http://", "")

			content := string(jsonBytes)
			fmt.Printf("Storing extracted data at %s in project '%s'...\n", uri, project)

			return RunNodeStore(project, uri, "Extracted Data", "Data extracted via smart-scrape: "+prompt, "", "", content)
		},
	}
	cmd.Flags().StringVarP(&project, "project", "P", "default", "Project namespace")
	cmd.Flags().StringVar(&prompt, "prompt", "", "Prompt to instruct LLM what to extract")
	cmd.Flags().BoolVar(&useRAG, "rag", false, "Use vector embeddings to parse massive documents without hitting token limits")
	cmd.Flags().BoolVar(&refinePrompt, "refine", false, "Use LLM to refine the prompt before scraping")
	return cmd
}
func newSearchScrapeCmd() *cobra.Command {
	var project string
	var prompt string
	var maxResults int

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search web for query and extract structured data from top results using LLM",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			if prompt == "" {
				return fmt.Errorf("prompt is required")
			}

			apiKey := GetAPIKey()
			if apiKey == "" {
				return fmt.Errorf("gemini api key required for search-scrape")
			}

			provider, err := llm.NewGeminiProvider(cmd.Context(), apiKey)
			if err != nil {
				return fmt.Errorf("failed to init LLM: %w", err)
			}

			fmt.Printf("Running search scrape for query '%s'...\n", query)

			graph := crawler.NewSearchScraperGraph(provider)
			results, err := graph.Run(cmd.Context(), query, prompt, maxResults)
			if err != nil {
				return fmt.Errorf("search scrape failed: %w", err)
			}

			if len(results) == 0 {
				fmt.Println("No data extracted from any search result.")
				return nil
			}

			jsonBytes, _ := json.MarshalIndent(results, "", "  ")
			fmt.Printf("\nExtracted Data:\n%s\n\n", string(jsonBytes))

			// Store as context node
			uri := fmt.Sprintf("contextfs://search/%s", url.QueryEscape(query))

			content := string(jsonBytes)
			fmt.Printf("Storing extracted data at %s in project '%s'...\n", uri, project)

			return RunNodeStore(project, uri, "Search Data: "+query, "Data extracted via search-scrape for query: "+query, "", "", content)
		},
	}
	cmd.Flags().StringVarP(&project, "project", "P", "default", "Project namespace")
	cmd.Flags().StringVar(&prompt, "prompt", "", "Prompt to instruct LLM what to extract")
	cmd.Flags().IntVar(&maxResults, "max-results", 3, "Maximum number of search results to process")
	return cmd
}
func newMultiScrapeCmd() *cobra.Command {
	var project string
	var prompt string
	var concurrency int

	cmd := &cobra.Command{
		Use:   "multi <url1> [url2...]",
		Short: "Fetch multiple URLs concurrently and extract structured data using LLM",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if prompt == "" {
				return fmt.Errorf("prompt is required")
			}

			apiKey := GetAPIKey()
			if apiKey == "" {
				return fmt.Errorf("gemini api key required for multi-scrape")
			}

			provider, err := llm.NewGeminiProvider(cmd.Context(), apiKey)
			if err != nil {
				return fmt.Errorf("failed to init LLM: %w", err)
			}

			fmt.Printf("Running multi-scrape on %d URLs...\n", len(args))

			graph := crawler.NewSmartScraperMultiGraph(provider, concurrency)
			data, err := graph.Run(cmd.Context(), args, prompt)
			if err != nil {
				return fmt.Errorf("multi-scrape failed: %w", err)
			}

			if len(data) == 0 {
				fmt.Println("No data extracted.")
				return nil
			}

			jsonBytes, _ := json.MarshalIndent(data, "", "  ")
			fmt.Printf("\nExtracted Data:\n%s\n\n", string(jsonBytes))

			// Optional: iterate and save each as context node
			for urlStr, result := range data {
				uri := fmt.Sprintf("contextfs://scrape/%s", strings.ReplaceAll(strings.ReplaceAll(urlStr, "https://", ""), "http://", ""))
				resBytes, _ := json.Marshal(result)
				RunNodeStore(project, uri, "Extracted Data", "Data extracted via multi-scrape: "+prompt, "", "", string(resBytes))
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "P", "default", "Project namespace")
	cmd.Flags().StringVar(&prompt, "prompt", "", "Prompt to instruct LLM what to extract")
	cmd.Flags().IntVar(&concurrency, "concurrency", 3, "Concurrent scraping requests")
	return cmd
}
func newScriptScrapeCmd() *cobra.Command {
	var prompt string
	var output string

	cmd := &cobra.Command{
		Use:   "script <url>",
		Short: "Generate a Go scraping script using goquery tailored for a specific URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetURL := args[0]
			if prompt == "" {
				return fmt.Errorf("prompt is required")
			}

			apiKey := GetAPIKey()
			if apiKey == "" {
				return fmt.Errorf("gemini api key required for script-scrape")
			}

			provider, err := llm.NewGeminiProvider(cmd.Context(), apiKey)
			if err != nil {
				return fmt.Errorf("failed to init LLM: %w", err)
			}

			fmt.Printf("Generating scraper script for %s...\n", targetURL)

			graph := crawler.NewScriptCreatorGraph(provider)
			scriptContent, err := graph.Run(cmd.Context(), targetURL, prompt)
			if err != nil {
				return fmt.Errorf("script generation failed: %w", err)
			}

			if scriptContent == "" {
				fmt.Println("No script generated.")
				return nil
			}

			if output != "" {
				err := os.WriteFile(output, []byte(scriptContent), 0644)
				if err != nil {
					return fmt.Errorf("failed to write script: %w", err)
				}
				fmt.Printf("Script saved to %s\n", output)
			} else {
				fmt.Printf("\n--- Generated Go Script ---\n\n%s\n\n", scriptContent)
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&prompt, "prompt", "", "Prompt to instruct what the script should scrape")
	cmd.Flags().StringVarP(&output, "output", "", "scraper.go", "Output file for the script (default: scraper.go)")
	return cmd
}
func newDepthScrapeCmd() *cobra.Command {
	var project string
	var prompt string
	var maxDepth int
	var concurrency int

	cmd := &cobra.Command{
		Use:   "depth <url>",
		Short: "Fetch a URL, discover relevant links up to depth K, and extract data concurrently",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			seedURL := args[0]
			if prompt == "" {
				return fmt.Errorf("prompt is required")
			}

			apiKey := GetAPIKey()
			if apiKey == "" {
				return fmt.Errorf("gemini api key required for depth-scrape")
			}

			provider, err := llm.NewGeminiProvider(cmd.Context(), apiKey)
			if err != nil {
				return fmt.Errorf("failed to init LLM: %w", err)
			}

			fmt.Printf("Running depth-scrape (depth: %d) on %s...\n", maxDepth, seedURL)

			graph := crawler.NewDepthSearchScraperGraph(provider, maxDepth, concurrency)
			data, err := graph.Run(cmd.Context(), seedURL, prompt)
			if err != nil {
				return fmt.Errorf("depth-scrape failed: %w", err)
			}

			if len(data) == 0 {
				fmt.Println("No data extracted.")
				return nil
			}

			jsonBytes, _ := json.MarshalIndent(data, "", "  ")
			fmt.Printf("\nExtracted Data:\n%s\n\n", string(jsonBytes))

			// Store each as context node
			for urlStr, result := range data {
				uri := fmt.Sprintf("contextfs://scrape/%s", strings.ReplaceAll(strings.ReplaceAll(urlStr, "https://", ""), "http://", ""))
				resBytes, _ := json.Marshal(result)
				RunNodeStore(project, uri, "Extracted Data", "Data extracted via depth-scrape: "+prompt, "", "", string(resBytes))
			}

			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "P", "default", "Project namespace")
	cmd.Flags().StringVar(&prompt, "prompt", "", "Prompt to instruct LLM what to extract and which links to follow")
	cmd.Flags().IntVar(&maxDepth, "max-depth", 1, "Maximum link depth to traverse (0 = only seed URL)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 3, "Concurrent scraping requests")
	return cmd
}
func newOmniScrapeCmd() *cobra.Command {
	var project string
	var prompt string
	var concurrency int

	cmd := &cobra.Command{
		Use:   "omni <url1> [url2...]",
		Short: "Fetch multiple URLs concurrently and merge extracted data into a single summary",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if prompt == "" {
				return fmt.Errorf("prompt is required")
			}

			apiKey := GetAPIKey()
			if apiKey == "" {
				return fmt.Errorf("gemini api key required for omni-scrape")
			}

			provider, err := llm.NewGeminiProvider(cmd.Context(), apiKey)
			if err != nil {
				return fmt.Errorf("failed to init LLM: %w", err)
			}

			fmt.Printf("Running omni-scrape on %d URLs...\n", len(args))

			graph := crawler.NewOmniScraperGraph(provider, concurrency)
			data, err := graph.Run(cmd.Context(), args, prompt)
			if err != nil {
				return fmt.Errorf("omni-scrape failed: %w", err)
			}

			if len(data) == 0 {
				fmt.Println("No data extracted.")
				return nil
			}

			jsonBytes, _ := json.MarshalIndent(data, "", "  ")
			fmt.Printf("\nMerged Extracted Data:\n%s\n\n", string(jsonBytes))

			// Store as context node
			uri := fmt.Sprintf("contextfs://omni-scrape/%s", strings.ReplaceAll(strings.ReplaceAll(args[0], "https://", ""), "http://", ""))
			if len(args) > 1 {
				uri += "-and-others"
			}

			content := string(jsonBytes)
			fmt.Printf("Storing merged data at %s in project '%s'...\n", uri, project)

			return RunNodeStore(project, uri, "Merged Omni Data", "Data merged via omni-scrape: "+prompt, "", "", content)
		},
	}
	cmd.Flags().StringVarP(&project, "project", "P", "default", "Project namespace")
	cmd.Flags().StringVar(&prompt, "prompt", "", "Prompt to instruct LLM what to extract and merge")
	cmd.Flags().IntVar(&concurrency, "concurrency", 3, "Concurrent scraping requests")
	return cmd
}

func newScrapeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scrape",
		Short: "Web scraping and content extraction tools",
	}

	cmd.AddCommand(
		newScrapeWebCmd(),
		newSmartScrapeCmd(),
		newSearchScrapeCmd(),
		newMultiScrapeCmd(),
		newScriptScrapeCmd(),
		newDepthScrapeCmd(),
		newOmniScrapeCmd(),
	)

	return cmd
}
