use crate::Result;
use serde::{Deserialize, Serialize};
#[derive(Clone, Deserialize, Serialize, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct RustDesk {
    pub id_server: String,
    pub relay_server: String,
    pub key: String,
    pub api_server: String,
}
#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Access {
    pub enabled: bool,
    pub password: String,
    pub password_version: u64,
    pub lock_password: bool,
}
#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
pub struct Policy {
    pub protocol_version: u32,
    pub server_time: i64,
    #[serde(default)]
    pub device_status: String,
    pub policy_version: u64,
    pub rustdesk: RustDesk,
    pub managed_access: Option<Access>,
}
impl Policy {
    pub fn validate(&self) -> Result<()> {
        use base64::Engine;
        if self.protocol_version != 1
            || self.policy_version == 0
            || !self.rustdesk.api_server.is_empty()
        {
            return Err("invalid policy".into());
        }
        if !valid_server(&self.rustdesk.id_server)
            || (!self.rustdesk.relay_server.is_empty()
                && !valid_server(&self.rustdesk.relay_server))
            || base64::engine::general_purpose::STANDARD
                .decode(&self.rustdesk.key)
                .map(|v| v.len())
                .unwrap_or(0)
                != 32
        {
            return Err("invalid RustDesk server configuration".into());
        }
        if !["", "pending", "approved", "rejected"].contains(&self.device_status.as_str()) {
            return Err("invalid device status".into());
        }
        match (&self.managed_access, self.device_status.as_str()) {
            (Some(a), "approved")
                if a.enabled
                    && a.lock_password
                    && a.password_version > 0
                    && (24..=128).contains(&a.password.len())
                    && a.password
                        .bytes()
                        .all(|c| c.is_ascii_alphanumeric() || c == b'-' || c == b'_') => {}
            (None, "" | "pending" | "rejected") => {}
            _ => return Err("invalid managed access".into()),
        }
        Ok(())
    }
    pub fn policy_changed(&self, applied: u64) -> bool {
        self.policy_version != applied
    }
    pub fn password_changed(&self, applied: u64) -> bool {
        self.managed_access
            .as_ref()
            .map(|a| a.password_version != applied)
            .unwrap_or(false)
    }
}
fn valid_server(s: &str) -> bool {
    if s.is_empty() || s.len() > 253 || s.chars().any(|c| c.is_whitespace() || "/\\@?#".contains(c))
    {
        return false;
    }
    url::Url::parse(&format!("https://{s}"))
        .map(|u| u.host_str().is_some())
        .unwrap_or(false)
}
#[cfg(test)]
mod tests {
    use super::*;
    fn fixture() -> Policy {
        serde_json::from_str(r#"{"protocol_version":1,"server_time":1,"device_status":"approved","policy_version":5,"rustdesk":{"id_server":"rd.example.com","relay_server":"rd.example.com","key":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","api_server":""},"managed_access":{"enabled":true,"password":"abcdefghijklmnopqrstuvwx","password_version":2,"lock_password":true}}"#).unwrap()
    }
    #[test]
    fn parses_and_compares_versions() {
        let p = fixture();
        p.validate().unwrap();
        assert!(p.policy_changed(4));
        assert!(!p.policy_changed(5));
        assert!(p.password_changed(1));
        assert!(!p.password_changed(2));
        assert!(p.password_changed(3));
    }
    #[test]
    fn rejects_invalid_policy() {
        let mut p = fixture();
        p.device_status = "pending".into();
        assert!(p.validate().is_err());
        p = fixture();
        p.rustdesk.api_server = "https://pro.example.com".into();
        assert!(p.validate().is_err());
        p = fixture();
        p.managed_access.as_mut().unwrap().password = "short".into();
        assert!(p.validate().is_err());
        p = fixture();
        p.rustdesk.id_server = "https://bad".into();
        assert!(p.validate().is_err());
        p = fixture();
        p.managed_access = None;
        assert!(p.validate().is_err());
    }
}
