# Agent Environment

## Language

**Agent Environment**:
一台机器上由 AGS 管理的 Codex、Claude Code、全局指令和 Skills 的整体状态。
_Avoid_: Skills 同步

**Environment Repository**:
保存可跨机器共享的环境声明和精确版本锁的私有 Git 仓库。
_Avoid_: 配置备份

**Environment Lock**:
固定 Agent npm 版本和第三方 Skill Git commit 的文件。
_Avoid_: 最新版本

**Profile**:
Environment Repository 中一组可供机器选择的 Skill 期望状态，不决定机器启用哪些 Agent。
_Avoid_: 主机、操作系统

**Local Selection**:
保存在本机 AGS 配置中的 Environment Repository、分支、Profile 和启用 Agent。
_Avoid_: Profile

**Managed Content**:
由 AGS 拥有的 Agent 包版本、全局指令和 Skill 条目。
_Avoid_: Agent 配置目录中的所有文件

**Drift**:
本机 Managed Content 与当前 Environment Repository 和 Environment Lock 不一致的状态。
