package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// DockerOps defines container-level operations that the executor can invoke.
type DockerOps interface {
	StopContainer(ctx context.Context, containerID string) error
	StartContainer(ctx context.Context, containerID string) error
	RestartContainer(ctx context.Context, containerID string) error
	ContainerLogs(ctx context.Context, containerID string, tail string) (string, error)
	InspectContainer(ctx context.Context, containerID string) (string, error)
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
