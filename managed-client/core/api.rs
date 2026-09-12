use crate::{auth, policy::Policy, storage::Identity, Result};
use reqwest::blocking::Client;
use serde::{de::DeserializeOwned, Serialize};
use std::{
    io::Read,
    time::{Duration, SystemTime, UNIX_EPOCH},
};
pub struct Api {
    client: Client,
    base: String,
}
pub fn timestamp() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs()
}
impl Api {
    pub fn new(base: &str) -> Result<Self> {
        let url = url::Url::parse(base)?;
        if url.scheme() != "https"
            || url.host_str().is_none()
            || !url.username().is_empty()
            || url.password().is_some()
            || url.query().is_some()
            || url.fragment().is_some()
            || url.path() != "/"
        {
            return Err("control URL must be an HTTPS origin".into());
        }
        Ok(Self {
            client: Client::builder()
                .https_only(true)
                .redirect(reqwest::redirect::Policy::none())
                .connect_timeout(Duration::from_secs(10))
                .timeout(Duration::from_secs(20))
                .build()?,
            base: base.trim_end_matches('/').into(),
        })
    }
    fn read<T: DeserializeOwned>(response: reqwest::blocking::Response) -> Result<T> {
        let response = response.error_for_status()?;
        let mut bytes = Vec::new();
        response.take(65537).read_to_end(&mut bytes)?;
        if bytes.len() > 65536 {
            return Err("response too large".into());
        }
        Ok(serde_json::from_slice(&bytes)?)
    }
    pub fn bootstrap(&self) -> Result<Policy> {
        Self::read(
            self.client
                .get(format!("{}/api/v1/agent/bootstrap", self.base))
                .send()?,
        )
    }
    pub fn post<B: Serialize, T: DeserializeOwned>(
        &self,
        path: &str,
        identity: &Identity,
        body: &B,
        ts: u64,
    ) -> Result<T> {
        let body = serde_json::to_vec(body)?;
        let ts = ts.to_string();
        let nonce = uuid::Uuid::new_v4().to_string();
        let signature = auth::sign(&identity.key()?, "POST", path, &ts, &nonce, &body);
        Self::read(
            self.client
                .post(format!("{}{}", self.base, path))
                .header("Content-Type", "application/json")
                .header("X-RDC-Device-ID", &identity.device_uuid)
                .header("X-RDC-Timestamp", ts)
                .header("X-RDC-Nonce", nonce)
                .header("X-RDC-Signature", signature)
                .body(body)
                .send()?,
        )
    }
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn refuses_non_https() {
        for url in [
            "http://localhost",
            "https://user:pass@example.com",
            "https://example.com/path",
            "https://example.com?q=x",
        ] {
            assert!(Api::new(url).is_err());
        }
    }
}
