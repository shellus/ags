# 0004：通过 Codex 原生配置禁用 Skill

Agent Environment Profile 使用 `disabled_skills` 声明需要对 Codex 隐藏的 Skill。`env apply` 将声明写入 Codex 原生 `[[skills.config]]`，并设置 `enabled = false`；不删除 `.system` 或其他 Skill 目录。

AGS 只拥有 `config.toml` 中带明确起止注释的禁用 Skill 配置块。应用时保留块外全部文本，配置写入与全局指令和受管 Skill 文件共用备份及回滚边界。移除 Profile 声明后，AGS 删除自己的配置块，不删除用户维护的其他 `[[skills.config]]` 条目。

该设计允许 Codex 升级重建内置 Skill，同时保持禁用状态；也避免 Agent Environment 接管模型、Provider、MCP 或其他本机配置。
