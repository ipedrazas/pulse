package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Event types for container and agent state transitions.
const (
	EventContainerDied    = "container_died"
	EventContainerStarted = "container_started"
	EventContainerRemoved = "container_removed"
	EventAgentOffline     = "agent_offline"
	EventAgentOnline      = "agent_online"
)

// Event represents a container state transition that triggers a webhook.
type Event struct {
	EventType      string `json:"event"`
	Timestamp      string `json:"timestamp"`
	TextField      string `json:"text"`
	ContainerID    string `json:"container_id"`
	ContainerName  string `json:"container_name"`
	NodeName       string `json:"node_name"`
	Image          string `json:"image"`
	ComposeProject string `json:"compose_project"`
	PreviousStatus string `json:"previous_status"`
	CurrentStatus  string `json:"current_status"`
}

// Text generates a human-readable message for the event,
// compatible with Slack, Discord, and Mattermost.
func (e *Event) Text() string {
	switch e.EventType {
	case EventContainerDied:
		return fmt.Sprintf("\U0001f534 Container *%s* on %s died (%s → %s)",
			e.ContainerName, e.NodeName, e.PreviousStatus, e.CurrentStatus)
	case EventContainerStarted:
		return fmt.Sprintf("\U0001f7e2 Container *%s* on %s started (%s → %s)",
			e.ContainerName, e.NodeName, e.PreviousStatus, e.CurrentStatus)
	case EventContainerRemoved:
		return fmt.Sprintf("\U000026aa Container *%s* on %s was removed",
			e.ContainerName, e.NodeName)
	case EventAgentOffline:
		return fmt.Sprintf("\U0001f534 Agent on %s went offline", e.NodeName)
	case EventAgentOnline:
		return fmt.Sprintf("\U0001f7e2 Agent on %s came online", e.NodeName)
	default:
		return fmt.Sprintf("Container *%s* on %s: %s → %s",
			e.ContainerName, e.NodeName, e.PreviousStatus, e.CurrentStatus)
	}
}

// Notifier sends webhook notifications for container state transitions.
type Notifier struct {
	url        string
	events     map[string]bool
	httpClient *http.Client
}

// NewNotifier creates a new webhook notifier. Returns nil if url is empty (webhooks disabled).
// If events is empty, all event types are sent.
func NewNotifier(url string, events []string) *Notifier {
	if url == "" {
		return nil
	}

	evtMap := make(map[string]bool)
	for _, e := range events {
		evtMap[e] = true
	}

	return &Notifier{
		url:    url,
		events: evtMap,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Send dispatches a webhook notification asynchronously.
// Failures are logged but do not block the caller.
func (n *Notifier) Send(event Event) {
	if n == nil {
		return
	}

	// Filter: if specific events are configured, only send those.
	if len(n.events) > 0 && !n.events[event.EventType] {
		return
	}

	event.Timestamp = time.Now().UTC().Format(time.RFC3339)
	event.TextField = event.Text()

	go n.post(event)
}

func (n *Notifier) post(event Event) {
	body, err := json.Marshal(event)
	if err != nil {
		slog.Error("webhook: failed to marshal event", "error", err)
		return
	}

	resp, err := n.httpClient.Post(n.url, "application/json", bytes.NewReader(body))
	if err != nil {
		slog.Error("webhook: failed to send", "url", n.url, "event", event.EventType, "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		slog.Warn("webhook: non-success status", "url", n.url, "event", event.EventType, "status", resp.StatusCode)
		return
	}

	slog.Debug("webhook: sent", "event", event.EventType, "container", event.ContainerName)
}
