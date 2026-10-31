# 维护快照：2026-10-31（北京时间）

当前核对：`new-api`、`new-api-cloudflared`、`qwen38-gateway`、`qwen38-slurm` 均 active。
邮箱申请版已上线：只填邮箱，管理员批准后自动邮件发放账号和初始密码。
两种新邮件模板已通过 Mailjet 测试接受；原有 SMTP 实际投递已确认。
公网注册页、新版本状态、令牌保护和既有账号最小推理已验证；详见 [上线记录](prlab-customization.md)。
域名与主机信息在公开仓库中使用示例值。

| 项目 | 固定版本或配置 |
| --- | --- |
| OS / cgroup | Ubuntu 22.04.5 / cgroup v2 |
| NVIDIA 驱动 | 580.119.02，部署时保留 |
| GPU | RTX 5090，32 GiB，Slurm 独占 GRES |
| Slurm / hwloc | 26.05.5 / 2.12.2 |
| Python / uv | 3.12.16 / 0.13.6，独立环境 |
| vLLM / Torch | 0.26.0 / 2.11.0+cu130 |
| Transformers | 5.8.1 |
| FlashInfer | 0.6.14，cubin 与 cu130 JIT cache 配套 |
| 模型 | Inferact/Qwen3.8-27B-NVFP4 |
| 模型提交 | 6128240ebaf4eaa7bad2b3d1c72c37d677c5f462 |
| New API | v0.13.2-prlab.6，源码和程序校验值见上线记录 |
| API 模型名 | qwen3.8-27b |
| 上下文 / 多模态 | 16384 token，1 张图片，约 1MP；不启用视频 |
| 并发 | 网关全局最多 4 个请求，第 5 个 429 |
| research / llm | PriorityTier 100 / 10；科研抢占 LLM，REQUEUE |

## 已有验收

- cgroup GPU、CPU、内存隔离及启动前 GPU 占用检查。
- vLLM 原生 NVFP4 内核、文本、图片、工具调用及 SSE。
- 真实模型被科研测试作业抢占、释放显存、重新排队并自动恢复。
- New API 六个初始化测试用户及其令牌已删除；真实申请账号保留，用户名改为注册邮箱，密码与额度不变。申请仍需管理员批准。
- 通过 New API 同时进行四路图文生成；第五个用户请求 429。
- New API 按用户记录成功请求用量。
- New API 的非回环访问由 systemd cgroup IP 规则阻止。

验收 JSON 是历史记录，不能替代实时状态。此次归档未重启模型、重跑抢占测试或改变生产配置。
MCP 使用原来的独立隧道，本仓库不包含其凭据和无关服务配置。

2026-10-31 上下文扩容：16K 已通过运行接口、10017-token 输入、单张 1024×1024 图片和四路普通请求验收。
本次 KV 缓存容量约 34816 tokens，四请求共享；不能保证四个满 16K 请求同时驻留。
未改变缓存精度、0.90 显存预算或科研优先级。详见 [上下文验收](evidence/vllm/context-16k-verification.json)。

2026-10-31 账号清理：六个初始化测试账号及六个令牌已删除；管理员用户名保留。旧网页会话统一失效一次，须重新登录。初始化模板改为仅创建管理员；验收使用临时管理员令牌并在结束时撤销。详见 [邮箱用户名发布记录](evidence/new-api/email-username-release.json)。
