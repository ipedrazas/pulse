package debounce

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/ipedrazas/pulse/agent/internal/docker"
)

// Tracker keeps SHA256 hashes of container metadata to detect changes.
type Tracker struct {
	hashes map[string]string // container_id -> hash
}

// NewTracker creates an empty debounce tracker.
func NewTracker() *Tracker {
	return &Tracker{hashes: make(map[string]string)}
}

// HasChanged returns true if the container's metadata hash differs from the
// last seen value, or if the container is new. It updates the stored hash.
func (t *Tracker) HasChanged(info docker.ContainerInfo) bool {
	h := computeHash(info)
	prev, exists := t.hashes[info.ID]
	t.hashes[info.ID] = h
	return !exists || prev != h
}

// Prune removes tracked containers that are no longer in the given set.
func (t *Tracker) Prune(activeIDs map[string]struct{}) {
	for id := range t.hashes {
		if _, ok := activeIDs[id]; !ok {
			delete(t.hashes, id)
		}
	}
}

func computeHash(info docker.ContainerInfo) string {
	h := sha256.New()
	h.Write([]byte(info.Image))
	h.Write([]byte(docker.SortedEnvString(info.Envs)))
	h.Write([]byte(mountsString(info.Mounts)))
	return hex.EncodeToString(h.Sum(nil))
}

func mountsString(mounts []docker.MountInfo) string {
	parts := make([]string, len(mounts))
	for i, m := range mounts {
		parts[i] = fmt.Sprintf("%s:%s:%s", m.Source, m.Destination, m.Mode)
	}
	sort.Strings(parts)
	return strings.Join(parts, "\n")
}
