# meego-mcp-go

## 项目介绍

`meego-mcp-go` 是飞书项目（Meego）的 Go 版 MCP 代理。它让 Codex 通过本地 stdio 查询工作项、评论和项目空间，也可以调用飞书项目为当前账号开放的写入工具。项目是独立实现，权限由飞书项目及用户 Token 决定。

Go 版以单个可执行文件运行，安装后无需 Go、Node.js 或 pnpm。它直接从本机 Codex 配置读取凭证，并保留远端工具定义、参数和结果中的 JSON 字段。

### 资源占用对比

同一台 Apple 芯片 Mac 上分别启动一套代理，两版都加载了 48 个工具，并查询同一条工作项 3 次。

| 指标 | 原 Node 版（含 pnpm 和辅助 shell） | Go 版 |
| --- | ---: | ---: |
| 进程数 | 3 | 1 |
| 查询后空闲 RSS 中位数 | 125.8 MiB | 21.8 MiB |
| 启动到完成查询的累计 CPU 时间 | 0.44 秒 | 0.03 秒 |

这次采样中，Go 版的空闲 RSS 合计低约 83%。进程 RSS 相加可能重复计入共享内存页。

## 安装

[安装教程](INSTALL.md) 提供下载、校验、Codex 与 Token 配置、只读验证和迁移步骤。 [下载 v1.0.0](https://github.com/yjcdon/meego-mcp-go/releases/tag/v1.0.0)
