# meego-mcp-go

`meego-mcp-go` 是一个用 Go 编写的飞书项目（Meego）MCP 代理，让 Codex 通过本地 stdio 连接飞书项目。它把工具列表和调用转发给飞书，运行只需一个可执行文件。

这是独立实现。工具和操作权限由飞书项目及你的 Token 决定，工具列表可能随账号权限变化。本地代理支持转发远端提供的查询和写入工具。

## 安装

[GitHub Releases](https://github.com/yjcdon/meego-mcp-go/releases) 提供 macOS Apple 芯片版。下载压缩包和同版本的 `SHA256SUMS`，进入下载目录运行：

```bash
shasum -a 256 -c SHA256SUMS
tar -xzf meego-mcp-go_darwin_arm64.tar.gz
mkdir -p "$HOME/.local/bin"
install -m 755 meego-mcp-go "$HOME/.local/bin/meego-mcp-go"
```

也可以用 Go 1.25 或更高版本从源码构建：

```bash
git clone https://github.com/yjcdon/meego-mcp-go.git
cd meego-mcp-go
mkdir -p "$HOME/.local/bin"
go build -trimpath -o "$HOME/.local/bin/meego-mcp-go" .
```

## 配置 Codex 和 Token

使用你的飞书项目账号对应的 **MCP 用户 Token**。如果现有的 `meego-mcp-cli` 已能连接，直接复用它配置里的 `MCP_USER_TOKEN`。只有项目链接时，向飞书项目管理员确认你所在空间的 MCP 接入方式和可用权限。

编辑本机的 `~/.codex/config.toml`，把示例中的路径替换为二进制的实际绝对路径。首次配置时填写自己的 Token，已有 `[mcp_servers.feishu_project_readonly_mcp.env]` 时保留原值，并在旧条目中设定 `enabled = false`，以便只启动 Go 版。已有的 TOML 表应在原处修改，避免重复声明。

```toml
[mcp_servers.feishu_project_readonly_mcp]
enabled = false

[mcp_servers.feishu_project_readonly_mcp.env]
MCP_USER_TOKEN = "REPLACE_WITH_YOUR_MEEGO_MCP_USER_TOKEN"

[mcp_servers.feishu_project_go]
command = "/Users/yourname/.local/bin/meego-mcp-go"
```

Go 代理直接从文件读取 Token。示例中 `env` 是旧配置的表名，启用的 Go 条目无需设置环境变量。把真实 Token 留在本机配置里，提交和分享时使用占位值。

重启 Codex，先试一次只读查询，例如「通过飞书项目 MCP 查询我最近访问的项目，不修改任何数据」。若需回滚，禁用 `feishu_project_go` 并重新启用原条目。 [Codex MCP 配置说明](https://learn.chatgpt.com/docs/extend/mcp?surface=app)

### 可选参数

| 参数 | 用途 |
| --- | --- |
| `--config /absolute/path/config.toml` | 指定凭证文件，默认读取 `~/.codex/config.toml` |
| `--credential-server NAME` | 指定凭证所在的 MCP 条目，默认为 `feishu_project_readonly_mcp` |
| `--domain https://project.feishu.cn` | 指定飞书项目的 HTTPS 域名，默认值如示例所示 |
| `--lang zh` 或 `--lang en` | 指定远端响应语言 |
| `--debug` | 将连接状态输出到标准错误 |

## 资源占用

一次 macOS 只读采样中，两版都加载了 48 个工具，并查询同一工作项 3 次。查询后空闲 RSS 从 Node 版的 125.8 MiB（含 pnpm 和辅助 shell）降到 Go 版的 21.8 MiB，进程数从 3 个降到 1 个。RSS 合计可能重复计入共享内存页。 [采样截图](assets/benchmark.jpg)
