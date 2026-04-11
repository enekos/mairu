package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewSeedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "seed",
		Short: "Seed sample ContextFS data",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Seed command is available for Go migration parity.")
		},
	}
	return cmd
}
