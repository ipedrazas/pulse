package repository

import "context"

type Repository interface {
	// Agents
	UpsertAgent(ctx context.Context, agent Agent) error
	GetAgent(ctx context.Context, name string) (*Agent, error)
	ListAgents(ctx context.Context) ([]Agent, error)
	SetAgentStatus(ctx context.Context, name, status string) error

	// Containers
	UpsertContainer(ctx context.Context, c Container) error
	GetContainer(ctx context.Context, containerID, agentName string) (*Container, error)
	ListContainers(ctx context.Context, agentName string, limit, offset int) ([]Container, int, error)
	MarkContainersRemoved(ctx context.Context, agentName string, activeIDs []string) error

	// Container Events
	InsertContainerEvent(ctx context.Context, event ContainerEvent) error

	// Commands
	CreateCommand(ctx context.Context, cmd Command) error
	GetCommand(ctx context.Context, id string) (*Command, error)
	GetPendingCommands(ctx context.Context, agentName string) ([]Command, error)
	CompleteCommand(ctx context.Context, id, result string, success bool) error
}
