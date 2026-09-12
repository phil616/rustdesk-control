use crate::Result;
use base64::{engine::general_purpose::STANDARD, Engine};
use fs2::FileExt;
use serde::{Deserialize, Serialize};
use sodiumoxide::crypto::sign::ed25519;
use std::{
    fs::{File, OpenOptions},
    io::Write,
    path::{Path, PathBuf},
};
#[derive(Deserialize, Serialize)]
pub struct Identity {
    pub device_uuid: String,
    pub private_key: String,
    pub public_key: String,
}
impl Identity {
    pub fn key(&self) -> Result<ed25519::SecretKey> {
        let key = ed25519::SecretKey::from_slice(&STANDARD.decode(&self.private_key)?)
            .ok_or("invalid identity key")?;
        if STANDARD.encode(key.public_key().as_ref()) != self.public_key
            || uuid::Uuid::parse_str(&self.device_uuid)?.get_version_num() != 4
        {
            return Err("invalid identity".into());
        }
        Ok(key)
    }
}
#[derive(Default, Deserialize, Serialize)]
pub struct State {
    pub policy_version: u64,
    pub password_version: u64,
}
pub struct Storage {
    dir: PathBuf,
    _lock: File,
}
fn private_file(path: &Path) -> Result<File> {
    let mut options = OpenOptions::new();
    options.write(true).create_new(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt;
        options.mode(0o600);
    }
    Ok(options.open(path)?)
}
impl Storage {
    pub fn lock(dir: PathBuf) -> Result<Self> {
        std::fs::create_dir_all(&dir)?;
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            std::fs::set_permissions(&dir, std::fs::Permissions::from_mode(0o700))?;
        }
        let path = dir.join("agent.lock");
        let lock = match private_file(&path) {
            Ok(f) => f,
            Err(_) => OpenOptions::new().read(true).write(true).open(path)?,
        };
        lock.try_lock_exclusive()?;
        Ok(Self { dir, _lock: lock })
    }
    pub fn identity(&self) -> Result<Identity> {
        let path = self.dir.join("identity.json");
        if path.exists() {
            let identity: Identity = serde_json::from_slice(&std::fs::read(path)?)?;
            let _ = identity.key()?;
            return Ok(identity);
        }
        let (pk, sk) = ed25519::gen_keypair();
        let identity = Identity {
            device_uuid: uuid::Uuid::new_v4().to_string(),
            private_key: STANDARD.encode(sk.as_ref()),
            public_key: STANDARD.encode(pk.as_ref()),
        };
        let mut file = private_file(&path)?;
        file.write_all(&serde_json::to_vec(&identity)?)?;
        file.sync_all()?;
        Ok(identity)
    }
    pub fn state(&self) -> Result<State> {
        match std::fs::read(self.dir.join("state.json")) {
            Ok(b) => Ok(serde_json::from_slice(&b)?),
            Err(e) if e.kind() == std::io::ErrorKind::NotFound => Ok(State::default()),
            Err(e) => Err(e.into()),
        }
    }
    pub fn save(&self, state: &State) -> Result<()> {
        // State contains only acknowledgements, never a password. A partial write causes a safe reapply.
        let path = self.dir.join("state.json");
        let mut file = OpenOptions::new()
            .write(true)
            .create(true)
            .truncate(true)
            .open(path)?;
        file.write_all(&serde_json::to_vec(state)?)?;
        file.sync_all()?;
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn identity_persists_and_only_one_holder() {
        sodiumoxide::init().unwrap();
        let dir = std::env::temp_dir().join(format!("rdc-test-{}", uuid::Uuid::new_v4()));
        let store = Storage::lock(dir.clone()).unwrap();
        let identity = store.identity().unwrap();
        assert!(Storage::lock(dir.clone()).is_err());
        drop(store);
        let store = Storage::lock(dir.clone()).unwrap();
        let again = store.identity().unwrap();
        assert_eq!(identity.device_uuid, again.device_uuid);
        assert_eq!(identity.public_key, again.public_key);
        #[cfg(unix)]
        {
            use std::os::unix::fs::PermissionsExt;
            assert_eq!(
                std::fs::metadata(dir.join("identity.json"))
                    .unwrap()
                    .permissions()
                    .mode()
                    & 0o777,
                0o600
            );
        }
        store
            .save(&State {
                policy_version: 4,
                password_version: 2,
            })
            .unwrap();
        assert_eq!(store.state().unwrap().password_version, 2);
        drop(store);
        std::fs::remove_dir_all(dir).unwrap();
    }
}
