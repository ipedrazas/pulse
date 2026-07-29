package cmd

import (
	"context"
	"fmt"
	"time"

	grpcclient "github.com/ipedrazas/pulse/cli/internal/grpc"
	pulsev1 "github.com/ipedrazas/pulse/proto/gen/pulse/v1"
	"github.com/spf13/cobra"
)

func newStatsCmd() *cobra.Command {
	var (
		node  string
		since string
	)

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show per-container restart, status, and uptime stats over a time window",
		Long:  "Reports per-container restart counts, status transitions, and uptime resets observed on a node over the given lookback window.",
		Example: `  pulse stats --since 24h
  pulse stats --node worker-1 --since 2h
  pulse stats --since 30m -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			window, err := time.ParseDuration(since)
			if err != nil {
				return fmt.Errorf("invalid --since duration %q: %w", since, err)
			}

			debugf("connecting to %s", apiAddr)
			client, conn, err := grpcclient.NewCLIClient(apiAddr)
			if err != nil {
				return err
			}
			defer conn.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			resolvedNode, err := resolveNode(ctx, client, node)
			if err != nil {
				return err
			}

			debugf("getting container stats (node=%q, since=%s)", resolvedNode, window)
			resp, err := client.GetContainerStats(ctx, &pulsev1.GetContainerStatsRequest{
				NodeName:     resolvedNode,
				SinceSeconds: int64(window.Seconds()),
			})
			if err != nil {
				return fmt.Errorf("get container stats: %w", err)
			}

			if output == "json" {
				return printJSON(resp.Containers)
			}

			headers := []string{"CONTAINER ID", "NAME", "STATUS", "RESTARTS", "STATUS CHANGES", "UPTIME RESETS"}
			var rows [][]string
			for _, c := range resp.Containers {
				rows = append(rows, []string{
					truncate(c.ContainerId, 12),
					c.Name,
					c.CurrentStatus,
					fmt.Sprint(c.RestartCount),
					fmt.Sprint(len(c.StatusTransitions)),
					fmt.Sprint(len(c.UptimeResets)),
				})
			}
			printTable(headers, rows)
			return nil
		},
	}

	cmd.Flags().StringVar(&node, "node", "", "Target node (defaults to the configured default node)")
	cmd.Flags().StringVar(&since, "since", "24h", "Lookback window (e.g. 24h, 2h, 30m)")

	return cmd
}
