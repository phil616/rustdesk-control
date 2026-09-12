use serde::Serialize;
#[derive(Serialize)]
pub struct Device {
    pub rustdesk_id: String,
    pub hostname: String,
    pub os: String,
    pub os_version: String,
    pub arch: String,
    pub rustdesk_version: String,
    pub managed_client_version: String,
}
