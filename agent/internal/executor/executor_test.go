package executor

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/ipedrazas/pulse/agent/internal/docker"
	monitorv1 "github.com/ipedrazas/pulse/proto/monitor/v1"
)

// mockDockerOps implements docker.DockerOps for testing.
type mockDockerOps struct {
	stopErr     error
	startErr    error
	restartErr  error
	logsOutput  string
	logsErr     error
	inspectOut  string
	inspectErr  error
	pullErr     error
	listContain []docker.ContainerInfo
	listErr     error
	recreateErr error
}

func (m *mockDockerOps) StopContainer(_ context.Context, _ string) error {
	return m.stopErr
}
func (m *mockDockerOps) StartContainer(_ context.Context, _ string) error {
	return m.startErr
}
func (m *mockDockerOps) RestartContainer(_ context.Context, _ string) error {
	return m.restartErr
}
func (m *mockDockerOps) ContainerLogs(_ context.Context, _ string, _ string) (string, error) {
	return m.logsOutput, m.logsErr
}
func (m *mockDockerOps) InspectContainer(_ context.Context, _ string) (string, error) {
	return m.inspectOut, m.inspectErr
}
func (m *mockDockerOps) PullImage(_ context.Context, _ string) error {
	return m.pullErr
}
func (m *mockDockerOps) ListProjectContainers(_ context.Context, _ string) ([]docker.ContainerInfo, error) {
	return m.listContain, m.listErr
}
func (m *mockDockerOps) RecreateContainer(_ context.Context, _ string) error {
	return m.recreateErr
}

var noopDocker = &mockDockerOps{}

func TestRun_DisallowedAction(t *testing.T) {
	cmd := &monitorv1.Command{
		CommandId: "cmd-1",
		Action:    "compose_update",
		Target:    "mystack",
	}
	allowed := map[string]bool{} // nothing allowed

	result := Run(context.Background(), cmd, allowed, noopDocker)

	if result.Success {
		t.Error("expected failure for disallowed action")
	}
	if !strings.Contains(result.Output, "not allowed") {
		t.Errorf("expected 'not allowed' in output, got %q", result.Output)
	}
}

func TestRun_UnknownAction(t *testing.T) {
	cmd := &monitorv1.Command{
		CommandId: "cmd-2",
		Action:    "nuke_everything",
		Target:    "mystack",
	}
	allowed := map[string]bool{"nuke_everything": true}

	result := Run(context.Background(), cmd, allowed, noopDocker)

	if result.Success {
		t.Error("expected failure for unknown action")
	}
	if !strings.Contains(result.Output, "unknown action") {
		t.Errorf("expected 'unknown action' in output, got %q", result.Output)
	}
}

func TestRun_ComposeUpdate_Success(t *testing.T) {
	cmd := &monitorv1.Command{
		CommandId: "cmd-update",
		Action:    "compose_update",
		Target:    "mystack",
	}
	allowed := map[string]bool{"compose_update": true}
	mock := &mockDockerOps{
		listContain: []docker.ContainerInfo{
			{ID: "c1", Name: "web", Image: "nginx:latest"},
			{ID: "c2", Name: "api", Image: "myapp:latest"},
		},
	}

	result := Run(context.Background(), cmd, allowed, mock)

	if !result.Success {
		t.Errorf("expected success, got output: %s", result.Output)
	}
	if !strings.Contains(result.Output, "pull nginx:latest: ok") {
		t.Errorf("expected pull ok in output, got %q", result.Output)
	}
	if !strings.Contains(result.Output, "recreate web: ok") {
		t.Errorf("expected recreate ok in output, got %q", result.Output)
	}
}

func TestRun_ComposeUpdate_NoContainers(t *testing.T) {
	cmd := &monitorv1.Command{
		CommandId: "cmd-update-empty",
		Action:    "compose_update",
		Target:    "emptyproject",
	}
	allowed := map[string]bool{"compose_update": true}
	mock := &mockDockerOps{listContain: nil}

	result := Run(context.Background(), cmd, allowed, mock)

	if !result.Success {
		t.Errorf("expected success for empty project, got output: %s", result.Output)
	}
	if !strings.Contains(result.Output, "no containers found") {
		t.Errorf("expected 'no containers found' in output, got %q", result.Output)
	}
}

func TestRun_ComposeUpdate_PullError(t *testing.T) {
	cmd := &monitorv1.Command{
		CommandId: "cmd-update-pullerr",
		Action:    "compose_update",
		Target:    "mystack",
	}
	allowed := map[string]bool{"compose_update": true}
	mock := &mockDockerOps{
		listContain: []docker.ContainerInfo{
			{ID: "c1", Name: "web", Image: "nginx:latest"},
		},
		pullErr: fmt.Errorf("pull denied"),
	}

	result := Run(context.Background(), cmd, allowed, mock)

	if result.Success {
		t.Error("expected failure when pull errors")
	}
	if !strings.Contains(result.Output, "pull denied") {
		t.Errorf("expected pull error in output, got %q", result.Output)
	}
}

func TestRun_ComposeUpdate_ListError(t *testing.T) {
	cmd := &monitorv1.Command{
		CommandId: "cmd-update-listerr",
		Action:    "compose_update",
		Target:    "mystack",
	}
	allowed := map[string]bool{"compose_update": true}
	mock := &mockDockerOps{
		listErr: fmt.Errorf("daemon unreachable"),
	}

	result := Run(context.Background(), cmd, allowed, mock)

	if result.Success {
		t.Error("expected failure when list errors")
	}
	if !strings.Contains(result.Output, "daemon unreachable") {
		t.Errorf("expected list error in output, got %q", result.Output)
	}
}

func TestRun_ComposeRestart_Success(t *testing.T) {
	cmd := &monitorv1.Command{
		CommandId: "cmd-restart-compose",
		Action:    "compose_restart",
		Target:    "mystack",
	}
	allowed := map[string]bool{"compose_restart": true}
	mock := &mockDockerOps{
		listContain: []docker.ContainerInfo{
			{ID: "c1", Name: "web", Image: "nginx:latest"},
			{ID: "c2", Name: "api", Image: "myapp:latest"},
		},
	}

	result := Run(context.Background(), cmd, allowed, mock)

	if !result.Success {
		t.Errorf("expected success, got output: %s", result.Output)
	}
	if !strings.Contains(result.Output, "restart web: ok") {
		t.Errorf("expected restart ok in output, got %q", result.Output)
	}
}

func TestRun_ContainerStop_Success(t *testing.T) {
	cmd := &monitorv1.Command{
		CommandId: "cmd-stop",
		Action:    "container_stop",
		Target:    "abc123",
	}
	allowed := map[string]bool{"container_stop": true}
	mock := &mockDockerOps{}

	result := Run(context.Background(), cmd, allowed, mock)

	if !result.Success {
		t.Errorf("expected success, got output: %s", result.Output)
	}
	if !strings.Contains(result.Output, "ok") {
		t.Errorf("expected 'ok' in output, got %q", result.Output)
	}
}

func TestRun_ContainerStop_Error(t *testing.T) {
	cmd := &monitorv1.Command{
		CommandId: "cmd-stop-err",
		Action:    "container_stop",
		Target:    "abc123",
	}
	allowed := map[string]bool{"container_stop": true}
	mock := &mockDockerOps{stopErr: fmt.Errorf("no such container")}

	result := Run(context.Background(), cmd, allowed, mock)

	if result.Success {
		t.Error("expected failure")
	}
	if !strings.Contains(result.Output, "no such container") {
		t.Errorf("expected error in output, got %q", result.Output)
	}
}

func TestRun_ContainerStart_Success(t *testing.T) {
	cmd := &monitorv1.Command{
		CommandId: "cmd-start",
		Action:    "container_start",
		Target:    "abc123",
	}
	allowed := map[string]bool{"container_start": true}
	mock := &mockDockerOps{}

	result := Run(context.Background(), cmd, allowed, mock)

	if !result.Success {
		t.Errorf("expected success, got output: %s", result.Output)
	}
}

func TestRun_ContainerRestart_Success(t *testing.T) {
	cmd := &monitorv1.Command{
		CommandId: "cmd-restart",
		Action:    "container_restart",
		Target:    "abc123",
	}
	allowed := map[string]bool{"container_restart": true}
	mock := &mockDockerOps{}

	result := Run(context.Background(), cmd, allowed, mock)

	if !result.Success {
		t.Errorf("expected success, got output: %s", result.Output)
	}
}

func TestRun_ContainerLogs_Success(t *testing.T) {
	cmd := &monitorv1.Command{
		CommandId: "cmd-logs",
		Action:    "container_logs",
		Target:    "abc123",
		Params:    map[string]string{"tail": "50"},
	}
	allowed := map[string]bool{"container_logs": true}
	mock := &mockDockerOps{logsOutput: "line1\nline2\nline3"}

	result := Run(context.Background(), cmd, allowed, mock)

	if !result.Success {
		t.Errorf("expected success, got output: %s", result.Output)
	}
	if !strings.Contains(result.Output, "line1") {
		t.Errorf("expected log output, got %q", result.Output)
	}
}

func TestRun_ContainerLogs_Error(t *testing.T) {
	cmd := &monitorv1.Command{
		CommandId: "cmd-logs-err",
		Action:    "container_logs",
		Target:    "abc123",
	}
	allowed := map[string]bool{"container_logs": true}
	mock := &mockDockerOps{logsErr: fmt.Errorf("container not found")}

	result := Run(context.Background(), cmd, allowed, mock)

	if result.Success {
		t.Error("expected failure")
	}
	if !strings.Contains(result.Output, "container not found") {
		t.Errorf("expected error in output, got %q", result.Output)
	}
}

func TestRun_ContainerInspect_Success(t *testing.T) {
	cmd := &monitorv1.Command{
		CommandId: "cmd-inspect",
		Action:    "container_inspect",
		Target:    "abc123",
	}
	allowed := map[string]bool{"container_inspect": true}
	mock := &mockDockerOps{inspectOut: `{"Id": "abc123", "Name": "/test"}`}

	result := Run(context.Background(), cmd, allowed, mock)

	if !result.Success {
		t.Errorf("expected success, got output: %s", result.Output)
	}
	if !strings.Contains(result.Output, "abc123") {
		t.Errorf("expected inspect JSON, got %q", result.Output)
	}
}

func TestRun_ContainerInspect_Error(t *testing.T) {
	cmd := &monitorv1.Command{
		CommandId: "cmd-inspect-err",
		Action:    "container_inspect",
		Target:    "abc123",
	}
	allowed := map[string]bool{"container_inspect": true}
	mock := &mockDockerOps{inspectErr: fmt.Errorf("no such container")}

	result := Run(context.Background(), cmd, allowed, mock)

	if result.Success {
		t.Error("expected failure")
	}
	if !strings.Contains(result.Output, "no such container") {
		t.Errorf("expected error in output, got %q", result.Output)
	}
}

func TestTruncate(t *testing.T) {
	short := "hello"
	if truncate(short, 100) != short {
		t.Error("short string should not be truncated")
	}

	long := strings.Repeat("x", 200)
	result := truncate(long, 50)
	if len(result) > 70 { // 50 + suffix
		t.Errorf("expected truncated length ~65, got %d", len(result))
	}
	if !strings.Contains(result, "truncated") {
		t.Error("expected truncation suffix")
	}
}
