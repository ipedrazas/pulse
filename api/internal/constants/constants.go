package constants

// Command types used across REST handlers, gRPC services, and the agent.
const (
	CmdRunContainer     = "run_container"
	CmdStopContainer    = "stop_container"
	CmdRestartContainer = "restart_container"
	CmdPullImage        = "pull_image"
	CmdComposeUp        = "compose_up"
	CmdSendFile         = "send_file"
	CmdRequestLogs      = "request_logs"
)

// Command statuses.
const (
	StatusPending   = "pending"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// Agent statuses.
const (
	AgentOnline  = "online"
	AgentOffline = "offline"
	AgentLost    = "lost"
)
