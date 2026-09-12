# 许可证与对应源码

本项目新增的 Go 控制面、Web UI、管理模块和构建脚本采用 **AGPL-3.0-only**，
许可证全文位于根目录 [LICENSE](../LICENSE)，项目声明位于 [NOTICE](../NOTICE)。
法律文件保留在根目录，便于 GitHub 和分发工具识别；使用与维护文档集中在 docs/。

Windows 客户端基于 RustDesk OSS 1.4.9，保留上游 AGPL、版权和第三方声明。
它是修改版，不是 RustDesk 官方发行版，也不提供 RustDesk Pro 商业授权。
修改范围与基线见 [managed-client/NOTICE](../managed-client/NOTICE)。
开源代码许可不等于获得 RustDesk 名称、商标或标志的额外授权。

## 两个二进制中的源码入口

AGPL 第 6 节规定目标代码分发时提供对应源码的方式，第 13 节规定修改程序通过网络
与用户交互时的源码获取要求。准确义务以 [许可证全文](../LICENSE) 为准。
本流程将源码和许可文本放在二进制内部，因此 Release 仍只上传两个文件。

### Linux amd64 控制面

登录页及后台底部都有“源码下载”、AGPL 和第三方许可证入口，不需要管理员权限即可访问：

- `/source.tar.gz`：构建时的本仓库源码，含 Web UI、管理模块、patch、锁文件及构建脚本。
- `/LICENSE`：AGPL 全文。
- `/THIRD-PARTY-NOTICES.txt`：构建环境中 Go/npm 依赖的许可声明。

这些文件与 Web UI 一同嵌入 Go 程序。部署时不要用反向代理屏蔽源码入口。
修改程序后重新运行 `scripts/build.sh`，以便用户取得与运行程序相匹配的修改源码。

### Windows x64 客户端

EXE 是上游自解压包。正常运行后，在运行该 EXE 的用户目录
`%LOCALAPPDATA%\rustdesk` 中取得：

- `corresponding-source.tar.gz`：本仓库源码，加上实际应用 patch 后的完整 RustDesk
  工作树、初始化的子模块（含修改后的 hbb_common）、管理模块及生成的 bridge 源码。
- `SOURCE-CODE.md`：本说明的构建时副本。
- `LICENSE`、`RUSTDESK-LICENCE`、`NOTICE`、`THIRD-PARTY-NOTICES.txt`：项目、上游及依赖声明。

无需安装服务或注册控制面即可取得解包后的源码。源码包中 `BUILD-SOURCE.json`
记录控制面提交、客户端基线和公开的编译配置；它不包含管理员密码或 master key。
解包目录可能被之后启动的其他版本覆盖，取得源码后请按版本保存。

客户端 About 中显示修改版标识和 `RUSTDESK_MANAGED_SOURCE_URL`。
GitHub workflow 和本地脚本默认使用 `https://github.com/phil616/rustdesk-control`，
其 README 提供本说明入口；Release 正文另行链接到本次提交的文档。使用者可以从对应
Release 下载同一 EXE 取得内置源码。其他部署覆盖此值时应使用真实可访问的源码/获取说明
URL，不能填写 example.com 或私有页面。请长期保留对应版本的 Release 和源码访问方式。

源码收集读取实际工作树，不使用会丢失未提交 patch 的 upstream `git archive`。
Git 忽略的凭据、数据库、缓存和构建输出不收集；不要将秘密提交到 Git。
GitHub 自带 Source code ZIP/tar.gz 只有本仓库及 patch，**不含展开的客户端树**，
不能将它单独描述为修改版客户端的完整源码。

## 第三方组件与再分发

依赖继续适用各自许可证，不能把第三方版权统一改写为本项目版权。
构建脚本收集安装依赖中的 LICENSE/LICENCE/COPYING/COPYRIGHT/NOTICE/AUTHORS 文本；
Windows 同时保留 Flutter bundle 中原有的资产和许可文件。
收集结果可能包含仅构建期使用的工具，不构成法律审核或完整组件清单。

源码包覆盖本项目与修改的 RustDesk/子模块。通过 Cargo、Pub、npm、Go、vcpkg
另行下载的外部依赖源码**没有全部复制入包**，其版本、地址和构建步骤保留在源码中。
分发者仍须核对实际链接/打包组件的义务，尤其是 FFmpeg 等 LGPL/GPL 组件：
仅有锁文件、上游下载链接或一份 NOTICE 并不自动满足提供源码、修改说明或重新链接的要求。
若实际依赖要求额外对应源码或重新链接材料，应将其纳入 EXE 内部源码载荷（或提供
许可证允许的持续源码服务）后再分发；不要为了维持两个附件而省略必要材料。

正式对外分发前，还需完成 [Windows 验收](ACCEPTANCE.md)，核对实际产物中的源码可读、
声明完整、About 链接可访问及依赖许可证兼容性。当前记录见 [VALIDATION.md](VALIDATION.md)。
