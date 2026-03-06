package grpc

import (
	"fmt"

	pulsev1 "github.com/ipedrazas/pulse/proto/gen/pulse/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewCLIClient(addr string) (pulsev1.CLIServiceClient, *grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, fmt.Errorf("connect to %s: %w", addr, err)
	}
	return pulsev1.NewCLIServiceClient(conn), conn, nil
}
