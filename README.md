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

配置文件本身是符号链接时，`ags` 修改链接目标并保留符号链接，不会用普通文件覆盖链接。

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
version: 2

defaults:
  codex:
    model: "gpt-default-example"
  claude:
    model: "claude-default-example"

providers:
  relay:
    codex:
      api_key: "sk-codex-example"
      base_url: "https://codex.example.com/v1"
    claude:
      auth_token: "sk-claude-example"
      base_url: "https://claude.example.com"

  codex-special:
    codex:
      api_key: "sk-codex-special-example"
      base_url: "https://codex-special.example.com/v1"
      model: "gpt-provider-override-example"
```

一个 Provider 可以只配置 Codex 或 Claude。`defaults.codex` 和 `defaults.claude` 支持对应 Agent 块的同一组字段：Codex 为 `api_key`、`base_url`、`model`，Claude 为 `auth_token`、`base_url`、`model`。Provider 中的非空字段覆盖全局默认值；默认值只补全 Provider 已声明的 Agent，不会让未声明的 Agent 自动获得支持。

每个 Agent 最终合并得到的密钥和 Base URL 必须非空。`model` 可以省略：Provider 和全局默认都没有模型时，切换不会修改目标 Agent 已有的模型字段，只更新密钥和 Base URL。版本 1 注册表迁移时只需将 `version` 改为 `2`，再按需增加 `defaults` 或 Provider 模型。

## 使用

```bash
# 依次选择 Agent 和 Provider
ags

# 选择指定 Agent 的 Provider
ags codex
ags claude
ags all

# 查看 Provider 名称、模型和 Base URL
ags list

# 查看当前配置匹配的 Provider
ags current

# 直接切换，不进入选择界面
ags codex relay
ags claude relay

# 同时切换 Codex 与 Claude
ags all relay
```

交互选择界面在常规终端使用方向键移动、Enter 确认；兼容模式下输入选项编号。Provider 选项会根据目标 Agent 过滤，并显示 Provider 名称、有效模型和对应 Base URL；模型显示为 `-` 时表示保持当前模型。

`all` 要求目标 Provider 同时包含 Codex 和 Claude 配置。全部目标文件生成并校验成功后才会开始写入；写入中途失败时恢复已经修改的文件。

## 修改范围

| 文件 | 修改字段 |
|---|---|
| `~/.codex/auth.json` | `OPENAI_API_KEY` |
| `~/.codex/config.toml` | 顶层 `model`，仅在有效模型非空时修改 |
| `~/.codex/config.toml` | `[model_providers.custom].base_url` |
| `~/.claude/settings.json` | 顶层 `model`，仅在有效模型非空时修改 |
| `~/.claude/settings.json` | `env.ANTHROPIC_AUTH_TOKEN` |
| `~/.claude/settings.json` | `env.ANTHROPIC_BASE_URL` |

Codex 的 `auth.json` 和 `config.toml` 必须已经存在，且 `config.toml` 必须包含 `[model_providers.custom]`。Claude 的 `settings.json` 不存在时会自动创建。

`ags current` 在 Provider 配置了有效模型时同时匹配模型；有效模型为空时只按密钥和 Base URL 匹配。命令输出不会显示 API Key 或认证令牌。`ags list` 和交互选择界面会显示模型与 Base URL，用于区分同名或相似渠道。

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
