package cmd

import (
	"context"
	"fmt"
	"time"

	grpcclient "github.com/ipedrazas/pulse/cli/internal/grpc"
	pulsev1 "github.com/ipedrazas/pulse/proto/gen/pulse/v1"
	"github.com/spf13/cobra"
)

func newPullCmd() *cobra.Command {
	var (
		node string
		wait bool
	)

	cmd := &cobra.Command{
		Use:   "pull [image]",
		Short: "Pull an image on a node",
		Long:  "Pull a container image on a remote node. The command is queued and executed asynchronously.",
		Example: `  pulse pull nginx:latest --node worker-1
  pulse pull ghcr.io/org/app:v1.2 --node worker-1
  pulse pull nginx:latest --node worker-1 --wait`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if node == "" {
				return fmt.Errorf("--node is required")
			}

			image := args[0]

			client, conn, err := grpcclient.NewCLIClient(apiAddr)
			if err != nil {
				return err
			}
			defer conn.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()

			resp, err := client.SendCommand(ctx, &pulsev1.SendCommandRequest{
				NodeName: node,
				Command: &pulsev1.SendCommandRequest_PullImage{
					PullImage: &pulsev1.PullImage{
						Image: image,
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
			fmt.Printf("Image pulled successfully (command %s)\n", result.CommandId)
			return nil
		},
	}

	cmd.Flags().StringVar(&node, "node", "", "Target node (required)")
	cmd.Flags().BoolVar(&wait, "wait", false, "Wait for the command to complete")
	return cmd
}
