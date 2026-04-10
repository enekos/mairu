package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func newSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Force process the search_outbox to sync DB with Meilisearch",
		RunE: func(cmd *cobra.Command, args []string) error {
			app := GetLocalApp()

			if err := app.Flush(context.Background()); err != nil {
				return fmt.Errorf("sync failed: %w", err)
			}

			fmt.Printf("Synced successfully\n")
			return nil
		},
	}
	return cmd
}
