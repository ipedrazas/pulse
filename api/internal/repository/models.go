package repository

// ContainerStatus represents the current status of a container,
// including its latest heartbeat data.
type ContainerStatus struct {
	ContainerID    string            `json:"container_id"`
	NodeName       string            `json:"node_name"`
	Name           string            `json:"name"`
	ImageTag       string            `json:"image_tag"`
	Status         *string           `json:"status"`
	UptimeSeconds  *int64            `json:"uptime_seconds"`
	LastSeen       *string           `json:"last_seen"`
	Labels         map[string]string `json:"labels"`
	EnvVars        map[string]string `json:"env_vars"`
	ComposeProject string            `json:"compose_project"`
}

// ActionResponse represents a command/action and its result.
type ActionResponse struct {
	CommandID  string            `json:"command_id"`
	NodeName   string            `json:"node_name"`
	Action     string            `json:"action"`
	Target     string            `json:"target"`
	Params     map[string]string `json:"params"`
	Status     string            `json:"status"`
	Output     string            `json:"output"`
	DurationMs int64             `json:"duration_ms"`
	CreatedAt  string            `json:"created_at"`
	UpdatedAt  string            `json:"updated_at"`
}

// AgentStatus represents the status of a monitoring agent.
type AgentStatus struct {
	NodeName     string `json:"node_name"`
	AgentVersion string `json:"agent_version"`
	FirstSeen    string `json:"first_seen"`
	LastSeen     string `json:"last_seen"`
	Online       bool   `json:"online"`
}

// ContainerInfo holds minimal container metadata used for webhook enrichment.
type ContainerInfo struct {
	ContainerID    string
	Name           string
	ImageTag       string
	ComposeProject string
}

// ContainerMetadata holds the full metadata for upserting a container record.
type ContainerMetadata struct {
	ContainerID    string
	NodeName       string
	Name           string
	ImageTag       string
	EnvsJSON       []byte
	MountsJSON     []byte
	LabelsJSON     []byte
	ComposeProject string
	ComposeDir     string
}

// PendingCommand represents a command claimed for execution.
type PendingCommand struct {
	CommandID string
	Action    string
	Target    string
	Params    map[string]string
}
