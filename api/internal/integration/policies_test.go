package integration

import (
	"context"
	"testing"
)

func TestTimescaleDBPoliciesExist(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	env := setupTestEnv(t)
	ctx := context.Background()

	// Verify hypertable exists
	var htCount int
	err := env.pool.QueryRow(ctx,
		`SELECT count(*) FROM timescaledb_information.hypertables
		 WHERE hypertable_name = 'container_heartbeats'`,
	).Scan(&htCount)
	if err != nil {
		t.Fatalf("failed to query hypertables: %v", err)
	}
	if htCount != 1 {
		t.Fatalf("expected 1 hypertable, got %d", htCount)
	}

	// Verify compression is enabled
	var compressionEnabled bool
	err = env.pool.QueryRow(ctx,
		`SELECT compression_enabled FROM timescaledb_information.hypertables
		 WHERE hypertable_name = 'container_heartbeats'`,
	).Scan(&compressionEnabled)
	if err != nil {
		t.Fatalf("failed to query compression: %v", err)
	}
	if !compressionEnabled {
		t.Fatal("compression should be enabled on container_heartbeats")
	}

	// Verify compression policy exists
	var compPolicyCount int
	err = env.pool.QueryRow(ctx,
		`SELECT count(*) FROM timescaledb_information.jobs
		 WHERE hypertable_name = 'container_heartbeats'
		   AND proc_name = 'policy_compression'`,
	).Scan(&compPolicyCount)
	if err != nil {
		t.Fatalf("failed to query compression policy: %v", err)
	}
	if compPolicyCount != 1 {
		t.Fatalf("expected 1 compression policy, got %d", compPolicyCount)
	}

	// Verify retention policy exists
	var retPolicyCount int
	err = env.pool.QueryRow(ctx,
		`SELECT count(*) FROM timescaledb_information.jobs
		 WHERE hypertable_name = 'container_heartbeats'
		   AND proc_name = 'policy_retention'`,
	).Scan(&retPolicyCount)
	if err != nil {
		t.Fatalf("failed to query retention policy: %v", err)
	}
	if retPolicyCount != 1 {
		t.Fatalf("expected 1 retention policy, got %d", retPolicyCount)
	}
}
