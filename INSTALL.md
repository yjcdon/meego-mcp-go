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

已有同名 TOML 表时在原处合并字段，避免重复声明。迁移旧版时，复制凭证之后将启动 `meego-mcp-cli` 的旧条目设为 `enabled = false`。

```toml
[mcp_servers.feishu_project_go]
command = "/Users/yourname/.local/bin/meego-mcp-go"
args = ["--credential-server", "feishu_project_credentials"]

[mcp_servers.feishu_project_credentials]
command = "/usr/bin/false"
enabled = false

[mcp_servers.feishu_project_credentials.env]
MCP_USER_TOKEN = "REPLACE_WITH_YOUR_MEEGO_MCP_USER_TOKEN"
```

Go 代理按 `--credential-server` 直接从配置文件读取凭证。Codex 要求禁用条目也有合法的传输配置，因此凭证条目同时包含 `command` 和 `enabled = false`。运行时只启动 `feishu_project_go`。真实 Token 请留在本机配置文件中。

## 3. 验证

运行 `codex mcp list`，确认 `feishu_project_go` 已启用，`feishu_project_credentials` 已禁用。这一步验证配置能被 Codex 读取。重启 Codex 后，在新会话试一次只读查询，例如「通过飞书项目 MCP 查询我最近访问的项目」。返回项目数据后，即可确认凭证和连接都可用。 [Codex MCP 配置说明](https://learn.chatgpt.com/docs/extend/mcp?surface=app)

如果代理无法连接，检查 Token 是否仍有效、是否有目标空间权限，以及 `--credential-server` 是否与凭证表名相同。若旧版安装仍保留，可以禁用 Go 条目并重新启用旧条目回滚。

### 可选参数

| 参数 | 用途 |
| --- | --- |
| `--config /absolute/path/config.toml` | 指定凭证文件，默认读取 `~/.codex/config.toml` |
| `--credential-server NAME` | 指定凭证所在的 MCP 条目，默认为 `feishu_project_readonly_mcp` |
| `--domain https://project.feishu.cn` | 指定飞书项目的 HTTPS 域名，默认值如示例所示 |
| `--lang zh` 或 `--lang en` | 指定远端响应语言 |
| `--debug` | 将连接状态输出到标准错误 |
