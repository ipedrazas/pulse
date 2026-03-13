package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	grpcclient "github.com/ipedrazas/pulse/cli/internal/grpc"
	pulsev1 "github.com/ipedrazas/pulse/proto/gen/pulse/v1"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var (
		node    string
		image   string
		name    string
		envVars []string
		ports   []string
		volumes []string
		rm      bool
		wait    bool
	)

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run a container on a node",
		Long:  "Run a new container on a remote node. Returns a command ID that can be used to track execution.",
		Example: `  pulse run --node worker-1 --image nginx
  pulse run --node worker-1 --image redis --name my-redis
  pulse run --node worker-1 --image nginx -p 8080:80 -e ENV=prod
  pulse run --node worker-1 --image nginx -v /data:/data --rm`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if node == "" {
				return fmt.Errorf("--node is required")
			}
			if image == "" {
				return fmt.Errorf("--image is required")
			}

			debugf("connecting to %s", apiAddr)
			client, conn, err := grpcclient.NewCLIClient(apiAddr)
			if err != nil {
				return err
			}
			defer conn.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			debugf("running container image=%q on node=%q", image, node)

			envMap := make(map[string]string)
			for _, e := range envVars {
				parts := strings.SplitN(e, "=", 2)
				if len(parts) != 2 || parts[0] == "" {
					return fmt.Errorf("invalid env var %q: expected KEY=VALUE format", e)
				}
				envMap[parts[0]] = parts[1]
			}

			var portMappings []*pulsev1.PortMapping
			for _, p := range ports {
				pm, err := parsePort(p)
				if err != nil {
					return fmt.Errorf("invalid port mapping %q: %w", p, err)
				}
				portMappings = append(portMappings, pm)
			}

			resp, err := client.SendCommand(ctx, &pulsev1.SendCommandRequest{
				NodeName: node,
				Command: &pulsev1.SendCommandRequest_RunContainer{
					RunContainer: &pulsev1.RunContainer{
						Image:   image,
						Name:    name,
						Env:     envMap,
						Ports:   portMappings,
						Volumes: volumes,
						Remove:  rm,
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
			fmt.Printf("Container started (command %s)\n", result.CommandId)
			return nil
		},
	}

	cmd.Flags().StringVar(&node, "node", "", "Target node (required)")
	cmd.Flags().StringVar(&image, "image", "", "Container image (required)")
	cmd.Flags().StringVar(&name, "name", "", "Container name")
	cmd.Flags().StringSliceVarP(&envVars, "env", "e", nil, "Environment variables (KEY=VALUE)")
	cmd.Flags().StringSliceVarP(&ports, "port", "p", nil, "Port mappings (host:container)")
	cmd.Flags().StringSliceVarP(&volumes, "volume", "v", nil, "Volume mounts")
	cmd.Flags().BoolVar(&rm, "rm", false, "Remove container when it exits")
	cmd.Flags().BoolVar(&wait, "wait", false, "Wait for the command to complete")

	return cmd
}

func parsePort(s string) (*pulsev1.PortMapping, error) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("expected host:container format")
	}
	var hostPort, containerPort uint32
	if n, _ := fmt.Sscanf(parts[0], "%d", &hostPort); n != 1 {
		return nil, fmt.Errorf("invalid host port %q", parts[0])
	}
	if n, _ := fmt.Sscanf(parts[1], "%d", &containerPort); n != 1 {
		return nil, fmt.Errorf("invalid container port %q", parts[1])
	}
	return &pulsev1.PortMapping{
		HostPort:      hostPort,
		ContainerPort: containerPort,
		Protocol:      "tcp",
	}, nil
}
