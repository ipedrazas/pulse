pub mod proto {
    pub mod pulse {
        pub mod v1 {
            tonic::include_proto!("pulse.v1");
        }
    }
}

use tracing::info;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    tracing_subscriber::fmt::init();
    info!("pulse-agent starting");
    Ok(())
}
