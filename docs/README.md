# 文档目录

文档统一维护在本目录。根 README 仅保留项目简介和入口；原任务草案已移除，
运行行为、协议及发布范围以这里的文档和实际代码为准。

| 文档 | 内容 |
| --- | --- |
| [架构与边界](ARCHITECTURE.md) | 控制面、OSS 服务、受管客户端职责及安全边界 |
| [控制面部署与运维](OPERATIONS.md) | Linux amd64 二进制、HTTPS、环境变量、备份及恢复 |
| [Windows 客户端构建](WINDOWS-BUILD.md) | Windows x64 工具链、编译变量及 EXE 打包 |
| [管理协议](PROTOCOL.md) | 签名、注册、心跳、版本同步和管理员 API |
| [发布流程](RELEASING.md) | tag 触发、仓库变量、两个产物、失败重试 |
| [许可证与源码](LICENSING.md) | AGPL、第三方声明、源码取得与分发责任 |
| [实际验收步骤](ACCEPTANCE.md) | Windows 安装、重启、远控、离线及轮换检查 |
| [验证记录](VALIDATION.md) | 已执行检查、历史尝试及仍未验证的限制 |

`validation/` 保存日志和截图作为历史证据，不是 Release 产物。
