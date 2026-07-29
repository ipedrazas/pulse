package grpcserver

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"github.com/ipedrazas/pulse/api/internal/repository"
	pulsev1 "github.com/ipedrazas/pulse/proto/gen/pulse/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// containerTimeline holds the restart/status signals derived from a
// container's event history within a time window.
type containerTimeline struct {
	restartCount      int32
	statusTransitions []*pulsev1.StatusTransition
	uptimeResets      []time.Time
}

// computeTimelines groups events (expected sorted by container_id, then
// time) and derives, per container, status transitions and uptime resets
// (a drop in uptime_seconds between consecutive samples signals a restart).
func computeTimelines(events []repository.ContainerEvent) map[string]*containerTimeline {
	timelines := make(map[string]*containerTimeline)
	var prev *repository.ContainerEvent

	for i := range events {
		e := &events[i]
		tl, ok := timelines[e.ContainerID]
		if !ok {
			tl = &containerTimeline{}
			timelines[e.ContainerID] = tl
			prev = nil
		}

		if prev != nil {
			if e.Status != prev.Status {
				tl.statusTransitions = append(tl.statusTransitions, &pulsev1.StatusTransition{
					Time:       timestamppb.New(e.Time),
					FromStatus: prev.Status,
					ToStatus:   e.Status,
				})
			}
			if e.UptimeSeconds < prev.UptimeSeconds {
				tl.restartCount++
				tl.uptimeResets = append(tl.uptimeResets, e.Time)
			}
		}
		prev = e
	}
	return timelines
}

func (s *CLIService) GetContainerStats(ctx context.Context, req *pulsev1.GetContainerStatsRequest) (*pulsev1.GetContainerStatsResponse, error) {
	if req.NodeName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "node_name is required")
	}

	since := time.Now().Add(-time.Duration(req.SinceSeconds) * time.Second)

	events, err := s.repo.ListContainerEvents(ctx, req.NodeName, since)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list container events: %v", err)
	}
	timelines := computeTimelines(events)

	containers, _, err := s.repo.ListContainers(ctx, req.NodeName, 1000, 0)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list containers: %v", err)
	}

	seen := make(map[string]bool, len(containers))
	var out []*pulsev1.ContainerStats
	for _, c := range containers {
		seen[c.ContainerID] = true
		out = append(out, buildContainerStats(c.ContainerID, c.Name, c.Status, timelines[c.ContainerID]))
	}

	// Containers with activity in the window that are no longer running
	// (and so weren't in the list above) still get reported.
	for containerID, tl := range timelines {
		if seen[containerID] {
			continue
		}
		name, curStatus := containerID, ""
		c, err := s.repo.GetContainer(ctx, containerID, req.NodeName)
		if err != nil {
			slog.Error("get container failed", "container", containerID, "error", err)
		} else if c != nil {
			name, curStatus = c.Name, c.Status
		}
		out = append(out, buildContainerStats(containerID, name, curStatus, tl))
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })

	return &pulsev1.GetContainerStatsResponse{Containers: out}, nil
}

func buildContainerStats(id, name, currentStatus string, tl *containerTimeline) *pulsev1.ContainerStats {
	cs := &pulsev1.ContainerStats{
		ContainerId:   id,
		Name:          name,
		CurrentStatus: currentStatus,
	}
	if tl == nil {
		return cs
	}
	cs.RestartCount = tl.restartCount
	cs.StatusTransitions = tl.statusTransitions
	for _, t := range tl.uptimeResets {
		cs.UptimeResets = append(cs.UptimeResets, timestamppb.New(t))
	}
	return cs
}

func (s *CLIService) DiffContainers(ctx context.Context, req *pulsev1.DiffContainersRequest) (*pulsev1.DiffContainersResponse, error) {
	if req.NodeName == "" {
		return nil, status.Errorf(codes.InvalidArgument, "node_name is required")
	}

	since := time.Now().Add(-time.Duration(req.SinceSeconds) * time.Second)

	before, err := s.repo.ListContainersAt(ctx, req.NodeName, since)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list containers at %s: %v", since, err)
	}
	now, _, err := s.repo.ListContainers(ctx, req.NodeName, 1000, 0)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list containers: %v", err)
	}
	events, err := s.repo.ListContainerEvents(ctx, req.NodeName, since)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list container events: %v", err)
	}
	timelines := computeTimelines(events)

	beforeByName := make(map[string]repository.Container, len(before))
	for _, c := range before {
		beforeByName[c.Name] = c
	}
	nowByName := make(map[string]repository.Container, len(now))
	for _, c := range now {
		nowByName[c.Name] = c
	}

	resp := &pulsev1.DiffContainersResponse{}

	for name, nc := range nowByName {
		bc, existed := beforeByName[name]
		if !existed {
			resp.Appeared = append(resp.Appeared, containerToProto(nc))
			continue
		}

		if bc.Image != nc.Image {
			resp.ChangedImage = append(resp.ChangedImage, &pulsev1.ImageChange{
				Name:           name,
				OldContainerId: bc.ContainerID,
				OldImage:       bc.Image,
				NewContainerId: nc.ContainerID,
				NewImage:       nc.Image,
			})
		}

		recreated := bc.ContainerID != nc.ContainerID
		tl := timelines[nc.ContainerID]
		uptimeReset := tl != nil && tl.restartCount > 0
		if recreated || uptimeReset {
			resp.Restarted = append(resp.Restarted, containerToProto(nc))
		}
	}

	for name, bc := range beforeByName {
		if _, existed := nowByName[name]; !existed {
			resp.Disappeared = append(resp.Disappeared, containerToProto(bc))
		}
	}

	sortContainerInfosByName(resp.Appeared)
	sortContainerInfosByName(resp.Disappeared)
	sortContainerInfosByName(resp.Restarted)
	sort.Slice(resp.ChangedImage, func(i, j int) bool { return resp.ChangedImage[i].Name < resp.ChangedImage[j].Name })

	return resp, nil
}

func sortContainerInfosByName(cs []*pulsev1.ContainerInfo) {
	sort.Slice(cs, func(i, j int) bool { return cs[i].Name < cs[j].Name })
}
