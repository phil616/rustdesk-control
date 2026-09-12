use base64::{engine::general_purpose::STANDARD, Engine};
use sha2::{Digest, Sha256};
use sodiumoxide::crypto::sign::ed25519;

pub fn canonical(method: &str, path: &str, timestamp: &str, nonce: &str, body: &[u8]) -> Vec<u8> {
    format!(
        "{}\n{}\n{}\n{}\n{:x}",
        method,
        path,
        timestamp,
        nonce,
        Sha256::digest(body)
    )
    .into_bytes()
}
pub fn sign(
    key: &ed25519::SecretKey,
    method: &str,
    path: &str,
    timestamp: &str,
    nonce: &str,
    body: &[u8],
) -> String {
    STANDARD.encode(
        ed25519::sign_detached(&canonical(method, path, timestamp, nonce, body), key).to_bytes(),
    )
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn canonical_vector() {
        assert_eq!(String::from_utf8(canonical("POST", "/api/v1/agent/heartbeat", "1", "nonce", b"{}")).unwrap(), "POST\n/api/v1/agent/heartbeat\n1\nnonce\n44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a");
    }
    #[test]
    fn request_signing() {
        sodiumoxide::init().unwrap();
        let (pk, sk) = ed25519::gen_keypair();
        let raw = STANDARD
            .decode(sign(&sk, "POST", "/test", "123", "nonce", b"{}"))
            .unwrap();
        let signature = ed25519::Signature::from_bytes(&raw).unwrap();
        assert!(ed25519::verify_detached(
            &signature,
            &canonical("POST", "/test", "123", "nonce", b"{}"),
            &pk
        ));
        assert!(!ed25519::verify_detached(
            &signature,
            &canonical("POST", "/test", "123", "nonce", b"bad"),
            &pk
        ));
    }
}
