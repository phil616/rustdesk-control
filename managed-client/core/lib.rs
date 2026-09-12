//! Thin management client. No screen, clipboard, user password or file collection.
pub mod agent;
pub mod api;
pub mod auth;
pub mod device;
pub mod policy;
pub mod storage;
pub const CONTROL_URL: &str = env!("RUSTDESK_CONTROL_URL");
pub const SOURCE_URL: &str = env!("RUSTDESK_MANAGED_SOURCE_URL");
pub type Result<T> = std::result::Result<T, Box<dyn std::error::Error + Send + Sync>>;
pub trait Host: Send + 'static {
    fn device(&self) -> device::Device;
    fn apply_servers(&self, value: &policy::RustDesk) -> Result<()>;
    fn apply_password(&self, password: &str) -> Result<()>;
    fn connected(&self, value: bool);
}
