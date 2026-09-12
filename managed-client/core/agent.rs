use crate::{
    api::{timestamp, Api},
    policy::Policy,
    storage::{State, Storage},
    Host, Result, CONTROL_URL,
};
use serde_json::json;
use std::{path::PathBuf, time::Duration};
const RETRY: [u64; 6] = [5, 10, 30, 60, 300, 600];
pub fn run<H: Host>(host: H, dir: PathBuf) -> Result<()> {
    sodiumoxide::init().map_err(|_| "crypto initialization failed")?;
    let storage = Storage::lock(dir)?;
    let identity = storage.identity()?;
    // Reapply on process startup so acknowledgements never outlive runtime configuration.
    let mut state = State::default();
    let api = Api::new(CONTROL_URL)?;
    let mut enrolled = false;
    let mut failures = 0usize;
    loop {
        let result = (|| -> Result<()> {
            if !enrolled {
                let bootstrap = api.bootstrap()?;
                bootstrap.validate()?;
                host.apply_servers(&bootstrap.rustdesk)?;
                state.policy_version = bootstrap.policy_version;
                let device = host.device();
                let ts = timestamp();
                let mut body = serde_json::to_value(device)?;
                let object = body.as_object_mut().ok_or("invalid device")?;
                object.insert("device_uuid".into(), json!(identity.device_uuid));
                object.insert("public_key".into(), json!(identity.public_key));
                object.insert("timestamp".into(), json!(ts));
                let _: serde_json::Value =
                    api.post("/api/v1/agent/enroll", &identity, &body, ts)?;
                enrolled = true;
                log::info!("Enrollment completed");
            }
            let device = host.device();
            let body = json!({"rustdesk_id":device.rustdesk_id,"hostname":device.hostname,"rustdesk_version":device.rustdesk_version,"managed_client_version":device.managed_client_version,"policy_version":state.policy_version,"password_version":state.password_version});
            let policy: Policy =
                api.post("/api/v1/agent/heartbeat", &identity, &body, timestamp())?;
            apply_policy(&host, &policy, &mut state)?;
            storage.save(&state)?;
            Ok(())
        })();
        match result {
            Ok(()) => {
                host.connected(true);
                failures = 0;
                let jitter = sodiumoxide::randombytes::randombytes_uniform(21) as u64;
                std::thread::sleep(Duration::from_secs(50 + jitter));
            }
            Err(_) => {
                host.connected(false);
                log::warn!("Heartbeat failed");
                enrolled = false;
                state = State::default();
                std::thread::sleep(Duration::from_secs(RETRY[failures.min(RETRY.len() - 1)]));
                failures = (failures + 1).min(RETRY.len() - 1);
            }
        }
    }
}

fn apply_policy<H: Host>(host: &H, policy: &Policy, state: &mut State) -> Result<()> {
    policy.validate()?;
    host.apply_servers(&policy.rustdesk)?;
    if policy.policy_changed(state.policy_version) {
        state.policy_version = policy.policy_version;
        log::info!("Policy version updated");
    }
    if policy.password_changed(state.password_version) {
        let access = policy.managed_access.as_ref().ok_or("missing access")?;
        host.apply_password(&access.password)?;
        state.password_version = access.password_version;
        log::info!("Managed password applied");
    }
    Ok(())
}
#[cfg(test)]
mod tests {
    use super::*;
    struct RejectPassword;
    impl Host for RejectPassword {
        fn device(&self) -> crate::device::Device {
            panic!("not collected during apply")
        }
        fn apply_servers(&self, _: &crate::policy::RustDesk) -> Result<()> {
            Ok(())
        }
        fn apply_password(&self, _: &str) -> Result<()> {
            Err("storage failure".into())
        }
        fn connected(&self, _: bool) {}
    }
    #[test]
    fn failed_password_is_not_acknowledged() {
        let policy: Policy = serde_json::from_str(r#"{"protocol_version":1,"server_time":1,"device_status":"approved","policy_version":5,"rustdesk":{"id_server":"rd.example.com","relay_server":"","key":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","api_server":""},"managed_access":{"enabled":true,"password":"abcdefghijklmnopqrstuvwx","password_version":2,"lock_password":true}}"#).unwrap();
        let mut state = State {
            policy_version: 4,
            password_version: 1,
        };
        assert!(apply_policy(&RejectPassword, &policy, &mut state).is_err());
        assert_eq!(state.policy_version, 5);
        assert_eq!(state.password_version, 1);
    }
}
