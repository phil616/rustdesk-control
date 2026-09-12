//! Upstream adapter. Only existing RustDesk service/server lifecycle calls start().
use hbb_common::config::{Config, Config2};
use managed_control_core::{device::Device, policy::RustDesk, Host, Result};
use std::sync::Once;
struct RustDeskHost;
impl Host for RustDeskHost {
    fn device(&self) -> Device {
        let system = hbb_common::sysinfo::System::new();
        Device {
            rustdesk_id: Config::get_id(),
            hostname: crate::common::hostname(),
            os: std::env::consts::OS.into(),
            os_version: system.os_version().unwrap_or_default(),
            arch: std::env::consts::ARCH.into(),
            rustdesk_version: crate::VERSION.into(),
            managed_client_version: "1.0.0".into(),
        }
    }
    fn apply_servers(&self, p: &RustDesk) -> Result<()> {
        let _restart = crate::ipc::CheckIfRestart::new();
        for (key, value) in [
            ("custom-rendezvous-server", p.id_server.as_str()),
            ("relay-server", p.relay_server.as_str()),
            ("key", p.key.as_str()),
            ("api-server", ""),
            ("allow-insecure-tls-fallback", "N"),
        ] {
            Config::set_option(key.into(), value.into());
            if Config::get_option(key) != value {
                return Err("managed option rejected".into());
            }
        }
        Config::set_option("managed-control-enabled".into(), "Y".into());
        Ok(())
    }
    fn apply_password(&self, password: &str) -> Result<()> {
        if !Config::set_managed_permanent_password(password) {
            return Err("managed password rejected".into());
        }
        for (k, v) in [
            ("verification-method", "use-permanent-password"),
            ("approve-mode", "password"),
            ("disable-change-permanent-password", "Y"),
        ] {
            Config::set_option(k.into(), v.into());
        }
        Ok(())
    }
    fn connected(&self, value: bool) {
        // Non-secret, timestamped status is synchronized by existing Config2 IPC.
        Config::set_option(
            "managed-control-last-contact".into(),
            if value {
                managed_control_core::api::timestamp().to_string()
            } else {
                "0".into()
            },
        );
    }
}
pub fn start() {
    if !crate::platform::is_root() {
        return;
    }
    static ONCE: Once = Once::new();
    ONCE.call_once(|| {
        std::thread::spawn(|| {
            // Service-owned config directory: /root on Linux; LocalSystem profile on Windows.
            // GUI/tray/connection-manager processes never call this entry point.
            let dir = Config::path("managed-control");
            loop {
                if managed_control_core::agent::run(RustDeskHost, dir.clone()).is_err() {
                    hbb_common::log::error!("Managed control agent could not start; retrying");
                }
                std::thread::sleep(std::time::Duration::from_secs(5));
            }
        });
    });
}
pub fn preserve_options(options: &mut std::collections::HashMap<String, String>) {
    if Config::get_option("managed-control-enabled") != "Y" {
        return;
    }
    for key in [
        "custom-rendezvous-server",
        "relay-server",
        "key",
        "api-server",
        "allow-insecure-tls-fallback",
        "managed-control-enabled",
        "managed-control-last-contact",
        "verification-method",
        "approve-mode",
        "disable-change-permanent-password",
    ] {
        let value = Config::get_option(key);
        if value.is_empty() {
            options.remove(key);
        } else {
            options.insert(key.into(), value);
        }
    }
}
pub fn preserve_config(config: &mut Config, config2: &mut Config2) {
    preserve_options(&mut config2.options);
    if Config::is_disable_change_permanent_password() {
        Config::preserve_managed_password(config);
    }
}
pub fn notice() -> String {
    let last = crate::ipc::get_options()
        .get("managed-control-last-contact")
        .and_then(|v| v.parse::<u64>().ok())
        .unwrap_or(0);
    let status = if managed_control_core::api::timestamp().saturating_sub(last) <= 120 {
        "Connected"
    } else {
        "Offline"
    };
    format!("Managed by rustdesk-control · Modified version\nControl status: {status}")
}

// Copy only management-owned values during ongoing root -> user synchronization.
pub fn merge_service_config(source: &Config, source2: &Config2, baseline: &mut (Config, Config2)) {
    let mut local = Config::get();
    if source2
        .options
        .get("disable-change-permanent-password")
        .map(String::as_str)
        == Some("Y")
    {
        local.copy_managed_password_from(source);
        baseline.0.copy_managed_password_from(source);
        Config::set(local);
    }
    for key in [
        "custom-rendezvous-server",
        "relay-server",
        "key",
        "api-server",
        "allow-insecure-tls-fallback",
        "managed-control-enabled",
        "managed-control-last-contact",
        "verification-method",
        "approve-mode",
        "disable-change-permanent-password",
    ] {
        let value = source2.options.get(key).cloned().unwrap_or_default();
        Config::set_option(key.into(), value.clone());
        if value.is_empty() {
            baseline.1.options.remove(key);
        } else {
            baseline.1.options.insert(key.into(), value);
        }
    }
}
