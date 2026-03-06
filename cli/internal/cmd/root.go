package cmd

import (
	"github.com/spf13/cobra"
)

var (
	apiAddr string
	output  string
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "pulse",
		Short: "Pulse — distributed container management CLI",
	}

	root.PersistentFlags().StringVar(&apiAddr, "api-addr", "localhost:9090", "API server gRPC address")
	root.PersistentFlags().StringVarP(&output, "output", "o", "table", "Output format: table, json")

	root.AddCommand(
		newNodesCmd(),
		newPsCmd(),
		newRunCmd(),
		newStopCmd(),
		newPullCmd(),
		newUpCmd(),
		newLogsCmd(),
		newSendCmd(),
		newVersionCmd(),
	)

	return root
}
