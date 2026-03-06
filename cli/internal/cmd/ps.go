package cmd

import (
	"context"
	"fmt"
	"time"

	pulsev1 "github.com/ipedrazas/pulse/proto/gen/pulse/v1"
	grpcclient "github.com/ipedrazas/pulse/cli/internal/grpc"
	"github.com/spf13/cobra"
)

func newPsCmd() *cobra.Command {
	var node string

	cmd := &cobra.Command{
		Use:   "ps",
		Short: "List containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, conn, err := grpcclient.NewCLIClient(apiAddr)
			if err != nil {
				return err
			}
			defer conn.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			resp, err := client.ListContainers(ctx, &pulsev1.ListContainersRequest{
				NodeName: node,
				PageSize: 100,
			})
			if err != nil {
				return fmt.Errorf("list containers: %w", err)
			}

			if output == "json" {
				return printJSON(resp.Containers)
			}

			headers := []string{"CONTAINER ID", "NAME", "IMAGE", "STATUS", "NODE", "UPTIME"}
			var rows [][]string
			for _, c := range resp.Containers {
				rows = append(rows, []string{
					truncate(c.Id, 12),
					c.Name,
					c.Image,
					c.Status,
					node,
					formatUptime(c.UptimeSeconds),
				})
			}
			printTable(headers, rows)
			return nil
		},
	}

	cmd.Flags().StringVar(&node, "node", "", "Filter by node name")
	return cmd
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func formatUptime(seconds int64) string {
	d := time.Duration(seconds) * time.Second
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
