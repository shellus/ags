# AGENTS.md

## 项目边界

- 本项目只负责读取 Provider 注册表并修改 Codex、Claude Code 配置文件中的白名单字段。
- 不加入请求代理、Provider 健康检查、测速、账号管理、Web 界面、数据库或配置历史功能；终端界面只保留 Agent 和 Provider 单选。
- Provider 注册表路径保持为用户主目录下的 `.agent-switch/providers.yaml`，不得依赖源码目录或当前工作目录。
- 命令名和项目名保持为 `ags`。

## 配置安全

- 命令输出、错误信息、测试失败信息和文档不得包含真实 API Key 或认证令牌。
- `list` 和 Provider 选择界面必须显示 Provider 名称与 Base URL，但不得显示密钥。
- 修改 Agent 配置时保留所有非白名单字段；新增支持项时必须明确列出允许修改的文件和字段。
- `all` 切换必须先完成所有文件的读取、解析和目标内容生成，再执行写入；部分写入失败时必须恢复已修改文件。

## 跨平台约束

- 用户主目录使用 `os.UserHomeDir`，路径使用 `path/filepath`，不得拼接 Linux 专用绝对路径。
- Codex 目录支持 `CODEX_HOME`，Claude Code 目录支持 `CLAUDE_CONFIG_DIR`。
- 文件替换逻辑必须同时考虑 Unix 可覆盖重命名和 Windows 不可直接覆盖目标文件的差异。
- 配置修改应保留原文件的 LF 或 CRLF 行尾。

## 验证边界

- 字段映射、无关配置保留、缺失配置、无效 Provider、交互选择分支、Base URL 展示和密钥不出现在输出中都必须有自动化测试。
- 修改文件写入和路径逻辑后必须运行 Linux 测试，并完成 Windows amd64 交叉构建。
- 不使用真实用户配置执行自动化测试；测试只操作临时目录和示例密钥。
