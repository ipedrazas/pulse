package docker

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

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
