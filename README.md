# AGS

AGS 是 Codex 和 Claude Code 的跨平台环境管理器，支持 Linux 和 Windows。

AGS 使用公开二进制和私有 Agent Environment Git 仓库完成：

- 安装、升级和卸载 Codex、Claude Code 的锁定 npm 版本。
- 应用两个 Agent 共用的全局指令。
- 获取、构建并同步 Profile 选择的 Skills。
- 管理 Codex、Claude Code 的 API Provider。
- 检查环境漂移、外部依赖和仓库访问。
- 从 GitHub Release 更新 AGS 自身。

AGS 不管理账号、请求代理、测速、Web 界面、数据库或会话历史。

## 安装

Linux amd64：

```bash
curl -fsSL https://raw.githubusercontent.com/shellus/ags/main/scripts/install.sh | bash
```

Windows PowerShell amd64：

```powershell
irm https://raw.githubusercontent.com/shellus/ags/main/scripts/install.ps1 | iex
```

AGS 要求系统已安装 Git、Node.js 和 npm。私有环境仓库认证由系统 Git、SSH Key 或 Git Credential Manager 处理。

## 首次配置

直接运行交互式控制台：

```bash
ags
```

非交互配置：

```bash
ags env source set git@github.com:shellus/agent-env.git
ags env configure --profile default --agents codex,claude
ags env diff
ags env apply --yes
```

每台机器独立保存当前 Profile 和启用 Agent；环境仓库只描述可部署内容。

## 命令

```text
ags
├── env
│   ├── source set/show
│   ├── configure
│   ├── apply
│   ├── status
│   ├── diff
│   ├── lock
│   └── validate
├── agent
│   ├── install
│   ├── uninstall
│   └── status
├── provider
│   ├── switch
│   ├── list
│   └── current
├── self
│   ├── update
│   └── version
└── doctor
```

### 环境应用

`ags env apply` 执行完整协调：

1. 拉取配置的 Agent Environment 分支。
2. 严格读取 `environment.lock`。
3. 在本机缓存中获取和构建第三方 Skills。
4. 安装或升级选中的 Agent npm 包。
5. 应用全局指令和受管 Skills。
6. 保存环境 commit、版本和文件哈希。

环境应用不会自动升级 lockfile。维护环境仓库时显式运行：

```bash
ags env lock --repo /path/to/agent-env
ags env validate --repo /path/to/agent-env
```

### Agent 卸载

默认保留认证、历史和非受管配置：

```bash
ags agent uninstall codex
```

彻底删除配置目录：

```bash
ags agent uninstall codex --purge --yes
```

### Provider

Provider 注册表位于操作系统用户配置目录的 `ags/providers.yaml`。

```yaml
version: 2

defaults:
  codex:
    model: "gpt-example"
  claude:
    model: "claude-example"

providers:
  shared-relay:
    universal:
      api_key: "sk-example"
      base_url: "https://relay.example.com"
```

`universal` 表示同一密钥和 Base URL 可供 Codex、Claude 使用。Agent 专用配置存在时完整替代该 Agent 的 Universal 来源，再由全局默认值补全空字段。

```bash
ags provider list
ags provider current
ags provider switch codex shared-relay
```

AGS 只修改以下字段，并保留其他配置：

| 文件 | 字段 |
|---|---|
| Codex `auth.json` | `OPENAI_API_KEY` |
| Codex `config.toml` | 顶层 `model`、`[model_providers.custom].base_url` |
| Claude `settings.json` | 顶层 `model`、`env.ANTHROPIC_AUTH_TOKEN`、`env.ANTHROPIC_BASE_URL` |

## 配置路径

| 数据 | Linux | Windows |
|---|---|---|
| AGS 配置 | `~/.config/ags/config.yaml` | `%AppData%\ags\config.yaml` |
| Provider | `~/.config/ags/providers.yaml` | `%AppData%\ags\providers.yaml` |
| 缓存 | `~/.cache/ags` | 系统用户缓存目录下的 `ags` |
| 状态 | `~/.local/state/ags` | 系统用户状态目录下的 `ags` |

Codex 支持 `CODEX_HOME`，Claude Code 支持 `CLAUDE_CONFIG_DIR`。

## 构建与验证

要求 Go 1.26 或更高版本。

```bash
gofmt -w ./cmd ./internal
go test ./...
go vet ./...
go build -o bin/ags ./cmd/ags
GOOS=windows GOARCH=amd64 go build -o bin/ags-windows-amd64.exe ./cmd/ags
```
