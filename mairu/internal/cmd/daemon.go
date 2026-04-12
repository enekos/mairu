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

// llmMarkdownSummarizer wraps a Provider to implement daemon.MarkdownSummarizer.
type llmMarkdownSummarizer struct {
	provider llm.Provider
	model    string
}

func (s *llmMarkdownSummarizer) SummarizeMarkdown(ctx context.Context, filename, content string) (string, string, error) {
	model := s.model
	if model == "" {
		model = "gemini-2.5-flash"
	}
	return llm.SummarizeMarkdownDoc(ctx, s.provider, model, filename, content)
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
			providerCfg := GetLLMProviderConfig()
			if providerCfg.APIKey != "" {
				if p, err := llm.NewProvider(providerCfg); err == nil {
					opts.MarkdownSummarizer = &llmMarkdownSummarizer{
						provider: p,
						model:    providerCfg.Model,
					}
				} else {
					slog.Warn("failed to init LLM for markdown summarization", "err", err)
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
