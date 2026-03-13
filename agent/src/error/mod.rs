use std::fmt;

/// Unified error type for the pulse agent.
#[derive(Debug)]
pub enum AgentError {
    /// Docker API errors (bollard).
    Docker(bollard::errors::Error),
    /// gRPC transport or protocol errors.
    Grpc(tonic::transport::Error),
    /// gRPC status errors (from stream operations).
    GrpcStatus(Box<tonic::Status>),
    /// I/O errors (filesystem, subprocess).
    Io(std::io::Error),
    /// Container not found or working directory missing.
    NotFound(String),
    /// Command not supported by the executor.
    UnsupportedCommand,
    /// A subprocess (e.g. docker compose) exited with a non-zero status.
    SubprocessFailed { stdout: String, stderr: String },
    /// Send channel closed or timed out.
    ChannelClosed(String),
}

impl fmt::Display for AgentError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::Docker(e) => write!(f, "docker: {e}"),
            Self::Grpc(e) => write!(f, "grpc: {e}"),
            Self::GrpcStatus(s) => write!(f, "grpc status: {}", s),
            Self::Io(e) => write!(f, "io: {e}"),
            Self::NotFound(msg) => write!(f, "not found: {msg}"),
            Self::UnsupportedCommand => write!(f, "unsupported command"),
            Self::SubprocessFailed { stderr, .. } => write!(f, "subprocess failed: {stderr}"),
            Self::ChannelClosed(msg) => write!(f, "channel closed: {msg}"),
        }
    }
}

impl std::error::Error for AgentError {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        match self {
            Self::Docker(e) => Some(e),
            Self::Grpc(e) => Some(e),
            Self::GrpcStatus(s) => Some(s.as_ref()),
            Self::Io(e) => Some(e),
            _ => None,
        }
    }
}

impl From<bollard::errors::Error> for AgentError {
    fn from(e: bollard::errors::Error) -> Self {
        Self::Docker(e)
    }
}

impl From<tonic::transport::Error> for AgentError {
    fn from(e: tonic::transport::Error) -> Self {
        Self::Grpc(e)
    }
}

impl From<tonic::Status> for AgentError {
    fn from(s: tonic::Status) -> Self {
        Self::GrpcStatus(Box::new(s))
    }
}

impl From<std::io::Error> for AgentError {
    fn from(e: std::io::Error) -> Self {
        Self::Io(e)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn display_unsupported() {
        let e = AgentError::UnsupportedCommand;
        assert_eq!(e.to_string(), "unsupported command");
    }

    #[test]
    fn display_not_found() {
        let e = AgentError::NotFound("/missing/dir".into());
        assert_eq!(e.to_string(), "not found: /missing/dir");
    }

    #[test]
    fn display_subprocess_failed() {
        let e = AgentError::SubprocessFailed {
            stdout: "out".into(),
            stderr: "err".into(),
        };
        assert_eq!(e.to_string(), "subprocess failed: err");
    }

    #[test]
    fn display_channel_closed() {
        let e = AgentError::ChannelClosed("send timed out".into());
        assert_eq!(e.to_string(), "channel closed: send timed out");
    }

    #[test]
    fn display_io() {
        let io_err = std::io::Error::new(std::io::ErrorKind::NotFound, "file missing");
        let e = AgentError::from(io_err);
        assert!(e.to_string().contains("file missing"));
    }

    #[test]
    fn from_io_error() {
        let io_err = std::io::Error::new(std::io::ErrorKind::Other, "test");
        let e: AgentError = io_err.into();
        assert!(matches!(e, AgentError::Io(_)));
    }

    #[test]
    fn source_returns_inner() {
        let io_err = std::io::Error::new(std::io::ErrorKind::Other, "inner");
        let e = AgentError::Io(io_err);
        assert!(std::error::Error::source(&e).is_some());
    }

    #[test]
    fn source_returns_none_for_leaf() {
        let e = AgentError::UnsupportedCommand;
        assert!(std::error::Error::source(&e).is_none());
    }
}
