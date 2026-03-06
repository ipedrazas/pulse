use std::time::Duration;

use tokio::sync::mpsc;
use tokio_stream::wrappers::ReceiverStream;
use tonic::transport::Channel;
use tracing::{error, info, warn};

use crate::proto::pulse::v1::agent_service_client::AgentServiceClient;
use crate::proto::pulse::v1::{AgentMessage, ServerCommand};

const BACKOFF_MIN: Duration = Duration::from_millis(500);
const BACKOFF_MAX: Duration = Duration::from_secs(30);

/// Connects to the API with exponential backoff, returning the client.
pub async fn connect_with_backoff(addr: &str) -> AgentServiceClient<Channel> {
    let mut delay = BACKOFF_MIN;

    loop {
        match AgentServiceClient::connect(addr.to_string()).await {
            Ok(client) => {
                info!("connected to API at {}", addr);
                return client;
            }
            Err(e) => {
                warn!("connection failed: {}, retrying in {:?}", e, delay);
                tokio::time::sleep(delay).await;
                delay = (delay * 2).min(BACKOFF_MAX);
            }
        }
    }
}

/// Establishes the bidirectional stream and returns channels for sending/receiving.
pub async fn establish_stream(
    client: &mut AgentServiceClient<Channel>,
) -> Result<
    (
        mpsc::Sender<AgentMessage>,
        tonic::Streaming<ServerCommand>,
    ),
    tonic::Status,
> {
    let (tx, rx) = mpsc::channel::<AgentMessage>(64);
    let stream = ReceiverStream::new(rx);

    let response = client.stream_link(stream).await?;
    let inbound = response.into_inner();

    Ok((tx, inbound))
}

/// Runs the main stream loop: sends messages from outbound channel,
/// receives commands and forwards them to the command handler.
pub async fn stream_loop(
    mut inbound: tonic::Streaming<ServerCommand>,
    cmd_tx: mpsc::Sender<ServerCommand>,
) {
    loop {
        match inbound.message().await {
            Ok(Some(cmd)) => {
                info!("received command: {}", cmd.command_id);
                if let Err(e) = cmd_tx.send(cmd).await {
                    error!("failed to forward command: {}", e);
                }
            }
            Ok(None) => {
                info!("stream closed by server");
                break;
            }
            Err(e) => {
                error!("stream error: {}", e);
                break;
            }
        }
    }
}
