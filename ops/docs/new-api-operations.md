# PRLab api：New API 本地环境与域名接入

2026-10-31 更新：邮箱申请版 v0.13.2-prlab.6 已上线；用户只填邮箱，管理员批准后邮件发放账号和初始密码，公网访问已验证。
当前版本、验收与恢复步骤见 [上线记录](prlab-customization.md)；以下初装配置和早期未验收状态保留为历史参考。

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

管理员用户名示例：`example-admin`。成员在网页申请，批准后以注册邮箱为用户名，初始密码通过邮件发放。
原六个初始化测试账号及令牌已经删除。管理员初始凭据只保存在服务器私密文件：
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
- 当前只接受邮箱申请；管理员批准后创建普通账号并邮件发放初始密码。初始化脚本仅创建管理员。
- 审批时默认内部额度 100,000,000，可由管理员调整；输入/输出倍率暂均为 1，保留 token 用量统计。
  令牌不另限额，但仍受账户余额约束。管理员可在“用户”中调整额度。
  未配置在线支付或商业收费；这些额度用于实验室内部计量。
- 渠道 `Qwen3.8 local Slurm`：OpenAI 兼容类型，地址 `http://127.0.0.1:8000`。
  渠道使用现有网关 user01 密钥作为内部凭据；普通用户使用 New API 新令牌。
  保留原有网关密钥兼容访问；通过旧入口的请求不计入 New API 账号用量。
- 关闭重试和渠道自动禁用，防止科研抢占期间的 503 导致渠道长期禁用。
- 默认关闭思考输出，以降低交互等待；保留图片理解、工具调用和流式输出。
- 全局最多四个生成请求，第五个返回 429；该限制由同一网关执行。
- 模型上下文现为 16384 token（2026-10-31 扩容），每请求最多 1 张图片，处理尺寸约 1MP；请求体上限 8 MiB。
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
- 历史初装曾创建六个普通测试用户及令牌；现已全部清理，保留管理员与真实申请账号。
- 无效令牌 401；模型列表可用。
- 真实文本、图片、结构化工具调用、SSE 流式输出通过。
- 四个不同账号同时生成成功，第五个账号 429；完成后网关活动请求数回到 0。
- 成功请求用量按用户分别记录；拒绝的第五个账号未产生成功用量。
- 主机内网 IP 上的 3001 访问被阻止，回环地址和临时 SSH 隧道可访问。
- New API 开机自启；域名隧道服务现为 active，公网地址尚待完整验收。
- 原模型 Slurm 作业 16 继续运行，没有重启模型或改变科研抢占配置。

验收详情：[API 结果](evidence/new-api/api-verification.json)、[状态快照](evidence/new-api/state-verification.json)、[发行校验](../new-api/config/release.json)。

## Mailjet SMTP 接入

准备记录：2026-10-31。发件地址及域名已在 Mailjet 激活；本节说明服务器录入流程。
Mailjet 凭据不进入 Git、聊天、命令参数或 shell 历史。真实发件人与审批收件人放在服务器私密配置。
后续管理员已完成隐藏录入、真实认证与测试投递，并确认收到注册申请通知；以下流程用于恢复或轮换凭据。
邮件接入与邀请审批版本部署分开进行，录入脚本不修改注册总开关和密码注册开关。

1. 将 [configure-mailjet.py](../new-api/bin/configure-mailjet.py) 和
   [refresh-mailjet-network.py](../new-api/bin/refresh-mailjet-network.py) 安装到部署目录的 `bin/`，权限 0700。
2. 在服务器创建 `config/mailjet-settings.json`，目录 0700、文件 0600；内容结构如下，
   实际值仅在服务器填写，`admin_username` 必须是现有超级管理员：

   ```json
   {"from":"noreply@example.org","from_name":"","recipient":"owner@example.org","admin_username":"example-admin"}
   ```

   `from_name` 是可选发件人显示名，留空或不填写时只显示发件邮箱，不自动补上域名或平台名。
   新版服务在私密 `service.env` 中使用 `SMTP_FROM_NAME` 指定此名称，与平台名称独立；
   显式设置 `SMTP_FROM_NAME=` 表示不使用显示名，完全未设置该变量时保留上游平台名默认行为；
   邀请审批通知还需设置 `PRLAB_APPROVAL_EMAIL` 为同一收件邮箱。
   修改前保留该文件的受限备份；此环境变量需要平台重启后才被读取。
3. 运行网络配置工具（需要部署账户已有的 sudo 权限）：

   ```bash
   python3 /home/llmops/new-api/bin/refresh-mailjet-network.py --apply
   python3 /home/llmops/new-api/bin/configure-mailjet.py --check
   ```

   它解析 `in-v3.mailjet.com` 的精确公网地址，先在临时受限服务内验证 STARTTLS，
   再写入 `/etc/systemd/system/new-api.service.d/40-mailjet.conf`。
   原 IPAddressDeny 与回环许可保留，仅增加 Mailjet 地址；仅重启 New API 并等待 HTTP 就绪。
   不请求 GPU、不修改模型服务或 Slurm。服务级 IP 规则按 IP 生效，不按端口过滤。
   如果 Mailjet DNS 地址变化，重新运行此工具刷新许可；不直接删除网络隔离规则。
4. 在管理员自己的终端运行（全部 Key 隐藏输入）：

   ```bash
   ssh -t lab-server /home/llmops/new-api/bin/configure-mailjet.py
   ```

   工具先验证现有 root 管理员会话并创建私密 SQLite 一致性备份；初始密码已更改时会隐藏提示输入现密码。
   输入 API Key 和 Secret Key 后，通过证书验证的 `587 + STARTTLS` 认证，向私密配置中的审批邮箱发送一封测试邮件。
   Mailjet 接受邮件后才通过本机管理员 API 保存 SMTP 设置，Key 不另存临时文件，退出时注销会话。
   `SMTPSSLEnabled=false` 表示使用 STARTTLS 升级加密；不是使用明文连接发送凭据。
   不要使用 shell 跟踪、终端录屏、SMTP 调试输出或将 Key 作为参数传入。
5. 检查收件箱和垃圾邮件，并在 Mailjet 事务邮件记录中核对投递结果。
   “Mailjet accepted”只表示提供商已接收，不代表收件邮箱一定收到。
   `config/mailjet-configured.json` 只记录无凭据状态；真实凭据保存在受限平台数据库中，数据库及备份不得提交。
   当前账号申请和账号发放邮件已接入，版本与完整流程见 [申请制上线说明](prlab-customization.md)。

故障处理：工具只输出错误类型，不输出异常详情或凭据。失败时保持注册关闭，按网络、认证、投递、平台保存分层检查。
如需撤销邮件网络许可，移除专用 `40-mailjet.conf`，执行 daemon-reload 并只重启 New API；
其他服务配置保持原样。SMTP 设置本身不会因撤销网络许可而自动清除。

Mailjet 官方参考：[SMTP 配置](https://dev.mailjet.com/docs/smtp-relay/configuration)、
[凭据获取](https://documentation.mailjet.com/hc/en-us/articles/360043229473-How-can-I-configure-my-SMTP-parameters)。

2026-10-31 公告调整：平台首页公告仅保留科研优先与服务忙时重试提示，不再统一展示并发、上下文、图片数量等模型专属限制。
这些参数仍在各模型运行配置和对应运维记录中维护，不通过公告描述全平台模型。首次安装模板已同步，未重新初始化平台。


2026-10-31 维护脚本更新：`config/verify-api.py --smoke` 使用短期管理员令牌执行最小验收，结束即撤销，
不依赖已删除的测试账号。完整验收依旧会占用模型资源。邮箱用户名改动上线时轮换了网页会话密钥，须重新登录一次；
管理员名称、真实账号密码和额度保持不变，渠道内部凭据保留。
