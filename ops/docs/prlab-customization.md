# PRLab 邀请注册功能：待部署

源码 fork：https://github.com/JingxuGu/PRLab-api
PR：https://github.com/JingxuGu/PRLab-api/pull/1
维护基线：prlab-v0.13.2（bee339d279ccecbf8c8a89e14ddbbd902f78bd5d）
功能分支：feat/invitation-approval
功能提交：5f823c225

当前线上仍为原 v0.13.2，站点名 PRLab api；本次仅完成源码、PR、测试和构建，未切换线上服务。
功能开发阶段选择了“尚无 SMTP，先完成邀请和审批功能”，因此当时未启用真实邮件发送。
2026-10-31：Mailjet 域名与发件地址已激活，凭据已在服务器隐藏录入，管理员确认收到测试邮件。
发件名通过 SMTP_FROM_NAME 独立配置；管理员选择不使用显示名，私密环境显式设置 SMTP_FROM_NAME=。
现有平台仍为上游 v0.13.2，邀请审批版尚未部署，完整注册通知链路仍需上线后验收。
实际审批收件邮箱须作为服务器私密环境变量配置，公开 fork 不包含此地址。

## 新功能

- 超级管理员在用户管理创建绑定邮箱的一次性邀请，默认有效 7 天，可撤销。
- 用户提交后仅创建待审批申请，未批准前没有正式用户或 API 令牌。
- 超级管理员批准才创建普通用户；拒绝会清除待审批密码哈希。
- 邀请消费与审批使用数据库事务和条件更新，防止重复使用或重复批准。
- OAuth/微信自动注册不能绕过邀请策略。
- SMTP 未配置或发送失败不会丢失申请，后台仍可人工审批及重发通知。
- 邮件链接进入需登录的用户管理页，不通过 GET 或邮件扫描器直接批准账号。

## 已验证

- model/controller/router/service 完整测试通过，包括 11 项新增安全测试。
- 前端冻结依赖的生产构建通过，完整 Go 程序构建通过。
- 编译均通过 Slurm 的无 GPU 作业执行；模型作业保留运行，本次通用发件名回归构建也未申请 GPU。
- 最终构建版本：v0.13.2-prlab.3。
- 产物：/home/llmops/new-api-build/bin/new-api-prlab-v0.13.2-prlab.3。
- SHA256：954c62602c4c5908c2aa392efed1e37c4473252f765ea484fdfe18058ca33949。
- 独立构建目录：/home/llmops/new-api-build，不修改系统 Go、Python 或科研环境。
- 数据库实际运行测试使用 SQLite，未启动额外的 MySQL/PostgreSQL 服务。

## 后续部署顺序

1. 备份 /home/llmops/new-api/data/new-api.db 和服务器私密 config。
2. 在数据库副本上验证新版本迁移与启动。
3. 保留现有 systemd CPU、内存、设备和回环访问限制。
4. 在私密环境中设置 INVITATION_REGISTRATION_ENABLED=true。
5. 根据管理员提供的地址设置 PRLAB_APPROVAL_EMAIL；真实值不提交 Git。
6. 原子替换为已验证程序并重启 New API，然后启用注册总开关和密码注册开关。
   邀请策略会接管公开注册，仍需要有效邀请和管理员审批。
7. 验证站点名称、现有账户/渠道/令牌、待审批限制、批准后登录及四请求网关。
8. Mailjet SMTP 已通过实际投递测试，现有 IPAddressDeny 保留，专用 drop-in 仅增加 Mailjet 精确地址许可。
   后续按 New API 运维文档刷新 DNS 地址许可；完整注册申请的通知仍需在新版本上验证。

回滚到上游旧程序前必须先关闭注册总开关，以防旧版本忽略邀请策略。
