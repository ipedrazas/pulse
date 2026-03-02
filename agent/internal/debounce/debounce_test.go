package debounce

import (
	"testing"

	"github.com/ipedrazas/pulse/agent/internal/docker"
)

func makeInfo(id, image string, envs map[string]string, mounts []docker.MountInfo) docker.ContainerInfo {
	return docker.ContainerInfo{
		ID:     id,
		Image:  image,
		Envs:   envs,
		Mounts: mounts,
	}
}

func TestTracker_NewContainer(t *testing.T) {
	tr := NewTracker()
	info := makeInfo("abc123", "nginx:latest", nil, nil)

	if !tr.HasChanged(info) {
		t.Fatal("new container should be detected as changed")
	}
}

func TestTracker_UnchangedContainer(t *testing.T) {
	tr := NewTracker()
	info := makeInfo("abc123", "nginx:latest", map[string]string{"FOO": "bar"}, nil)

	tr.HasChanged(info) // first call — new
	tr.Commit(info)     // persist the hash

	if tr.HasChanged(info) {
		t.Fatal("unchanged container should not be detected as changed")
	}
}

func TestTracker_ChangedImage(t *testing.T) {
	tr := NewTracker()
	info := makeInfo("abc123", "nginx:1.25", nil, nil)
	tr.HasChanged(info)
	tr.Commit(info)

	info.Image = "nginx:1.26"
	if !tr.HasChanged(info) {
		t.Fatal("image change should be detected")
	}
}

func TestTracker_ChangedEnvs(t *testing.T) {
	tr := NewTracker()
	info := makeInfo("abc123", "nginx:latest", map[string]string{"A": "1"}, nil)
	tr.HasChanged(info)
	tr.Commit(info)

	info.Envs = map[string]string{"A": "2"}
	if !tr.HasChanged(info) {
		t.Fatal("env change should be detected")
	}
}

func TestTracker_ChangedMounts(t *testing.T) {
	tr := NewTracker()
	info := makeInfo("abc123", "nginx:latest", nil, []docker.MountInfo{
		{Source: "/data", Destination: "/mnt", Mode: "rw"},
	})
	tr.HasChanged(info)
	tr.Commit(info)

	info.Mounts = []docker.MountInfo{
		{Source: "/data2", Destination: "/mnt", Mode: "ro"},
	}
	if !tr.HasChanged(info) {
		t.Fatal("mount change should be detected")
	}
}

func TestTracker_EnvOrderInsensitive(t *testing.T) {
	tr := NewTracker()
	info1 := makeInfo("abc123", "nginx:latest", map[string]string{"B": "2", "A": "1"}, nil)
	tr.HasChanged(info1)
	tr.Commit(info1)

	info2 := makeInfo("abc123", "nginx:latest", map[string]string{"A": "1", "B": "2"}, nil)
	if tr.HasChanged(info2) {
		t.Fatal("env order should not matter")
	}
}

func TestTracker_Prune(t *testing.T) {
	tr := NewTracker()
	for _, id := range []string{"a", "b", "c"} {
		info := makeInfo(id, "img", nil, nil)
		tr.HasChanged(info)
		tr.Commit(info)
	}

	// Only "a" is still active
	tr.Prune(map[string]struct{}{"a": {}})

	// "b" should be treated as new again
	if !tr.HasChanged(makeInfo("b", "img", nil, nil)) {
		t.Fatal("pruned container should be detected as new")
	}

	// "a" should still be tracked
	if tr.HasChanged(makeInfo("a", "img", nil, nil)) {
		t.Fatal("non-pruned container should still be tracked")
	}
}

func TestTracker_HasChanged_WithoutCommit(t *testing.T) {
	tr := NewTracker()
	info := makeInfo("abc123", "nginx:latest", nil, nil)

	// Without Commit, HasChanged must keep returning true
	for i := range 3 {
		if !tr.HasChanged(info) {
			t.Fatalf("call %d: HasChanged should return true without Commit", i+1)
		}
	}
}

func TestTracker_Reset(t *testing.T) {
	tr := NewTracker()
	info := makeInfo("abc123", "nginx:latest", nil, nil)
	tr.HasChanged(info)
	tr.Commit(info)

	// Confirm it's now tracked
	if tr.HasChanged(info) {
		t.Fatal("committed container should not be detected as changed")
	}

	tr.Reset()

	// After reset every container should appear new
	if !tr.HasChanged(info) {
		t.Fatal("after Reset, container should be detected as changed")
	}
}
