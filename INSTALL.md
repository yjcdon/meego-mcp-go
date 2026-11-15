# 安装 meego-mcp-go

首选路径适用于 Apple 芯片 Mac、已安装 Codex 且具备飞书项目 MCP 用户 Token 的使用者。其他平台可从源码构建。安装和迁移均使用本机用户目录，现有凭证应在本机配置中读取或填写。

## 1. 下载并安装

[v1.0.0](https://github.com/yjcdon/meego-mcp-go/releases/tag/v1.0.0) 提供 macOS Apple 芯片版（`uname -m` 为 `arm64`）。下面的命令下载并校验发布包，将程序安装到 `~/.local/bin/`，最后输出配置所需的绝对路径。

```bash
(
  set -eu
  cd "$(mktemp -d)"
  release_url="https://github.com/yjcdon/meego-mcp-go/releases/download/v1.0.0"
  curl -fL --retry 2 -o meego-mcp-go_darwin_arm64.tar.gz "$release_url/meego-mcp-go_darwin_arm64.tar.gz"
  curl -fL --retry 2 -o SHA256SUMS "$release_url/SHA256SUMS"
  shasum -a 256 -c SHA256SUMS
  tar -xzf meego-mcp-go_darwin_arm64.tar.gz
  mkdir -p "$HOME/.local/bin"
  install -m 755 meego-mcp-go "$HOME/.local/bin/meego-mcp-go"
  printf '%s\n' "$HOME/.local/bin/meego-mcp-go"
)
```

也可以用 Go 1.25 或更高版本从源码构建：

```bash
git clone https://github.com/yjcdon/meego-mcp-go.git
cd meego-mcp-go
mkdir -p "$HOME/.local/bin"
go build -trimpath -o "$HOME/.local/bin/meego-mcp-go" .
```

## 2. 配置 Codex 和 Token

编辑本机的 `~/.codex/config.toml`，把 `command` 的占位路径替换为安装命令输出的绝对路径。填入你的**飞书项目 MCP 用户 Token**。如果原来的 `meego-mcp-cli` 已能连接，可以直接在本机配置中拷贝旧条目的 `MCP_USER_TOKEN`。只有项目链接时，请向飞书项目管理员确认如何取得相应的 MCP 凭证。

首次安装只需下面这一个 MCP 条目。已有同名表时替换 `command`，保留 `MCP_USER_TOKEN`，并移除原来传给 `meego-mcp-cli` 的 `args` 和 `DOTENV_CONFIG_QUIET`。

```toml
[mcp_servers.feishu_project_readonly_mcp]
command = "/Users/yourname/.local/bin/meego-mcp-go"

[mcp_servers.feishu_project_readonly_mcp.env]
MCP_USER_TOKEN = "REPLACE_WITH_YOUR_MEEGO_MCP_USER_TOKEN"
```

Go 代理直接从这个条目读取凭证，不依赖其他 MCP。日后可以在 Codex 的 MCP 设置中打开 `feishu_project_readonly_mcp`，在环境变量区域编辑 `MCP_USER_TOKEN`，也可以编辑同一段 TOML。真实 Token 请留在本机配置文件中。

## 3. 验证

运行 `codex mcp list`，确认 `feishu_project_readonly_mcp` 已启用，启动命令指向 Go 二进制。这一步验证配置能被 Codex 读取。重启 Codex 后，在新会话试一次只读查询，例如「通过飞书项目 MCP 查询我最近访问的项目」。返回项目数据后，即可确认凭证和连接都可用。条目名里的 `readonly` 沿用旧版命名，工具权限由飞书项目账号决定。 [Codex MCP 配置说明](https://learn.chatgpt.com/docs/extend/mcp?surface=app)

Codex 若弹出确认框，核对工具名为 `search_project_info` 后批准这次查询。已获授权的非交互验证可以使用下面的命令，临时只开放这个 MCP 的查询工具，并让本次会话直接调用。命令行选项不会写回配置文件。

```bash
codex exec --ephemeral --sandbox read-only \
  -c 'mcp_servers.feishu_project_readonly_mcp.enabled_tools=["search_project_info"]' \
  -c 'mcp_servers.feishu_project_readonly_mcp.tools.search_project_info.approval_mode="approve"' \
  '只调用飞书项目 MCP 的 search_project_info，参数 page_num 设为 1，查询最近访问的项目。'
```

如果代理无法连接，检查 Token 是否仍有效，以及账号是否有目标空间权限。如果选择了其他条目名，通过 `--credential-server` 指明凭证所在的表。若旧版安装仍保留，可以恢复原来的启动命令和参数回滚。

### 可选参数

| 参数 | 用途 |
| --- | --- |
| `--config /absolute/path/config.toml` | 指定凭证文件，默认读取 `~/.codex/config.toml` |
| `--credential-server NAME` | 指定凭证所在的 MCP 条目，默认为 `feishu_project_readonly_mcp` |
| `--domain https://project.feishu.cn` | 指定飞书项目的 HTTPS 域名，默认值如示例所示 |
| `--lang zh` 或 `--lang en` | 指定远端响应语言 |
| `--debug` | 将连接状态输出到标准错误 |
