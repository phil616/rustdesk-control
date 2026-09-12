[CmdletBinding()]
param(
    [string]$VcpkgRoot = $env:VCPKG_ROOT,
    [string]$RustToolchain = '1.94.0-x86_64-pc-windows-msvc'
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$root = Split-Path $PSScriptRoot -Parent
$client = Join-Path $root 'rustdesk-managed-client'
function Run([string]$Program, [string[]]$Arguments) {
    & $Program @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$Program failed with exit code $LASTEXITCODE" }
}
function Apply-Patch([string]$Directory, [string]$Patch) {
    & git -C $Directory apply --check $Patch 2>$null
    if ($LASTEXITCODE -eq 0) { Run git @('-C', $Directory, 'apply', $Patch); return }
    & git -C $Directory apply --reverse --check $Patch 2>$null
    if ($LASTEXITCODE -ne 0) { throw "Patch conflicts with local changes: $Patch" }
}
if ($env:OS -ne 'Windows_NT') { throw 'Run this script on Windows x64 in a Visual Studio Developer PowerShell.' }
if (!$env:RUSTDESK_CONTROL_URL) { $env:RUSTDESK_CONTROL_URL = 'https://rustdesk-control.altasci.com' }
if (!$env:RUSTDESK_MANAGED_SOURCE_URL) { $env:RUSTDESK_MANAGED_SOURCE_URL = 'https://github.com/phil616/rustdesk-control' }
foreach ($name in @('RUSTDESK_CONTROL_URL','RUSTDESK_MANAGED_SOURCE_URL')) {
    $value = [Environment]::GetEnvironmentVariable($name)
    $uri = $null
    if (![Uri]::TryCreate($value, [UriKind]::Absolute, [ref]$uri) -or $uri.Scheme -ne 'https' -or $uri.UserInfo -or $uri.Fragment) { throw "$name must be an HTTPS URL without credentials or fragment" }
    if ($name -eq 'RUSTDESK_CONTROL_URL' -and ($uri.AbsolutePath -ne '/' -or $uri.Query)) { throw 'Control URL must be an HTTPS origin without a path or query' }
}
foreach ($tool in @('git','rustup','cargo','python','flutter','clang','cmake','cl.exe','rc.exe')) {
    if (!(Get-Command $tool -ErrorAction SilentlyContinue)) { throw "Missing $tool. See docs/WINDOWS-BUILD.md" }
}
if (!$VcpkgRoot -or !(Test-Path (Join-Path $VcpkgRoot 'vcpkg.exe'))) { throw 'Pass -VcpkgRoot pointing to the pinned, bootstrapped vcpkg checkout' }
$vcpkgCommit = (& git -C $VcpkgRoot rev-parse HEAD | Out-String).Trim()
if ($LASTEXITCODE -ne 0 -or $vcpkgCommit -ne '120deac3062162151622ca4860575a33844ba10b') { throw 'vcpkg must be pinned to 120deac3062162151622ca4860575a33844ba10b' }
$flutterVersion = (& flutter --version --machine | Out-String) | ConvertFrom-Json
if ($flutterVersion.frameworkVersion -ne '3.24.5') { throw 'Use Flutter 3.24.5 for this Windows x64 build' }
Run rustup @('toolchain','install',$RustToolchain,'--profile','minimal','--component','rustfmt')
# Keep Rust and Flutter outputs at the paths used by upstream CMake and packer.
$env:RUSTUP_TOOLCHAIN = $RustToolchain
Remove-Item Env:CARGO_BUILD_TARGET -ErrorAction SilentlyContinue
Remove-Item Env:CARGO_TARGET_DIR -ErrorAction SilentlyContinue
$hostInfo = (& rustc -vV | Out-String)
if ($hostInfo -notmatch 'host: x86_64-pc-windows-msvc') { throw 'An x64 MSVC Rust toolchain is required' }
if (!(Test-Path (Join-Path $client '.git'))) { Run git @('clone','--depth','1','--branch','1.4.9','https://github.com/rustdesk/rustdesk.git',$client) }
$commit = (& git -C $client rev-parse HEAD | Out-String).Trim()
if ($commit -ne '6c578292e8ebbbec708b76986ba8c4bc7c509747') { throw 'Unexpected RustDesk baseline; preserving local checkout' }
Run git @('-C',$client,'submodule','update','--init','--recursive')
& git -C $client show-ref --verify --quiet refs/heads/rustdesk-control/1.4.9
if ($LASTEXITCODE -ne 0) { Run git @('-C',$client,'switch','-c','rustdesk-control/1.4.9') }
Apply-Patch $client (Join-Path $root 'managed-client/patches/rustdesk.patch')
Apply-Patch (Join-Path $client 'libs/hbb_common') (Join-Path $root 'managed-client/patches/hbb_common.patch')
$module = Join-Path $client 'src/managed_control'
New-Item -ItemType Directory -Force $module | Out-Null
Copy-Item "$root/managed-client/core/*.rs", "$root/managed-client/core/Cargo.toml", "$root/managed-client/core/Cargo.lock" $module -Force
$env:VCPKG_ROOT = (Resolve-Path $VcpkgRoot).Path
$env:VCPKG_DEFAULT_HOST_TRIPLET = 'x64-windows-static'
if (!$env:LIBCLANG_PATH) { $env:LIBCLANG_PATH = Split-Path (Get-Command clang).Source -Parent }
Push-Location $client
try {
    Run (Join-Path $VcpkgRoot 'vcpkg.exe') @('install','--triplet','x64-windows-static',"--x-install-root=$VcpkgRoot/installed")
    Run cargo @('install','cargo-expand','--version','1.0.95','--locked')
    Run cargo @('install','flutter_rust_bridge_codegen','--version','1.80.1','--features','uuid','--locked')
    Push-Location flutter
    try { Run flutter @('config','--enable-windows-desktop'); Run flutter @('pub','get') } finally { Pop-Location }
    # Always regenerate: an old bridge must not survive an FFI change.
    Run flutter_rust_bridge_codegen @('--rust-input','./src/flutter_ffi.rs','--dart-output','./flutter/lib/generated_bridge.dart','--c-output','./flutter/macos/Runner/bridge_generated.h')
    Run cargo @('build','--locked','--release','--lib','--features','managed-control,flutter,hwcodec')
    Push-Location flutter
    try { Run flutter @('build','windows','--release') } finally { Pop-Location }
    $bundle = Join-Path $client 'flutter/build/windows/x64/runner/Release'
    Copy-Item 'target/release/deps/dylib_virtual_display.dll' $bundle -Force
    Copy-Item "$root/managed-client/NOTICE" $bundle -Force
    Copy-Item "$root/LICENSE" $bundle -Force
    Copy-Item (Join-Path $client 'LICENCE') (Join-Path $bundle 'RUSTDESK-LICENCE') -Force
    Run python @("$root/scripts/collect-notices.py", '--client', $client, '--vcpkg', $VcpkgRoot, '--output', (Join-Path $bundle 'THIRD-PARTY-NOTICES.txt'))
    foreach ($file in @('rustdesk.exe','librustdesk.dll','flutter_windows.dll','data/flutter_assets')) {
        if (!(Test-Path (Join-Path $bundle $file))) { throw "Incomplete Windows bundle: missing $file" }
    }
    # Source and license files are inside the EXE payload, not separate release assets.
    Run python @("$root/scripts/package-source.py", '--client', $client, '--output', (Join-Path $bundle 'corresponding-source.tar.gz'))
    Copy-Item "$root/docs/LICENSING.md" (Join-Path $bundle 'SOURCE-CODE.md') -Force
    Push-Location libs/portable
    try {
        Run python @('-m','pip','install','-r','requirements.txt')
        Run python @('./generate.py','-f',$bundle,'-o','.','-e',(Join-Path $bundle 'rustdesk.exe'))
    } finally { Pop-Location }
    $output = Join-Path $root 'dist/windows-x64'
    New-Item -ItemType Directory -Force $output | Out-Null
    $installer = Join-Path $output 'rustdesk-managed-1.4.9-x86_64-install.exe'
    Copy-Item 'target/release/rustdesk-portable-packer.exe' $installer -Force
    Get-FileHash $installer -Algorithm SHA256 | Format-List
    Write-Host "Complete unsigned Windows EXE: $installer"
    Write-Host 'Run it and use the normal RustDesk Install action (UAC is retained). Managed enrollment requires the installed service.'
} finally { Pop-Location }
