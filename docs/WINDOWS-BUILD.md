# Build the Windows x64 managed client

This builds the **RustDesk desktop client**, not the Go control-plane executable.
Run on Windows 10/11 x64 or Windows Server 2022 with an x64 MSVC toolchain.
The script builds the Rust DLL, Flutter UI, runtime bundle and upstream self-extracting
EXE. It does not require Linux or MinGW. It has not yet been executed on a Windows
build host in this session; successful packaging and actual installation are still
required before distributing the result.

## 1. Install build tools once

Use a short checkout path such as `C:\src\rustdesk-control`.
Install:

- Visual Studio 2022 / Build Tools: Desktop development with C++, MSVC v143 x64/x86,
  Windows 10/11 SDK (including rc.exe), and C++ CMake tools for Windows.
- Git for Windows, Python 3 with pip, Rustup.
- Flutter **3.24.5** Windows SDK; add its `bin` directory to PATH.
- LLVM **15.0.6** (clang and libclang.dll), CMake and NASM on PATH.

Open **Developer PowerShell for VS 2022**, with x64 target architecture.
Run `flutter doctor -v` and resolve Windows desktop/Visual Studio errors.
Android Studio and Android SDK are not needed.

Prepare vcpkg in a separate directory:

```powershell
git clone https://github.com/microsoft/vcpkg.git C:\src\vcpkg
git -C C:\src\vcpkg checkout 120deac3062162151622ca4860575a33844ba10b
& C:\src\vcpkg\bootstrap-vcpkg.bat -disableMetrics
```

The script uses Rust 1.94.0 MSVC (the management dependency lockfile requires a
newer compiler than upstream's old 1.75 CI setting), cargo-expand 1.0.95 and
flutter_rust_bridge_codegen 1.80.1. It installs the Rust toolchain and generators.
Flutter 3.24.5 and vcpkg versions follow the pinned upstream Windows x64 workflow.

## 2. Set deployment URLs and build

Bring this repository to the Windows machine, including `managed-client/core`
and both patches. You do not need to copy the large ignored RustDesk checkout;
the script clones and patches the exact baseline itself.

```powershell
Set-Location C:\src\rustdesk-control
$env:RUSTDESK_CONTROL_URL = 'https://rustdesk-control.altasci.com'
$env:RUSTDESK_MANAGED_SOURCE_URL = 'https://github.com/phil616/rustdesk-control'
$env:LIBCLANG_PATH = 'C:\Program Files\LLVM\bin'
.\scripts\build-client.ps1 -VcpkgRoot C:\src\vcpkg
```

Both URLs above are literal defaults in the script and GitHub Actions; setting
them explicitly is optional. Override them for another deployment. The control origin is compiled into the client and
cannot be supplied later through ordinary UI. It must serve the deployed Go
control plane over valid HTTPS. The source URL must publish corresponding
modified client source when distributing the binary; it is shown in About.
No admin password or encryption master key is supplied to this build.

The script checks tools, verifies the baseline, applies the managed feature,
installs native dependencies, generates the bridge, builds Rust and Flutter,
checks required bundle files, then invokes the upstream packer. Every native
command failure stops the script. Logs identify the failing step. Network access
is needed for Git, Cargo, Pub, vcpkg and pip downloads.

## 3. Retrieve and install

Successful output:

```text
dist/windows-x64/rustdesk-managed-1.4.9-x86_64-install.exe
```

The EXE contains the Flutter bundle, license notices and expanded modified source
archive. See [LICENSING.md](LICENSING.md) for extraction and distribution duties. Launch it and use RustDesk's normal **Install**
action, accepting the normal UAC prompt. The management agent requires the
installed RustDesk service running as LocalSystem; merely running the portable
UI does not enroll. Do not copy only the `rustdesk.exe` from the uncompressed
bundle: it needs its DLLs and `data` directory.

Before installing, deploy the control plane and save ID Server / Relay / Public
Key in Settings. After installation, approve the pending device in the console,
wait for password sync, then test normal ID + managed password login and reboot.
See [ACCEPTANCE.md](ACCEPTANCE.md) for the Windows checks.

This output is unsigned. A trusted Authenticode signature is a separate release
step using your own signing certificate. The script does not suppress OS warnings.
It produces one EXE; no bundle ZIP or MSI is generated. Optional upstream printer/USB virtual
display driver packages and the custom upstream Flutter engine are not bundled by
this script; those auxiliary features require the additional upstream release
steps. Core desktop capture/input and managed access are the intended scope.

## Common failures

- `cl.exe` or `rc.exe` missing: use the VS Developer PowerShell and install C++/SDK.
- `libclang.dll` missing: set LIBCLANG_PATH to the LLVM bin directory.
- `bridge_generated.rs` missing: bridge generation failed; fix that step first.
- vcpkg error: inspect `C:\src\vcpkg\buildtrees` logs; do not substitute MinGW triplets.
- Rust DLL built but no EXE: Flutter or packer failed; the DLL alone is not a client.
- No pending device: check normal installation/service status, the compiled HTTPS
  origin, valid control TLS certificate and saved server policy.

## GitHub runner dependency preparation

The workflow keeps its pinned checkout in `RDC_VCPKG_ROOT=C:\rdc-vcpkg`.
Visual Studio developer-shell setup exports its own `VCPKG_ROOT`; it must not
be used as the clone destination. The build script receives `-VcpkgRoot`
explicitly and sets `VCPKG_ROOT` for Cargo/native dependency discovery afterward.

`dylib_virtual_display` is a separate Cargo workspace member. The script builds
it explicitly before the main Rust library and Flutter app, verifies the DLL,
and includes it in the EXE bundle. Building only the root RustDesk `--lib`
does not produce this required DLL.
