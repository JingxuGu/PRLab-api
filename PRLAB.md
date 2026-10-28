# PRLab 维护入口

本仓库同时保存 New API 源码和 PRLab 部署维护资料。上游项目标识、许可证及说明保留不变。

- [运维总览及目录](ops/README.md)
- [迁移说明与完整文件清单](ops/MIGRATION.md)
- [部署状态及版本快照](ops/docs/STATUS.md)
- [Slurm 与科研 GPU 调度](ops/docs/slurm-operations.txt)
- [vLLM、图片理解及全局四请求网关](ops/docs/vllm-operations.txt)
- [New API、账户和 Cloudflare Tunnel](ops/docs/new-api-operations.md)
- [邀请注册与管理员审批部署](ops/docs/prlab-customization.md)
- [恢复及回滚](ops/docs/RECOVERY.md)
- [凭据规则](ops/docs/SECRETS.md)

维护资料以 `ops/` 为唯一入口，后续更新不再依赖原维护仓库。
目录中的主机、用户名、路径及域名均为公开示例，使用前须替换；不含服务器密钥或数据库。

版本边界：`main` 保留上游最新源码；本维护快照对应 v0.13.2，邀请审批定制见
[PR #1](https://github.com/JingxuGu/PRLab-api/pull/1)，以 `prlab-v0.13.2` 为基线。
迁移运维资料不代表合并该功能、升级版本或部署线上服务。
