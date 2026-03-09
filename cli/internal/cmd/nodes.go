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
	cmd := &cobra.Command{
		Use:   "nodes",
		Short: "Manage nodes",
		RunE:  listNodesRun,
	}

	cmd.AddCommand(newNodesLsCmd())
	cmd.AddCommand(newNodesRmCmd())

	return cmd
}

func newNodesLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List all nodes",
		RunE:  listNodesRun,
	}
}

func listNodesRun(_ *cobra.Command, _ []string) error {
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
}

func newNodesRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a node",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			client, conn, err := grpcclient.NewCLIClient(apiAddr)
			if err != nil {
				return err
			}
			defer conn.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			_, err = client.DeleteNode(ctx, &pulsev1.DeleteNodeRequest{Name: args[0]})
			if err != nil {
				return fmt.Errorf("delete node: %w", err)
			}

			fmt.Printf("Node %q removed\n", args[0])
			return nil
		},
	}
}
