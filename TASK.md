# rustdesk-control — Codex 实现任务文档

## 1. 项目目标

实现一个名为：

`rustdesk-control`

的 RustDesk OSS 集中管理系统。

系统由两个明确分离的部分组成：

1. `rustdesk-control`

   * Go 后端
   * React Web 管理端
   * SQLite 数据库

2. `rustdesk-managed-client`

   * 基于 RustDesk OSS 官方客户端源码修改
   * 保留 RustDesk 原有远程桌面能力
   * 增加一个很薄的 `rustdesk-control` 管理模块

项目仅用于组织拥有、管理或者已得到明确授权管理的设备。

不得实现隐藏安装、隐藏运行、绕过用户安全提示、关闭操作系统安全机制、反检测、持久化逃逸等能力。

RustDesk 客户端中必须能够看到：

`Managed by rustdesk-control`

或等效受管状态说明。

---

# 2. RustDesk 基线

不要直接使用 upstream `master`。

必须：

```text
repository: rustdesk/rustdesk
tag: 1.4.9
commit: 6c57829
```

基于该版本建立自己的维护分支。

例如：

```text
rustdesk-control/1.4.9
```

不要修改：

* RustDesk 通信协议
* hbbs 协议
* hbbr 协议
* NAT 穿透协议
* 中继协议
* 屏幕传输协议

除非实现本任务确实无法避免，否则不得修改 RustDesk 核心远程控制逻辑。

目标是最小侵入修改。

---

# 3. 总体架构

最终架构：

```text
                         HTTPS
┌─────────────────┐   management API
│ Managed RustDesk│ ─────────────────────┐
│ Client A        │                      │
└────────┬────────┘                      │
         │                               ▼
         │                     ┌──────────────────────┐
         │                     │ rustdesk-control     │
         │                     │                      │
         │                     │ Go API               │
         │                     │ React WebGUI         │
         │                     │ SQLite               │
         │                     └──────────────────────┘
         │
         │ RustDesk protocol
         ▼
┌────────────────────────────┐
│ RustDesk Server OSS        │
│                            │
│ hbbs        hbbr           │
└────────────────────────────┘
```

`rustdesk-control` 不是 hbbs/hbbr 的替代品。

hbbs/hbbr 继续运行官方 RustDesk Server OSS。

rustdesk-control 只负责：

* 客户端注册
* RustDesk Server 参数分发
* 设备清单
* 设备在线状态
* 企业受管固定密码
* 密码轮换
* 管理审计

---

# 4. 固定 Control URL

修改后的 RustDesk 客户端必须包含一个编译期固定的 Control Plane URL。

例如编译变量：

```text
RUSTDESK_CONTROL_URL
```

Release 构建时必须指定：

```text
https://control.example.com
```

这里只是示例。

不要在源码中写死真实生产域名。

Rust 代码从编译期读取该变量。

普通最终用户：

* 不需要填写 rustdesk-control 地址
* 不提供修改 Control URL 的普通 UI
* 不通过 RustDesk Server 配置决定 Control URL

Release 构建如果没有设置该变量：

构建必须失败。

只允许：

```text
https://
```

生产构建拒绝：

```text
http://
```

---

# 5. RustDesk Server 配置管理

rustdesk-control 后台保存全局 RustDesk Server 配置：

```text
ID Server
Relay Server
Public Key
```

例如：

```text
ID Server:
rustdesk.example.com

Relay Server:
rustdesk.example.com

Key:
xxxxxxxxxxxxxxxxxxxxxxxx
```

由于使用 OSS Server：

第一版不实现 RustDesk Pro API Server。

API Server 保持空。

---

# 6. 客户端配置 RustDesk 的方式

禁止：

* 手工编辑 RustDesk TOML
* shell 调用 rustdesk.exe
* 启动外部 PowerShell
* 调用外部 bash
* 修改注册表模拟 RustDesk 设置

必须直接复用 RustDesk 内部配置逻辑。

主要配置项：

```text
custom-rendezvous-server
relay-server
key
api-server
```

其中：

```text
api-server = ""
```

优先复用 RustDesk 当前自己的：

```rust
ui_interface::set_option(...)
```

或者对应 IPC/config 内部路径。

必须按照 RustDesk 自己“Network Settings / --config”使用的配置路径执行，而不是另外创建配置格式。

应用配置后不得删除：

* RustDesk ID
* RustDesk 本地密钥
* 历史记录
* 用户其它非受管配置

---

# 7. Managed Control Agent

在 RustDesk 客户端源码中增加模块：

```text
src/managed_control/
```

建议结构：

```text
managed_control/
├── mod.rs
├── agent.rs
├── api.rs
├── auth.rs
├── device.rs
├── policy.rs
└── storage.rs
```

不要创建第二个操作系统后台服务。

Managed Agent 必须运行在 RustDesk 已有后台/server 生命周期中。

不要让：

* GUI
* tray
* connection manager
* 多个 RustDesk 进程

分别启动一份 agent。

整个设备同时只能存在一个有效 Agent 实例。

---

# 8. Agent 启动流程

RustDesk 后台进程启动后执行：

```text
RustDesk 初始化
        ↓
启动 Managed Agent
        ↓
读取本地 managed identity
        ↓
不存在 → 创建 identity
        ↓
访问 rustdesk-control
        ↓
获取 RustDesk Server 配置
        ↓
应用 ID Server / Relay / Key
        ↓
读取 RustDesk ID
        ↓
注册设备
        ↓
进入 heartbeat loop
```

Control Plane 无法访问时：

不得阻止 RustDesk 启动。

继续使用最后一次成功获取的配置。

---

# 9. 独立设备身份

不要把 RustDesk ID 当成 Control Plane 身份认证。

每个安装实例生成独立：

```text
device_uuid
Ed25519 key pair
```

建议使用 RustDesk 已有加密依赖完成 Ed25519。

不要自己实现密码学算法。

本地保存：

```text
device_uuid
private_key
public_key
```

第一次注册时：

客户端将：

```text
device_uuid
public_key
rustdesk_id
hostname
os
os_version
arch
rustdesk_version
managed_client_version
```

发送给服务器。

不得收集：

* 用户文件
* 浏览器信息
* 剪贴板
* 屏幕内容
* 键盘输入
* 用户密码
* 操作系统登录密码

---

# 10. Enrollment

第一版采用：

**Zero-touch enrollment + Admin approval**

模式。

客户端第一次出现时：

```text
POST /api/v1/agent/enroll
```

Control Plane 创建：

```text
status = pending
```

此时管理员已经可以在后台看到设备。

但是：

`pending` 设备不能获得受管 RustDesk 密码。

管理员在 WebGUI 点击：

```text
Approve
```

之后状态：

```text
approved
```

服务器才给设备分配 Managed Password。

这样不需要把一个共享 enrollment secret 固化进所有客户端。

---

# 11. Enrollment 请求签名

客户端使用自己的 Ed25519 private key 对 enrollment 请求签名。

服务器保存 public key。

Enrollment payload 至少：

```json
{
  "device_uuid": "uuid",
  "public_key": "base64",
  "rustdesk_id": "123456789",
  "hostname": "DESKTOP-001",
  "os": "windows",
  "os_version": "11",
  "arch": "x86_64",
  "rustdesk_version": "1.4.9",
  "managed_client_version": "1.0.0",
  "timestamp": 0
}
```

服务器必须验证：

* 签名正确
* timestamp 合理
* UUID 格式正确
* public key 格式正确

同一个 `device_uuid` 已经存在且 public key 不同时：

拒绝自动覆盖。

必须由管理员执行：

```text
Reset Identity
```

才可以重新 enrollment。

---

# 12. 后续 Agent 请求认证

所有后续 Agent API 使用 Ed25519 请求签名。

请求 header：

```text
X-RDC-Device-ID
X-RDC-Timestamp
X-RDC-Nonce
X-RDC-Signature
```

Canonical payload：

```text
HTTP_METHOD
REQUEST_PATH
TIMESTAMP
NONCE
SHA256(BODY)
```

服务器：

1. 查询 device public key
2. 校验 timestamp
3. 校验签名
4. 校验 nonce
5. 才处理请求

Timestamp 最大偏差：

```text
5 minutes
```

---

# 13. Heartbeat

客户端每：

```text
60 seconds
```

发送 heartbeat。

加入：

```text
±10 seconds jitter
```

防止大量设备同时请求。

API：

```text
POST /api/v1/agent/heartbeat
```

Body：

```json
{
  "rustdesk_id": "123456789",
  "hostname": "DESKTOP-001",
  "rustdesk_version": "1.4.9",
  "managed_client_version": "1.0.0",
  "policy_version": 4,
  "password_version": 2
}
```

服务器更新：

```text
last_seen_at
rustdesk_id
hostname
versions
```

---

# 14. 在线状态

服务器不需要 WebSocket 判断 Agent 是否在线。

定义：

```text
now - last_seen_at <= 120 seconds
```

则：

```text
online
```

否则：

```text
offline
```

前端每 15 秒刷新设备列表。

---

# 15. Policy Response

Heartbeat response 同时作为配置下发机制。

例如：

```json
{
  "server_time": 0,
  "device_status": "approved",
  "policy_version": 5,
  "rustdesk": {
    "id_server": "rustdesk.example.com",
    "relay_server": "rustdesk.example.com",
    "key": "xxxxxxxx",
    "api_server": ""
  },
  "managed_access": {
    "enabled": true,
    "password": "SECRET",
    "password_version": 2,
    "lock_password": true
  }
}
```

如果：

```text
device_status != approved
```

则不得返回：

```text
password
```

---

# 16. Policy Version

SQLite 全局维护：

```text
policy_version
```

管理员修改：

* ID Server
* Relay Server
* Key

任意一项：

```text
policy_version++
```

客户端 heartbeat 发现版本不同：

应用新配置。

应用成功后：

下一个 heartbeat 回报新的版本。

---

# 17. 企业受管固定密码

不要尝试：

```text
读取用户当前 RustDesk 固定密码
```

不要尝试：

```text
上传 RustDesk 用户原有密码
```

rustdesk-control 自己创建：

```text
Managed Permanent Password
```

管理员批准设备时生成。

使用加密安全随机数。

至少：

```text
128 bits entropy
```

建议生成约 24～32 个 Base64URL 字符。

例如逻辑：

```text
crypto/rand
↓
random bytes
↓
base64 RawURLEncoding
```

---

# 18. 客户端设置 Managed Password

客户端收到：

```text
managed_access.password
```

以后直接调用 RustDesk 内部固定密码设置逻辑。

不得执行：

```text
rustdesk --password PASSWORD
```

不得通过 shell。

不得把密码放进 command line。

复用 RustDesk：

```text
Config::set_permanent_password(...)
```

或者其内部等效函数。

设置成功后：

```text
password_version = server version
```

存储到 Managed Agent 自己的状态中。

---

# 19. 固定密码策略

受管设备启用：

```text
verification-method=use-permanent-password
approve-mode=password
```

成功设置受管密码以后：

```text
disable-change-permanent-password=Y
```

这样普通用户不能通过 RustDesk UI 或 CLI 修改 Managed Password。

Managed Agent 内部仍然必须能够执行管理员发起的 password rotation。

不要为了实现 rotation 暴露一个新的公共 CLI。

---

# 20. 密码轮换

管理后台提供：

```text
Rotate Password
```

执行：

```text
生成新随机密码
        ↓
password_version++
        ↓
加密保存
        ↓
下次 heartbeat 下发
        ↓
客户端设置
        ↓
客户端回报 password_version
```

WebGUI 显示：

```text
Password synced
```

或者：

```text
Password pending
```

---

# 21. 密码数据库安全

绝对不能把固定密码明文保存到 SQLite。

服务器需要：

```text
RUSTDESK_CONTROL_MASTER_KEY
```

环境变量。

格式：

```text
32-byte random key
Base64 encoded
```

使用：

```text
AES-256-GCM
```

保存：

```text
nonce
ciphertext
```

SQLite 中不得出现明文密码。

应用启动时如果没有：

```text
RUSTDESK_CONTROL_MASTER_KEY
```

必须拒绝启动。

不得自动生成并写进 SQLite。

---

# 22. 管理员查看密码

WebGUI Device Detail 页面提供：

```text
Reveal Password
Copy Password
```

后端：

```text
GET /api/v1/admin/devices/{id}/credential
```

只有管理员已认证时允许访问。

响应必须：

```text
Cache-Control: no-store
```

密码：

* 不写 access log
* 不写 application log
* 不写 audit log 内容
* 不进入 URL
* 不进入 query string

Audit log 只记录：

```text
credential revealed
```

不得记录：

```text
具体 password
```

---

# 23. 不实现密码 URL 自动连接

第一版不要实现：

```text
rustdesk://...?password=xxx
```

也不要让浏览器生成：

```text
rustdesk --connect ID --password PASSWORD
```

原因：

密码可能进入：

* URL history
* shell history
* process argv
* browser logs
* reverse proxy logs

WebGUI 设备页面提供：

```text
RustDesk ID
Copy ID
Reveal Password
Copy Password
```

即可。

后续如果需要真正的一键连接，应单独设计本地管理员 Helper/IPC，不应使用 URL 携带明文密码。

---

# 24. Go 后端

项目名称：

```text
rustdesk-control
```

使用：

```text
Go
database/sql
SQLite
net/http
chi
```

SQLite driver：

```text
modernc.org/sqlite
```

原因：

避免 CGO，方便部署单一二进制文件。

不要使用大型 ORM。

SQL 必须明确可读。

---

# 25. Go 项目结构

建议：

```text
rustdesk-control/
├── cmd/
│   └── rustdesk-control/
│       └── main.go
│
├── internal/
│   ├── api/
│   ├── agent/
│   ├── admin/
│   ├── auth/
│   ├── config/
│   ├── crypto/
│   ├── database/
│   ├── device/
│   ├── policy/
│   ├── audit/
│   └── web/
│
├── migrations/
│
├── web/
│   ├── src/
│   ├── package.json
│   └── vite.config.ts
│
├── go.mod
├── README.md
└── LICENSE
```

---

# 26. React 前端

使用：

```text
React
TypeScript
Vite
Ant Design
TanStack Query
React Router
```

不要自己制作：

* Button
* Table
* Modal
* Form
* Input
* Pagination
* Notification
* Layout

优先使用 Ant Design。

界面必须完全响应式。

桌面端优先。

---

# 27. 前端页面

第一版必须包含：

```text
/login

/dashboard

/devices

/devices/:id

/approvals

/settings

/audit
```

---

# 28. Dashboard

显示：

```text
Total Devices
Online
Offline
Pending Approval
Password Pending
```

并显示最近设备。

---

# 29. Devices

使用 Ant Design Table。

列：

```text
RustDesk ID

Hostname

OS

RustDesk Version

Managed Client Version

Status

Approval Status

Password Status

Last Seen

Actions
```

支持：

```text
搜索
分页
online/offline filter
pending/approved filter
```

搜索：

```text
RustDesk ID
Hostname
```

---

# 30. Device Detail

显示：

```text
Device UUID
RustDesk ID
Hostname
OS
Architecture

RustDesk Version
Managed Client Version

First Seen
Last Seen

Online Status

Approval Status
Policy Version
Password Version
```

操作：

```text
Approve
Reject
Rotate Password
Reveal Password
Copy ID
Copy Password
Reset Identity
Delete Device
```

危险操作必须二次确认。

---

# 31. Settings

设置：

```text
ID Server
Relay Server
RustDesk Public Key
```

点击 Save：

验证字段。

成功后：

```text
policy_version++
```

前端明确提示：

```text
This configuration will be distributed to all managed devices.
```

---

# 32. Admin Authentication

第一版只需要一个管理员账户体系。

首次启动通过：

```text
RUSTDESK_CONTROL_ADMIN_PASSWORD
```

初始化 admin。

密码使用：

```text
Argon2id
```

保存 hash。

不得保存明文。

之后允许管理员在 WebGUI 修改密码。

---

# 33. Session

使用服务器 session。

不要使用长期 JWT LocalStorage 模式。

Cookie：

```text
HttpOnly
Secure
SameSite=Strict
```

Session 保存 SQLite。

默认有效时间：

```text
24 hours
```

Logout 立即失效。

---

# 34. CSRF

所有管理写操作必须有 CSRF 防护。

前端和 API 使用 same-origin。

不要默认开放 CORS。

---

# 35. 数据库

至少建立：

```text
admins

admin_sessions

settings

devices

device_auth

device_credentials

audit_logs
```

---

# 36. devices

字段建议：

```text
id
device_uuid

rustdesk_id
hostname

os
os_version
arch

rustdesk_version
managed_client_version

status

first_seen_at
last_seen_at

approved_at

applied_policy_version
applied_password_version

created_at
updated_at
```

`device_uuid`：

```text
UNIQUE
```

RustDesk ID 建 index。

不要假定 RustDesk ID 永远不会改变。

---

# 37. device_auth

```text
device_id
ed25519_public_key
created_at
updated_at
```

---

# 38. device_credentials

保存：

```text
device_id
password_version
nonce
ciphertext
created_at
updated_at
```

不得有：

```text
password TEXT
```

---

# 39. settings

第一版只有一套全局配置。

字段：

```text
id_server
relay_server
public_key
policy_version
updated_at
```

---

# 40. Audit Log

必须记录：

```text
admin login

device approved

device rejected

device deleted

identity reset

credential revealed

password rotated

RustDesk server settings changed
```

记录：

```text
timestamp
admin
action
device
metadata
```

禁止记录任何密码。

---

# 41. Admin API

实现：

```text
POST /api/v1/admin/login
POST /api/v1/admin/logout

GET  /api/v1/admin/session

GET  /api/v1/admin/dashboard

GET  /api/v1/admin/devices

GET  /api/v1/admin/devices/{id}

POST /api/v1/admin/devices/{id}/approve
POST /api/v1/admin/devices/{id}/reject

POST /api/v1/admin/devices/{id}/rotate-password

GET  /api/v1/admin/devices/{id}/credential

POST /api/v1/admin/devices/{id}/reset-identity

DELETE /api/v1/admin/devices/{id}

GET /api/v1/admin/settings/rustdesk
PUT /api/v1/admin/settings/rustdesk

GET /api/v1/admin/audit
```

---

# 42. Agent API

实现：

```text
GET  /api/v1/agent/bootstrap

POST /api/v1/agent/enroll

POST /api/v1/agent/heartbeat
```

不要创建不必要的大型 API。

---

# 43. Bootstrap API

允许未 enrollment 客户端访问。

只返回非秘密数据：

```json
{
  "protocol_version": 1,
  "rustdesk": {
    "id_server": "rustdesk.example.com",
    "relay_server": "rustdesk.example.com",
    "key": "PUBLIC_KEY",
    "api_server": ""
  },
  "policy_version": 1
}
```

RustDesk Server 的 public key 本身不是 Control Plane 身份秘密。

Bootstrap API：

必须 rate limit。

---

# 44. Agent Retry

网络失败：

指数退避：

```text
5s
10s
30s
60s
5m
10m
```

最大：

```text
10 minutes
```

连接恢复：

立即 heartbeat。

不要因为 rustdesk-control 暂时离线影响 RustDesk 原有远程桌面能力。

---

# 45. TLS

Managed Client 只允许 HTTPS Control URL。

必须进行正常 CA certificate validation。

绝对不要：

```text
skip TLS verification
```

不要启用 RustDesk：

```text
allow-insecure-tls-fallback=Y
```

受管版本应保持：

```text
allow-insecure-tls-fallback=N
```

---

# 46. 日志

客户端可以记录：

```text
Control plane connected

Enrollment completed

Policy version updated

RustDesk server config updated

Managed password applied

Heartbeat failed
```

不得记录：

```text
managed password
private key
administrator password
master encryption key
```

---

# 47. WebGUI 安全要求

所有密码相关 response：

```text
Cache-Control: no-store
```

前端密码默认：

```text
••••••••
```

只有用户主动点击：

```text
Reveal
```

才请求后端。

页面切换后：

清除前端内存中的 credential。

不得保存：

```text
LocalStorage
SessionStorage
IndexedDB
```

---

# 48. RustDesk UI 修改

不要大规模修改 Flutter UI。

只增加一个非常小的受管状态展示。

在 About 或 Settings 合适位置显示：

```text
Managed by rustdesk-control
```

以及：

```text
Control status: Connected / Offline
```

不要显示：

```text
Managed Password
```

---

# 49. AGPL / Source Notice

保留 RustDesk 原有：

```text
copyright
license
AGPL notices
```

不得删除。

对修改过的 RustDesk Client：

必须明显标记这是 modified version。

Release 构建增加：

```text
RUSTDESK_MANAGED_SOURCE_URL
```

About 页面提供：

```text
Source Code
```

入口。

Release 构建没有 Source URL：

构建失败。

修改日期和项目说明写入：

```text
NOTICE
```

不要把 `rustdesk-control` Go 后端源码直接混入 RustDesk 源码模块。

保持：

```text
Control Plane

Modified RustDesk Client
```

为两个清晰分离的代码工程。

---

# 50. Upstream 可维护性

这是重要要求。

所有 RustDesk 修改尽量集中在：

```text
managed_control/
```

只在必要位置增加极少量 hook。

例如：

```text
mod managed_control;
```

以及：

```text
managed_control::start();
```

避免修改几十个 upstream 文件。

目标：

未来升级：

```text
1.4.9
    ↓
1.4.x / 1.5.x
```

时能够方便 rebase。

---

# 51. Feature Flag

Managed Client 增加 Cargo Feature：

```text
managed-control
```

官方兼容构建：

```text
cargo build
```

不启用 managed control。

企业构建：

```text
cargo build --features managed-control
```

只有开启该 feature：

才编译 Agent。

---

# 52. 支持平台

第一版本正式验收：

```text
Windows x86_64
Linux x86_64
```

Linux 重点：

```text
Debian
Ubuntu
```

不要在第一版本加入：

```text
Android
iOS
Web Client
```

macOS 保持源码尽可能可编译，但不作为第一版验收目标。

---

# 53. React 构建和 Go Embed

运行：

```text
npm run build
```

生成：

```text
web/dist
```

Go 使用：

```go
//go:embed
```

把前端打进最终 Go binary。

最终发布：

```text
rustdesk-control
```

单一 Go executable。

SQLite 文件作为外部运行数据。

---

# 54. Runtime

默认：

```text
./rustdesk-control
```

读取：

```text
RUSTDESK_CONTROL_MASTER_KEY
RUSTDESK_CONTROL_ADMIN_PASSWORD
```

支持：

```text
--listen
--database
```

例如：

```text
--listen :8080
--database ./data/rustdesk-control.db
```

生产 TLS 推荐由：

```text
Caddy
Nginx
Traefik
```

等反向代理负责。

Go 应正确处理 reverse proxy，但不得无条件信任来自公网的伪造 forwarding headers。

---

# 55. 数据库迁移

使用嵌入式 SQL migration。

启动自动执行 migration。

不要要求管理员手工运行 SQL。

migration 必须带 schema version。

---

# 56. 测试

Go 必须覆盖：

```text
Admin authentication

Session validation

Password encryption/decryption

Ed25519 request validation

Timestamp rejection

Invalid signature rejection

Device enrollment

Device approval

Password rotation

Policy version update

Heartbeat

Online/offline calculation
```

---

# 57. Rust 测试

至少测试：

```text
canonical request generation

request signing

policy parsing

invalid policy rejection

policy version comparison

password version comparison
```

不得为了单元测试真实连接生产 RustDesk Server。

---

# 58. Integration Test

实现一个独立 fake agent。

它能够：

```text
生成 Ed25519 key
enroll
heartbeat
receive policy
report version
```

用于完整测试 Control Plane。

不要要求每次 API 测试都启动真正的 RustDesk GUI。

---

# 59. Windows 验收

必须实际验证：

```text
安装修改版 RustDesk
        ↓
无需输入服务器配置
        ↓
自动获得 ID Server / Relay / Key
        ↓
后台注册 rustdesk-control
        ↓
WebGUI 出现 Pending Device
        ↓
管理员 Approve
        ↓
客户端获得 Managed Password
        ↓
RustDesk 使用 permanent-password
        ↓
管理员能够使用 RustDesk ID + Managed Password 登录
```

重启 Windows 后：

仍然正常。

---

# 60. Linux 验收

Debian/Ubuntu 安装以后执行同样流程。

必须验证 system service/reboot 场景。

不能依赖：

```text
用户手工启动 GUI
```

才能 heartbeat。

---

# 61. Control Plane 离线验收

关闭 rustdesk-control。

Managed RustDesk：

必须继续使用之前已经保存的：

```text
ID Server
Relay Server
Key
Managed Password
```

RustDesk Remote Desktop 仍然可以工作。

Control Plane 恢复以后：

客户端自动恢复 heartbeat。

---

# 62. RustDesk Server 配置修改验收

后台把：

```text
ID Server A
```

改成：

```text
ID Server B
```

保存。

应当：

```text
policy_version++
```

所有 Managed Client 最迟在正常 heartbeat 周期内收到新配置。

不要求重新安装客户端。

---

# 63. Password Rotation 验收

管理员点击：

```text
Rotate Password
```

旧密码不得继续作为管理后台展示密码。

客户端接收到新版本后：

应用新密码。

随后：

```text
password_version
```

同步。

WebGUI 显示：

```text
Synced
```

---

# 64. 不允许出现的实现

不得：

```text
从 RustDesk 配置文件提取用户原来的明文密码
```

不得：

```text
抓取 random temporary password
```

不得：

```text
把 password 写入日志
```

不得：

```text
把 password 放进 URL
```

不得：

```text
通过 RustDesk CLI argv 下发 managed password
```

不得：

```text
禁用 TLS certificate validation
```

不得：

```text
隐藏 Managed 状态
```

不得：

```text
修改 RustDesk Server 协议实现一套私有后门
```

不得：

```text
为了方便而取消管理员登录
```

---

# 65. README

README 必须包含：

```text
Architecture

Building rustdesk-control

Building Managed RustDesk

Environment variables

Running the server

Reverse proxy example

RustDesk OSS server requirements

Enrollment workflow

Approval workflow

Password security model

Upgrade strategy

Backup SQLite

Restore SQLite

AGPL considerations
```

---

# 66. Codex 工作方式

不要只生成设计稿。

不要停留在 TODO。

不要分阶段等待人工确认。

直接：

1. 创建 rustdesk-control 完整工程。
2. 下载并 checkout RustDesk 1.4.9。
3. 建立 managed-control feature。
4. 实现 Managed Agent。
5. 实现 Go API。
6. 实现 SQLite migrations。
7. 实现 React WebGUI。
8. 实现认证。
9. 实现 enrollment。
10. 实现 heartbeat。
11. 实现 policy。
12. 实现 managed password。
13. 实现 password rotation。
14. 实现 audit。
15. 编写测试。
16. 编写构建脚本。
17. 编写 README。
18. 尝试完成 Windows/Linux 构建验证。
19. 修复编译错误。
20. 最后输出修改说明和仍然存在的真实限制。

发现 RustDesk 1.4.9 源码结构与本文描述有小范围差异时：

以实际源码为准。

但是：

不得因此改变整体架构。

---

# 67. 最终完成标准

完成以后，我应该能够部署：

```text
RustDesk Server OSS
+
rustdesk-control
+
Modified RustDesk Client
```

然后新电脑只需要：

```text
安装 Modified RustDesk Client
```

不需要用户配置：

```text
ID Server
Relay Server
Key
Control URL
```

设备自动出现在：

```text
rustdesk-control WebGUI
```

管理员批准设备以后：

自动获得受管 permanent password。

管理员最终能够在一个 Web 页面里明确看到：

```text
设备名称
RustDesk ID
在线状态
客户端版本
服务器配置同步状态
密码同步状态
受管固定密码
```

并使用该 ID 和受管固定密码，通过正常 RustDesk 客户端连接受管电脑。

这就是第一版 rustdesk-control 的完整交付目标。
