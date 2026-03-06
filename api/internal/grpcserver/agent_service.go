package grpcserver

import (
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"time"

	pulsev1 "github.com/ipedrazas/pulse/proto/gen/pulse/v1"
	"github.com/ipedrazas/pulse/api/internal/repository"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AgentService implements the bidirectional streaming AgentService.
type AgentService struct {
	pulsev1.UnimplementedAgentServiceServer
	repo    repository.Repository
	streams *StreamRegistry
}

func NewAgentService(repo repository.Repository) *AgentService {
	return &AgentService{
		repo:    repo,
		streams: NewStreamRegistry(),
	}
}

// StreamLink handles the long-lived bidirectional stream from an agent.
func (s *AgentService) StreamLink(stream pulsev1.AgentService_StreamLinkServer) error {
	ctx := stream.Context()
	var nodeName string

	for {
		select {
		case <-ctx.Done():
			if nodeName != "" {
				slog.Info("agent disconnected", "node", nodeName)
				_ = s.repo.SetAgentStatus(ctx, nodeName, "offline")
				s.streams.Remove(nodeName)
			}
			return ctx.Err()
		default:
		}

		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			if nodeName != "" {
				slog.Warn("stream error", "node", nodeName, "error", err)
				_ = s.repo.SetAgentStatus(ctx, nodeName, "offline")
				s.streams.Remove(nodeName)
			}
			return status.Errorf(codes.Internal, "recv: %v", err)
		}

		switch payload := msg.Payload.(type) {
		case *pulsev1.AgentMessage_Heartbeat:
			nodeName = payload.Heartbeat.NodeName
			now := time.Now()
			agent := repository.Agent{
				Name:     nodeName,
				Status:   "online",
				Version:  payload.Heartbeat.AgentVersion,
				LastSeen: &now,
			}
			if err := s.repo.UpsertAgent(ctx, agent); err != nil {
				slog.Error("upsert agent failed", "node", nodeName, "error", err)
			}
			s.streams.Set(nodeName, stream)

			// Send any pending commands
			cmds, err := s.repo.GetPendingCommands(ctx, nodeName)
			if err != nil {
				slog.Error("get pending commands failed", "node", nodeName, "error", err)
				continue
			}
			for _, cmd := range cmds {
				serverCmd, err := commandToProto(cmd)
				if err != nil {
					slog.Error("convert command failed", "id", cmd.ID, "error", err)
					continue
				}
				if err := stream.Send(serverCmd); err != nil {
					slog.Error("send command failed", "id", cmd.ID, "error", err)
				}
			}

		case *pulsev1.AgentMessage_ContainerReport:
			report := payload.ContainerReport
			nodeName = report.NodeName

			var activeIDs []string
			for _, ci := range report.Containers {
				activeIDs = append(activeIDs, ci.Id)
				c := protoToContainer(ci, nodeName)
				if err := s.repo.UpsertContainer(ctx, c); err != nil {
					slog.Error("upsert container failed", "id", ci.Id, "error", err)
				}
				event := repository.ContainerEvent{
					Time:          time.Now(),
					ContainerID:   ci.Id,
					AgentName:     nodeName,
					Status:        ci.Status,
					UptimeSeconds: ci.UptimeSeconds,
				}
				if err := s.repo.InsertContainerEvent(ctx, event); err != nil {
					slog.Error("insert event failed", "id", ci.Id, "error", err)
				}
			}
			if err := s.repo.MarkContainersRemoved(ctx, nodeName, activeIDs); err != nil {
				slog.Error("mark removed failed", "node", nodeName, "error", err)
			}

		case *pulsev1.AgentMessage_CommandResult:
			result := payload.CommandResult
			output := result.Output
			if result.Error != "" {
				output = result.Error
			}
			if err := s.repo.CompleteCommand(ctx, result.CommandId, output, result.Success); err != nil {
				slog.Error("complete command failed", "id", result.CommandId, "error", err)
			}
		}
	}

	return nil
}

// SendToAgent sends a command to a connected agent via its stream.
func (s *AgentService) SendToAgent(nodeName string, cmd *pulsev1.ServerCommand) error {
	stream, ok := s.streams.Get(nodeName)
	if !ok {
		return status.Errorf(codes.NotFound, "agent %q not connected", nodeName)
	}
	return stream.Send(cmd)
}

func protoToContainer(ci *pulsev1.ContainerInfo, agentName string) repository.Container {
	envVars := ci.EnvVars
	if envVars == nil {
		envVars = map[string]string{}
	}
	labels := ci.Labels
	if labels == nil {
		labels = map[string]string{}
	}
	var ports []repository.Port
	for _, p := range ci.Ports {
		ports = append(ports, repository.Port{
			HostIP:        p.HostIp,
			HostPort:      p.HostPort,
			ContainerPort: p.ContainerPort,
			Protocol:      p.Protocol,
		})
	}
	return repository.Container{
		ContainerID:    ci.Id,
		AgentName:      agentName,
		Name:           ci.Name,
		Image:          ci.Image,
		Status:         ci.Status,
		EnvVars:        envVars,
		Mounts:         ci.Mounts,
		Labels:         labels,
		Ports:          ports,
		ComposeProject: ci.ComposeProject,
		Command:        ci.Command,
		UptimeSeconds:  ci.UptimeSeconds,
	}
}

func commandToProto(cmd repository.Command) (*pulsev1.ServerCommand, error) {
	sc := &pulsev1.ServerCommand{CommandId: cmd.ID}
	switch cmd.Type {
	case "run_container":
		var rc pulsev1.RunContainer
		if err := json.Unmarshal(cmd.Payload, &rc); err != nil {
			return nil, err
		}
		sc.Payload = &pulsev1.ServerCommand_RunContainer{RunContainer: &rc}
	case "stop_container":
		var sc2 pulsev1.StopContainer
		if err := json.Unmarshal(cmd.Payload, &sc2); err != nil {
			return nil, err
		}
		sc.Payload = &pulsev1.ServerCommand_StopContainer{StopContainer: &sc2}
	case "pull_image":
		var pi pulsev1.PullImage
		if err := json.Unmarshal(cmd.Payload, &pi); err != nil {
			return nil, err
		}
		sc.Payload = &pulsev1.ServerCommand_PullImage{PullImage: &pi}
	case "compose_up":
		var cu pulsev1.ComposeUp
		if err := json.Unmarshal(cmd.Payload, &cu); err != nil {
			return nil, err
		}
		sc.Payload = &pulsev1.ServerCommand_ComposeUp{ComposeUp: &cu}
	}
	return sc, nil
}

// StreamRegistry tracks active agent streams by node name.
type StreamRegistry struct {
	mu      sync.RWMutex
	streams map[string]pulsev1.AgentService_StreamLinkServer
}

func NewStreamRegistry() *StreamRegistry {
	return &StreamRegistry{streams: make(map[string]pulsev1.AgentService_StreamLinkServer)}
}

func (r *StreamRegistry) Set(name string, stream pulsev1.AgentService_StreamLinkServer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.streams[name] = stream
}

func (r *StreamRegistry) Get(name string) (pulsev1.AgentService_StreamLinkServer, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.streams[name]
	return s, ok
}

func (r *StreamRegistry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.streams, name)
}
