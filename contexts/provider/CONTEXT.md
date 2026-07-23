# Provider Switching

## Language

**Provider**:
一个有名称的上游服务配置，可支持一个或多个 Agent。

**Universal 配置**:
Provider 内可同时供 Codex 和 Claude 使用的一组 Base URL 与 API Key。
_Avoid_: Universal Provider、Universal 类型

**Agent 专用配置**:
Provider 内只面向某个 Agent 的连接配置；存在时替代该 Agent 的 Universal 配置来源，不从 Universal 逐字段继承。
_Avoid_: 局部覆盖、半覆盖

**全局默认值**:
按 Agent 定义的字段默认值，只在 Provider 已声明对应 Agent 后补全空字段。
_Avoid_: 自动启用 Agent

**有效配置**:
AGS 先按“Agent 专用配置优先，否则使用 Universal 配置”选择来源，再应用该 Agent 全局默认值得到的最终配置。
_Avoid_: 合并配置
