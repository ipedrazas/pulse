package cmd

import (
	"context"
	"fmt"
	"time"

	pulsev1 "github.com/ipedrazas/pulse/proto/gen/pulse/v1"
	grpcclient "github.com/ipedrazas/pulse/cli/internal/grpc"
	"github.com/spf13/cobra"
)

func newPullCmd() *cobra.Command {
	var node string

	cmd := &cobra.Command{
		Use:   "pull [image]",
		Short: "Pull an image on a node",
		Args:  cobra.MaximumNArgs(1),
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

			image := ""
			if len(args) > 0 {
				image = args[0]
			}

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

			fmt.Printf("Command queued: %s\n", resp.CommandId)
			return nil
		},
	}

	cmd.Flags().StringVar(&node, "node", "", "Target node (required)")
	return cmd
}
