use std::env;
use std::time::Duration;

#[derive(Debug, Clone)]
pub struct Config {
    pub api_addr: String,
    pub node_name: String,
    pub poll_interval: Duration,
    pub redact_patterns: Vec<String>,
    pub tls_cert: Option<String>,
    pub tls_key: Option<String>,
    pub tls_ca: Option<String>,
}

impl Config {
    pub fn from_env() -> Self {
        let poll_secs: u64 = env::var("PULSE_POLL_INTERVAL")
            .ok()
            .and_then(|v| v.trim_end_matches('s').parse().ok())
            .unwrap_or(30);

        let redact_patterns: Vec<String> = env::var("PULSE_REDACT_PATTERNS")
            .unwrap_or_else(|_| "PASSWORD,SECRET,KEY,TOKEN,CREDENTIAL".to_string())
            .split(',')
            .map(|s| s.trim().to_uppercase())
            .filter(|s| !s.is_empty())
            .collect();

        Self {
            api_addr: env::var("PULSE_API_ADDR").unwrap_or_else(|_| "http://localhost:9090".to_string()),
            node_name: env::var("PULSE_NODE_NAME").unwrap_or_else(|_| hostname()),
            poll_interval: Duration::from_secs(poll_secs),
            redact_patterns,
            tls_cert: env::var("PULSE_TLS_CERT").ok(),
            tls_key: env::var("PULSE_TLS_KEY").ok(),
            tls_ca: env::var("PULSE_TLS_CA").ok(),
        }
    }
}

fn hostname() -> String {
    std::env::var("HOSTNAME")
        .or_else(|_| {
            std::process::Command::new("hostname")
                .output()
                .map(|o| String::from_utf8_lossy(&o.stdout).trim().to_string())
        })
        .unwrap_or_else(|_| "unknown".to_string())
}
