# 维护快照：2026-10-28（北京时间）

当前核对：`new-api`、`new-api-cloudflared`、`qwen38-gateway`、`qwen38-slurm` 均 active。
Cloudflare 隧道已由管理员输入 token 并启动；本次从服务器检查公网地址未成功，域名和公网链路尚待端到端验收。
不要将“隧道进程 active”视为“api.example.org 已可用”。

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
| New API | v0.13.2，官方发行文件 SHA256 已核对 |
| API 模型名 | qwen3.8-27b |
| 上下文 / 多模态 | 8192 token，1 张图片，约 1MP；不启用视频 |
| 并发 | 网关全局最多 4 个请求，第 5 个 429 |
| research / llm | PriorityTier 100 / 10；科研抢占 LLM，REQUEUE |

## 已有验收

- cgroup GPU、CPU、内存隔离及启动前 GPU 占用检查。
- vLLM 原生 NVFP4 内核、文本、图片、工具调用及 SSE。
- 真实模型被科研测试作业抢占、释放显存、重新排队并自动恢复。
- New API 6 个独立普通用户、6 个模型受限令牌，公开注册关闭。
- 通过 New API 同时进行四路图文生成；第五个用户请求 429。
- New API 按用户记录成功请求用量。
- New API 的非回环访问由 systemd cgroup IP 规则阻止。

验收 JSON 是历史记录，不能替代实时状态。此次归档未重启模型、重跑抢占测试或改变生产配置。
MCP 使用原来的独立隧道，本仓库不包含其凭据和无关服务配置。
