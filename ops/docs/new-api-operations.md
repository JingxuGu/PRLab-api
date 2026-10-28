# PRLab api：New API 本地环境与域名接入

已部署在 `ssh lab-server`，账户 `llmops`。日期：2026-10-28（北京时间）。

当前访问链路：New API（3001）→ 全局四请求网关（8000）→ vLLM（8001）→ Slurm GPU 作业。
New API 管理账号、令牌和用量；科研仍可抢占模型。

状态更新（2026-10-28）：管理员已在主机输入 token，独立隧道服务当前 active。
本次服务器端公网检查未成功，域名路由与公网可用性尚待端到端验收；下面保留完整首次配置步骤。

## 立即在个人电脑使用

在个人电脑终端运行，并保持窗口打开：

```bash
ssh -N -o ExitOnForwardFailure=yes -L 127.0.0.1:13001:127.0.0.1:3001 lab-server
```

浏览器打开 **http://localhost:13001**。

管理员用户名：`example-admin`。普通用户：`user01` 到 `user06`，每人独立账户、密码和令牌。
初始密码与令牌只保存在服务器私密文件：
`/home/llmops/new-api/config/initial-credentials.json`，权限 0600。通过你自己的 SSH 终端私下查看、分发，不要粘贴到聊天、工单或 Git。
登录后可在个人设置修改密码，在“令牌”页面管理 API Key；修改后此初始文件不会自动更新。

客户端设置：

| 项目 | 当前本地值 | 域名开通后 |
| --- | --- | --- |
| Base URL | `http://localhost:13001/v1` | `https://api.example.org/v1` |
| Model | `qwen3.8-27b` | 相同 |
| API Key | 对应用户的 New API 令牌 | 相同 |

New API 是管理与分发平台；可接入支持 OpenAI 接口的聊天客户端。
图片使用 `image_url` 内容项，并将图片编码为 `data:image/...;base64,...`。

## 已完成的配置

- 官方正式版 `v0.13.2`，固定发布文件并通过 GitHub 官方 SHA256 校验。
  发布资产本身未嵌入版本号，已用 `VERSION=v0.13.2` 标记；校验值以 release.json 为准。
- 独立目录 `/home/llmops/new-api`，原生 systemd 服务 `new-api.service`，开机自动启动。
- 独立 SQLite 数据库；此规模下不额外部署 Redis 或数据库服务。
- 平台最多使用 1 个 CPU 核的计算配额、1 GiB 内存，降低 CPU 权重，隔离 GPU 设备。
- 平台自身的网络仅允许回环地址，由 systemd cgroup 网络规则实施。
  程序自身监听 `*:3001`，实际已验证通过主机内网 IP 无法访问；保留服务中的 IPAddressDeny/Allow 规则。
  后续 Cloudflare Tunnel 也从本机回环访问。当前不开放局域网端口。
- 关闭公开注册；管理员创建 6 个普通账户，6 个令牌均限制为 `qwen3.8-27b`。
- 每个用户初始内部额度 100,000,000；输入/输出倍率暂均为 1，保留 token 用量统计。
  令牌不另限额，但仍受账户余额约束。管理员可在“用户”中调整额度。
  未配置在线支付或商业收费；这些额度用于实验室内部计量。
- 渠道 `Qwen3.8 local Slurm`：OpenAI 兼容类型，地址 `http://127.0.0.1:8000`。
  渠道使用现有网关 user01 密钥作为内部凭据；普通用户使用 New API 新令牌。
  保留原有网关密钥兼容访问；通过旧入口的请求不计入 New API 账号用量。
- 关闭重试和渠道自动禁用，防止科研抢占期间的 503 导致渠道长期禁用。
- 默认关闭思考输出，以降低交互等待；保留图片理解、工具调用和流式输出。
- 全局最多四个生成请求，第五个返回 429；该限制由同一网关执行。
- 模型上下文 8192 token，每请求最多 1 张图片，处理尺寸约 1MP；请求体上限 8 MiB。
- 科研抢占模型时，正在进行的推理可能中断，模型不可用时返回 503，稍后重试。
- 因平台仅可访问本机，外部图片 URL 拉取不可用；使用 data URL 传图。
- 不改动 Slurm、CUDA、NVIDIA 驱动、科研 Python 环境或已有 MCP 隧道。

## 之后配置 api.example.org

使用 **独立 Cloudflare Tunnel**，名称建议 `prlab-api`。它与已有 MCP 隧道分开。
服务文件已安装，管理员已启用主机端连接器。以下是首次接入或恢复时的配置参考，不代表域名已通过验收。

1. 确认 `example.org` 已在 Cloudflare DNS 中处于 Active 状态。
2. 打开 Cloudflare Zero Trust 控制台，进入 **Networks → Connectors → Cloudflare Tunnels**
   （部分界面显示 Networks → Tunnels），新建 **Cloudflared** 类型隧道，命名 `prlab-api`。
3. 在连接器安装页面选择 Linux，获取其 Tunnel token。
   主机已经安装 cloudflared，无需运行页面上的安装命令。
   只取 `cloudflared service install` 或 `--token` 后面的完整长 token（通常 eyJ 开头）。
   Tunnel ID 是 UUID，不是 token；页面遮盖星号不能作为真实 token 输入。私下保存，勿发送到聊天。
4. 在个人电脑运行：

   ```bash
   ssh -t lab-server /home/llmops/new-api/bin/activate-tunnel
   ```

   在隐藏输入提示中粘贴该新隧道的 token。脚本只在服务器保存权限 0600 的 token 文件，
   然后启用 `new-api-cloudflared.service`。token 不写入进程参数或聊天。
   如已有 token 文件，脚本保留文件并停止，避免误覆盖已有接入。
5. 在该隧道中添加 **Published application / Public hostname**：

   | 配置项 | 值 |
   | --- | --- |
   | Subdomain | `api` |
   | Domain | `example.org` |
   | Path | 留空 |
   | Service Type | `HTTP` |
   | URL | `127.0.0.1:3001` |

   保存后 Cloudflare 通常会自动创建 `api` 的代理 CNAME。
   如果提示同名记录已存在，先确认该记录用途；不要覆盖其他服务记录。
   科研主机的 `192.0.2.10` 是内网地址，通过 Tunnel 接入，无需公网端口映射。
6. 在 DNS 中核对 `api` 记录指向该 Tunnel，代理已开启。
   启用 HTTPS 重定向；为 `api.example.org` 配置缓存绕过，避免缓存 API 和登录页面。
   常规 API 客户端使用 New API Bearer 令牌认证；交互式 Cloudflare Access 登录会影响普通 API 客户端调用。
7. 访问 `https://api.example.org` 登录管理员，在系统设置中把“服务器地址”从
   `http://localhost:13001` 改为 **`https://api.example.org`**。
8. 客户端 Base URL 改成 `https://api.example.org/v1`；使用 New API 令牌，模型名保持不变。
   建议使用 `stream: true` 流式输出。

域名开通后验收：网页登录正常；无令牌访问 `/v1/models` 返回 401；
有令牌可列出模型，文本和 data URL 图片生成正常，流式输出持续返回。
目前仅完成本地链路验收，公网连通性需域名配置后再验证。

Cloudflare 官方步骤：
https://developers.cloudflare.com/cloudflare-one/networks/connectors/cloudflare-tunnel/get-started/create-remote-tunnel/

## 维护

在服务器上：

```bash
systemctl status new-api --no-pager
curl -fsS http://127.0.0.1:3001/api/status
sudo systemctl restart new-api
python3 /home/llmops/new-api/bin/backup-db.py
```

数据库：`/home/llmops/new-api/data/new-api.db`。
数据库在线一致性备份：`/home/llmops/new-api/backups/`，仅本机保存，不是异地容灾备份。
备份和 config 中包含凭据，应妥善保管，不要上传 Git。升级前备份数据库和 config，固定版本后再升级。
当前未配置自动升级或自动定期备份。普通平台操作使用 UI，不要重新运行初始化脚本。

服务参数：`/home/llmops/new-api/config/service.env`，权限 0600，包含会话密钥。
服务单位：`/etc/systemd/system/new-api.service`。
日志：`sudo journalctl -u new-api -n 80 --no-pager` 和 `/home/llmops/new-api/logs/`。
用量日志已验证写入数据库，记录用户、模型、token 与额度元数据；未开启请求正文调试日志。

未来隧道状态：`systemctl status new-api-cloudflared --no-pager`。
隧道 token：`/home/llmops/new-api/config/tunnel.token`，服务器上私下保存，禁止提交 Git。
隧道固定使用 HTTP/2、IPv4，避免校园网络中 UDP 隧道的不稳定；指标端口仅本机 20242。
停止公网入口：`sudo systemctl stop new-api-cloudflared`。
停止管理平台：`sudo systemctl stop new-api`；这不会停止 GPU 模型。
停止模型释放 GPU：`sudo systemctl stop qwen38-slurm`。

## 本地验收

- 官方下载文件 SHA256 校验通过。
- 管理员、6 个普通用户与 6 个模型受限令牌存在；公开注册关闭。
- 无效令牌 401；模型列表可用。
- 真实文本、图片、结构化工具调用、SSE 流式输出通过。
- 四个不同账号同时生成成功，第五个账号 429；完成后网关活动请求数回到 0。
- 成功请求用量按用户分别记录；拒绝的第五个账号未产生成功用量。
- 主机内网 IP 上的 3001 访问被阻止，回环地址和临时 SSH 隧道可访问。
- New API 开机自启；域名隧道服务现为 active，公网地址尚待完整验收。
- 原模型 Slurm 作业 16 继续运行，没有重启模型或改变科研抢占配置。

验收详情：[API 结果](evidence/new-api/api-verification.json)、[状态快照](evidence/new-api/state-verification.json)、[发行校验](../new-api/config/release.json)。
