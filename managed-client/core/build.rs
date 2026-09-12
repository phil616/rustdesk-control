fn main() {
    for name in ["RUSTDESK_CONTROL_URL", "RUSTDESK_MANAGED_SOURCE_URL"] {
        println!("cargo:rerun-if-env-changed={name}");
        match std::env::var(name) {
            Ok(value) => {
                assert!(
                    value.starts_with("https://")
                        && value.len() > 8
                        && !value.chars().any(char::is_whitespace),
                    "{name} must use HTTPS"
                );
                println!("cargo:rustc-env={name}={value}");
            }
            Err(_) if std::env::var("PROFILE").as_deref() == Ok("release") => {
                panic!("{name} is required for managed release builds")
            }
            Err(_) => println!("cargo:rustc-env={name}=https://unconfigured.invalid"),
        }
    }
}
