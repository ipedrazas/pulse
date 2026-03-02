package alerts

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestNewNotifier_NilWhenURLEmpty(t *testing.T) {
	n := NewNotifier("", nil)
	if n != nil {
		t.Fatal("expected nil notifier for empty URL")
	}
}

func TestNewNotifier_ReturnsNotifier(t *testing.T) {
	n := NewNotifier("http://example.com/hook", nil)
	if n == nil {
		t.Fatal("expected non-nil notifier")
	}
	if n.url != "http://example.com/hook" {
		t.Errorf("expected url http://example.com/hook, got %s", n.url)
	}
}

func TestNewNotifier_ParsesEvents(t *testing.T) {
	n := NewNotifier("http://example.com/hook", []string{EventContainerDied, EventContainerStarted})
	if n == nil {
		t.Fatal("expected non-nil notifier")
	}
	if !n.events[EventContainerDied] {
		t.Error("expected container_died in events")
	}
	if !n.events[EventContainerStarted] {
		t.Error("expected container_started in events")
	}
	if n.events[EventContainerRemoved] {
		t.Error("did not expect container_removed in events")
	}
}

func TestSend_NilNotifierIsNoop(t *testing.T) {
	var n *Notifier
	// Should not panic.
	n.Send(Event{EventType: EventContainerDied})
}

func TestSend_PostsCorrectJSON(t *testing.T) {
	var mu sync.Mutex
	var received Event
	var gotRequest bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		gotRequest = true

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("failed to decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := NewNotifier(srv.URL, nil)
	n.Send(Event{
		EventType:      EventContainerDied,
		ContainerID:    "abc123",
		ContainerName:  "myapp",
		NodeName:       "node-1",
		Image:          "myapp:latest",
		ComposeProject: "mystack",
		PreviousStatus: "running",
		CurrentStatus:  "exited",
	})

	// Wait for async goroutine to complete.
	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		done := gotRequest
		mu.Unlock()
		if done {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for webhook request")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()

	if received.EventType != EventContainerDied {
		t.Errorf("expected event container_died, got %s", received.EventType)
	}
	if received.ContainerID != "abc123" {
		t.Errorf("expected container_id abc123, got %s", received.ContainerID)
	}
	if received.ContainerName != "myapp" {
		t.Errorf("expected container_name myapp, got %s", received.ContainerName)
	}
	if received.NodeName != "node-1" {
		t.Errorf("expected node_name node-1, got %s", received.NodeName)
	}
	if received.TextField == "" {
		t.Error("expected non-empty text field")
	}
	if received.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}
}

func TestSend_FiltersEvents(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Only subscribe to container_died events.
	n := NewNotifier(srv.URL, []string{EventContainerDied})

	// This should be sent.
	n.Send(Event{EventType: EventContainerDied, ContainerName: "app", NodeName: "n1"})
	// These should be filtered out.
	n.Send(Event{EventType: EventContainerStarted, ContainerName: "app", NodeName: "n1"})
	n.Send(Event{EventType: EventContainerRemoved, ContainerName: "app", NodeName: "n1"})

	// Wait for the one expected request.
	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		count := requestCount
		mu.Unlock()
		if count >= 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for webhook request")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	// Give a little time to ensure no extra requests arrive.
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if requestCount != 1 {
		t.Errorf("expected 1 request (only container_died), got %d", requestCount)
	}
}

func TestText_ContainerDied(t *testing.T) {
	e := &Event{
		EventType:      EventContainerDied,
		ContainerName:  "myapp",
		NodeName:       "node-1",
		PreviousStatus: "running",
		CurrentStatus:  "exited",
	}
	text := e.Text()
	if text != "\U0001f534 Container *myapp* on node-1 died (running → exited)" {
		t.Errorf("unexpected text: %s", text)
	}
}

func TestText_ContainerStarted(t *testing.T) {
	e := &Event{
		EventType:      EventContainerStarted,
		ContainerName:  "myapp",
		NodeName:       "node-1",
		PreviousStatus: "exited",
		CurrentStatus:  "running",
	}
	text := e.Text()
	if text != "\U0001f7e2 Container *myapp* on node-1 started (exited → running)" {
		t.Errorf("unexpected text: %s", text)
	}
}

func TestText_ContainerRemoved(t *testing.T) {
	e := &Event{
		EventType:     EventContainerRemoved,
		ContainerName: "myapp",
		NodeName:      "node-1",
	}
	text := e.Text()
	if text != "\U000026aa Container *myapp* on node-1 was removed" {
		t.Errorf("unexpected text: %s", text)
	}
}

func TestSend_AllEventsWhenNoFilter(t *testing.T) {
	var mu sync.Mutex
	requestCount := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// No event filter — all events should be sent.
	n := NewNotifier(srv.URL, nil)

	n.Send(Event{EventType: EventContainerDied, ContainerName: "app", NodeName: "n1"})
	n.Send(Event{EventType: EventContainerStarted, ContainerName: "app", NodeName: "n1"})
	n.Send(Event{EventType: EventContainerRemoved, ContainerName: "app", NodeName: "n1"})

	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		count := requestCount
		mu.Unlock()
		if count >= 3 {
			break
		}
		select {
		case <-deadline:
			mu.Lock()
			t.Fatalf("timed out, got %d requests, expected 3", requestCount)
			mu.Unlock()
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}
