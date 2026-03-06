package cmd

import (
	"context"
	"fmt"
	"time"

	pulsev1 "github.com/ipedrazas/pulse/proto/gen/pulse/v1"
	grpcclient "github.com/ipedrazas/pulse/cli/internal/grpc"
	"github.com/spf13/cobra"
)

func newUpCmd() *cobra.Command {
	var (
		node       string
		file       string
		projectDir string
		detach     bool
		pull       bool
	)

	cmd := &cobra.Command{
		Use:   "up",
		Short: "Run docker compose up on a node",
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
				Command: &pulsev1.SendCommandRequest_ComposeUp{
					ComposeUp: &pulsev1.ComposeUp{
						ProjectDir: projectDir,
						File:       file,
						Detach:     detach,
						Pull:       pull,
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
	cmd.Flags().StringVarP(&file, "file", "f", "", "Compose file path or oci:// reference")
	cmd.Flags().StringVar(&projectDir, "project-dir", "", "Project directory on the agent")
	cmd.Flags().BoolVarP(&detach, "detach", "d", true, "Run in detached mode")
	cmd.Flags().BoolVar(&pull, "pull", false, "Pull images before starting")

	return cmd
}
