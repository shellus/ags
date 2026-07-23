# AGS Context Map

AGS 包含三个独立但协作的领域上下文：

| 上下文 | 词汇 | 实现位置 |
|---|---|---|
| Provider Switching | [contexts/provider/CONTEXT.md](./contexts/provider/CONTEXT.md) | `internal/registry`、`internal/switcher` |
| Agent Environment | [contexts/environment/CONTEXT.md](./contexts/environment/CONTEXT.md) | `internal/agent`、`internal/environment`、`internal/localconfig` |
| Self Distribution | [contexts/self-distribution/CONTEXT.md](./contexts/self-distribution/CONTEXT.md) | `internal/selfupdate`、`.github/workflows`、`scripts` |

共享内容仅限 Agent 名称、系统路径、命令执行和文件事务。Provider 不决定环境 Profile；环境仓库不保存 Provider 密钥；Self Distribution 不读取 Agent Environment。
