use std::sync::Arc;

use pulse_agent::proto::pulse::v1::{AgentMessage, Heartbeat, agent_message};
use pulse_agent::{config, docker, executor, grpc, sysinfo};
use tokio::signal::unix::{SignalKind, signal};
use tokio::sync::mpsc;
use tracing::{error, info, warn};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Log level: RUST_LOG takes precedence, then PULSE_LOG_LEVEL, then default "info"
    let log_filter = tracing_subscriber::EnvFilter::try_from_default_env().unwrap_or_else(|_| {
        let level = std::env::var("PULSE_LOG_LEVEL").unwrap_or_else(|_| "info".to_string());
        tracing_subscriber::EnvFilter::new(level)
    });
    tracing_subscriber::fmt().with_env_filter(log_filter).init();

    let cfg = config::Config::from_env();
    info!(
        node_name = %cfg.node_name,
        api_addr = %cfg.api_addr,
        poll_interval = ?cfg.poll_interval,
        version = env!("CARGO_PKG_VERSION"),
        "pulse-agent starting"
    );

    let mut poller = docker::DockerPoller::new(cfg.node_name.clone(), cfg.redact_patterns.clone())?;
    let exec = Arc::new(executor::Executor::new(cfg.node_name.clone())?);

    let mut sigterm = signal(SignalKind::terminate())?;
    let mut sigint = signal(SignalKind::interrupt())?;

    // Main reconnection loop
    loop {
        info!("connecting to API at {}", cfg.api_addr);

        // Wait for connection, but honour shutdown signals during backoff
        let mut client = tokio::select! {
            c = grpc::connect_with_backoff(&cfg.api_addr) => c,
            _ = sigterm.recv() => {
                info!("SIGTERM received during connect, shutting down");
                break;
            }
            _ = sigint.recv() => {
                info!("SIGINT received during connect, shutting down");
                break;
            }
        };

        match grpc::establish_stream(&mut client).await {
            Ok((outbound_tx, inbound)) => {
                info!("stream established");

                // Collect and send node metadata once on connection
                let metadata = sysinfo::collect();
                let initial_heartbeat = AgentMessage {
                    payload: Some(agent_message::Payload::Heartbeat(Heartbeat {
                        node_name: cfg.node_name.clone(),
                        agent_version: env!("CARGO_PKG_VERSION").to_string(),
                        timestamp: Some(prost_types::Timestamp::from(std::time::SystemTime::now())),
                        metadata: Some(metadata),
                    })),
                };
                if !grpc::send_msg(&outbound_tx, initial_heartbeat).await {
                    continue;
                }

                // Channel for commands received from server
                let (cmd_tx, mut cmd_rx) = mpsc::channel(32);

                // Spawn stream reader
                let stream_handle = tokio::spawn(grpc::stream_loop(inbound, cmd_tx));

                let mut poll_interval = tokio::time::interval(cfg.poll_interval);
                let mut stream_broken = false;
                let mut shutdown = false;

                while !stream_broken && !shutdown {
                    tokio::select! {
                        _ = poll_interval.tick() => {
                            // Send heartbeat (no metadata — sent once on connect)
                            let heartbeat = AgentMessage {
                                payload: Some(agent_message::Payload::Heartbeat(Heartbeat {
                                    node_name: cfg.node_name.clone(),
                                    agent_version: env!("CARGO_PKG_VERSION").to_string(),
                                    timestamp: Some(prost_types::Timestamp::from(std::time::SystemTime::now())),
                                    metadata: None,
                                })),
                            };
                            if !grpc::send_msg(&outbound_tx, heartbeat).await {
                                stream_broken = true;
                                continue;
                            }

                            // Poll Docker and send report if changed
                            if let Some(report) = poller.poll().await {
                                let msg = AgentMessage {
                                    payload: Some(agent_message::Payload::ContainerReport(report)),
                                };
                                if !grpc::send_msg(&outbound_tx, msg).await {
                                    stream_broken = true;
                                }
                            }
                        }

                        Some(cmd) = cmd_rx.recv() => {
                            let exec = Arc::clone(&exec);
                            let tx = outbound_tx.clone();
                            tokio::spawn(async move {
                                info!(command_id = %cmd.command_id, "executing command");
                                let result = exec.execute(&cmd).await;
                                info!(command_id = %result.command_id, success = result.success, duration_ms = result.duration_ms, "command completed");
                                let msg = AgentMessage {
                                    payload: Some(agent_message::Payload::CommandResult(result)),
                                };
                                grpc::send_msg(&tx, msg).await;
                            });
                        }

                        _ = sigterm.recv() => {
                            info!("SIGTERM received, shutting down");
                            shutdown = true;
                        }
                        _ = sigint.recv() => {
                            info!("SIGINT received, shutting down");
                            shutdown = true;
                        }
                    }
                }

                stream_handle.abort();
                if shutdown {
                    // Drop the outbound sender to close the gRPC stream cleanly
                    drop(outbound_tx);
                    info!("shutdown complete");
                    return Ok(());
                }
                warn!("stream broken, reconnecting...");
            }
            Err(e) => {
                error!("failed to establish stream: {}", e);
            }
        }

        // Brief pause before reconnecting, but honour shutdown signals
        tokio::select! {
            _ = tokio::time::sleep(std::time::Duration::from_secs(1)) => {}
            _ = sigterm.recv() => {
                info!("SIGTERM received, shutting down");
                break;
            }
            _ = sigint.recv() => {
                info!("SIGINT received, shutting down");
                break;
            }
        }
    }

    info!("shutdown complete");
    Ok(())
}
