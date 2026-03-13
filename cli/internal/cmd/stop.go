package cmd

import (
	"context"
	"fmt"
	"time"

	grpcclient "github.com/ipedrazas/pulse/cli/internal/grpc"
	pulsev1 "github.com/ipedrazas/pulse/proto/gen/pulse/v1"
	"github.com/spf13/cobra"
)

func newStopCmd() *cobra.Command {
	var (
		node string
		wait bool
	)

	cmd := &cobra.Command{
		Use:   "stop [container_id]",
		Short: "Stop a container on a node",
		Example: `  pulse stop abc123def456 --node worker-1
  pulse stop abc123def456 --node worker-1 --wait`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if node == "" {
				return fmt.Errorf("--node is required")
			}

			client, conn, err := grpcclient.NewCLIClient(apiAddr)
			if err != nil {
				return err
			}
			defer conn.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			resp, err := client.SendCommand(ctx, &pulsev1.SendCommandRequest{
				NodeName: node,
				Command: &pulsev1.SendCommandRequest_StopContainer{
					StopContainer: &pulsev1.StopContainer{
						ContainerId: args[0],
					},
				},
			})
			if err != nil {
				return fmt.Errorf("send command: %w", err)
			}

			if !wait {
				fmt.Printf("Command queued: %s\n", resp.CommandId)
				return nil
			}

			result, err := waitForCommand(ctx, client, resp.CommandId)
			if err != nil {
				return err
			}
			fmt.Printf("Container stopped (command %s)\n", result.CommandId)
			return nil
		},
	}

	cmd.Flags().StringVar(&node, "node", "", "Target node (required)")
	cmd.Flags().BoolVar(&wait, "wait", false, "Wait for the command to complete")
	return cmd
}
