# Provider Switching

该上下文描述 AGS 如何表达一个上游 Provider 对不同 Agent 的连接能力。

## Language

**Provider**:
一个有名称的上游服务配置，可支持一个或多个 Agent。

**Universal 配置**:
Provider 内可同时供 Codex 和 Claude 使用的一组 Base URL 与 API Key。
_Avoid_: Universal Provider、Universal 类型

**Agent 专用配置**:
Provider 内只面向某个 Agent 的完整连接配置；存在时覆盖该 Agent 从 Universal 配置继承的连接信息。
_Avoid_: 局部覆盖、半覆盖

**有效配置**:
AGS 按“Agent 专用配置优先，否则使用 Universal 配置”解析出的某个 Agent 最终连接配置。
_Avoid_: 合并配置
