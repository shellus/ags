# 0001：环境数据与 AGS 执行器分离

## 状态

Accepted

## 背景

原有 Agent Skills 从一台 Linux 主机编译后通过 SSH、tar 和 Bash 推送。该流程无法自然覆盖 Windows，也把环境数据、凭证、构建器和远端部署混在同一目录。

## 决策

- AGS 作为公开、跨平台的环境执行器。
- 私有 Agent Environment 仓库保存全局指令、Skills、Profile 和精确版本锁。
- 每台机器上的 AGS 使用系统 Git 主动拉取环境仓库并在本机构建。
- Git、Node.js 和 npm 是外部前置依赖，AGS 不接管系统包管理器。
- 环境应用和版本升级分离：`env apply` 只读取锁，`env lock` 才更新锁。

## 结果

- Windows 不需要接受 SSH 连接。
- 每台机器按本机平台生成依赖。
- 环境仓库 commit 和 lockfile 可复现部署结果。
- AGS 和私有环境数据可以独立发布与授权。
