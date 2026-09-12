# Linux amd64 控制面部署与运维

控制面是内嵌 React 管理界面的单个 Go 程序，使用 SQLite。正式支持 Linux amd64。
它不替代 RustDesk OSS 的 hbbs/hbbr；Windows 客户端参见 [构建指南](WINDOWS-BUILD.md)。

## 获取程序

下载 Release 的 `rustdesk-control-linux-amd64`，按 Release 正文核对 SHA-256。
也可以使用仓库 go.mod 指定的 Go 版本、Node.js 24、Python 3 在本地构建：

```sh
./scripts/build.sh
```

输出 `bin/rustdesk-control`，固定 `GOOS=linux GOARCH=amd64 CGO_ENABLED=0`。
前端、许可证和本次源码包均嵌入程序，不需要部署 Node.js 或另外复制 web/dist。
修改前端后必须重新构建整个程序。

## 配置与首次启动

| 配置 | 用途 |
| --- | --- |
| `RUSTDESK_CONTROL_MASTER_KEY` | 始终必需；标准 Base64 编码的 32 字节随机密钥 |
| `RUSTDESK_CONTROL_ADMIN_PASSWORD` | 首次创建数据库必需，至少 12 字符；此后忽略 |
| `--listen` | 默认 `:8080`；代理部署建议 `127.0.0.1:8080` |
| `--database` | 默认 `./data/rustdesk-control.db` |
| `--trusted-proxies` | 可信反向代理 CIDR，默认不信任转发头 |

使用 `openssl rand -base64 32` 生成 master key，保存到组织的秘密管理系统。
密钥不自动保存到数据库。丢失密钥会使现有受管密码无法解密。
数据库首次初始化后，可从运行环境移除初始管理员密码。

下面以 systemd 部署为例。以管理员身份创建独立账户和目录，安装已下载的程序：

```sh
useradd --system --home-dir /var/lib/rustdesk-control --shell /usr/sbin/nologin rustdesk-control
install -d -m 0700 -o rustdesk-control -g rustdesk-control /var/lib/rustdesk-control
install -d -m 0700 /etc/rustdesk-control
install -m 0755 rustdesk-control-linux-amd64 /usr/local/bin/rustdesk-control
```

创建 `/etc/rustdesk-control/env`，权限设为 `0600`，仅 root 可读。
填入真实随机密钥和初始密码，不要使用示例值：

```ini
RUSTDESK_CONTROL_MASTER_KEY=REPLACE_WITH_BASE64_32_BYTE_KEY
RUSTDESK_CONTROL_ADMIN_PASSWORD=REPLACE_WITH_INITIAL_ADMIN_PASSWORD
```

创建 `/etc/systemd/system/rustdesk-control.service`：

```ini
[Unit]
Description=RustDesk control plane
After=network.target

[Service]
User=rustdesk-control
Group=rustdesk-control
WorkingDirectory=/var/lib/rustdesk-control
EnvironmentFile=/etc/rustdesk-control/env
ExecStart=/usr/local/bin/rustdesk-control --listen 127.0.0.1:8080 --database /var/lib/rustdesk-control/control.db --trusted-proxies 127.0.0.1/32,::1/128
Restart=on-failure
UMask=0077
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/rustdesk-control

[Install]
WantedBy=multi-user.target
```

```sh
systemctl daemon-reload
systemctl enable --now rustdesk-control
journalctl -u rustdesk-control
```

迁移 SQL 随二进制嵌入，启动时自动执行。SQLite 使用 WAL、foreign keys 和 busy timeout。

## HTTPS 反向代理

浏览器与 API 必须同源，必须使用 HTTPS（登录 cookie 始终为 Secure）。
例如在同一主机部署 Caddy，将真实域名解析到服务器：

```caddyfile
rustdesk-control.altasci.com {
    reverse_proxy 127.0.0.1:8080
}
```

代理保留 Host，正确覆盖/追加 X-Forwarded-For。只有真实代理地址才应列入
`--trusted-proxies`，不要信任整个公网。后端端口仅监听回环地址。
不要记录登录、心跳或凭据接口的请求/响应正文。
保持 `/source.tar.gz`、`/LICENSE`、`/THIRD-PARTY-NOTICES.txt` 可访问，见 [许可证说明](LICENSING.md)。

## 接入设备

1. 单独部署官方 OSS hbbs/hbbr，准备 ID Server、Relay 和 hbbs 公钥。
2. HTTPS 登录控制台，在设置中保存参数。公钥为标准 Base64 编码的 32 字节。
3. 使用编译了此控制面 HTTPS 地址的 Windows 客户端正常安装 RustDesk 服务。
4. 审核待审批设备；批准后，下次心跳下发服务器策略和受管密码。
5. 在设备详情查看 ID 和密码，用普通 RustDesk 客户端连接。

心跳间隔为 60±10 秒，最后心跳不超过 120 秒显示在线；列表每 15 秒刷新。
控制面离线时客户端保留已有服务器配置和密码。拒绝、删除或重置设备只停止后续下发，
不会擦除离线设备的已有密码，也不会断开已建立的远程连接。

登录页及后台可切换简体中文 / English，首次跟随浏览器语言。
仅语言偏好写入 localStorage；密码不写入浏览器存储。
更改管理员密码会结束所有会话；查看受管密码会记录审计。

## 更新、备份与恢复

更新前备份数据库和密钥。停止服务后替换程序，保留数据目录和环境文件，再启动服务。
不要覆盖密钥，也不要随意回退已迁移数据库的程序版本。

停机备份应复制整个数据目录（包括可能存在的 WAL/SHM），另行安全备份 master key。
在线备份使用 SQLite Online Backup API 或 sqlite3 的 `.backup`，不要只复制活动数据库主文件。
恢复前停止服务，恢复一致性备份和权限，以原 master key 启动。
恢复旧库后让设备重新同步，并轮换凭据、复查会话和审计。

## 开发验证

全新 checkout 先构建前端，Go embed 才能编译。常用入口：

```sh
./scripts/test.sh
./scripts/test-browser.sh
```

独立 Rust 核心单元测试不生成 Linux 桌面客户端发布包。
协议参见 [PROTOCOL.md](PROTOCOL.md)，已完成和未完成的验证参见 [VALIDATION.md](VALIDATION.md)。
