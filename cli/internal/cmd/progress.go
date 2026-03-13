package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	pulsev1 "github.com/ipedrazas/pulse/proto/gen/pulse/v1"
)

// waitForCommand polls until the command completes or the context is cancelled,
// displaying a spinner on stderr. Returns the final result or an error.
func waitForCommand(ctx context.Context, client pulsev1.CLIServiceClient, commandID string) (*pulsev1.GetCommandResultResponse, error) {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	i := 0
	for {
		select {
		case <-ctx.Done():
			fmt.Fprint(os.Stderr, "\r\033[K")
			return nil, fmt.Errorf("timed out waiting for command %s", commandID)
		case <-ticker.C:
			fmt.Fprintf(os.Stderr, "\r%s Waiting for command %s...", frames[i%len(frames)], truncate(commandID, 8))
			i++

			result, err := client.GetCommandResult(ctx, &pulsev1.GetCommandResultRequest{
				CommandId: commandID,
			})
			if err != nil {
				fmt.Fprint(os.Stderr, "\r\033[K")
				return nil, fmt.Errorf("get result: %w", err)
			}
			switch result.Status {
			case "completed":
				fmt.Fprint(os.Stderr, "\r\033[K")
				return result, nil
			case "failed":
				fmt.Fprint(os.Stderr, "\r\033[K")
				return result, fmt.Errorf("command failed: %s", result.Result)
			}
			// still pending
		}
	}
}
