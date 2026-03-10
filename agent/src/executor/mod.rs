use std::collections::HashMap;

use bollard::Docker;
use bollard::container::{Config, CreateContainerOptions, StopContainerOptions};
use bollard::image::CreateImageOptions;
use futures_util::StreamExt;
use tracing::{error, info};

use crate::proto::pulse::v1::{
    CommandResult, RequestLogs, RestartContainer, RunContainer, ServerCommand, StopContainer,
    server_command,
};
use bollard::container::LogsOptions;

pub struct Executor {
    docker: Docker,
    node_name: String,
}

impl Executor {
    pub fn new(node_name: String) -> Result<Self, bollard::errors::Error> {
        let docker = Docker::connect_with_local_defaults()?;
        Ok(Self { docker, node_name })
    }

    pub async fn execute(&self, cmd: &ServerCommand) -> CommandResult {
        let command_id = cmd.command_id.clone();
        let start = std::time::Instant::now();

        let (success, output, err_msg) = match &cmd.payload {
            Some(server_command::Payload::RunContainer(rc)) => self.run_container(rc).await,
            Some(server_command::Payload::StopContainer(sc)) => self.stop_container(sc).await,
            Some(server_command::Payload::PullImage(pi)) => self.pull_image(&pi.image).await,
            Some(server_command::Payload::ComposeUp(cu)) => self.compose_up(cu).await,
            Some(server_command::Payload::RequestLogs(rl)) => self.request_logs(rl).await,
            Some(server_command::Payload::RestartContainer(rc)) => {
                self.restart_container(rc).await
            }
            _ => (false, String::new(), "unsupported command".to_string()),
        };

        let duration_ms = start.elapsed().as_millis() as i64;

        CommandResult {
            command_id,
            node_name: self.node_name.clone(),
            success,
            output,
            error: err_msg,
            duration_ms,
        }
    }

    async fn run_container(&self, rc: &RunContainer) -> (bool, String, String) {
        // Pull the image first
        let (pull_ok, _, pull_err) = self.pull_image(&rc.image).await;
        if !pull_ok {
            return (false, String::new(), format!("pull failed: {}", pull_err));
        }

        let env: Vec<String> = rc.env.iter().map(|(k, v)| format!("{}={}", k, v)).collect();

        // Build port bindings
        let mut exposed_ports = HashMap::new();
        let mut port_bindings = HashMap::new();
        for p in &rc.ports {
            let container_port = format!(
                "{}/{}",
                p.container_port,
                if p.protocol.is_empty() {
                    "tcp"
                } else {
                    &p.protocol
                }
            );
            exposed_ports.insert(container_port.clone(), HashMap::new());
            port_bindings.insert(
                container_port,
                Some(vec![bollard::models::PortBinding {
                    host_ip: Some(if p.host_ip.is_empty() {
                        "0.0.0.0".to_string()
                    } else {
                        p.host_ip.clone()
                    }),
                    host_port: Some(p.host_port.to_string()),
                }]),
            );
        }

        let host_config = bollard::models::HostConfig {
            binds: Some(rc.volumes.clone()),
            port_bindings: Some(port_bindings),
            auto_remove: Some(rc.remove),
            ..Default::default()
        };

        let config = Config {
            image: Some(rc.image.clone()),
            env: Some(env),
            cmd: if rc.command.is_empty() {
                None
            } else {
                Some(rc.command.clone())
            },
            exposed_ports: Some(exposed_ports),
            host_config: Some(host_config),
            ..Default::default()
        };

        let name = if rc.name.is_empty() {
            None
        } else {
            Some(rc.name.clone())
        };
        let options = name.map(|n| CreateContainerOptions {
            name: n,
            platform: None,
        });

        match self.docker.create_container(options, config).await {
            Ok(resp) => match self.docker.start_container::<String>(&resp.id, None).await {
                Ok(_) => {
                    info!("started container {}", resp.id);
                    (true, resp.id, String::new())
                }
                Err(e) => {
                    error!("failed to start container: {}", e);
                    (false, String::new(), e.to_string())
                }
            },
            Err(e) => {
                error!("failed to create container: {}", e);
                (false, String::new(), e.to_string())
            }
        }
    }

    async fn stop_container(&self, sc: &StopContainer) -> (bool, String, String) {
        let timeout = if sc.timeout_seconds > 0 {
            sc.timeout_seconds
        } else {
            10
        };

        let options = StopContainerOptions { t: timeout.into() };
        match self
            .docker
            .stop_container(&sc.container_id, Some(options))
            .await
        {
            Ok(_) => {
                info!("stopped container {}", sc.container_id);
                (true, format!("stopped {}", sc.container_id), String::new())
            }
            Err(e) => {
                error!("failed to stop container: {}", e);
                (false, String::new(), e.to_string())
            }
        }
    }

    async fn restart_container(&self, rc: &RestartContainer) -> (bool, String, String) {
        let timeout = if rc.timeout_seconds > 0 {
            rc.timeout_seconds
        } else {
            10
        };

        let options = bollard::container::RestartContainerOptions {
            t: timeout as isize,
        };
        match self
            .docker
            .restart_container(&rc.container_id, Some(options))
            .await
        {
            Ok(_) => {
                info!("restarted container {}", rc.container_id);
                (true, format!("restarted {}", rc.container_id), String::new())
            }
            Err(e) => {
                error!("failed to restart container: {}", e);
                (false, String::new(), e.to_string())
            }
        }
    }

    async fn pull_image(&self, image: &str) -> (bool, String, String) {
        info!("pulling image {}", image);
        let options = CreateImageOptions {
            from_image: image,
            ..Default::default()
        };

        let mut stream = self.docker.create_image(Some(options), None, None);
        while let Some(result) = stream.next().await {
            match result {
                Ok(info) => {
                    if let Some(status) = info.status {
                        tracing::debug!("pull: {}", status);
                    }
                }
                Err(e) => {
                    error!("pull failed: {}", e);
                    return (false, String::new(), e.to_string());
                }
            }
        }
        info!("pulled image {}", image);
        (true, format!("pulled {}", image), String::new())
    }

    async fn request_logs(&self, rl: &RequestLogs) -> (bool, String, String) {
        let tail = if rl.tail > 0 {
            rl.tail.to_string()
        } else {
            "all".to_string()
        };

        let options = LogsOptions::<String> {
            stdout: true,
            stderr: true,
            tail,
            follow: false,
            ..Default::default()
        };

        let mut stream = self.docker.logs(&rl.container_id, Some(options));
        let mut lines = Vec::new();

        while let Some(result) = stream.next().await {
            match result {
                Ok(output) => lines.push(output.to_string()),
                Err(e) => {
                    error!("log stream error: {}", e);
                    return (false, lines.join(""), e.to_string());
                }
            }
        }

        (true, lines.join(""), String::new())
    }

    async fn compose_up(&self, cu: &crate::proto::pulse::v1::ComposeUp) -> (bool, String, String) {
        let mut cmd = tokio::process::Command::new("docker");
        cmd.arg("compose");

        if !cu.file.is_empty() {
            cmd.args(["-f", &cu.file]);
        }

        cmd.arg("up");

        if cu.detach {
            cmd.arg("-d");
        }
        if cu.pull {
            cmd.arg("--pull=always");
        }

        if !cu.project_dir.is_empty() {
            cmd.current_dir(&cu.project_dir);
        }

        match cmd.output().await {
            Ok(output) => {
                let stdout = String::from_utf8_lossy(&output.stdout).to_string();
                let stderr = String::from_utf8_lossy(&output.stderr).to_string();
                if output.status.success() {
                    info!("compose up succeeded");
                    (true, stdout, String::new())
                } else {
                    error!("compose up failed: {}", stderr);
                    (false, stdout, stderr)
                }
            }
            Err(e) => {
                error!("compose up exec failed: {}", e);
                (false, String::new(), e.to_string())
            }
        }
    }
}
