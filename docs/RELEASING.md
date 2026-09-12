# Tag 自动发布

`.github/workflows/release.yml` 监听 GitHub 上的 `v*` tag push，要求格式如
`v1.0.0` 或 `v1.0.0-rc.1`。仅创建本地 tag 不会触发；当前其他 Git 托管平台
上的 remote 也不会运行 GitHub Actions，需把包含工作流的提交和 tag 推到 GitHub。

## 一次性配置

1. 使用公开的 GitHub 源码仓库，启用 Actions。当前发布工作流不支持私有源码仓库。
2. 无需创建变量即可使用内置默认值：
   - `RUSTDESK_CONTROL_URL=https://rustdesk-control.altasci.com`
   - `RUSTDESK_MANAGED_SOURCE_URL=https://github.com/phil616/rustdesk-control`
   如需覆盖，在 Settings → Secrets and variables → Actions → Variables 设置同名变量。
   两者是公开的编译配置，不是 secret；控制面地址必须是无路径/query/账号密码的 HTTPS origin。
3. 仓库/组织策略需允许发布 job 使用 `GITHUB_TOKEN` 的 `contents: write`。
   构建 job 只有读取权限，不需要个人 access token。
4. 保留根 LICENSE、NOTICE、upstream notices 和对应源码；阅读 [LICENSING.md](LICENSING.md)。

`RUSTDESK_MANAGED_SOURCE_URL` 显示在客户端 About。默认源码仓库的 README 链接到
许可证/源码获取说明；Release 正文另行链接本次提交的文档，EXE 内源码固定于本次构建。

## 发布

确认当前提交包含所有需要发布的改动，再执行，例如 GitHub remote 名为 `github`：

```sh
git tag -a v1.0.0 -m 'Release v1.0.0'
git push github v1.0.0
```

不要覆盖/移动已发布 tag。无须手动创建 Release。只在两个构建都成功后，发布 job
创建 draft、上传两个文件、核对附件名称后转为正式发布；带 `-` 的版本标记 prerelease。

| Job | 构建 | Release 文件 |
| --- | --- | --- |
| `control-linux-amd64` | Ubuntu runner，Go + 内嵌前端，CGO=0 | `rustdesk-control-linux-amd64` |
| `client-windows-amd64` | Windows 2022 runner，x64 MSVC + Flutter | `rustdesk-managed-windows-amd64.exe` |

不构建 Windows 控制面、Linux 客户端、ARM、macOS、MSI 或额外 bundle ZIP。
原生 DLL、Flutter bundle 和嵌入源码包是 EXE 的必要中间文件，不单独上传。
SHA-256 写在 Release 正文，不增加第三个附件。GitHub 自动生成的 Source code
(zip/tar.gz) 链接属于平台自带源码快照，不是工作流上传的附件，也无法通过此流程禁用。

## 源码与许可证

控制面内嵌 `/source.tar.gz` 和 `/LICENSE`；Web 登录页和后台提供源码入口。
Windows EXE 的正常解包目录包含 `corresponding-source.tar.gz`，保存实际应用 patch
后的 RustDesk、hbb_common、管理模块及控制仓库构建脚本；还保留许可证和声明。
详见 [LICENSING.md](LICENSING.md)，不要只把自动生成的 GitHub 源码快照称为完整客户端源码。

## 重试和限制

构建失败不会发布只有一个附件的 Release。上传中断后保留 draft，可重跑失败 job；
已有正式 Release 不会被覆盖，有意更新必须使用新 tag。异常的 draft 附件需人工检查，
脚本不会静默删除它们。Actions 内部中转 artifact 保留 7 天，不是永久源码下载地址。

Windows 完整流水线尚未在本会话访问 GitHub runner 执行；本地可完成语法和结构检查，
首次 tag 仍可能暴露 upstream 原生库/Flutter 工具链兼容问题。编译成功也不替代
Windows 安装、重启和真实远控验收。EXE 默认未签名，保留 Windows/UAC 提示；
Authenticode 签名需发布者自己的证书，本流程不生成或伪造证书。
