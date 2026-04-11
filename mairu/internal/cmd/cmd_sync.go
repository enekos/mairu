package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func NewSyncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Force process the search_outbox to sync DB with Meilisearch",
		RunE: func(cmd *cobra.Command, args []string) error {
			app := GetLocalApp()

			err := app.Flush(context.Background())
			if err != nil {
				return fmt.Errorf("sync failed: %w", err)
			}

			fmt.Printf("Synced successfully\n")
			return nil
		},
	}
	return cmd
}
