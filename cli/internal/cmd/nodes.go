package cmd

import (
	"context"
	"fmt"
	"time"

	grpcclient "github.com/ipedrazas/pulse/cli/internal/grpc"
	pulsev1 "github.com/ipedrazas/pulse/proto/gen/pulse/v1"
	"github.com/spf13/cobra"
)

func newNodesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "nodes",
		Short: "List all agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, conn, err := grpcclient.NewCLIClient(apiAddr)
			if err != nil {
				return err
			}
			defer conn.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			resp, err := client.ListNodes(ctx, &pulsev1.ListNodesRequest{})
			if err != nil {
				return fmt.Errorf("list nodes: %w", err)
			}

			if output == "json" {
				return printJSON(resp.Nodes)
			}

			headers := []string{"NAME", "STATUS", "VERSION", "CONTAINERS", "LAST SEEN"}
			var rows [][]string
			for _, n := range resp.Nodes {
				lastSeen := ""
				if n.LastSeen != nil {
					lastSeen = n.LastSeen.AsTime().Format(time.RFC3339)
				}
				rows = append(rows, []string{
					n.Name,
					n.Status,
					n.AgentVersion,
					fmt.Sprint(n.ContainerCount),
					lastSeen,
				})
			}
			printTable(headers, rows)
			return nil
		},
	}
}
