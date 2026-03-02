package docker

import (
	"testing"

	"github.com/docker/docker/api/types/container"
)

func TestParseEnvs(t *testing.T) {
	input := []string{"FOO=bar", "PATH=/usr/bin", "EMPTY=", "NOEQUAL"}
	result := parseEnvs(input)

	tests := []struct {
		key, want string
	}{
		{"FOO", "bar"},
		{"PATH", "/usr/bin"},
		{"EMPTY", ""},
		{"NOEQUAL", ""},
	}

	for _, tt := range tests {
		if got := result[tt.key]; got != tt.want {
			t.Errorf("parseEnvs[%s] = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestParseEnvs_Empty(t *testing.T) {
	result := parseEnvs(nil)
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestParseMounts(t *testing.T) {
	input := []container.MountPoint{
		{Source: "/host/data", Destination: "/data", Mode: "rw"},
		{Source: "/host/config", Destination: "/config", Mode: "ro"},
	}

	result := parseMounts(input)

	if len(result) != 2 {
		t.Fatalf("expected 2 mounts, got %d", len(result))
	}

	if result[0].Source != "/host/data" || result[0].Destination != "/data" || result[0].Mode != "rw" {
		t.Errorf("mount[0] = %+v, unexpected", result[0])
	}
	if result[1].Source != "/host/config" || result[1].Destination != "/config" || result[1].Mode != "ro" {
		t.Errorf("mount[1] = %+v, unexpected", result[1])
	}
}

func TestParseMounts_Empty(t *testing.T) {
	result := parseMounts(nil)
	if len(result) != 0 {
		t.Errorf("expected empty slice, got %d entries", len(result))
	}
}

func TestSortedEnvString_Deterministic(t *testing.T) {
	envs := map[string]string{"Z": "3", "A": "1", "M": "2"}

	s1 := SortedEnvString(envs)
	s2 := SortedEnvString(envs)

	if s1 != s2 {
		t.Fatal("SortedEnvString should be deterministic")
	}

	want := "A=1\nM=2\nZ=3\n"
	if s1 != want {
		t.Errorf("SortedEnvString = %q, want %q", s1, want)
	}
}

func TestSortedEnvString_Empty(t *testing.T) {
	if got := SortedEnvString(nil); got != "" {
		t.Errorf("SortedEnvString(nil) = %q, want empty", got)
	}
}

func TestSortedEnvString_SingleEntry(t *testing.T) {
	envs := map[string]string{"KEY": "val"}
	want := "KEY=val\n"
	if got := SortedEnvString(envs); got != want {
		t.Errorf("SortedEnvString = %q, want %q", got, want)
	}
}
