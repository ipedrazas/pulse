package alerts

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNotifier_EmptyURL(t *testing.T) {
	n := NewNotifier("")
	assert.Nil(t, n)
}

func TestNewNotifier_WithURL(t *testing.T) {
	n := NewNotifier("http://example.com/webhook")
	assert.NotNil(t, n)
}

func TestNilNotifier_AgentOnline(t *testing.T) {
	var n *Notifier
	assert.NotPanics(t, func() {
		n.AgentOnline("node-1")
	})
}

func TestNilNotifier_AgentOffline(t *testing.T) {
	var n *Notifier
	assert.NotPanics(t, func() {
		n.AgentOffline("node-1")
	})
}

func TestAgentOnline_SendsPayload(t *testing.T) {
	var mu sync.Mutex
	var received map[string]string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &received)
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer srv.Close()

	n := NewNotifier(srv.URL)
	n.AgentOnline("node-1")

	mu.Lock()
	defer mu.Unlock()
	require.NotNil(t, received)
	assert.Contains(t, received["text"], "node-1")
	assert.Contains(t, received["text"], "online")
}

func TestAgentOffline_SendsPayload(t *testing.T) {
	var mu sync.Mutex
	var received map[string]string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &received)
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer srv.Close()

	n := NewNotifier(srv.URL)
	n.AgentOffline("node-1")

	mu.Lock()
	defer mu.Unlock()
	require.NotNil(t, received)
	assert.Contains(t, received["text"], "node-1")
	assert.Contains(t, received["text"], "offline")
}

func TestWebhook_ServerError_DoesNotPanic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	n := NewNotifier(srv.URL)
	assert.NotPanics(t, func() {
		n.AgentOnline("node-1")
	})
}
