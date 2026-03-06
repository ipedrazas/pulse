package cmd

import (
	"context"
	"fmt"
	"time"

	grpcclient "github.com/ipedrazas/pulse/cli/internal/grpc"
	pulsev1 "github.com/ipedrazas/pulse/proto/gen/pulse/v1"
	"github.com/spf13/cobra"
)

func newLogsCmd() *cobra.Command {
	var (
		node   string
		follow bool
		tail   int32
	)

	cmd := &cobra.Command{
		Use:   "logs [container_id]",
		Short: "Fetch container logs from a node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if node == "" {
				return fmt.Errorf("--node is required")
			}

			client, conn, err := grpcclient.NewCLIClient(apiAddr)
			if err != nil {
				return err
			}
			defer conn.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			resp, err := client.SendCommand(ctx, &pulsev1.SendCommandRequest{
				NodeName: node,
				Command: &pulsev1.SendCommandRequest_RequestLogs{
					RequestLogs: &pulsev1.RequestLogs{
						ContainerId: args[0],
						Follow:      follow,
						Tail:        tail,
					},
				},
			})
			if err != nil {
				return fmt.Errorf("send command: %w", err)
			}

			// Poll for the result
			ticker := time.NewTicker(500 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return fmt.Errorf("timed out waiting for logs (command %s)", resp.CommandId)
				case <-ticker.C:
					result, err := client.GetCommandResult(ctx, &pulsev1.GetCommandResultRequest{
						CommandId: resp.CommandId,
					})
					if err != nil {
						return fmt.Errorf("get result: %w", err)
					}
					switch result.Status {
					case "completed":
						fmt.Print(result.Result)
						return nil
					case "failed":
						return fmt.Errorf("command failed: %s", result.Result)
					}
					// still pending, keep polling
				}
			}
		},
	}

	cmd.Flags().StringVar(&node, "node", "", "Target node (required)")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	cmd.Flags().Int32Var(&tail, "tail", 100, "Number of lines from the end")

	return cmd
}
