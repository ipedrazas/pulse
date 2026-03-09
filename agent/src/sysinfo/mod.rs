use std::fs;
use std::net::UdpSocket;
use std::process::Command;

use crate::proto::pulse::v1::NodeMetadata;

/// Collects system metadata about the current node.
pub fn collect() -> NodeMetadata {
    NodeMetadata {
        hostname: hostname(),
        ip_address: local_ip(),
        os_name: os_field("NAME"),
        os_version: os_field("VERSION_ID"),
        kernel_version: kernel_version(),
        uptime_seconds: uptime_seconds(),
        packages_to_update: packages_to_update(),
    }
}

fn hostname() -> String {
    fs::read_to_string("/etc/hostname")
        .map(|s| s.trim().to_string())
        .or_else(|_| {
            Command::new("hostname")
                .output()
                .map(|o| String::from_utf8_lossy(&o.stdout).trim().to_string())
        })
        .unwrap_or_default()
}

fn local_ip() -> String {
    // Connect to a public DNS to determine the local outbound IP
    UdpSocket::bind("0.0.0.0:0")
        .and_then(|s| {
            s.connect("8.8.8.8:53")?;
            s.local_addr()
        })
        .map(|addr| addr.ip().to_string())
        .unwrap_or_default()
}

fn os_field(key: &str) -> String {
    let content = fs::read_to_string("/etc/os-release").unwrap_or_default();
    for line in content.lines() {
        if let Some(val) = line
            .strip_prefix(key)
            .and_then(|rest| rest.strip_prefix('='))
        {
            return val.trim_matches('"').to_string();
        }
    }
    String::new()
}

fn kernel_version() -> String {
    Command::new("uname")
        .arg("-r")
        .output()
        .map(|o| String::from_utf8_lossy(&o.stdout).trim().to_string())
        .unwrap_or_default()
}

fn uptime_seconds() -> i64 {
    // Linux: /proc/uptime contains "seconds.fraction idle_seconds.fraction"
    fs::read_to_string("/proc/uptime")
        .ok()
        .and_then(|s| {
            s.split_whitespace()
                .next()?
                .split('.')
                .next()?
                .parse::<i64>()
                .ok()
        })
        .unwrap_or(0)
}

fn packages_to_update() -> i32 {
    // Try apt (Debian/Ubuntu)
    if let Ok(output) = Command::new("apt").args(["list", "--upgradable"]).output()
        && output.status.success()
    {
        let stdout = String::from_utf8_lossy(&output.stdout);
        // First line is "Listing..." header, count the rest
        let count = stdout.lines().skip(1).filter(|l| !l.is_empty()).count();
        return count as i32;
    }
    -1 // unknown / not applicable
}
