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
) -> Result<(mpsc::Sender<AgentMessage>, tonic::Streaming<ServerCommand>), tonic::Status> {
    let (tx, rx) = mpsc::channel::<AgentMessage>(64);
    let stream = ReceiverStream::new(rx);

    let response = client.stream_link(stream).await?;
    let inbound = response.into_inner();

    Ok((tx, inbound))
}

/// How long to wait for a message before treating the stream as stale.
/// This must be longer than the heartbeat/poll interval so idle periods
/// (no commands from server) don't trigger a false timeout.
const STREAM_READ_TIMEOUT: Duration = Duration::from_secs(120);

/// Timeout for sending a message on the outbound channel.
const SEND_TIMEOUT: Duration = Duration::from_secs(10);

/// Sends an AgentMessage on the outbound channel with a timeout.
/// Returns `true` on success, `false` if the send timed out or the channel closed.
pub async fn send_msg(tx: &mpsc::Sender<AgentMessage>, msg: AgentMessage) -> bool {
    match tokio::time::timeout(SEND_TIMEOUT, tx.send(msg)).await {
        Ok(Ok(())) => true,
        Ok(Err(e)) => {
            error!("outbound channel closed: {}", e);
            false
        }
        Err(_) => {
            error!("outbound send timed out after {:?}", SEND_TIMEOUT);
            false
        }
    }
}

/// Runs the inbound stream loop, forwarding commands to the command handler.
/// Breaks on stream close, error, or if no message arrives within the read timeout.
pub async fn stream_loop(
    mut inbound: tonic::Streaming<ServerCommand>,
    cmd_tx: mpsc::Sender<ServerCommand>,
) {
    loop {
        match tokio::time::timeout(STREAM_READ_TIMEOUT, inbound.message()).await {
            Ok(Ok(Some(cmd))) => {
                info!("received command: {}", cmd.command_id);
                if let Err(e) = cmd_tx.send(cmd).await {
                    error!("failed to forward command: {}", e);
                }
            }
            Ok(Ok(None)) => {
                info!("stream closed by server");
                break;
            }
            Ok(Err(e)) => {
                error!("stream error: {}", e);
                break;
            }
            Err(_) => {
                warn!("no message received in {:?}, treating stream as stale", STREAM_READ_TIMEOUT);
                break;
            }
        }
    }
}
