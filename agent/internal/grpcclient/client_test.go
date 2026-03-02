package grpcclient

import (
	"context"
	"testing"

	"google.golang.org/grpc/metadata"
)

func TestNew_CreatesClient(t *testing.T) {
	c, err := New("localhost:50051", "test-token", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer c.Close()

	if c.token != "test-token" {
		t.Errorf("expected token 'test-token', got %s", c.token)
	}
	if c.conn == nil {
		t.Fatal("expected non-nil connection")
	}
	if c.service == nil {
		t.Fatal("expected non-nil service client")
	}
}

func TestClose(t *testing.T) {
	c, err := New("localhost:50051", "test-token", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := c.Close(); err != nil {
		t.Fatalf("unexpected close error: %v", err)
	}
}

func TestWithToken_AddsMetadata(t *testing.T) {
	c := &Client{token: "my-secret"}

	ctx := c.withToken(context.Background())
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok {
		t.Fatal("expected outgoing metadata")
	}

	values := md.Get("x-monitor-token")
	if len(values) != 1 || values[0] != "my-secret" {
		t.Errorf("expected token 'my-secret', got %v", values)
	}
}
