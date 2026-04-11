package cmd

import (
	"context"
	"log/slog"
	"os"

	"mairu/internal/daemon"
	"mairu/internal/enricher"
	"mairu/internal/llm"

	"github.com/spf13/cobra"
)

// geminiMarkdownSummarizer wraps GeminiProvider to implement daemon.MarkdownSummarizer.
type geminiMarkdownSummarizer struct {
	provider *llm.GeminiProvider
}

func (s *geminiMarkdownSummarizer) SummarizeMarkdown(ctx context.Context, filename, content string) (string, string, error) {
	return llm.SummarizeMarkdownDoc(ctx, s.provider, "gemini-2.5-flash", filename, content)
}

type remoteManager struct{}

func (remoteManager) UpsertFileContextNode(ctx context.Context, uri, name, abstractText, overviewText, content, parentURI, project string, metadata map[string]any) error {
	payload := map[string]any{
		"uri":      uri,
		"project":  project,
		"name":     name,
		"abstract": abstractText,
		"overview": overviewText,
		"content":  content,
		"metadata": metadata,
	}
	if parentURI != "" {
		payload["parent_uri"] = parentURI
	}
	_, err := ContextPost("/api/context", payload)
	return err
}

func (remoteManager) DeleteContextNode(ctx context.Context, uri string) error {
	_, err := ContextDelete("/api/context", map[string]string{"uri": uri})
	return err
}

func NewDaemonCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "daemon [dir]",
		Short: "Run local codebase daemon scan",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}
			if _, err := os.Stat(dir); err != nil {
				return err
			}
			opts := daemon.Options{}
			if apiKey := os.Getenv("GEMINI_API_KEY"); apiKey != "" {
				if p, err := llm.NewGeminiProvider(cmd.Context(), apiKey); err == nil {
					opts.MarkdownSummarizer = &geminiMarkdownSummarizer{provider: p}
				} else {
					slog.Warn("failed to init Gemini for markdown summarization", "err", err)
				}
			}

			appCfg := GetConfig()
			var enrichers []enricher.Enricher
			if appCfg.Enricher.GitIntent.Enabled {
				enrichers = append(enrichers, &enricher.GitIntentEnricher{
					MaxCommits: appCfg.Enricher.GitIntent.MaxCommits,
				})
			}
			if appCfg.Enricher.ChangeVelocity.Enabled {
				enrichers = append(enrichers, &enricher.ChangeVelocityEnricher{
					LookbackDays: appCfg.Enricher.ChangeVelocity.LookbackDays,
				})
			}
			if len(enrichers) > 0 {
				opts.EnricherPipeline = enricher.NewPipeline(enrichers)
			}

			d := daemon.New(remoteManager{}, project, dir, opts)
			d.LoadCache()
			if err := d.ProcessAllFiles(context.Background()); err != nil {
				return err
			}
			slog.Info("Daemon scan complete", "dir", dir)
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "P", "default", "Project name")
	return cmd
}
