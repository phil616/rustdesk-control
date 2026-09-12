# rustdesk-control

RustDesk OSS 集中管理控制台：Go + SQLite + React，以及独立的受管 RustDesk
客户端源码改动。仅管理组织拥有或明确授权的设备。客户端保留原有远程桌面协议，
在 About 显示 **Managed by rustdesk-control · Modified version** 和连接状态。

## Architecture

```text
Managed RustDesk ── HTTPS / Ed25519 ── Go API + embedded React + SQLite
       │
       └── original RustDesk protocol ── official OSS hbbs / hbbr
```

控制台不替代 hbbs/hbbr，不实现 Pro API Server。普通客户端的 ID Server、Relay、Key
由控制台全局策略分发，API Server 固定为空。设备 UUID/Ed25519 身份与 RustDesk ID
相互独立。控制台原始规范见 [TASK.md](TASK.md)，协议见 [docs/PROTOCOL.md](docs/PROTOCOL.md)。

## Building rustdesk-control

需要 Go 1.26 和 Node.js 22.12+（本次使用 Node 24）。

```sh
./scripts/build.sh
./bin/rustdesk-control --help
```

`npm run build` 输出 `web/dist`，Go `embed` 将其打入单一 executable。全新 checkout
必须先构建前端，再运行 `go test ./...` 或构建 Go。`web/dist` 不提交到 Git。

```sh
go test -race ./...
go vet ./...
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/rustdesk-control.exe ./cmd/rustdesk-control
```

## Environment variables / Running the server

- `RUSTDESK_CONTROL_MASTER_KEY`：32 字节随机密钥的标准 Base64，始终必需。
- `RUSTDESK_CONTROL_ADMIN_PASSWORD`：第一次创建数据库时必需，至少 12 字符；以后忽略。
- `--listen`：默认 `:8080`，建议代理部署时指定 `127.0.0.1:8080`。
- `--database`：默认 `./data/rustdesk-control.db`。

使用组织的秘密管理工具保存并注入环境变量；可以用 `openssl rand -base64 32` 生成
master key。密钥不自动写入数据库，不随数据库备份一起公开。管理员初始密码完成
初始化后可从运行环境移除。

```sh
./bin/rustdesk-control --listen 127.0.0.1:8080 --database ./data/rustdesk-control.db
```

迁移 SQL 随二进制嵌入，启动时按 `schema_migrations` 自动执行。SQLite 启用 WAL、
foreign keys 和 busy timeout。数据库目录默认 0700，数据库文件 0600。

## Reverse proxy example

例如 Caddy（域名仅为示例）：

```caddyfile
control.example.com {
    reverse_proxy 127.0.0.1:8080
}
```

浏览器和 API 必须 same-origin，代理保留原始 Host。必须使用 HTTPS；Session Cookie
始终 Secure。后端默认不信任 forwarding headers，不开放 CORS。使用本机代理时显式设置
`--trusted-proxies 127.0.0.1/32,::1/128`，按可信代理链右侧首个非可信 IP 限流。
未指定时按 TCP 对端 IP 限流：bootstrap 60/min、enrollment 30/min、登录 5/min。
代理必须覆盖或正确追加 X-Forwarded-For；不要把公网全网段设为可信代理。
不要开启请求/响应正文日志，尤其是登录、heartbeat 和 credential 路径。

## RustDesk OSS server requirements

独立运行官方 RustDesk Server OSS `hbbs` 和 `hbbr`，按官方部署方式开放其需要的
TCP/UDP 端口。把 hbbs 公钥填入控制台 Settings。支持 hostname/IP 和可选端口，
IPv6 携带端口时使用 `[address]:port`。公钥要求标准 Base64 编码的 32 字节。
先保存有效服务器配置，再安装受管客户端；未配置的 bootstrap 不会被客户端应用。

Windows 完整客户端构建请直接阅读 [Windows 构建指南](docs/WINDOWS-BUILD.md)。

## Building Managed RustDesk

两个工程保持分离：

- 当前仓库：控制台、协议、测试、客户端可复现 overlay/patch。
- `rustdesk-managed-client/`：独立 upstream checkout，有自己的 Git 历史和维护分支。

固定基线：`rustdesk/rustdesk` tag **1.4.9**，完整 commit
`6c578292e8ebbbec708b76986ba8c4bc7c509747`；hbb_common 固定
`7e1c392c62d39c364127307cd408421dd5f8cfb0`。

```sh
./scripts/prepare-client.sh
export RUSTDESK_CONTROL_URL=https://control.example.com
export RUSTDESK_MANAGED_SOURCE_URL=https://source.example.com/your-modified-client
./scripts/build-client.sh
```

Windows 在准备好的源码中运行 `scripts/build-client.ps1`；构建需要 upstream 的
MSVC、vcpkg 原生库、Flutter SDK 等完整工具链。Linux 可使用 upstream 的
`linux-pkg-config` feature，但需准备兼容的 libyuv/libvpx/opus、GTK、GStreamer、
PulseAudio、PAM、X11/Wayland 等开发依赖。

在初始化 Flutter SDK 并安装该 tag 对应 Dart 依赖后，安装上游 bridge generator：

```sh
cargo install flutter_rust_bridge_codegen --version 1.80.1 --features uuid --locked
(cd rustdesk-managed-client/flutter && flutter pub get)
```

构建脚本在缺少 `src/bridge_generated.rs` 时先运行 bridge generator。
构建脚本生成 RustDesk Flutter 原生库；使用该 tag 的 upstream Flutter 桌面打包流程
将库放入 Flutter bundle，再生成并安装对应平台软件包。`cargo build` **不等于**
完整的 Flutter 安装包；本仓库没有将未验收的库称为可部署客户端。

`managed-control` 默认关闭。不开启时不会编译 agent 或管理钩子。
开启时 release 缺少任意 URL 都失败；HTTP 被拒绝。调试构建未设 URL 时仅使用
`https://unconfigured.invalid`，不会联系任何真实生产服务。Control URL 必须为
HTTPS origin，没有用户名、密码、路径、query 或 fragment；Source URL 可含路径。
正常系统 CA 验证始终启用，不提供 insecure fallback。

核心单元测试可以独立运行，不需要桌面/编解码器工具链：

```sh
cargo test --manifest-path rustdesk-managed-client/src/managed_control/Cargo.toml
```

Linux/macOS 管理 agent 在现有 root OS service 中启动；Windows 在现有 LocalSystem
`--server` 生命周期中启动。GUI/tray/connection manager 不启动 agent。进程内 Once +
服务配置目录下的排他文件锁保证同一服务身份仅一个实例。Windows 必须正常安装服务，
便携版或普通用户手动启动的 server 不 enroll。私钥保存在服务账户受限配置目录的
`managed-control/identity.json`，不传给用户态同步通道。

Linux 通过已有的 root/service Config IPC，将受管设置和 RustDesk 自有密码存储同步到
会话 server；仅添加受管字段的持续拉取，不创建新 IPC 协议或第二个 OS service。
密码更新调用内部 setter，普通 UI/CLI 仍受锁限制。没有密码 argv、shell 或 TOML 手工编辑。

## Enrollment / Approval workflow

1. 启动控制台，HTTPS 登录 `/login`，在 Settings 保存 OSS Server 参数。
2. 安装受管客户端，正常启用 RustDesk 自有服务。
3. 客户端生成 UUID/Ed25519 身份，bootstrap 后自签名注册为 pending。
4. 管理员在 Approvals 查看设备，并二次确认 Approve。
5. 服务器创建受管密码，下次 heartbeat 下发，客户端应用后回报版本。
6. Device Detail 提供 Copy ID、Reveal Password、Copy Password；使用普通 RustDesk
   客户端输入 ID 和密码连接。没有携带密码的连接 URL。

设备列表每 15 秒刷新。最后 heartbeat 距今不超过 120 秒显示 Online。
heartbeat 为 60±10 秒，失败按 5/10/30/60/300/600 秒退避；成功后立即进入正常心跳。
控制台离线时不清理已保存的服务器参数和密码，不阻塞 RustDesk 启动。

## Password security model

- 审批和轮换生成 24 字节 crypto/rand 随机数，Base64URL 后 32 字符（192 bits entropy）。
- SQLite 只存 AES-256-GCM nonce/ciphertext；设备 ID 作为 associated data 防止跨设备替换。
- 管理员密码存 Argon2id hash（64 MiB，3 次迭代，2 lanes，随机 16-byte salt）。
- Session 只存 token hash、CSRF token 和 24 小时 expiry，logout 立即删除。
- 查看密码需要登录，记录 `credential revealed`，审计不含密码。
- 前端凭据仅为页面组件内存状态，跳转或隐藏清除，不进入 React Query cache 或 Web Storage。
- 修改管理员密码会结束所有现有 Session。
- Reject/Delete/Reset 停止未来下发，不能撤销离线设备已保存的密码或断开现有连接。
  需回收设备访问时，应结合组织的设备回收/网络访问流程。

## Fake agent / Integration tests

```sh
go run ./cmd/fake-agent --url https://control.example.com
```

每次运行生成测试身份，模拟 enroll/heartbeat/版本回报，不创建真实远控能力、不打印密码。
`--once` 单次运行。Go integration test 使用临时 TLS 服务器和临时数据库覆盖审批及轮换。

浏览器测试仅使用临时测试数据和自签名测试 TLS，不改变 agent 的证书验证行为：

```sh
# terminal 1: prints a temporary HTTPS URL
 go run ./scripts/browser
# terminal 2
 cd web
 RDC_TEST_URL=https://127.0.0.1:PORT npx playwright test
```

## Upgrade strategy

客户端修改集中在 `managed-client/core` 及少量 upstream hooks，保存为两个可审查 patch。
`prepare-client.sh` 校验固定 commit，patch 可重复执行；冲突时保留本地修改并停止。
更新 overlay 后需同步到客户端源码并重新生成 patch。升级 upstream 时重新建立固定 tag
维护分支、重放 patch，重点复测内部 setter、配置同步、服务账户身份和 Flutter About。
Go 后端与客户端分开发布；协议版本当前为 1。

## Backup SQLite / Restore SQLite

停止控制台后备份整个数据目录（包含可能存在的 `-wal` 和 `-shm`），另行安全备份 master
key。运行中备份应使用 SQLite Online Backup API 或 `sqlite3 database '.backup backup.db'`，
不要只复制活动数据库主文件。备份含管理员 hash 和加密凭据，仍须限制访问权限。

恢复时停止服务，把一致性备份放回 `--database` 位置，恢复正确文件权限，注入**相同**
master key 后启动。丢失 master key 无法恢复已有密码。恢复旧数据库会恢复旧密码版本，
需要让客户端重新同步；恢复后建议轮换凭据、结束旧 Session，并复查审计。

## AGPL considerations

本项目管理代码采用 AGPL-3.0-only，见 LICENSE。保留 upstream copyright/license/AGPL
notice；客户端新增 NOTICE 标注修改日期和用途。发布者必须提供完整对应修改源码，
包括 RustDesk、修改后的 hbb_common、模块及构建脚本，并将实际可访问地址写入
`RUSTDESK_MANAGED_SOURCE_URL`。Go Web 应用源码也应按许可证要求向网络用户提供。
示例域名不是可用于正式分发的源码地址。

## Validation

实际执行结果和无法在本环境完成的验收记录在 [docs/VALIDATION.md](docs/VALIDATION.md)。
尤其是 Windows/Linux 安装、重启、真实远程连接和控制台离线场景，不能用单元测试替代。
