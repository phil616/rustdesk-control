# 架构与边界

```text
Windows Managed RustDesk ── HTTPS + Ed25519 ── Linux amd64 Go 控制面
           │                                      │
           │                                  SQLite + React UI
           └── 原始 RustDesk 协议 ── 官方 OSS hbbs / hbbr
```

控制面不替代 hbbs/hbbr。它管理设备清单、零接触注册、审批、ID Server/Relay/Key
分发、受管永久密码、轮换与审计。API Server 固定为空，不实现 RustDesk Pro 服务。

客户端固定 upstream tag `1.4.9`、commit `6c578292e8ebbbec708b76986ba8c4bc7c509747`，
修改集中在 `managed-client/core` 和两个 upstream patch。`rustdesk-managed-client/`
为构建时展开的独立 checkout，不直接提交本仓库。

Windows agent 在现有 LocalSystem RustDesk server 生命周期启动，不增加系统服务。
GUI/tray/connection manager 不单独启动 agent。UUID/Ed25519 身份与 RustDesk ID 独立，
私钥放在服务账户配置目录。必须正常安装服务，便携界面本身不会注册。

新设备 pending，不接收受管密码；审批后获得随机永久密码，应用后回报版本。
密码仅在 Go 数据库中以 AES-256-GCM 加密保存。普通管理员列表不返回密码，
主动查看需要登录并产生审计记录。RustDesk 使用原有内部配置与密码存储，不通过 argv 下发。

Control URL 编译时固定为 HTTPS；控制面离线不会删除已保存的参数和密码。
Reject/Delete/Reset 停止后续凭据下发，不等于撤销设备已有离线访问或终止现有会话。

不实现隐藏安装、关闭安全提示、采集个人文件或绕过 OS 安全机制。
About 显示受管/修改版状态及 Source Code。原始协议与远程桌面逻辑保持原有路径。

发行范围只有 Linux amd64 控制面和 Windows x64 EXE。源码中保留的其他平台历史
钩子不代表受支持的构建目标；Windows 实机验收状态见 [VALIDATION.md](VALIDATION.md)。
