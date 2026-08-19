# AGENTS.md

## 项目边界

- AGS 是 Codex 和 Claude Code 的跨平台环境管理器。
- AGS 管理 Agent npm 安装版本、Provider、本机环境选择、私有环境仓库、全局指令、Skills、状态检查和自身更新。
- AGS 不管理账号生命周期、请求代理、Provider 健康检查、测速、Web 界面、数据库或会话历史。
- 只支持 Codex 和 Claude Code，不加入 pi、Antigravity CLI 或 Claude 自定义 agents。
- 环境数据与执行器分离：AGS 是公开执行器，私有环境数据由独立 Agent Environment 仓库提供。

## 领域边界

- Provider Switching 只负责 Provider 注册表解析和 Agent 白名单字段修改。
- Agent Environment 负责环境仓库、Profile、版本锁、Agent 包、全局指令、Skills，以及 Profile 声明的 Codex 禁用 Skill 配置。
- Self Distribution 负责 AGS Release、首次安装和自更新。
- 三个领域共享 Agent 名称和系统路径，不复制 Provider、环境或更新规则。
- 领域术语和映射见 `CONTEXT-MAP.md`。

## 配置与状态

- Linux 和 Windows 的配置目录统一为 `~/.ags/`，包含 `config.yaml` 和 `providers.yaml`。
- 缓存目录通过 `os.UserCacheDir` 解析，保存环境仓库 checkout、第三方来源和构建暂存。
- Linux 状态默认位于 `~/.local/state/ags`；Windows 使用本机状态目录。
- Codex 支持 `CODEX_HOME`，Claude 支持 `CLAUDE_CONFIG_DIR`。
- 不读取旧 `.agent-switch` 或 `/data/coding` 路径，不保留兼容分支。

## 环境应用规则

- `env apply` 只应用 `environment.lock` 中的精确版本，不隐式升级依赖。
- `env lock` 只能修改明确指定的本地环境仓库，不自动 commit 或 push。
- `env vendor` 只在维护 Agent Environment 仓库时获取上游来源、执行构建和 patch，并原子发布 `skills/vendor` 快照。
- `env apply` 只消费环境仓库中已提交的 `skills/local` 和 `skills/vendor`，不得获取或处理上游 Skill Git 仓库。
- Agent Environment 仓库是受信任代码源；快照内 Skill 可以声明正常 npm 运行依赖。
- 所有已发布 Skill 先进入本机构建暂存，运行依赖准备完成后才修改 Agent。
- 未受管同名 Skill 必须报告冲突；只删除 `.ags-managed.json` 记录的 Skill。
- 全局指令由环境仓库完整拥有，不做本机局部合并。
- Codex 禁用 Skill 使用原生 `[[skills.config]]`，AGS 只替换带明确起止标记的专属配置块，不覆盖整个 `config.toml`。
- Agent 包更新和受管文件应用失败时必须回滚已完成步骤。
- 卸载默认保留认证、会话、缓存和非受管设置；`--purge` 才删除整个 Agent 配置目录。

## Provider 模型

- `universal` 是 Provider 的共享配置层，不是独立 Provider 类型。
- Agent 专用配置存在时优先选择整个 Agent 专用块，不从 Universal 逐字段混合。
- 全局默认值只补全 Provider 已声明或通过 Universal 支持的 Agent。
- `model` 为空时不修改目标 Agent 的现有模型。
- 切换、状态识别、列表和交互选择必须统一调用 Provider 有效配置解析。
- 命令输出、错误和测试不得包含真实密钥或认证令牌。
- `all` 切换必须先生成所有目标内容，再通过事务统一写入和回滚。

## 交互和命令

- 无参数 `ags` 是主要交互入口；所有交互操作必须调用与非交互命令相同的服务。
- 非 TTY 环境缺少必要参数时返回用法错误，不启动 TUI。
- 破坏性操作和非交互覆盖要求 `--yes`；完整清理还要求 `--purge`。
- 不保留旧根命令别名；命令按 `env`、`agent`、`provider`、`self` 和 `doctor` 分组。

## 跨平台与发布

- 配置目录基于 `os.UserHomeDir`；缓存、状态和 Agent 目标路径使用系统目录 API 与 `path/filepath`。
- 外部依赖只要求系统已有 Git、Node.js 和 npm；AGS 不调用 apt、winget 或其他系统包管理器。
- Git 私有仓库认证完全交给系统 Git、SSH 和 credential helper。
- 修改路径、事务、安装或更新逻辑后必须运行 Linux 测试和 Windows amd64 交叉构建。
- Release 必须同时包含 Linux amd64、Windows amd64 和 SHA-256 校验文件。

## 验证

```bash
gofmt -w ./cmd ./internal
go test ./...
go vet ./...
go build -o bin/ags ./cmd/ags
GOOS=windows GOARCH=amd64 go build -o bin/ags-windows-amd64.exe ./cmd/ags
```

- 测试只操作临时目录和示例密钥，不读取或修改真实用户配置。
- 环境编译测试必须覆盖来源锁、Profile 选择、patch、Node 依赖、受管冲突和回滚。
- 交互层测试只验证选择映射；业务行为由共享服务测试。
