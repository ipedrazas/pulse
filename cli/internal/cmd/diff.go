package cmd

import (
	"context"
	"fmt"
	"time"

	grpcclient "github.com/ipedrazas/pulse/cli/internal/grpc"
	pulsev1 "github.com/ipedrazas/pulse/proto/gen/pulse/v1"
	"github.com/spf13/cobra"
)

func newDiffCmd() *cobra.Command {
	var (
		node  string
		since string
	)

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Compare current container state against a point in the past",
		Long:  "Shows containers that appeared, disappeared, changed image, or restarted on a node since the given lookback window.",
		Example: `  pulse diff --since 2h
  pulse diff --node worker-1 --since 30m
  pulse diff --since 2h -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if since == "" {
				return fmt.Errorf("--since is required")
			}
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

			debugf("diffing containers (node=%q, since=%s)", resolvedNode, window)
			resp, err := client.DiffContainers(ctx, &pulsev1.DiffContainersRequest{
				NodeName:     resolvedNode,
				SinceSeconds: int64(window.Seconds()),
			})
			if err != nil {
				return fmt.Errorf("diff containers: %w", err)
			}

			if output == "json" {
				return printJSON(resp)
			}

			printContainerDiffSection("APPEARED", resp.Appeared)
			printContainerDiffSection("DISAPPEARED", resp.Disappeared)
			printContainerDiffSection("RESTARTED", resp.Restarted)
			printImageChanges(resp.ChangedImage)

			return nil
		},
	}

	cmd.Flags().StringVar(&node, "node", "", "Target node (defaults to the configured default node)")
	cmd.Flags().StringVar(&since, "since", "", "Lookback window to diff against, e.g. 2h, 30m (required)")

	return cmd
}

func printContainerDiffSection(title string, containers []*pulsev1.ContainerInfo) {
	fmt.Printf("%s (%d)\n", title, len(containers))
	if len(containers) == 0 {
		fmt.Println("  (none)")
		fmt.Println()
		return
	}
	headers := []string{"CONTAINER ID", "NAME", "IMAGE", "STATUS"}
	var rows [][]string
	for _, c := range containers {
		rows = append(rows, []string{truncate(c.Id, 12), c.Name, c.Image, c.Status})
	}
	printTable(headers, rows)
	fmt.Println()
}

func printImageChanges(changes []*pulsev1.ImageChange) {
	fmt.Printf("CHANGED IMAGE (%d)\n", len(changes))
	if len(changes) == 0 {
		fmt.Println("  (none)")
		return
	}
	headers := []string{"NAME", "OLD IMAGE", "NEW IMAGE"}
	var rows [][]string
	for _, c := range changes {
		rows = append(rows, []string{c.Name, c.OldImage, c.NewImage})
	}
	printTable(headers, rows)
}
