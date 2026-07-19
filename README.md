# ags

`ags` 是一个只修改配置文件指定字段的 Provider 切换 CLI，当前支持 Codex CLI 和 Claude Code。

项目不管理账号、不代理请求，也不提供 Web、数据库、测速或历史记录。终端界面只用于选择 Agent 和 Provider；Provider 由一个 YAML 文件维护，切换时保留目标配置文件中的其他字段。

## 配置路径

Provider 注册表固定使用用户主目录下的文件：

```text
~/.agent-switch/providers.yaml
```

Agent 配置路径按以下规则解析：

| Agent | 默认目录 | 可选目录覆盖 |
|---|---|---|
| Codex | `~/.codex` | `CODEX_HOME` |
| Claude Code | `~/.claude` | `CLAUDE_CONFIG_DIR` |

路径通过操作系统的用户主目录和路径 API 生成，同一套代码支持 Linux 和 Windows。

## Provider 文件

复制示例并填写真实 Provider：

```bash
mkdir -p ~/.agent-switch
cp providers.example.yaml ~/.agent-switch/providers.yaml
```

Windows PowerShell：

```powershell
New-Item -ItemType Directory -Force "$HOME\.agent-switch"
Copy-Item .\providers.example.yaml "$HOME\.agent-switch\providers.yaml"
```

配置格式：

```yaml
version: 1

providers:
  shared-relay:
    universal:
      api_key: "sk-shared-example"
      base_url: "https://shared.example.com"

  agent-specific-relay:
    codex:
      api_key: "sk-codex-example"
      base_url: "https://codex.example.com/v1"
    claude:
      auth_token: "sk-claude-example"
      base_url: "https://claude.example.com"

  mixed-relay:
    universal:
      api_key: "sk-shared-example"
      base_url: "https://shared.example.com"
    codex:
      api_key: "sk-codex-override-example"
      base_url: "https://codex-override.example.com/v1"
```

`universal` 表示同一组 Base URL 和 API Key 可以直接用于 Codex 与 Claude。Agent 专用配置存在时，完整覆盖该 Agent 的 Universal 配置；不进行 URL 与 Key 的局部混合。

一个 Provider 可以只包含 Universal 配置、只配置某个 Agent，或同时包含 Universal 与 Agent 专用覆盖。未知字段、空密钥和空地址会导致切换失败。

## 使用

```bash
# 依次选择 Agent 和 Provider
ags

# 选择指定 Agent 的 Provider
ags codex
ags claude
ags all

# 查看 Provider 名称、配置模式和 Base URL
ags list

# 查看当前配置匹配的 Provider
ags current

# 直接切换，不进入选择界面
ags codex relay
ags claude relay

# 同时切换 Codex 与 Claude
ags all relay
```

进入交互选择时会先显示 Codex 和 Claude 当前匹配的 Provider。选择列表会根据目标 Agent 过滤，显示 Provider 名称和对应 Base URL，给当前项添加 `[current]` 标记，并默认定位到当前项。

交互选择界面在常规终端使用方向键移动、Enter 确认；兼容模式下输入选项编号。

`all` 要求目标 Provider 能解析出 Codex 和 Claude 的有效配置；只包含 Universal 配置的 Provider 自动满足该条件。全部目标文件生成并校验成功后才会开始写入；写入中途失败时恢复已经修改的文件。

## 修改范围

| 文件 | 修改字段 |
|---|---|
| `~/.codex/auth.json` | `OPENAI_API_KEY` |
| `~/.codex/config.toml` | `[model_providers.custom].base_url` |
| `~/.claude/settings.json` | `env.ANTHROPIC_AUTH_TOKEN` |
| `~/.claude/settings.json` | `env.ANTHROPIC_BASE_URL` |

Codex 的 `auth.json` 和 `config.toml` 必须已经存在，且 `config.toml` 必须包含 `[model_providers.custom]`。Claude 的 `settings.json` 不存在时会自动创建。

命令输出不会显示 API Key 或认证令牌。`ags list` 和交互选择界面会显示 Base URL，用于区分同名或相似渠道。

## 构建与安装

要求 Go 1.26 或更高版本。

Linux：

```bash
go build -o bin/ags ./cmd/ags
sudo install -m 0755 bin/ags /usr/local/bin/ags
```

Windows PowerShell：

```powershell
go build -o .\bin\ags.exe .\cmd\ags
```

将 `bin` 目录加入 `PATH` 后，可在任意目录执行 `ags`。

## 验证

```bash
gofmt -w ./cmd ./internal
go test ./...
go vet ./...
go build -o bin/ags ./cmd/ags
GOOS=windows GOARCH=amd64 go build -o bin/ags-windows-amd64.exe ./cmd/ags
```
