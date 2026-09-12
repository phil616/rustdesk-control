# rustdesk-control

RustDesk OSS 集中管理：设备注册与审批、服务器配置下发、在线状态、受管固定密码轮换及审计。
Go 控制面内嵌中英文 React 管理界面，使用 SQLite；受管客户端基于 RustDesk OSS 1.4.9。
仅用于组织拥有或已明确授权管理的设备。

正式发布仅提供：

| 文件 | 平台 |
| --- | --- |
| `rustdesk-control-linux-amd64` | Linux amd64 控制面 |
| `rustdesk-managed-windows-amd64.exe` | Windows x64 受管客户端 |

控制面与官方 OSS `hbbs/hbbr` 分开部署。客户端正常安装服务后自动注册，管理员批准后下发密码。

- [文档目录](docs/README.md)
- [控制面部署](docs/OPERATIONS.md)
- [Windows 客户端构建](docs/WINDOWS-BUILD.md)
- [Tag 自动发布](docs/RELEASING.md)
- [验收状态与限制](docs/VALIDATION.md)

项目修改采用 [AGPL-3.0-only](LICENSE)，保留 RustDesk 及第三方许可证。
分发、网络源码入口及对应源码说明见 [许可证文档](docs/LICENSING.md)。
