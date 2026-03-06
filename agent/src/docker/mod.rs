use std::collections::HashMap;

use bollard::Docker;
use bollard::container::ListContainersOptions;
use sha2::{Digest, Sha256};
use tracing::{error, warn};

use crate::proto::pulse::v1::{ContainerInfo, ContainerReport, PortMapping};
use crate::redact;

pub struct DockerPoller {
    client: Docker,
    node_name: String,
    redact_patterns: Vec<String>,
    last_hash: String,
}

impl DockerPoller {
    pub fn new(
        node_name: String,
        redact_patterns: Vec<String>,
    ) -> Result<Self, bollard::errors::Error> {
        let client = Docker::connect_with_local_defaults()?;
        Ok(Self {
            client,
            node_name,
            redact_patterns,
            last_hash: String::new(),
        })
    }

    /// Polls Docker and returns a ContainerReport. Returns None if nothing changed.
    pub async fn poll(&mut self) -> Option<ContainerReport> {
        let containers = match self.list_containers().await {
            Ok(c) => c,
            Err(e) => {
                error!("failed to list containers: {}", e);
                return None;
            }
        };

        let hash = compute_hash(&containers);
        if hash == self.last_hash {
            return None; // no changes
        }
        self.last_hash = hash;

        Some(ContainerReport {
            node_name: self.node_name.clone(),
            containers,
        })
    }

    async fn list_containers(&self) -> Result<Vec<ContainerInfo>, bollard::errors::Error> {
        let options = ListContainersOptions::<String> {
            all: true,
            ..Default::default()
        };

        let containers = self.client.list_containers(Some(options)).await?;
        let mut infos = Vec::new();

        for c in containers {
            let id = c.id.unwrap_or_default();
            let names = c.names.unwrap_or_default();
            let name = names
                .first()
                .map(|n| n.trim_start_matches('/').to_string())
                .unwrap_or_default();
            let image = c.image.unwrap_or_default();
            let status = c.state.unwrap_or_default();
            let labels = c.labels.unwrap_or_default();
            let command = c.command.unwrap_or_default();

            // Extract compose project from labels
            let compose_project = labels
                .get("com.docker.compose.project")
                .cloned()
                .unwrap_or_default();

            // Redact env vars — we need to inspect for full env
            let env_vars = match self.client.inspect_container(&id, None).await {
                Ok(inspect) => {
                    let env_list = inspect.config.and_then(|cfg| cfg.env).unwrap_or_default();
                    let env_map: HashMap<String, String> = env_list
                        .iter()
                        .filter_map(|e| {
                            let mut parts = e.splitn(2, '=');
                            Some((
                                parts.next()?.to_string(),
                                parts.next().unwrap_or("").to_string(),
                            ))
                        })
                        .collect();
                    redact::redact_env_vars(&env_map, &self.redact_patterns)
                }
                Err(e) => {
                    warn!("failed to inspect container {}: {}", id, e);
                    HashMap::new()
                }
            };

            // Mounts
            let mounts: Vec<String> = c
                .mounts
                .unwrap_or_default()
                .iter()
                .map(|m| {
                    format!(
                        "{}:{}",
                        m.source.as_deref().unwrap_or(""),
                        m.destination.as_deref().unwrap_or("")
                    )
                })
                .collect();

            // Ports
            let ports: Vec<PortMapping> = c
                .ports
                .unwrap_or_default()
                .iter()
                .map(|p| PortMapping {
                    host_ip: p.ip.clone().unwrap_or_default(),
                    host_port: p.public_port.unwrap_or(0) as u32,
                    container_port: p.private_port as u32,
                    protocol: p
                        .typ
                        .as_ref()
                        .map(|t| format!("{:?}", t).to_lowercase())
                        .unwrap_or_default(),
                })
                .collect();

            // Uptime
            let uptime = c.created.unwrap_or(0);
            let uptime_seconds = if uptime > 0 {
                let now = std::time::SystemTime::now()
                    .duration_since(std::time::UNIX_EPOCH)
                    .unwrap_or_default()
                    .as_secs() as i64;
                (now - uptime).max(0)
            } else {
                0
            };

            infos.push(ContainerInfo {
                id,
                name,
                image,
                status,
                env_vars,
                mounts,
                labels,
                ports,
                uptime_seconds,
                compose_project,
                command,
            });
        }

        Ok(infos)
    }
}

pub fn compute_hash(containers: &[ContainerInfo]) -> String {
    let mut hasher = Sha256::new();
    for c in containers {
        hasher.update(c.id.as_bytes());
        hasher.update(c.image.as_bytes());
        hasher.update(c.status.as_bytes());

        // Sort env keys for deterministic hashing
        let mut keys: Vec<&String> = c.env_vars.keys().collect();
        keys.sort();
        for k in keys {
            hasher.update(k.as_bytes());
            hasher.update(c.env_vars[k].as_bytes());
        }

        for m in &c.mounts {
            hasher.update(m.as_bytes());
        }
    }
    hex::encode(hasher.finalize())
}

#[cfg(test)]
mod tests {
    use super::*;

    fn make_container(id: &str, image: &str, status: &str) -> ContainerInfo {
        ContainerInfo {
            id: id.to_string(),
            name: String::new(),
            image: image.to_string(),
            status: status.to_string(),
            env_vars: HashMap::new(),
            mounts: vec![],
            labels: HashMap::new(),
            ports: vec![],
            uptime_seconds: 0,
            compose_project: String::new(),
            command: String::new(),
        }
    }

    #[test]
    fn test_compute_hash_deterministic() {
        let containers = vec![make_container("c1", "nginx", "running")];
        let h1 = compute_hash(&containers);
        let h2 = compute_hash(&containers);
        assert_eq!(h1, h2);
    }

    #[test]
    fn test_compute_hash_different_status() {
        let c1 = vec![make_container("c1", "nginx", "running")];
        let c2 = vec![make_container("c1", "nginx", "exited")];
        assert_ne!(compute_hash(&c1), compute_hash(&c2));
    }

    #[test]
    fn test_compute_hash_env_key_order_independent() {
        let mut c1 = make_container("c1", "nginx", "running");
        c1.env_vars.insert("A".to_string(), "1".to_string());
        c1.env_vars.insert("B".to_string(), "2".to_string());

        let mut c2 = make_container("c1", "nginx", "running");
        c2.env_vars.insert("B".to_string(), "2".to_string());
        c2.env_vars.insert("A".to_string(), "1".to_string());

        assert_eq!(compute_hash(&[c1]), compute_hash(&[c2]));
    }

    #[test]
    fn test_compute_hash_empty_containers() {
        let h = compute_hash(&[]);
        assert!(!h.is_empty());
        // Should still produce a valid hex hash
        assert!(h.chars().all(|c| c.is_ascii_hexdigit()));
    }
}
