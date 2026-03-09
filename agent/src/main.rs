pub mod proto {
    pub mod pulse {
        pub mod v1 {
            tonic::include_proto!("pulse.v1");
        }
    }
}

mod config;
mod docker;
mod executor;
mod grpc;
mod redact;
mod sysinfo;

use proto::pulse::v1::{AgentMessage, Heartbeat, agent_message};
use tokio::sync::mpsc;
use tracing::{error, info, warn};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| tracing_subscriber::EnvFilter::new("info")),
        )
        .init();

    let cfg = config::Config::from_env();
    info!(
        node_name = %cfg.node_name,
        api_addr = %cfg.api_addr,
        poll_interval = ?cfg.poll_interval,
        version = env!("CARGO_PKG_VERSION"),
        "pulse-agent starting"
    );

    let mut poller = docker::DockerPoller::new(cfg.node_name.clone(), cfg.redact_patterns.clone())?;
    let exec = executor::Executor::new(cfg.node_name.clone())?;

    // Main reconnection loop
    loop {
        info!("connecting to API at {}", cfg.api_addr);
        let mut client = grpc::connect_with_backoff(&cfg.api_addr).await;

        match grpc::establish_stream(&mut client).await {
            Ok((outbound_tx, inbound)) => {
                info!("stream established");

                // Channel for commands received from server
                let (cmd_tx, mut cmd_rx) = mpsc::channel(32);

                // Spawn stream reader
                let stream_handle = tokio::spawn(grpc::stream_loop(inbound, cmd_tx));

                let mut poll_interval = tokio::time::interval(cfg.poll_interval);
                let mut stream_broken = false;

                while !stream_broken {
                    tokio::select! {
                        _ = poll_interval.tick() => {
                            // Send heartbeat with node metadata
                            let metadata = sysinfo::collect();
                            let heartbeat = AgentMessage {
                                payload: Some(agent_message::Payload::Heartbeat(Heartbeat {
                                    node_name: cfg.node_name.clone(),
                                    agent_version: env!("CARGO_PKG_VERSION").to_string(),
                                    timestamp: Some(prost_types::Timestamp::from(std::time::SystemTime::now())),
                                    metadata: Some(metadata),
                                })),
                            };
                            if let Err(e) = outbound_tx.send(heartbeat).await {
                                error!("failed to send heartbeat: {}", e);
                                stream_broken = true;
                                continue;
                            }

                            // Poll Docker and send report if changed
                            if let Some(report) = poller.poll().await {
                                let msg = AgentMessage {
                                    payload: Some(agent_message::Payload::ContainerReport(report)),
                                };
                                if let Err(e) = outbound_tx.send(msg).await {
                                    error!("failed to send report: {}", e);
                                    stream_broken = true;
                                }
                            }
                        }

                        Some(cmd) = cmd_rx.recv() => {
                            info!("executing command {}", cmd.command_id);
                            let result = exec.execute(&cmd).await;

                            let msg = AgentMessage {
                                payload: Some(agent_message::Payload::CommandResult(result)),
                            };
                            if let Err(e) = outbound_tx.send(msg).await {
                                error!("failed to send command result: {}", e);
                                stream_broken = true;
                            }
                        }
                    }
                }

                stream_handle.abort();
                warn!("stream broken, reconnecting...");
            }
            Err(e) => {
                error!("failed to establish stream: {}", e);
            }
        }

        // Brief pause before reconnecting
        tokio::time::sleep(std::time::Duration::from_secs(1)).await;
    }
}
