package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type Notifier struct {
	webhookURL string
	client     *http.Client
}

func NewNotifier(webhookURL string) *Notifier {
	if webhookURL == "" {
		return nil
	}
	return &Notifier{
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (n *Notifier) AgentOnline(nodeName string) {
	if n == nil {
		return
	}
	n.send(fmt.Sprintf(":green_circle: Agent **%s** is now **online**", nodeName))
}

func (n *Notifier) AgentOffline(nodeName string) {
	if n == nil {
		return
	}
	n.send(fmt.Sprintf(":red_circle: Agent **%s** went **offline**", nodeName))
}

func (n *Notifier) send(message string) {
	// Slack/Discord compatible payload
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
