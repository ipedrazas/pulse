package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	pulsev1 "github.com/ipedrazas/pulse/proto/gen/pulse/v1"
	grpcclient "github.com/ipedrazas/pulse/cli/internal/grpc"
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
			for offset := int64(0); offset < totalSize; offset += chunkSize {
				end := offset + chunkSize
				if end > totalSize {
					end = totalSize
				}
				chunk := data[offset:end]
				isFinal := end == totalSize

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
					return fmt.Errorf("send chunk at offset %d: %w", offset, err)
				}

				if isFinal {
					fmt.Printf("File sent: %s → %s (command: %s)\n", filePath, destPath, resp.CommandId)
				}
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&node, "node", "", "Target node (required)")
	cmd.Flags().StringVar(&filePath, "file", "", "Local file path to send (required)")
	cmd.Flags().StringVar(&destPath, "dest", "", "Destination path on the agent (defaults to filename)")

	return cmd
}
