package grpcserver

import (
	"encoding/json"
	"testing"

	"github.com/ipedrazas/pulse/api/internal/repository"
	pulsev1 "github.com/ipedrazas/pulse/proto/gen/pulse/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- protoToContainer ---

func TestProtoToContainer_FullFields(t *testing.T) {
	ci := &pulsev1.ContainerInfo{
		Id:     "abc123",
		Name:   "web",
		Image:  "nginx:latest",
		Status: "running",
		EnvVars: map[string]string{
			"ENV": "prod",
		},
		Mounts: []string{"/data:/data"},
		Labels: map[string]string{
			"app": "web",
		},
		Ports: []*pulsev1.PortMapping{
			{HostIp: "0.0.0.0", HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
		},
		UptimeSeconds:  3600,
		ComposeProject: "myproject",
		Command:        "nginx -g daemon off",
	}

	c := protoToContainer(ci, "node-1")

	assert.Equal(t, "abc123", c.ContainerID)
	assert.Equal(t, "node-1", c.AgentName)
	assert.Equal(t, "web", c.Name)
	assert.Equal(t, "nginx:latest", c.Image)
	assert.Equal(t, "running", c.Status)
	assert.Equal(t, map[string]string{"ENV": "prod"}, c.EnvVars)
	assert.Equal(t, []string{"/data:/data"}, c.Mounts)
	assert.Equal(t, map[string]string{"app": "web"}, c.Labels)
	assert.Len(t, c.Ports, 1)
	assert.Equal(t, uint32(80), c.Ports[0].ContainerPort)
	assert.Equal(t, int64(3600), c.UptimeSeconds)
	assert.Equal(t, "myproject", c.ComposeProject)
	assert.Equal(t, "nginx -g daemon off", c.Command)
}

func TestProtoToContainer_NilMaps(t *testing.T) {
	ci := &pulsev1.ContainerInfo{
		Id:      "def456",
		Name:    "api",
		EnvVars: nil,
		Labels:  nil,
	}

	c := protoToContainer(ci, "node-2")

	assert.NotNil(t, c.EnvVars)
	assert.NotNil(t, c.Labels)
	assert.Empty(t, c.EnvVars)
	assert.Empty(t, c.Labels)
}

func TestProtoToContainer_WithPorts(t *testing.T) {
	ci := &pulsev1.ContainerInfo{
		Id: "ghi789",
		Ports: []*pulsev1.PortMapping{
			{HostIp: "0.0.0.0", HostPort: 443, ContainerPort: 443, Protocol: "tcp"},
			{HostIp: "", HostPort: 53, ContainerPort: 53, Protocol: "udp"},
		},
	}

	c := protoToContainer(ci, "node-3")

	assert.Len(t, c.Ports, 2)
	assert.Equal(t, "tcp", c.Ports[0].Protocol)
	assert.Equal(t, "udp", c.Ports[1].Protocol)
	assert.Equal(t, uint32(53), c.Ports[1].ContainerPort)
}

// --- commandToProto ---

func TestCommandToProto_RunContainer(t *testing.T) {
	payload, _ := json.Marshal(&pulsev1.RunContainer{Image: "nginx", Name: "web"})
	cmd := repository.Command{ID: "cmd-1", Type: "run_container", Payload: payload}

	sc, err := commandToProto(cmd)
	require.NoError(t, err)
	assert.Equal(t, "cmd-1", sc.CommandId)
	assert.NotNil(t, sc.GetRunContainer())
	assert.Equal(t, "nginx", sc.GetRunContainer().Image)
}

func TestCommandToProto_StopContainer(t *testing.T) {
	payload, _ := json.Marshal(&pulsev1.StopContainer{ContainerId: "c1"})
	cmd := repository.Command{ID: "cmd-2", Type: "stop_container", Payload: payload}

	sc, err := commandToProto(cmd)
	require.NoError(t, err)
	assert.NotNil(t, sc.GetStopContainer())
}

func TestCommandToProto_PullImage(t *testing.T) {
	payload, _ := json.Marshal(&pulsev1.PullImage{Image: "redis:7"})
	cmd := repository.Command{ID: "cmd-3", Type: "pull_image", Payload: payload}

	sc, err := commandToProto(cmd)
	require.NoError(t, err)
	assert.NotNil(t, sc.GetPullImage())
}

func TestCommandToProto_ComposeUp(t *testing.T) {
	payload, _ := json.Marshal(&pulsev1.ComposeUp{ProjectDir: "/app"})
	cmd := repository.Command{ID: "cmd-4", Type: "compose_up", Payload: payload}

	sc, err := commandToProto(cmd)
	require.NoError(t, err)
	assert.NotNil(t, sc.GetComposeUp())
}

func TestCommandToProto_UnknownType(t *testing.T) {
	cmd := repository.Command{ID: "cmd-5", Type: "unknown_cmd", Payload: []byte("{}")}

	sc, err := commandToProto(cmd)
	// Unknown types don't set a payload but don't error either
	require.NoError(t, err)
	assert.Equal(t, "cmd-5", sc.CommandId)
}

func TestCommandToProto_InvalidJSON(t *testing.T) {
	cmd := repository.Command{ID: "cmd-6", Type: "run_container", Payload: []byte("not json")}

	_, err := commandToProto(cmd)
	assert.Error(t, err)
}

// --- containerToProto ---

func TestContainerToProto_FullFields(t *testing.T) {
	c := repository.Container{
		ContainerID:    "abc",
		AgentName:      "node-1",
		Name:           "web",
		Image:          "nginx",
		Status:         "running",
		EnvVars:        map[string]string{"K": "V"},
		Mounts:         []string{"/a:/b"},
		Labels:         map[string]string{"app": "web"},
		Ports:          []repository.Port{{HostIP: "0.0.0.0", HostPort: 80, ContainerPort: 80, Protocol: "tcp"}},
		UptimeSeconds:  100,
		ComposeProject: "proj",
		Command:        "nginx",
	}

	ci := containerToProto(c)

	assert.Equal(t, "abc", ci.Id)
	assert.Equal(t, "web", ci.Name)
	assert.Equal(t, "nginx", ci.Image)
	assert.Equal(t, "running", ci.Status)
	assert.Equal(t, map[string]string{"K": "V"}, ci.EnvVars)
	assert.Equal(t, int64(100), ci.UptimeSeconds)
	assert.Len(t, ci.Ports, 1)
	assert.Equal(t, uint32(80), ci.Ports[0].ContainerPort)
}

func TestContainerToProto_EmptyPorts(t *testing.T) {
	c := repository.Container{ContainerID: "x", Ports: nil}
	ci := containerToProto(c)
	assert.Empty(t, ci.Ports)
}

func TestContainerToProto_MultiplePorts(t *testing.T) {
	c := repository.Container{
		ContainerID: "y",
		Ports: []repository.Port{
			{HostPort: 80, ContainerPort: 80, Protocol: "tcp"},
			{HostPort: 443, ContainerPort: 443, Protocol: "tcp"},
			{HostPort: 53, ContainerPort: 53, Protocol: "udp"},
		},
	}
	ci := containerToProto(c)
	assert.Len(t, ci.Ports, 3)
}

// --- marshalCommand ---

func TestMarshalCommand_RunContainer(t *testing.T) {
	req := &pulsev1.SendCommandRequest{
		NodeName: "node-1",
		Command: &pulsev1.SendCommandRequest_RunContainer{
			RunContainer: &pulsev1.RunContainer{Image: "nginx"},
		},
	}
	typ, data, err := marshalCommand(req)
	require.NoError(t, err)
	assert.Equal(t, "run_container", typ)
	assert.Contains(t, string(data), "nginx")
}

func TestMarshalCommand_StopContainer(t *testing.T) {
	req := &pulsev1.SendCommandRequest{
		Command: &pulsev1.SendCommandRequest_StopContainer{
			StopContainer: &pulsev1.StopContainer{ContainerId: "c1"},
		},
	}
	typ, _, err := marshalCommand(req)
	require.NoError(t, err)
	assert.Equal(t, "stop_container", typ)
}

func TestMarshalCommand_NilCommand(t *testing.T) {
	req := &pulsev1.SendCommandRequest{Command: nil}
	_, _, err := marshalCommand(req)
	assert.Error(t, err)
}

// --- StreamRegistry ---

func TestStreamRegistry_SetGetRemove(t *testing.T) {
	r := NewStreamRegistry()

	// Get missing key
	_, ok := r.Get("node-1")
	assert.False(t, ok)

	// Set and get
	r.Set("node-1", nil)
	_, ok = r.Get("node-1")
	assert.True(t, ok)

	// Remove
	r.Remove("node-1")
	_, ok = r.Get("node-1")
	assert.False(t, ok)
}

func TestStreamRegistry_GetMissingKey(t *testing.T) {
	r := NewStreamRegistry()
	_, ok := r.Get("nonexistent")
	assert.False(t, ok)
}
