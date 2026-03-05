package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

// DockerOps defines container-level operations that the executor can invoke.
type DockerOps interface {
	StopContainer(ctx context.Context, containerID string) error
	StartContainer(ctx context.Context, containerID string) error
	RestartContainer(ctx context.Context, containerID string) error
	ContainerLogs(ctx context.Context, containerID string, tail string) (string, error)
	InspectContainer(ctx context.Context, containerID string) (string, error)
	PullImage(ctx context.Context, image string) error
	ListProjectContainers(ctx context.Context, project string) ([]ContainerInfo, error)
	RecreateContainer(ctx context.Context, containerID string) error
}

// ContainerInfo holds the extracted data from a running Docker container.
type ContainerInfo struct {
	ID            string
	Name          string
	Image         string
	Envs          map[string]string
	Labels        map[string]string
	Mounts        []MountInfo
	Status        string
	UptimeSeconds int64
}

// MountInfo represents a container mount point.
type MountInfo struct {
	Source      string
	Destination string
	Mode        string
}

// Poller lists running containers from the Docker daemon.
type Poller struct {
	client client.APIClient
}

// NewPoller creates a Poller using the default Docker client (unix socket).
func NewPoller() (*Poller, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &Poller{client: cli}, nil
}

// Close releases the Docker client resources.
func (p *Poller) Close() error {
	return p.client.Close()
}

// StopContainer stops a container by ID.
func (p *Poller) StopContainer(ctx context.Context, containerID string) error {
	return p.client.ContainerStop(ctx, containerID, container.StopOptions{})
}

// StartContainer starts a stopped container by ID.
func (p *Poller) StartContainer(ctx context.Context, containerID string) error {
	return p.client.ContainerStart(ctx, containerID, container.StartOptions{})
}

// RestartContainer restarts a container by ID.
func (p *Poller) RestartContainer(ctx context.Context, containerID string) error {
	return p.client.ContainerRestart(ctx, containerID, container.StopOptions{})
}

// ContainerLogs fetches the last `tail` lines of logs from a container.
func (p *Poller) ContainerLogs(ctx context.Context, containerID string, tail string) (string, error) {
	if tail == "" {
		tail = "100"
	}
	reader, err := p.client.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tail,
	})
	if err != nil {
		return "", err
	}
	defer reader.Close()

	// Docker log stream uses a multiplexed format with 8-byte headers.
	// Read raw bytes and strip the headers for clean output.
	raw, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("reading logs: %w", err)
	}

	return stripDockerLogHeaders(raw), nil
}

// stripDockerLogHeaders removes the 8-byte multiplexed stream headers
// that Docker prepends to each log line.
func stripDockerLogHeaders(raw []byte) string {
	var b strings.Builder
	for len(raw) >= 8 {
		// bytes 4-7 are big-endian payload size
		size := int(raw[4])<<24 | int(raw[5])<<16 | int(raw[6])<<8 | int(raw[7])
		raw = raw[8:]
		if size > len(raw) {
			size = len(raw)
		}
		b.Write(raw[:size])
		raw = raw[size:]
	}
	// If there's leftover data without a header, include it as-is (e.g. TTY mode).
	if len(raw) > 0 {
		b.Write(raw)
	}
	return b.String()
}

// InspectContainer returns the full Docker inspect JSON for a container.
func (p *Poller) InspectContainer(ctx context.Context, containerID string) (string, error) {
	detail, err := p.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", err
	}
	data, err := json.MarshalIndent(detail, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling inspect: %w", err)
	}
	return string(data), nil
}

// PullImage pulls a Docker image, draining the response reader to completion.
func (p *Poller) PullImage(ctx context.Context, img string) error {
	reader, err := p.client.ImagePull(ctx, img, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pulling image %s: %w", img, err)
	}
	defer reader.Close()
	// Docker won't finish pulling until the reader is fully consumed.
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return fmt.Errorf("reading pull response for %s: %w", img, err)
	}
	return nil
}

// ListProjectContainers returns all containers belonging to a Docker Compose project.
func (p *Poller) ListProjectContainers(ctx context.Context, project string) ([]ContainerInfo, error) {
	f := filters.NewArgs(filters.Arg("label", "com.docker.compose.project="+project))
	containers, err := p.client.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return nil, fmt.Errorf("listing containers for project %s: %w", project, err)
	}

	results := make([]ContainerInfo, 0, len(containers))
	for _, c := range containers {
		info, err := p.inspect(ctx, c)
		if err != nil {
			continue
		}
		results = append(results, info)
	}
	return results, nil
}

// RecreateContainer stops, removes, and recreates a container with the same
// configuration. Named volumes are preserved (RemoveVolumes is not set).
func (p *Poller) RecreateContainer(ctx context.Context, containerID string) error {
	// 1. Inspect to capture current config.
	detail, err := p.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return fmt.Errorf("inspecting container %s: %w", containerID, err)
	}

	name := strings.TrimPrefix(detail.Name, "/")

	// 2. Stop the container (ignore "not running" errors).
	if err := p.client.ContainerStop(ctx, containerID, container.StopOptions{}); err != nil {
		slog.Warn("stop before recreate failed (may already be stopped)", "container", name, "error", err)
	}

	// 3. Remove the old container (Force handles the case where stop didn't finish cleanly).
	if err := p.client.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("removing container %s: %w", name, err)
	}

	// 4. Prepare networking — first network goes into ContainerCreate,
	//    additional networks are connected after create.
	var networkingConfig *network.NetworkingConfig
	var extraNetworks []string

	if detail.NetworkSettings != nil && len(detail.NetworkSettings.Networks) > 0 {
		first := true
		for netName := range detail.NetworkSettings.Networks {
			if first {
				networkingConfig = &network.NetworkingConfig{
					EndpointsConfig: map[string]*network.EndpointSettings{
						netName: {},
					},
				}
				first = false
			} else {
				extraNetworks = append(extraNetworks, netName)
			}
		}
	}

	// 5. Create a new container with the same config.
	resp, err := p.client.ContainerCreate(ctx, detail.Config, detail.HostConfig, networkingConfig, nil, name)
	if err != nil {
		return fmt.Errorf("creating container %s: %w", name, err)
	}

	// 6. Connect additional networks.
	for _, netName := range extraNetworks {
		if err := p.client.NetworkConnect(ctx, netName, resp.ID, &network.EndpointSettings{}); err != nil {
			slog.Warn("failed to connect extra network", "container", name, "network", netName, "error", err)
		}
	}

	// 7. Start the new container.
	if err := p.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("starting container %s: %w", name, err)
	}

	slog.Info("container recreated", "name", name, "old_id", containerID, "new_id", resp.ID)
	return nil
}

// Poll returns info for all running containers.
func (p *Poller) Poll(ctx context.Context) ([]ContainerInfo, error) {
	containers, err := p.client.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return nil, err
	}

	results := make([]ContainerInfo, 0, len(containers))
	for _, c := range containers {
		info, err := p.inspect(ctx, c)
		if err != nil {
			continue // container may have been removed between list and inspect
		}
		results = append(results, info)
	}
	return results, nil
}

func (p *Poller) inspect(ctx context.Context, c container.Summary) (ContainerInfo, error) {
	detail, err := p.client.ContainerInspect(ctx, c.ID)
	if err != nil {
		return ContainerInfo{}, err
	}

	envs := parseEnvs(detail.Config.Env)
	labels := detail.Config.Labels
	mounts := parseMounts(detail.Mounts)

	var uptime int64
	if detail.State != nil && detail.State.StartedAt != "" {
		if started, err := time.Parse(time.RFC3339Nano, detail.State.StartedAt); err == nil {
			uptime = int64(time.Since(started).Seconds())
		}
	}

	name := strings.TrimPrefix(detail.Name, "/")

	return ContainerInfo{
		ID:            c.ID,
		Name:          name,
		Image:         detail.Config.Image,
		Envs:          envs,
		Labels:        labels,
		Mounts:        mounts,
		Status:        detail.State.Status,
		UptimeSeconds: uptime,
	}, nil
}

// parseEnvs converts Docker's KEY=VALUE slice into a map.
func parseEnvs(envSlice []string) map[string]string {
	m := make(map[string]string, len(envSlice))
	for _, e := range envSlice {
		k, v, _ := strings.Cut(e, "=")
		m[k] = v
	}
	return m
}

func parseMounts(mounts []container.MountPoint) []MountInfo {
	result := make([]MountInfo, len(mounts))
	for i, m := range mounts {
		result[i] = MountInfo{
			Source:      m.Source,
			Destination: m.Destination,
			Mode:        m.Mode,
		}
	}
	return result
}

// SortedMapString returns a deterministic string representation of a string map for hashing.
func SortedMapString(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(m[k])
		b.WriteByte('\n')
	}
	return b.String()
}

// SortedEnvString returns a deterministic string representation of envs for hashing.
func SortedEnvString(envs map[string]string) string {
	keys := make([]string, 0, len(envs))
	for k := range envs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(envs[k])
		b.WriteByte('\n')
	}
	return b.String()
}
