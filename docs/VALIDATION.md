# Validation record — 2026-09-12

## Completed

- Go API: `go test -race ./...` passed, including an independent fake agent over
  HTTPS, admin authentication/session/CSRF, encryption, signature/time/replay
  rejection, enrollment/approval/reset, rotation, policy updates and online cutoff.
- `go vet ./...` passed.
- React TypeScript/Vite production build passed.
- Playwright Chromium workflow passed: login, pending approval, password reveal/
  hide, rotation, server settings save and responsive desktop/mobile rendering.
  No JavaScript page errors; no localStorage/sessionStorage credential persistence.
- Historical Go executable checks covered Linux/Windows amd64. Current release
  scope is Linux amd64 control plane and Windows x64 desktop client only.
- Rust management core: seven tests passed, including cross-language canonical
  vectors, signatures, invalid policy rejection, independent version comparison,
  persistence/single-instance lock, and failed password application not acknowledged.
- Rust management core Windows GNU x86_64 cargo check passed after building
  libsodium with its MinGW-compatible configure target.
- Four release build-environment checks passed: missing URLs and HTTP rejected,
  supplied HTTPS URLs accepted.
- Patches applied successfully to a fresh pinned RustDesk checkout. Re-running
  prepare on the existing modified checkout is idempotent.

See `validation/go-tests.txt`, `validation/rust-tests.txt`,
`validation/browser-tests.txt`, `validation/dashboard.png`, `validation/mobile.png`.

## Not yet completed — do not treat as a production-ready managed client

The complete RustDesk desktop binary and Flutter installer have not been built
successfully in this environment. Linux checks used an isolated extracted native dependency sysroot and reached
the RustDesk main library. The final errors are missing `src/bridge_generated.rs`
and consequent `EventToUI: IntoIntoDart` errors. No management adapter type errors
were reported in this pass, but the overall compilation failed.
Windows full check encountered a missing `x86_64-w64-mingw32-g++`. Flutter SDK is
not available. Passing the standalone core does not type-check all upstream hooks.

No Windows VM was installed/rebooted with the modified client. There is
no real hbbs/hbbr deployment for end-to-end desktop authentication. Therefore
Windows installation, reboot, actual ID/password remote login, offline control
plane and real server migration are **unverified**. Follow
`ACCEPTANCE.md` before distribution.

Platform-specific risks still requiring validation:

- Windows service-account identity directory ACL and stable identity across session
  transitions; no dedicated Windows ACL mutation has been added.
- Complete Flutter About rendering and native library packaging.
- Full adapter compilation and regression checks with feature both enabled/disabled.

The Go control plane and UI can be built and exercised independently. The client
source, patches and tests are implementation deliverables, not a validated
installation package. No production server or device was modified.

## Web i18n update

Added Simplified Chinese / English UI switching with browser-language detection,
optional persisted preference, Ant Design locale and localized dates/errors/audit
labels. The language preference is the only new Web Storage entry; credentials
remain in component memory. Production build and three Playwright tests passed,
including English regression, Chinese workflow/persistence/form preservation,
and disabled-storage fallback. Historical Windows/Linux control-plane executables were rebuilt
with the updated embedded UI; Windows control-plane builds are no longer a release target. See `validation/i18n-tests.txt` and
`validation/chinese-mobile.png`.
