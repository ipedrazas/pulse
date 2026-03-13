package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/ipedrazas/pulse/cli/internal/config"
	"github.com/spf13/cobra"
)

var (
	apiAddr string
	output  string
	verbose bool
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "pulse",
		Short: "Pulse — distributed container management CLI",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			resolveAPIAddr(cmd)

			if verbose {
				slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
				slog.Debug("config resolved", "api-addr", apiAddr, "output", output)
			}
		},
	}

	root.PersistentFlags().StringVar(&apiAddr, "api-addr", "", "API server gRPC address")
	root.PersistentFlags().StringVarP(&output, "output", "o", "table", "Output format: table, json")
	root.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose output for troubleshooting")

	root.AddCommand(
		newNodesCmd(),
		newPsCmd(),
		newRunCmd(),
		newStopCmd(),
		newPullCmd(),
		newUpCmd(),
		newLogsCmd(),
		newSendCmd(),
		newVersionCmd(),
		newCompletionCmd(),
	)

	return root
}

// resolveAPIAddr applies config precedence: flag > env > config file > default.
func resolveAPIAddr(cmd *cobra.Command) {
	// If the user passed --api-addr explicitly, keep it.
	if cmd.Flags().Changed("api-addr") {
		return
	}

	// Environment variable.
	if v := os.Getenv("PULSE_API_ADDR"); v != "" {
		apiAddr = v
		return
	}

	// Config file.
	cfg := config.Load()
	if cfg.APIAddr != "" {
		apiAddr = cfg.APIAddr
		return
	}

	// Default.
	apiAddr = "localhost:9090"
}

// debugf logs a debug message when verbose mode is enabled.
func debugf(format string, args ...any) {
	if verbose {
		fmt.Fprintf(os.Stderr, "[debug] "+format+"\n", args...)
	}
}
