pub mod proto {
    pub mod pulse {
        pub mod v1 {
            tonic::include_proto!("pulse.v1");
        }
    }
}

pub mod config;
pub mod docker;
pub mod error;
pub mod executor;
pub mod grpc;
pub mod redact;
pub mod sysinfo;
