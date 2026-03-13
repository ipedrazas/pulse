package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// executeCmd creates a fresh root command, sets args, and executes it.
// Returns stdout, stderr, and any error.
func executeCmd(args ...string) (string, string, error) {
	root := NewRootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

// --- version ---

func TestVersionCmd(t *testing.T) {
	// version command uses fmt.Printf (writes to os.Stdout), so we just
	// verify it runs without error.
	_, _, err := executeCmd("version")
	if err != nil {
		t.Fatalf("version command failed: %v", err)
	}
}

// --- run: flag validation ---

func TestRunCmd_MissingNode(t *testing.T) {
	_, _, err := executeCmd("run", "--image", "nginx")
	if err == nil {
		t.Fatal("expected error for missing --node")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("--node is required")) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCmd_MissingImage(t *testing.T) {
	_, _, err := executeCmd("run", "--node", "n1")
	if err == nil {
		t.Fatal("expected error for missing --image")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("--image is required")) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCmd_InvalidEnvVar(t *testing.T) {
	_, _, err := executeCmd("run", "--node", "n1", "--image", "nginx", "-e", "NOEQUALS")
	if err == nil {
		t.Fatal("expected error for invalid env var")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("invalid env var")) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCmd_InvalidPort(t *testing.T) {
	_, _, err := executeCmd("run", "--node", "n1", "--image", "nginx", "-p", "badport")
	if err == nil {
		t.Fatal("expected error for invalid port")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("invalid port mapping")) {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- stop: flag validation ---

func TestStopCmd_MissingNode(t *testing.T) {
	_, _, err := executeCmd("stop", "container123")
	if err == nil {
		t.Fatal("expected error for missing --node")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("--node is required")) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStopCmd_MissingArg(t *testing.T) {
	_, _, err := executeCmd("stop")
	if err == nil {
		t.Fatal("expected error for missing container arg")
	}
}

// --- pull: flag validation ---

func TestPullCmd_MissingNode(t *testing.T) {
	_, _, err := executeCmd("pull", "nginx")
	if err == nil {
		t.Fatal("expected error for missing --node")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("--node is required")) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPullCmd_MissingArg(t *testing.T) {
	_, _, err := executeCmd("pull", "--node", "n1")
	if err == nil {
		t.Fatal("expected error for missing image arg")
	}
}

// --- logs: flag validation ---

func TestLogsCmd_MissingNode(t *testing.T) {
	_, _, err := executeCmd("logs", "container123")
	if err == nil {
		t.Fatal("expected error for missing --node")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("--node is required")) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLogsCmd_MissingArg(t *testing.T) {
	_, _, err := executeCmd("logs")
	if err == nil {
		t.Fatal("expected error for missing container arg")
	}
}

// --- up: flag validation ---

func TestUpCmd_MissingNode(t *testing.T) {
	_, _, err := executeCmd("up")
	if err == nil {
		t.Fatal("expected error for missing --node")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("--node is required")) {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- send: flag validation ---

func TestSendCmd_MissingNode(t *testing.T) {
	_, _, err := executeCmd("send", "--file", "foo.txt")
	if err == nil {
		t.Fatal("expected error for missing --node")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("--node is required")) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSendCmd_MissingFile(t *testing.T) {
	_, _, err := executeCmd("send", "--node", "n1")
	if err == nil {
		t.Fatal("expected error for missing --file")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("--file is required")) {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- nodes rm: validation ---

func TestNodesRmCmd_MissingArg(t *testing.T) {
	_, _, err := executeCmd("nodes", "rm")
	if err == nil {
		t.Fatal("expected error for missing node name arg")
	}
}

// --- api-addr resolution ---

func TestAPIAddr_Default(t *testing.T) {
	// Unset env var to test default
	t.Setenv("PULSE_API_ADDR", "")

	root := NewRootCmd()
	root.SetArgs([]string{"version"})
	root.SetOut(&bytes.Buffer{})
	_ = root.Execute()

	if apiAddr != "localhost:9090" {
		t.Errorf("default apiAddr = %q, want localhost:9090", apiAddr)
	}
}

func TestAPIAddr_EnvVar(t *testing.T) {
	t.Setenv("PULSE_API_ADDR", "envhost:5555")

	root := NewRootCmd()
	root.SetArgs([]string{"version"})
	root.SetOut(&bytes.Buffer{})
	_ = root.Execute()

	if apiAddr != "envhost:5555" {
		t.Errorf("apiAddr = %q, want envhost:5555", apiAddr)
	}
}

func TestAPIAddr_FlagOverridesEnv(t *testing.T) {
	t.Setenv("PULSE_API_ADDR", "envhost:5555")

	root := NewRootCmd()
	root.SetArgs([]string{"--api-addr", "flaghost:7777", "version"})
	root.SetOut(&bytes.Buffer{})
	_ = root.Execute()

	if apiAddr != "flaghost:7777" {
		t.Errorf("apiAddr = %q, want flaghost:7777", apiAddr)
	}
}

func TestAPIAddr_ConfigFile(t *testing.T) {
	t.Setenv("PULSE_API_ADDR", "")

	// Create a temp config file and override HOME
	dir := t.TempDir()
	pulseDir := filepath.Join(dir, ".pulse")
	if err := os.MkdirAll(pulseDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pulseDir, "config.yaml"), []byte("api-addr: filehost:3333\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", dir)

	root := NewRootCmd()
	root.SetArgs([]string{"version"})
	root.SetOut(&bytes.Buffer{})
	_ = root.Execute()

	if apiAddr != "filehost:3333" {
		t.Errorf("apiAddr = %q, want filehost:3333", apiAddr)
	}
}

// --- verbose flag ---

func TestVerboseFlag_Accepted(t *testing.T) {
	// --verbose is a valid global flag and should not error.
	_, _, err := executeCmd("--verbose", "version")
	if err != nil {
		t.Fatalf("version with --verbose failed: %v", err)
	}
}

// --- send: file error ---

func TestSendCmd_NonexistentFile(t *testing.T) {
	_, _, err := executeCmd("send", "--node", "n1", "--file", "/nonexistent/path/file.txt")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("read file")) {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- run: multiple invalid flags ---

func TestRunCmd_InvalidPortNonNumericHost(t *testing.T) {
	_, _, err := executeCmd("run", "--node", "n1", "--image", "nginx", "-p", "abc:80")
	if err == nil {
		t.Fatal("expected error for non-numeric host port")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("invalid port mapping")) {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCmd_EmptyEnvKey(t *testing.T) {
	_, _, err := executeCmd("run", "--node", "n1", "--image", "nginx", "-e", "=value")
	if err == nil {
		t.Fatal("expected error for empty env key")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("invalid env var")) {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- completion ---

func TestCompletionCmd_Bash(t *testing.T) {
	root := NewRootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"completion", "bash"})
	if err := root.Execute(); err != nil {
		t.Fatalf("completion bash failed: %v", err)
	}
}

func TestCompletionCmd_InvalidShell(t *testing.T) {
	_, _, err := executeCmd("completion", "powershell")
	if err == nil {
		t.Fatal("expected error for invalid shell")
	}
}

func TestCompletionCmd_MissingArg(t *testing.T) {
	_, _, err := executeCmd("completion")
	if err == nil {
		t.Fatal("expected error for missing shell arg")
	}
}

// --- subcommand structure ---

func TestNodesLsCmd_IsRegistered(t *testing.T) {
	root := NewRootCmd()
	nodesCmd, _, _ := root.Find([]string{"nodes", "ls"})
	if nodesCmd == nil || nodesCmd.Name() != "ls" {
		t.Fatal("nodes ls subcommand not found")
	}
}

func TestUnknownSubcommand(t *testing.T) {
	_, _, err := executeCmd("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
}

// --- global flags ---

func TestOutputFlag_Default(t *testing.T) {
	root := NewRootCmd()
	root.SetArgs([]string{"version"})
	root.SetOut(&bytes.Buffer{})
	_ = root.Execute()

	if output != "table" {
		t.Errorf("default output = %q, want table", output)
	}
}

func TestOutputFlag_Json(t *testing.T) {
	root := NewRootCmd()
	root.SetArgs([]string{"-o", "json", "version"})
	root.SetOut(&bytes.Buffer{})
	_ = root.Execute()

	if output != "json" {
		t.Errorf("output = %q, want json", output)
	}
}
