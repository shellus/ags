# Agent Environment

## Language

**Agent Environment**:
一台机器上由 AGS 管理的 Codex、Claude Code、全局指令和 Skills 的整体状态。
_Avoid_: Skills 同步

**Environment Repository**:
保存可跨机器共享的环境声明、精确版本锁和已发布 Skill 快照的私有 Git 仓库。
_Avoid_: 配置备份

**Environment Lock**:
固定 Agent npm 版本和维护 Skill 快照所用上游 Git commit 的文件。
_Avoid_: 最新版本

**Published Skill Snapshot**:
环境仓库中已经完成上游获取、模板生成和 patch，可由客户端直接应用的 Skill 内容。
_Avoid_: Skill 源码缓存、本机构建结果

**Upstream Skill Source**:
仅供环境仓库维护流程获取和生成 Published Skill Snapshot 的外部 Git 来源。
_Avoid_: 客户端依赖

**Profile**:
Environment Repository 中一组可供机器选择的 Skill 期望状态，不决定机器启用哪些 Agent。
_Avoid_: 主机、操作系统

**Local Selection**:
保存在本机 AGS 配置中的 Environment Repository、分支、Profile 和启用 Agent。
_Avoid_: Profile

**Managed Content**:
由 AGS 拥有的 Agent 包版本、全局指令和 Skill 条目；首次应用时，环境仓库声明的同名 Skill 会被备份并接管，其他非托管条目保持不变。
_Avoid_: Agent 配置目录中的所有文件

**Drift**:
本机 Managed Content 与当前 Environment Repository 和 Environment Lock 不一致的状态。
