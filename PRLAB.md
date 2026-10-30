# PRLab 维护入口

本仓库同时保存 New API 源码和 PRLab 部署维护资料。上游项目标识、许可证及说明保留不变。

- [运维总览及目录](ops/README.md)
- [迁移说明与完整文件清单](ops/MIGRATION.md)
- [部署状态及版本快照](ops/docs/STATUS.md)
- [Slurm 与科研 GPU 调度](ops/docs/slurm-operations.txt)
- [vLLM、图片理解及全局四请求网关](ops/docs/vllm-operations.txt)
- [New API、账户和 Cloudflare Tunnel](ops/docs/new-api-operations.md)
- [邮箱申请与账号发放](ops/docs/prlab-customization.md)
- [恢复及回滚](ops/docs/RECOVERY.md)
- [凭据规则](ops/docs/SECRETS.md)

维护资料以 `ops/` 为唯一入口，后续更新不再依赖原维护仓库。
目录中的主机、用户名、路径及域名均为公开示例，使用前须替换；不含服务器密钥或数据库。

版本边界：`main` 保留上游最新源码；本维护快照对应 v0.13.2，邮箱申请定制见
[PR #2](https://github.com/JingxuGu/PRLab-api/pull/2)，以 `prlab-v0.13.2` 为基线。
2026-10-31：PR #2 已合并至维护分支，邮箱申请版 v0.13.2-prlab.5 已上线；
main 继续保留上游新版本，当前发布请使用维护分支，详见上线记录。
