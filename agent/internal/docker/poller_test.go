package docker

import (
	"testing"
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
