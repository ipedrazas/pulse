package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Notifier sends agent status change notifications.
type Notifier interface {
	AgentOnline(nodeName string)
	AgentOffline(nodeName string)
}

// NewNotifier returns a webhook-backed notifier if url is non-empty,
// otherwise a no-op implementation.
func NewNotifier(url string) Notifier {
	if url == "" {
		return noopNotifier{}
	}
	return &webhookNotifier{
		webhookURL: url,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// noopNotifier silently discards all notifications.
type noopNotifier struct{}

func (noopNotifier) AgentOnline(_ string)  {}
func (noopNotifier) AgentOffline(_ string) {}

// webhookNotifier sends Slack/Discord-compatible webhook payloads.
type webhookNotifier struct {
	webhookURL string
	client     *http.Client
}

func (n *webhookNotifier) AgentOnline(nodeName string) {
	n.send(fmt.Sprintf(":green_circle: Agent **%s** is now **online**", nodeName))
}

func (n *webhookNotifier) AgentOffline(nodeName string) {
	n.send(fmt.Sprintf(":red_circle: Agent **%s** went **offline**", nodeName))
}

func (n *webhookNotifier) send(message string) {
	payload := map[string]string{"text": message, "content": message}
	body, _ := json.Marshal(payload)

	resp, err := n.client.Post(n.webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		slog.Error("webhook send failed", "error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		slog.Warn("webhook non-2xx response", "status", resp.StatusCode)
	}
}
