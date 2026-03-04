package repository

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

// ContainerRepository provides access to container data.
type ContainerRepository interface {
	ListContainers(ctx context.Context) ([]ContainerStatus, error)
	GetContainer(ctx context.Context, containerID string) (ContainerStatus, error)
	ListContainersByNode(ctx context.Context, nodeName string) ([]ContainerStatus, error)
	ListContainersByNodeForStacks(ctx context.Context, nodeName string) ([]ContainerStatus, error)
	GetComposeDir(ctx context.Context, nodeName, project string) (string, error)
	UpsertMetadata(ctx context.Context, m ContainerMetadata) error
	InsertHeartbeat(ctx context.Context, containerID, status string, uptimeSeconds int64) error
	GetPreviousStatus(ctx context.Context, containerID string) (string, error)
	GetContainerInfoForRemoval(ctx context.Context, nodeName string, containerIDs []string) ([]ContainerInfo, error)
	GetContainerMetadataForEvent(ctx context.Context, containerID string) (ContainerInfo, string, error)
	MarkRemoved(ctx context.Context, nodeName string, containerIDs []string) (int64, error)
	SweepStale(ctx context.Context, maxAge time.Duration) (int64, error)
}

// ActionRepository provides access to command/action data.
type ActionRepository interface {
	CreateAction(ctx context.Context, nodeName, action, target string, paramsJSON []byte) (ActionResponse, error)
	ListActions(ctx context.Context, nodeName string) ([]ActionResponse, error)
	GetAction(ctx context.Context, commandID, nodeName string) (ActionResponse, error)
	ClaimPendingCommands(ctx context.Context, nodeName string) ([]PendingCommand, error)
	UpdateCommandResult(ctx context.Context, commandID, status, output string, durationMs int64) error
}

// AgentRepository provides access to agent data.
type AgentRepository interface {
	UpsertAgent(ctx context.Context, nodeName, agentVersion string) error
	ListOnlineAgents(ctx context.Context) ([]string, error)
	ListAgents(ctx context.Context) ([]AgentStatus, error)
}

// HealthChecker checks the health of the data store.
type HealthChecker interface {
	Ping(ctx context.Context) error
}
