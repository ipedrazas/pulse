//! Integration tests for DockerPoller and Executor against a real Docker daemon.
//!
//! These tests require a running Docker daemon. They are skipped automatically
//! when Docker is unavailable. Run with:
//!
//!   cargo test --test docker_integration

use std::collections::HashMap;

use bollard::Docker;
use bollard::container::{
    Config, CreateContainerOptions, RemoveContainerOptions, StopContainerOptions,
};
use bollard::image::CreateImageOptions;
use futures_util::StreamExt;
use pulse_agent::docker::DockerPoller;
use pulse_agent::executor::Executor;
use pulse_agent::proto::pulse::v1::{
    RequestLogs, RunContainer, ServerCommand, StopContainer, server_command,
};

/// Skip the test if Docker is not reachable.
async fn require_docker() -> Docker {
    match Docker::connect_with_local_defaults() {
        Ok(d) => match d.ping().await {
            Ok(_) => d,
            Err(_) => {
                eprintln!("SKIP: Docker daemon not reachable (ping failed)");
                std::process::exit(0);
            }
        },
        Err(_) => {
            eprintln!("SKIP: cannot connect to Docker");
            std::process::exit(0);
        }
    }
}

const TEST_IMAGE: &str = "alpine:3.21";
const TEST_PREFIX: &str = "pulse-test-";

/// Ensure the test image is pulled locally.
async fn ensure_image(docker: &Docker) {
    let options = CreateImageOptions {
        from_image: TEST_IMAGE,
        ..Default::default()
    };
    let mut stream = docker.create_image(Some(options), None, None);
    while let Some(result) = stream.next().await {
        if let Err(e) = result {
            panic!("failed to pull test image {}: {}", TEST_IMAGE, e);
        }
    }
}

/// Create a named container from alpine that sleeps, returning its ID.
async fn create_test_container(docker: &Docker, name: &str) -> String {
    let full_name = format!("{}{}", TEST_PREFIX, name);

    // Remove leftover from previous test run
    let _ = docker
        .remove_container(
            &full_name,
            Some(RemoveContainerOptions {
                force: true,
                ..Default::default()
            }),
        )
        .await;

    let config = Config {
        image: Some(TEST_IMAGE.to_string()),
        cmd: Some(vec!["sleep".to_string(), "30".to_string()]),
        env: Some(vec![
            "TEST_VAR=hello".to_string(),
            "SECRET_KEY=s3cret".to_string(),
        ]),
        ..Default::default()
    };
    let options = CreateContainerOptions {
        name: full_name.clone(),
        platform: None,
    };
    let resp = docker
        .create_container(Some(options), config)
        .await
        .expect("failed to create test container");

    docker
        .start_container::<String>(&resp.id, None)
        .await
        .expect("failed to start test container");

    resp.id
}

/// Remove a test container by ID (force).
async fn cleanup_container(docker: &Docker, id: &str) {
    let _ = docker
        .remove_container(
            id,
            Some(RemoveContainerOptions {
                force: true,
                ..Default::default()
            }),
        )
        .await;
}

// ─── DockerPoller tests ─────────────────────────────────────────────────────

#[tokio::test]
async fn test_poller_discovers_running_container() {
    let docker = require_docker().await;
    ensure_image(&docker).await;

    let id = create_test_container(&docker, "poll-discover").await;

    let mut poller = DockerPoller::new("test-node".to_string(), vec![]).unwrap();
    let report = poller.poll().await;

    assert!(report.is_some(), "first poll should always return a report");
    let report = report.unwrap();
    assert_eq!(report.node_name, "test-node");

    let found = report.containers.iter().find(|c| c.id == id);
    assert!(found.is_some(), "test container should appear in report");

    let container = found.unwrap();
    assert_eq!(container.status, "running");
    assert!(
        container.name.contains("poll-discover"),
        "name should contain our label: {}",
        container.name
    );
    assert_eq!(container.image, TEST_IMAGE);

    cleanup_container(&docker, &id).await;
}

#[tokio::test]
async fn test_poller_detects_env_vars() {
    let docker = require_docker().await;
    ensure_image(&docker).await;

    let id = create_test_container(&docker, "poll-env").await;

    let mut poller = DockerPoller::new("test-node".to_string(), vec![]).unwrap();
    let report = poller.poll().await.unwrap();

    let container = report.containers.iter().find(|c| c.id == id).unwrap();
    assert_eq!(
        container.env_vars.get("TEST_VAR").map(|s| s.as_str()),
        Some("hello")
    );

    cleanup_container(&docker, &id).await;
}

#[tokio::test]
async fn test_poller_redacts_secrets() {
    let docker = require_docker().await;
    ensure_image(&docker).await;

    let id = create_test_container(&docker, "poll-redact").await;

    let patterns = vec!["SECRET".to_string(), "KEY".to_string()];
    let mut poller = DockerPoller::new("test-node".to_string(), patterns).unwrap();
    let report = poller.poll().await.unwrap();

    let container = report.containers.iter().find(|c| c.id == id).unwrap();
    let secret_val = container.env_vars.get("SECRET_KEY").map(|s| s.as_str());
    assert_eq!(secret_val, Some("***REDACTED***"));

    // TEST_VAR should NOT be redacted
    assert_eq!(
        container.env_vars.get("TEST_VAR").map(|s| s.as_str()),
        Some("hello")
    );

    cleanup_container(&docker, &id).await;
}

#[tokio::test]
async fn test_poller_returns_none_when_unchanged() {
    let docker = require_docker().await;
    ensure_image(&docker).await;

    let id = create_test_container(&docker, "poll-unchanged").await;

    let mut poller = DockerPoller::new("test-node".to_string(), vec![]).unwrap();

    // First poll should return a report
    let first = poller.poll().await;
    assert!(first.is_some());

    // Second poll with no changes should return None.
    // Retry with short delays because concurrent tests may cause transient hash changes.
    let mut got_none = false;
    for _ in 0..10 {
        if poller.poll().await.is_none() {
            got_none = true;
            break;
        }
        // If we got Some, the hash changed due to external activity.
        // Brief pause to let concurrent tests settle before retrying.
        tokio::time::sleep(std::time::Duration::from_millis(200)).await;
    }
    assert!(
        got_none,
        "poll should eventually return None when container state is stable"
    );

    cleanup_container(&docker, &id).await;
}

#[tokio::test]
async fn test_poller_detects_state_change() {
    let docker = require_docker().await;
    ensure_image(&docker).await;

    let id = create_test_container(&docker, "poll-change").await;

    let mut poller = DockerPoller::new("test-node".to_string(), vec![]).unwrap();

    // First poll
    let first = poller.poll().await;
    assert!(first.is_some());

    // Stop the container — state changes
    docker
        .stop_container(&id, Some(StopContainerOptions { t: 1 }))
        .await
        .unwrap();

    // Wait for Docker to fully update the container state
    for _ in 0..10 {
        let info = docker.inspect_container(&id, None).await.unwrap();
        let running = info
            .state
            .as_ref()
            .and_then(|s| s.running)
            .unwrap_or(false);
        if !running {
            break;
        }
        tokio::time::sleep(std::time::Duration::from_millis(200)).await;
    }

    // Next poll should detect the change
    let changed = poller.poll().await;
    assert!(
        changed.is_some(),
        "poll should detect state change after stop"
    );

    let report = changed.unwrap();
    let container = report.containers.iter().find(|c| c.id == id).unwrap();
    assert_ne!(container.status, "running");

    cleanup_container(&docker, &id).await;
}

// ─── Executor tests ─────────────────────────────────────────────────────────

#[tokio::test]
async fn test_executor_stop_container() {
    let docker = require_docker().await;
    ensure_image(&docker).await;

    let id = create_test_container(&docker, "exec-stop").await;

    let executor = Executor::new("test-node".to_string()).unwrap();
    let cmd = ServerCommand {
        command_id: "test-stop-1".to_string(),
        payload: Some(server_command::Payload::StopContainer(StopContainer {
            container_id: id.clone(),
            timeout_seconds: 2,
        })),
    };

    let result = executor.execute(&cmd).await;
    assert!(result.success, "stop should succeed: {}", result.error);
    assert_eq!(result.command_id, "test-stop-1");
    assert!(result.duration_ms >= 0);

    cleanup_container(&docker, &id).await;
}

#[tokio::test]
async fn test_executor_stop_nonexistent_container() {
    let _ = require_docker().await;

    let executor = Executor::new("test-node".to_string()).unwrap();
    let cmd = ServerCommand {
        command_id: "test-stop-missing".to_string(),
        payload: Some(server_command::Payload::StopContainer(StopContainer {
            container_id: "nonexistent_container_12345".to_string(),
            timeout_seconds: 1,
        })),
    };

    let result = executor.execute(&cmd).await;
    assert!(!result.success, "stop of nonexistent container should fail");
    assert!(!result.error.is_empty());
}

#[tokio::test]
async fn test_executor_request_logs() {
    let docker = require_docker().await;
    ensure_image(&docker).await;

    // Create a container that produces output
    let full_name = format!("{}exec-logs", TEST_PREFIX);
    let _ = docker
        .remove_container(
            &full_name,
            Some(RemoveContainerOptions {
                force: true,
                ..Default::default()
            }),
        )
        .await;

    let config = Config {
        image: Some(TEST_IMAGE.to_string()),
        cmd: Some(vec![
            "sh".to_string(),
            "-c".to_string(),
            "echo hello-from-test && sleep 30".to_string(),
        ]),
        ..Default::default()
    };
    let options = CreateContainerOptions {
        name: full_name.clone(),
        platform: None,
    };
    let resp = docker
        .create_container(Some(options), config)
        .await
        .unwrap();
    docker
        .start_container::<String>(&resp.id, None)
        .await
        .unwrap();

    // Give the container a moment to produce output
    tokio::time::sleep(std::time::Duration::from_secs(1)).await;

    let executor = Executor::new("test-node".to_string()).unwrap();
    let cmd = ServerCommand {
        command_id: "test-logs-1".to_string(),
        payload: Some(server_command::Payload::RequestLogs(RequestLogs {
            container_id: resp.id.clone(),
            follow: false,
            tail: 10,
        })),
    };

    let result = executor.execute(&cmd).await;
    assert!(result.success, "logs should succeed: {}", result.error);
    assert!(
        result.output.contains("hello-from-test"),
        "output should contain our echo: {}",
        result.output
    );

    cleanup_container(&docker, &resp.id).await;
}

#[tokio::test]
async fn test_executor_run_container() {
    let docker = require_docker().await;
    ensure_image(&docker).await;

    let container_name = format!("{}exec-run", TEST_PREFIX);

    // Clean up from previous run
    let _ = docker
        .remove_container(
            &container_name,
            Some(RemoveContainerOptions {
                force: true,
                ..Default::default()
            }),
        )
        .await;

    let executor = Executor::new("test-node".to_string()).unwrap();
    let cmd = ServerCommand {
        command_id: "test-run-1".to_string(),
        payload: Some(server_command::Payload::RunContainer(RunContainer {
            image: TEST_IMAGE.to_string(),
            name: container_name.clone(),
            env: HashMap::from([("MY_VAR".to_string(), "test_val".to_string())]),
            ports: vec![],
            volumes: vec![],
            command: vec!["sleep".to_string(), "10".to_string()],
            remove: false,
        })),
    };

    let result = executor.execute(&cmd).await;
    assert!(result.success, "run should succeed: {}", result.error);
    assert!(
        !result.output.is_empty(),
        "output should contain container ID"
    );

    // Verify the container is running
    let inspect = docker.inspect_container(&result.output, None).await;
    assert!(inspect.is_ok(), "container should exist after run");

    cleanup_container(&docker, &result.output).await;
}

#[tokio::test]
async fn test_executor_unsupported_command() {
    let _ = require_docker().await;

    let executor = Executor::new("test-node".to_string()).unwrap();
    let cmd = ServerCommand {
        command_id: "test-unsupported".to_string(),
        payload: None,
    };

    let result = executor.execute(&cmd).await;
    assert!(!result.success);
    assert_eq!(result.error, "unsupported command");
}
