# PRLab 科研主机 LLM 运维

Slurm + cgroup v2 + RTX 5090 + vLLM + Qwen3.8-27B NVFP4 + New API。
目标：科研任务优先，5–6 人共用，最多同时接纳 4 个生成请求，支持图片理解。

## 架构

```text
客户端 → api.example.org（Cloudflare Tunnel，域名待验收）
       → New API :3001（账号、令牌、用量）
       → 并发网关 127.0.0.1:8000（全局 4 请求）
       → vLLM 127.0.0.1:8001（Slurm llm 分区，1 张完整 GPU）

Slurm research 分区：高优先级，可抢占并重新排队 llm 作业。
```

公开示例：SSH 别名 `lab-server`，部署用户及节点名 `llmops`，主目录 `/home/llmops`，域名 `api.example.org`。
这些值均已替换真实部署信息；使用前须按照[迁移说明](MIGRATION.md)适配。
维护快照：2026-10-28（Asia/Shanghai）。本仓库只保存维护源码、非密配置、说明和脱敏验收结果。

## 文档入口

- [迁移范围、示例替换与来源校验](MIGRATION.md)
- [当前状态与版本](docs/STATUS.md)
- [PRLab 邀请注册与审批部署](docs/prlab-customization.md)
- [Slurm 调度和科研提交](docs/slurm-operations.txt)
- [vLLM、图文推理与并发网关](docs/vllm-operations.txt)
- [New API、账号管理与域名接入](docs/new-api-operations.md)
- [Mailjet SMTP 安全录入与测试](docs/new-api-operations.md#mailjet-smtp-接入)
- [部署目录与恢复步骤](docs/RECOVERY.md)
- [凭据与提交规则](docs/SECRETS.md)

## 文件布局

| 目录 | 内容 |
| --- | --- |
| `slurm/config` | 根据维护快照脱敏的调度、cgroup、GPU GRES、Prolog 和服务配置 |
| `slurm/scripts` | 构建与首次安装脚本，执行前阅读恢复说明 |
| `slurm/examples` | 科研 GPU 作业模板 |
| `slurm/tests` | GPU 隔离、占用保护和抢占验证脚本 |
| `vllm/bin` | 并发网关、作业管理、权重下载校验和分区启用脚本 |
| `vllm/config` | 当前作业参数、依赖锁定、公开下载地址与文件哈希 |
| `vllm/systemd` | 网关和 Slurm 作业管理服务 |
| `vllm/tests` | 模拟网关及真实 API、抢占验收脚本 |
| `new-api/bin` | 数据库备份和隐藏输入的隧道激活脚本 |
| `new-api/config` | 无密钥的环境生成、首次初始化脚本和发行校验元数据 |
| `new-api/systemd` | New API 与独立 Cloudflare 隧道服务 |
| `new-api/tests` | 平台调用和不含凭据的状态检查 |
| `docs/evidence` | 历史验收结果，时间和作业编号仅代表当时观测 |
| `scripts` | 提交前语法检查、敏感文件及密钥模式扫描 |

## 常用维护

在主机上只读检查：

```bash
sinfo
squeue
systemctl is-active new-api new-api-cloudflared qwen38-gateway qwen38-slurm
curl -fsS http://127.0.0.1:8000/health
curl -fsS http://127.0.0.1:3001/api/status
```

暂停模型、释放 GPU：`sudo systemctl stop qwen38-slurm`。
恢复模型：`sudo systemctl start qwen38-slurm`。
平台数据库备份：`python3 /home/llmops/new-api/bin/backup-db.py`。
单独取消模型作业会被管理服务重新提交，应通过管理服务暂停模型。

所有 GPU 科研任务通过 Slurm 提交；绕过 Slurm 的任务和 root 可绕过调度隔离。
科研抢占会中断正在进行的推理；重新排队、等待 GPU 和加载模型均需要时间。
测试中观测到的恢复时间不代表服务时延承诺。

## 修改与提交

在 PRLab-api 仓库根目录：

```bash
python3 ops/scripts/validate.py
# 可选：仅当当前仓库没有需要保留的自定义 hooks 时启用
 git config core.hooksPath ops/.githooks
```

检查只解析 Python / JSON、检查 Bash 语法和扫描待跟踪文件，不运行 GPU 测试。
`verify-preemption.py`、`verify-busy-guard.py` 等会调度或抢占 GPU，只在维护窗口人工执行。
Git 提交不自动部署。配置生效前先备份主机原文件，再按组件说明安装、验证与回滚。

公开维护目录不保存真实密码、令牌、Munge 密钥、数据库、备份、日志、模型权重或二进制程序。
代码内的凭据文件路径与随机生成逻辑可以提交；真实值保留在服务器私密文件中。
