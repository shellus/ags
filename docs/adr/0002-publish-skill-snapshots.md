# 0002：客户端只应用已发布 Skill 快照

Agent Environment 仓库保存经过上游获取、模板生成和 patch 的最终 Skill 快照；`env vendor` 仅在维护仓库时生成快照，客户端 `env apply` 不访问上游 Skill Git 仓库。快照哈希将文本文件的 CRLF 规范化为 LF、保留二进制文件原始字节，避免 Git 在 Windows 检出时造成误报。该边界增加环境仓库体积，但让 Linux 和 Windows 应用同一份受审查内容，并避免每台机器重复下载和处理第三方来源。
