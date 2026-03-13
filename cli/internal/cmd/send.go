package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	grpcclient "github.com/ipedrazas/pulse/cli/internal/grpc"
	pulsev1 "github.com/ipedrazas/pulse/proto/gen/pulse/v1"
	"github.com/spf13/cobra"
)

const chunkSize = 64 * 1024 // 64KB

func newSendCmd() *cobra.Command {
	var (
		node     string
		filePath string
		destPath string
	)

	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send a file to a node",
		Long:  "Transfer a local file to a remote node. Files are sent in 64KB chunks.",
		Example: `  pulse send --node worker-1 --file ./config.yaml
  pulse send --node worker-1 --file ./app.tar.gz --dest /opt/app/app.tar.gz`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if node == "" {
				return fmt.Errorf("--node is required")
			}
			if filePath == "" {
				return fmt.Errorf("--file is required")
			}

			data, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}

			if destPath == "" {
				destPath = filepath.Base(filePath)
			}

			client, conn, err := grpcclient.NewCLIClient(apiAddr)
			if err != nil {
				return err
			}
			defer conn.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			totalSize := int64(len(data))

			// Send file in chunks
			var lastCmdID string
			for offset := int64(0); offset < totalSize; offset += chunkSize {
				end := offset + chunkSize
				if end > totalSize {
					end = totalSize
				}
				chunk := data[offset:end]
				isFinal := end == totalSize

				pct := int(float64(end) / float64(totalSize) * 100)
				fmt.Fprintf(os.Stderr, "\rSending %s... %d%%", filepath.Base(filePath), pct)

				resp, err := client.SendCommand(ctx, &pulsev1.SendCommandRequest{
					NodeName: node,
					Command: &pulsev1.SendCommandRequest_SendFile{
						SendFile: &pulsev1.SendFile{
							Path:       destPath,
							Data:       chunk,
							Offset:     offset,
							TotalSize:  totalSize,
							FinalChunk: isFinal,
						},
					},
				})
				if err != nil {
					fmt.Fprint(os.Stderr, "\r\033[K")
					return fmt.Errorf("send chunk at offset %d: %w", offset, err)
				}
				lastCmdID = resp.CommandId
			}
			fmt.Fprint(os.Stderr, "\r\033[K")
			fmt.Printf("File sent: %s → %s (command: %s)\n", filePath, destPath, lastCmdID)

			return nil
		},
	}

	cmd.Flags().StringVar(&node, "node", "", "Target node (required)")
	cmd.Flags().StringVar(&filePath, "file", "", "Local file path to send (required)")
	cmd.Flags().StringVar(&destPath, "dest", "", "Destination path on the agent (defaults to filename)")

	return cmd
}
