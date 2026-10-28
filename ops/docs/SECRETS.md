# 凭据与提交规则

本目录发布于公开仓库；下列路径使用示例用户，凭据只保存在实际服务器上。以下实际文件禁止提交：

| 文件 | 用途 |
| --- | --- |
| `/etc/munge/munge.key` | Slurm 节点认证 |
| `/home/llmops/llm/config/backend-key` | vLLM 后端认证 |
| `/home/llmops/llm/config/client-keys.json` | 网关客户端密钥 |
| `/home/llmops/new-api/config/service.env` | 会话和加密密钥 |
| `/home/llmops/new-api/config/initial-credentials.json` | 初始管理员及普通用户密码、平台 API 令牌 |
| `/home/llmops/new-api/config/tunnel.token` | Cloudflare Tunnel 连接器密钥 |

同样不提交数据库、备份和日志，因为其中可能包含渠道密钥、用户信息或请求内容。
Munge 密钥保持原安装权限；其他私密配置所在目录 0700、文件 0600。
不将 token 放入命令参数、shell 历史、聊天或工单；隧道脚本使用隐藏输入和 token 文件。

`.gitignore` 和提交前扫描降低误提交概率，不能代替人工检查。扫描失败只报告路径、行号和规则，不输出匹配到的秘密。
文件路径、公开下载哈希、令牌生成代码及脱敏测试结果可以提交。

密码或令牌如曾被提交，应立即在对应系统撤销或轮换；只删除当前文件无法移除 Git 历史中的值。
服务器私密配置和数据库需要单独进行受限备份，Git 仓库不能独立恢复这些状态。
