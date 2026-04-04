package cmd

import (
	"context"
	"fmt"
	"os"

	"mairu/internal/daemon"

	"github.com/spf13/cobra"
)

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
	_, err := contextPost("/nodes", payload)
	return err
}

func (remoteManager) DeleteContextNode(ctx context.Context, uri string) error {
	_, err := contextDelete("/nodes/"+uri, nil)
	return err
}

func init() {
	var project string
	daemonCmd := &cobra.Command{
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
			d := daemon.New(remoteManager{}, project, dir, daemon.Options{})
			if err := d.ProcessAllFiles(context.Background()); err != nil {
				return err
			}
			fmt.Printf("Daemon scan complete for %s\n", dir)
			return nil
		},
	}
	daemonCmd.Flags().StringVarP(&project, "project", "P", "default", "Project name")
	rootCmd.AddCommand(daemonCmd)
}
