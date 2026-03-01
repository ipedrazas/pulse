package grpcserver

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestTokenAuthInterceptor(t *testing.T) {
	const validToken = "test-secret-token"
	interceptor := TokenAuthInterceptor(validToken)

	noopHandler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Method"}

	tests := []struct {
		name     string
		ctx      context.Context
		wantCode codes.Code
		wantOK   bool
	}{
		{
			name:     "valid token",
			ctx:      metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-monitor-token", validToken)),
			wantCode: codes.OK,
			wantOK:   true,
		},
		{
			name:     "invalid token",
			ctx:      metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-monitor-token", "wrong")),
			wantCode: codes.Unauthenticated,
		},
		{
			name:     "missing token header",
			ctx:      metadata.NewIncomingContext(context.Background(), metadata.Pairs()),
			wantCode: codes.Unauthenticated,
		},
		{
			name:     "no metadata at all",
			ctx:      context.Background(),
			wantCode: codes.Unauthenticated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := interceptor(tt.ctx, nil, info, noopHandler)

			if tt.wantOK {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				if resp != "ok" {
					t.Fatalf("expected 'ok', got %v", resp)
				}
				return
			}

			if err == nil {
				t.Fatal("expected error, got nil")
			}
			st, ok := status.FromError(err)
			if !ok {
				t.Fatalf("expected gRPC status error, got %v", err)
			}
			if st.Code() != tt.wantCode {
				t.Fatalf("expected code %v, got %v", tt.wantCode, st.Code())
			}
		})
	}
}
