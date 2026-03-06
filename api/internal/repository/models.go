package repository

import "time"

type Agent struct {
	Name      string     `json:"name"`
	Status    string     `json:"status"`
	Version   string     `json:"version"`
	LastSeen  *time.Time `json:"last_seen,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type Container struct {
	ContainerID    string            `json:"container_id"`
	AgentName      string            `json:"agent_name"`
	Name           string            `json:"name"`
	Image          string            `json:"image"`
	Status         string            `json:"status"`
	EnvVars        map[string]string `json:"env_vars"`
	Mounts         []string          `json:"mounts"`
	Labels         map[string]string `json:"labels"`
	Ports          []Port            `json:"ports"`
	ComposeProject string            `json:"compose_project"`
	Command        string            `json:"command"`
	UptimeSeconds  int64             `json:"uptime_seconds"`
	CreatedAt      time.Time         `json:"created_at"`
	RemovedAt      *time.Time        `json:"removed_at,omitempty"`
}

type Port struct {
	HostIP        string `json:"host_ip"`
	HostPort      uint32 `json:"host_port"`
	ContainerPort uint32 `json:"container_port"`
	Protocol      string `json:"protocol"`
}

type Command struct {
	ID          string     `json:"id"`
	AgentName   string     `json:"agent_name"`
	Type        string     `json:"type"`
	Payload     []byte     `json:"payload"`
	Status      string     `json:"status"`
	Result      string     `json:"result"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type ContainerEvent struct {
	Time          time.Time `json:"time"`
	ContainerID   string    `json:"container_id"`
	AgentName     string    `json:"agent_name"`
	Status        string    `json:"status"`
	UptimeSeconds int64     `json:"uptime_seconds"`
}
