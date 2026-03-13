package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	grpcclient "github.com/ipedrazas/pulse/cli/internal/grpc"
	pulsev1 "github.com/ipedrazas/pulse/proto/gen/pulse/v1"
	"github.com/spf13/cobra"
)

func newNodesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nodes",
		Short: "Manage nodes",
		Long:  "List, inspect, and manage compute nodes registered with the Pulse control plane.",
		Example: `  pulse nodes
  pulse nodes ls
  pulse nodes rm my-node
  pulse nodes rm my-node --yes`,
		RunE: listNodesRun,
	}

	cmd.AddCommand(newNodesLsCmd())
	cmd.AddCommand(newNodesRmCmd())

	return cmd
}

func newNodesLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "ls",
		Short:   "List all nodes",
		Example: `  pulse nodes ls`,
		RunE:    listNodesRun,
	}
}

func listNodesRun(_ *cobra.Command, _ []string) error {
	debugf("connecting to %s", apiAddr)
	client, conn, err := grpcclient.NewCLIClient(apiAddr)
	if err != nil {
		return err
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	debugf("listing nodes")
	resp, err := client.ListNodes(ctx, &pulsev1.ListNodesRequest{})
	if err != nil {
		return fmt.Errorf("list nodes: %w", err)
	}
	debugf("received %d nodes", len(resp.Nodes))

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
	var yes bool

	cmd := &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a node",
		Example: `  pulse nodes rm my-node
  pulse nodes rm my-node --yes`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]

			if !yes {
				fmt.Fprintf(os.Stderr, "Remove node %q? This cannot be undone. [y/N] ", name)
				reader := bufio.NewReader(os.Stdin)
				answer, _ := reader.ReadString('\n')
				if strings.TrimSpace(strings.ToLower(answer)) != "y" {
					fmt.Fprintln(os.Stderr, "Aborted.")
					return nil
				}
			}

			client, conn, err := grpcclient.NewCLIClient(apiAddr)
			if err != nil {
				return err
			}
			defer conn.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			_, err = client.DeleteNode(ctx, &pulsev1.DeleteNodeRequest{Name: name})
			if err != nil {
				return fmt.Errorf("delete node: %w", err)
			}

			fmt.Printf("Node %q removed\n", name)
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt")
	return cmd
}
