# 安装 meego-mcp-go

需要 Codex 和飞书项目 MCP 用户 Token。发布包支持 Apple 芯片 Mac 和 Windows x64。

## 1. 下载并安装

### Apple 芯片 Mac

```bash
(
  set -eu
  if [ "$(uname -s):$(uname -m)" != "Darwin:arm64" ]; then
    printf '%s\n' '此发布包适用于 Apple 芯片 Mac。' >&2
    exit 1
  fi
  cd "$(mktemp -d)"
  release_url='https://github.com/yjcdon/meego-mcp-go/releases/download/v2.0.0'
  archive='meego-mcp-go_darwin_arm64.tar.gz'
  curl -fL --retry 2 -o "$archive" "$release_url/$archive"
  tar -xzf "$archive"
  mkdir -p "$HOME/.local/bin"
  install -m 755 meego-mcp-go "$HOME/.local/bin/meego-mcp-go"
  printf '%s\n' "$HOME/.local/bin/meego-mcp-go"
)
```

### Windows x64

在 PowerShell 中运行以下命令。

```powershell
$ErrorActionPreference = 'Stop'
$releaseUrl = 'https://github.com/yjcdon/meego-mcp-go/releases/download/v2.0.0'
$archive = 'meego-mcp-go_windows_amd64.zip'
$temp = Join-Path ([System.IO.Path]::GetTempPath()) ([guid]::NewGuid().ToString())
$zip = Join-Path $temp $archive
$bin = Join-Path $HOME '.local\bin'
New-Item -ItemType Directory -Path $temp, $bin -Force | Out-Null
Invoke-WebRequest -UseBasicParsing -Uri "$releaseUrl/$archive" -OutFile $zip
Expand-Archive -LiteralPath $zip -DestinationPath $temp
Copy-Item -LiteralPath (Join-Path $temp 'meego-mcp-go.exe') -Destination $bin -Force
(Join-Path $bin 'meego-mcp-go.exe').Replace('\', '/')
```

## 2. 配置 Codex 和 Token

在本机 `~/.codex/config.toml` 中配置以下条目。将 `command` 替换为安装命令输出的绝对路径，例如 Windows 上的 `C:/Users/yourname/.local/bin/meego-mcp-go.exe`。填入飞书项目 MCP 用户 Token。

```toml
[mcp_servers.feishu_project_mcp]
command = "/Users/yourname/.local/bin/meego-mcp-go"

[mcp_servers.feishu_project_mcp.env]
MCP_USER_TOKEN = "REPLACE_WITH_YOUR_MEEGO_MCP_USER_TOKEN"
```

启动参数留空。在 Codex 的 `feishu_project_mcp` 设置中，可以通过环境变量区域编辑 `MCP_USER_TOKEN`。真实 Token 请留在本机配置文件中。

## 3. 验证

运行 `codex mcp list`，确认 `feishu_project_mcp` 已启用，启动命令指向 Go 二进制。这一步验证配置能被 Codex 读取。重启 Codex 后，在新会话试一次只读查询，例如「通过飞书项目 MCP 查询我最近访问的项目」。返回项目数据后，即可确认凭证和连接都可用。工具权限由飞书项目账号决定。 [Codex MCP 配置说明](https://learn.chatgpt.com/docs/extend/mcp?surface=app)

Codex 若弹出确认框，核对工具名为 `search_project_info` 后批准这次查询。在 macOS 上，已获授权的非交互验证也可以使用下面的命令，临时只开放这个 MCP 的查询工具。命令行选项不会写回配置文件。

```bash
codex exec --ephemeral --sandbox read-only \
  -c 'mcp_servers.feishu_project_mcp.enabled_tools=["search_project_info"]' \
  -c 'mcp_servers.feishu_project_mcp.tools.search_project_info.approval_mode="approve"' \
  '只调用飞书项目 MCP 的 search_project_info，参数 page_num 设为 1，查询最近访问的项目。'
```

如果代理无法连接，检查网络和 Token 是否有效，以及账号是否有目标空间权限。
