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

## 文档与 tag 发布改造 — 2026-09-13

本次发行范围固定为 Linux amd64 Go 控制面和 Windows x64 客户端 EXE。
文档已集中到 docs/，移除旧任务草案及 Linux 客户端构建脚本。
Actions 与本地 Windows 构建默认使用 `https://rustdesk-control.altasci.com` 和
`https://github.com/phil616/rustdesk-control`，无需额外仓库变量。

本地已完成：

- `scripts/build.sh`：Linux amd64 控制面构建成功，内嵌最新中英文 UI、源码和许可证。
- `go test -race ./...`、`go vet ./...`：通过；包括公开源码/许可下载与缺失文件 404 检查。
- Playwright：3 项通过，覆盖英文操作、中文切换/偏好和禁用存储回退。
- `scripts/test-release.py`：8 项通过，覆盖仅两个附件、正式版本禁止覆盖、草稿重试、
  上传失败保留草稿、修改源码保留和编译缓存排除。GitHub 调用使用 mock，未实际发布。
- actionlint 1.7.7：工作流静态检查通过（未启用 shellcheck）。
- PowerShell 7.4 parser：Windows 构建脚本语法检查通过。
- 实际源码包检查：含修改后的 RustDesk、hbb_common、管理模块、源码仓库及许可文件；
  不含 Git 元数据、数据库、node_modules 或 Rust target 缓存。
- 文档相对链接与 `git diff --check`：通过。

尚未在 GitHub Windows runner 完整编译、打包或执行 EXE；未实际推送 tag 或创建 Release。
Windows 安装、重启、真实连接及完整第三方分发合规核查仍须完成，不以静态检查代替。

## GitHub Actions failure diagnosis — 2026-09-13

- [Run 34728009144](https://github.com/phil616/rustdesk-control/actions/runs/34728009144):
  Windows dependency preparation failed because Visual Studio overwrote `VCPKG_ROOT`
  with its bundled, nonempty vcpkg directory. The workflow now uses a dedicated
  `RDC_VCPKG_ROOT` and passes that path explicitly to the build script.
- [Run 34702651109](https://github.com/phil616/rustdesk-control/actions/runs/34702651109):
  the Linux job passed; the Windows Rust release library and Flutter EXE both
  compiled successfully. Packaging then failed because `dylib_virtual_display.dll`
  had not been built. The script now builds the separate workspace member and
  checks/copies its DLL into the bundle.
- After these fixes, actionlint, PowerShell syntax validation, the pinned upstream
  DLL target check and all eight offline release tests passed locally.
  A fresh tag containing these changes is still needed to validate the entire
  GitHub packaging/release flow. Re-running an old run uses the old commit.
