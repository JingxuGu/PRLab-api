# 部署目录与恢复

该仓库是现有部署的维护记录，不是一键重装工具。恢复前先确认目标主机、GPU 占用和私密备份。
维护时备份被替换文件；按 Slurm → 模型与网关 → New API → 隧道顺序验证。
仅替换管理平台或网关文件不需要重启模型；模型参数变更才停止并重新启动 qwen38-slurm。

## 源文件到主机的对应

| 仓库 | 主机 |
| --- | --- |
| `slurm/config/*.conf`、`gpu-idle-prolog.sh` | `/etc/slurm/` |
| `slurm/config/*.service` | `/etc/systemd/system/` |
| `slurm/scripts/configure-slurm.sh` | `/home/llmops/slurm-setup/config/configure-slurm.sh` |
| `slurm/scripts/build-slurm.sh` | `/home/llmops/slurm-setup/source/build-slurm.sh` |
| `slurm/examples/` | `/home/llmops/slurm-setup/examples/` |
| `slurm/tests/` | `/home/llmops/slurm-setup/tests/` |
| `vllm/bin/` | `/home/llmops/llm/bin/` |
| `vllm/config/` | `/home/llmops/llm/config/` |
| `vllm/systemd/` | `/etc/systemd/system/` |
| `vllm/tests/` | 原脚本在 `/home/llmops/llm/bin/`，代码内使用主机绝对路径 |
| `new-api/bin/` | `/home/llmops/new-api/bin/` |
| `new-api/config/` | `/home/llmops/new-api/config/` |
| `new-api/tests/` | 原脚本在 `/home/llmops/new-api/config/`，代码内使用主机绝对路径 |
| `new-api/systemd/` | `/etc/systemd/system/` |

首次 Slurm 安装脚本明确拒绝覆盖已有配置，需将相邻配置放入其指定目录后执行。
当前 `slurm.conf` 为已启用 LLM 的运行快照。全新恢复时先暂设 llm 分区 State=DOWN，
在权重、环境和网关完成后才用 `enable-model.sh` 启用；它只修改 llm 分区。
构建脚本所用 Slurm/hwloc 源码包需另行从官方获取，脚本内有固定 SHA256。
Munge 私钥从受限备份恢复或在全新安装中生成，不能从 Git 取回。

## vLLM 环境与模型

- 独立环境：`/home/llmops/llm/venv`；Python 3.12.16，uv 0.13.6。
- 精确包版本见 `vllm/config/requirements.lock.txt`。不改系统 Python 或科研环境。
- FlashInfer cubin 0.6.14 与 JIT cache 0.6.14+cu130 的官方 wheel URL 清单在 config 中。
  首次部署用匹配 CUDA 13.0、Torch 2.11 的 wheel；不能将不匹配的 JIT 包混用。
- 模型下载清单为官方 Hugging Face API 的公开文件哈希快照，固定模型 revision。
  `download-model.py` 对照该清单逐文件校验后才生成 verified 标记。
- 需要自行生成或恢复 `backend-key` 和 `client-keys.json`，而非提交真实值。
  网关读取 JSON 对象的 values 作为入口密钥；后端 key 供模型和网关使用。
- 先验证网关与后端的同一密钥，再启动 qwen38-slurm；确认 /health 就绪。

## New API

- 官方 Linux x86_64 程序下载 URL 和 SHA256 见 `new-api/config/release.json`，
  校验后放到 `/home/llmops/new-api/bin/new-api-v0.13.2`，设置可执行权限。
- 首次部署可用 `create-env.py` 创建私密环境文件；它保留已有文件。
  该版本发布程序自身显示 v0.0.0；应在 service.env 设置 `VERSION=v0.13.2`，
  程序身份仍以已核对的 SHA256 为准。
- 恢复现有实例时恢复原 config 和 SQLite 一致性备份，保留原会话/加密密钥。
- `initialize.py` 用于首次部署，密码和令牌在服务器上随机生成并私下保存。
  它会调整平台设置及账号初始额度；现有部署维护不重新执行。
- 渠道使用网关现有 user01 密钥。若轮换该密钥，在 New API UI 同步更新渠道密钥。
- New API 服务自身监听通配地址，但 cgroup IPAddressDeny/Allow 限制仅回环可访问；
  恢复时保留这些服务规则并重新测试内网 IP 不可访问。
- `backup-db.py` 做在线 SQLite 一致性备份。数据库、环境文件与初始凭据不在 Git 中。

## 独立隧道

`activate-tunnel` 从 TTY 隐藏输入 token，凭据由 Cloudflare 管理员在平台获取。
它仅在 token 文件不存在时写入并启用服务，不覆盖现有凭据。
当前主机已启用该服务；常规维护用 systemctl 查看或重启即可。
需要更换隧道时先核对云端路由与凭据，不直接重跑首次激活脚本。
域名设置和连通验收步骤见 New API 运维文档。

## 回滚

修改 systemd 单位后运行 `sudo systemctl daemon-reload`；单独重启受影响服务。
Slurm 配置先检查再 `sudo scontrol reconfigure`，避免无故重启 GPU 作业。
失败时恢复已备份配置和匹配版本。New API 数据库升级后回滚需要匹配备份，不能只替换程序。
包含抢占的验收脚本会中断 LLM，仅在人工安排的维护窗口运行。
