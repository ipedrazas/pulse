package cmd

import (
	"fmt"

	"github.com/ipedrazas/pulse/cli/internal/version"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   "Print the CLI version",
		Example: `  pulse version`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("pulse %s (commit: %s, built: %s)\n",
				version.Version, version.Commit, version.BuildDate)
		},
	}
}
