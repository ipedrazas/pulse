use std::env;
use std::time::Duration;

#[derive(Debug, Clone)]
pub struct Config {
    pub api_addr: String,
    pub node_name: String,
    pub poll_interval: Duration,
    pub redact_patterns: Vec<String>,
    pub _tls_cert: Option<String>,
    pub _tls_key: Option<String>,
    pub _tls_ca: Option<String>,
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
            api_addr: env::var("PULSE_API_ADDR")
                .unwrap_or_else(|_| "http://localhost:9090".to_string()),
            node_name: env::var("PULSE_NODE_NAME").unwrap_or_else(|_| hostname()),
            poll_interval: Duration::from_secs(poll_secs),
            redact_patterns,
            _tls_cert: env::var("PULSE_TLS_CERT").ok(),
            _tls_key: env::var("PULSE_TLS_KEY").ok(),
            _tls_ca: env::var("PULSE_TLS_CA").ok(),
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

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Mutex;
    use std::time::Duration;

    // Env-var tests must run serially since env is process-global.
    static ENV_LOCK: Mutex<()> = Mutex::new(());

    unsafe fn clear_pulse_env() {
        for key in [
            "PULSE_API_ADDR",
            "PULSE_NODE_NAME",
            "PULSE_POLL_INTERVAL",
            "PULSE_REDACT_PATTERNS",
            "PULSE_TLS_CERT",
            "PULSE_TLS_KEY",
            "PULSE_TLS_CA",
            "HOSTNAME",
        ] {
            unsafe { env::remove_var(key) };
        }
    }

    #[test]
    fn test_defaults() {
        let _lock = ENV_LOCK.lock().unwrap();
        unsafe { clear_pulse_env() };

        let cfg = Config::from_env();
        assert_eq!(cfg.api_addr, "http://localhost:9090");
        assert_eq!(cfg.poll_interval, Duration::from_secs(30));
        assert_eq!(
            cfg.redact_patterns,
            vec!["PASSWORD", "SECRET", "KEY", "TOKEN", "CREDENTIAL"]
        );
        assert!(cfg._tls_cert.is_none());
        assert!(cfg._tls_key.is_none());
        assert!(cfg._tls_ca.is_none());
    }

    #[test]
    fn test_custom_api_addr() {
        let _lock = ENV_LOCK.lock().unwrap();
        unsafe { clear_pulse_env() };
        unsafe { env::set_var("PULSE_API_ADDR", "http://api.example.com:9090") };

        let cfg = Config::from_env();
        assert_eq!(cfg.api_addr, "http://api.example.com:9090");
    }

    #[test]
    fn test_poll_interval_with_suffix() {
        let _lock = ENV_LOCK.lock().unwrap();
        unsafe { clear_pulse_env() };
        unsafe { env::set_var("PULSE_POLL_INTERVAL", "10s") };

        let cfg = Config::from_env();
        assert_eq!(cfg.poll_interval, Duration::from_secs(10));
    }

    #[test]
    fn test_poll_interval_without_suffix() {
        let _lock = ENV_LOCK.lock().unwrap();
        unsafe { clear_pulse_env() };
        unsafe { env::set_var("PULSE_POLL_INTERVAL", "15") };

        let cfg = Config::from_env();
        assert_eq!(cfg.poll_interval, Duration::from_secs(15));
    }

    #[test]
    fn test_custom_redact_patterns() {
        let _lock = ENV_LOCK.lock().unwrap();
        unsafe { clear_pulse_env() };
        unsafe { env::set_var("PULSE_REDACT_PATTERNS", "FOO,BAR,BAZ") };

        let cfg = Config::from_env();
        assert_eq!(cfg.redact_patterns, vec!["FOO", "BAR", "BAZ"]);
    }

    #[test]
    fn test_empty_redact_patterns() {
        let _lock = ENV_LOCK.lock().unwrap();
        unsafe { clear_pulse_env() };
        unsafe { env::set_var("PULSE_REDACT_PATTERNS", "") };

        let cfg = Config::from_env();
        assert!(cfg.redact_patterns.is_empty());
    }

    #[test]
    fn test_hostname_from_env() {
        let _lock = ENV_LOCK.lock().unwrap();
        unsafe { clear_pulse_env() };
        unsafe { env::set_var("PULSE_NODE_NAME", "my-node") };

        let cfg = Config::from_env();
        assert_eq!(cfg.node_name, "my-node");
    }
}
