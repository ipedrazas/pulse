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
// last seen value, or if the container is new. It does NOT store the hash;
// call Commit after a successful sync to persist the new hash.
func (t *Tracker) HasChanged(info docker.ContainerInfo) bool {
	h := computeHash(info)
	prev, exists := t.hashes[info.ID]
	return !exists || prev != h
}

// Commit stores the current hash for the container. Call this after a
// successful metadata sync so that subsequent HasChanged calls see the
// up-to-date value.
func (t *Tracker) Commit(info docker.ContainerInfo) {
	t.hashes[info.ID] = computeHash(info)
}

// Reset clears all stored hashes, forcing every container to be treated as
// new on the next poll. Useful for periodic metadata resync.
func (t *Tracker) Reset() {
	clear(t.hashes)
}

// Prune removes tracked containers that are no longer in the given set and
// returns the IDs that were removed.
func (t *Tracker) Prune(activeIDs map[string]struct{}) []string {
	var removed []string
	for id := range t.hashes {
		if _, ok := activeIDs[id]; !ok {
			removed = append(removed, id)
			delete(t.hashes, id)
		}
	}
	return removed
}

func computeHash(info docker.ContainerInfo) string {
	h := sha256.New()
	h.Write([]byte(info.Image))
	h.Write([]byte(docker.SortedEnvString(info.Envs)))
	h.Write([]byte(docker.SortedMapString(info.Labels)))
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
