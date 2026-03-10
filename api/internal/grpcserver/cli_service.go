package grpcserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/ipedrazas/pulse/api/internal/repository"
	pulsev1 "github.com/ipedrazas/pulse/proto/gen/pulse/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// CLIService implements unary RPCs used by the CLI and UI.
type CLIService struct {
	pulsev1.UnimplementedCLIServiceServer
	repo         repository.Repository
	agentService *AgentService
}

func NewCLIService(repo repository.Repository, agentSvc *AgentService) *CLIService {
	return &CLIService{
		repo:         repo,
		agentService: agentSvc,
	}
}

func (s *CLIService) ListNodes(ctx context.Context, _ *pulsev1.ListNodesRequest) (*pulsev1.ListNodesResponse, error) {
	agents, err := s.repo.ListAgents(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list agents: %v", err)
	}

	var nodes []*pulsev1.NodeInfo
	for _, a := range agents {
		// Get container count for this agent
		_, count, err := s.repo.ListContainers(ctx, a.Name, 0, 0)
		if err != nil {
			slog.Error("count containers failed", "node", a.Name, "error", err)
		}
		node := agentToNodeInfo(a, int32(count))
		nodes = append(nodes, node)
	}
	return &pulsev1.ListNodesResponse{Nodes: nodes}, nil
}

func (s *CLIService) GetNode(ctx context.Context, req *pulsev1.GetNodeRequest) (*pulsev1.GetNodeResponse, error) {
	agent, err := s.repo.GetAgent(ctx, req.Name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get agent: %v", err)
	}
	if agent == nil {
		return nil, status.Errorf(codes.NotFound, "agent %q not found", req.Name)
	}

	containers, _, err := s.repo.ListContainers(ctx, req.Name, 100, 0)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list containers: %v", err)
	}

	node := agentToNodeInfo(*agent, int32(len(containers)))

	var protoContainers []*pulsev1.ContainerInfo
	for _, c := range containers {
		protoContainers = append(protoContainers, containerToProto(c))
	}

	return &pulsev1.GetNodeResponse{
		Node:       node,
		Containers: protoContainers,
	}, nil
}

func (s *CLIService) DeleteNode(ctx context.Context, req *pulsev1.DeleteNodeRequest) (*pulsev1.DeleteNodeResponse, error) {
	if req.Name == "" {
		return nil, status.Errorf(codes.InvalidArgument, "name is required")
	}
	if err := s.repo.DeleteAgent(ctx, req.Name); err != nil {
		return nil, status.Errorf(codes.NotFound, "node %q not found", req.Name)
	}
	s.agentService.streams.Remove(req.Name)
	slog.Info("node deleted", "node", req.Name)
	return &pulsev1.DeleteNodeResponse{}, nil
}

func (s *CLIService) ListContainers(ctx context.Context, req *pulsev1.ListContainersRequest) (*pulsev1.ListContainersResponse, error) {
	pageSize := int(req.PageSize)
	if pageSize <= 0 {
		pageSize = 50
	}
	offset := 0
	if req.PageToken != "" {
		if v, err := strconv.Atoi(req.PageToken); err == nil {
			offset = v
		}
	}

	containers, total, err := s.repo.ListContainers(ctx, req.NodeName, pageSize, offset)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list containers: %v", err)
	}

	var protoContainers []*pulsev1.ContainerInfo
	for _, c := range containers {
		protoContainers = append(protoContainers, containerToProto(c))
	}

	var nextToken string
	if offset+pageSize < total {
		nextToken = strconv.Itoa(offset + pageSize)
	}

	return &pulsev1.ListContainersResponse{
		Containers:    protoContainers,
		NextPageToken: nextToken,
	}, nil
}

func (s *CLIService) GetContainer(ctx context.Context, req *pulsev1.GetContainerRequest) (*pulsev1.GetContainerResponse, error) {
	c, err := s.repo.GetContainer(ctx, req.ContainerId, "")
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get container: %v", err)
	}
	if c == nil {
		return nil, status.Errorf(codes.NotFound, "container %q not found", req.ContainerId)
	}
	return &pulsev1.GetContainerResponse{Container: containerToProto(*c)}, nil
}

func (s *CLIService) SendCommand(ctx context.Context, req *pulsev1.SendCommandRequest) (*pulsev1.SendCommandResponse, error) {
	if req.NodeName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "node_name is required")
	}

	cmdID := uuid.New().String()
	cmdType, payload, err := marshalCommand(req)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "marshal command: %v", err)
	}

	cmd := repository.Command{
		ID:        cmdID,
		AgentName: req.NodeName,
		Type:      cmdType,
		Payload:   payload,
		Status:    "pending",
		CreatedAt: time.Now(),
	}
	if err := s.repo.CreateCommand(ctx, cmd); err != nil {
		return nil, status.Errorf(codes.Internal, "create command: %v", err)
	}

	// Try to send immediately if agent is connected
	serverCmd, err := commandToProto(cmd)
	if err == nil {
		if sendErr := s.agentService.SendToAgent(req.NodeName, serverCmd); sendErr != nil {
			slog.Info("agent not connected, command queued", "node", req.NodeName, "id", cmdID)
		}
	}

	return &pulsev1.SendCommandResponse{
		CommandId: cmdID,
		Accepted:  true,
	}, nil
}

func (s *CLIService) GetCommandResult(ctx context.Context, req *pulsev1.GetCommandResultRequest) (*pulsev1.GetCommandResultResponse, error) {
	cmd, err := s.repo.GetCommand(ctx, req.CommandId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get command: %v", err)
	}
	if cmd == nil {
		return nil, status.Errorf(codes.NotFound, "command %q not found", req.CommandId)
	}
	return &pulsev1.GetCommandResultResponse{
		CommandId: cmd.ID,
		Status:    cmd.Status,
		Result:    cmd.Result,
	}, nil
}

func marshalCommand(req *pulsev1.SendCommandRequest) (string, []byte, error) {
	switch cmd := req.Command.(type) {
	case *pulsev1.SendCommandRequest_RunContainer:
		data, err := json.Marshal(cmd.RunContainer)
		return "run_container", data, err
	case *pulsev1.SendCommandRequest_StopContainer:
		data, err := json.Marshal(cmd.StopContainer)
		return "stop_container", data, err
	case *pulsev1.SendCommandRequest_PullImage:
		data, err := json.Marshal(cmd.PullImage)
		return "pull_image", data, err
	case *pulsev1.SendCommandRequest_ComposeUp:
		data, err := json.Marshal(cmd.ComposeUp)
		return "compose_up", data, err
	case *pulsev1.SendCommandRequest_SendFile:
		data, err := json.Marshal(cmd.SendFile)
		return "send_file", data, err
	case *pulsev1.SendCommandRequest_RequestLogs:
		data, err := json.Marshal(cmd.RequestLogs)
		return "request_logs", data, err
	case *pulsev1.SendCommandRequest_RestartContainer:
		data, err := json.Marshal(cmd.RestartContainer)
		return "restart_container", data, err
	default:
		return "", nil, status.Errorf(codes.InvalidArgument, "unknown command type")
	}
}

func agentToNodeInfo(a repository.Agent, containerCount int32) *pulsev1.NodeInfo {
	node := &pulsev1.NodeInfo{
		Name:           a.Name,
		Status:         a.Status,
		AgentVersion:   a.Version,
		ContainerCount: containerCount,
	}
	if a.LastSeen != nil {
		node.LastSeen = timestamppb.New(*a.LastSeen)
	}
	if a.Metadata != nil {
		node.Metadata = &pulsev1.NodeMetadata{
			Hostname:         a.Metadata.Hostname,
			IpAddress:        a.Metadata.IPAddress,
			OsName:           a.Metadata.OSName,
			OsVersion:        a.Metadata.OSVersion,
			KernelVersion:    a.Metadata.KernelVersion,
			UptimeSeconds:    a.Metadata.UptimeSeconds,
			PackagesToUpdate: a.Metadata.PackagesToUpdate,
		}
	}
	return node
}

func containerToProto(c repository.Container) *pulsev1.ContainerInfo {
	var ports []*pulsev1.PortMapping
	for _, p := range c.Ports {
		ports = append(ports, &pulsev1.PortMapping{
			HostIp:        p.HostIP,
			HostPort:      p.HostPort,
			ContainerPort: p.ContainerPort,
			Protocol:      p.Protocol,
		})
	}
	return &pulsev1.ContainerInfo{
		Id:             c.ContainerID,
		Name:           c.Name,
		Image:          c.Image,
		Status:         c.Status,
		EnvVars:        c.EnvVars,
		Mounts:         c.Mounts,
		Labels:         c.Labels,
		Ports:          ports,
		UptimeSeconds:  c.UptimeSeconds,
		ComposeProject: c.ComposeProject,
		Command:        c.Command,
	}
}
