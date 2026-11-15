# 控制面租户登录与授权

在 `server.local.yaml` 中配置 `data.repo.proxy_auth.database_dsn` 或 `data.repo.usage.database_dsn`，并对该数据库执行下文的完整 Goose `up`。同时启用认证和用量时，两项配置使用同一 DSN。客户注册后自动创建租户、唯一管理员、零余额钱包及零流量账户。运营通过数据库初始化，使用独立登录入口。 [首版商业交付 Spec](../../../../docs/features/proxy-first-release/proxy-first-release-spec.md)

| HTTP 接口 | 用途 |
| --- | --- |
| `POST /v1/access/register` | 提交 `phone` 和 `password`，返回租户和主账号名。带或不带 +86 是同一账号，密码至少 12 位。 |
| `POST /v1/access/login` | 提交 `phone` 和 `password`，获取租户管理员会话。 |
| `GET /v1/tenant/account` | 通过会话读取所属租户。 |
| `POST /v1/operations/access/login` | 独立的运营账号登录，提交 `username` 和 `password`。 |
| `POST /v1/operations/tenants/{tenant_id}:reset-password` | 运营核实租户归属后提交 `password`。需要 `manage_tenant_accounts`，重置后原有会话失效。 |

运营账号由受权部署方写入 `operators` 和 `operator_permissions`。给需要的账号分别授予 `read_operations`、`manage_billing` 或 `manage_tenant_accounts`。密码摘要格式为 `<16 字节随机盐的 hex>:<32 字节摘要的 hex>`，使用 PBKDF2-HMAC-SHA256，迭代 600000 次。测试初始化示例见 [本地 Docker 测试环境](../../../../docs/operations/local-test-environment.md)。

网关认证和商业计量等待首版子账号与流量链路接入。当前可验证开户、Recorder 水位、生命周期和回放。

## Recorder 上报与租户用量

在 `server.local.yaml` 中填写 `data.repo.usage.database_dsn`，并按需调整 `biz.usage` 的上报限制。

用量归档和流量额度结算由 `MaintainUsage` 调度。 [Cron 配置与热更新](../../../../docs/standards/proxy-development-conventions.md#cron)

从仓库根目录执行 Goose 迁移。`00002_usage.sql` 依赖 `00001_proxy_auth.sql` 的网关基础表，按版本顺序迁移。用量归档按上海自然日维护分钟分区，运行账号需要相应的 DDL 权限以及计量表的读写权限。

```sh
./.tools/bin/goose -dir apps/proxy-control/migrations postgres "<PostgreSQL DSN>" up
```

生成模型时，数据库账号需能通过 PostgreSQL 的 `information_schema` 读取主键约束，例如表所有者通过 `default_transaction_read_only=on` 建立只读连接。 [数据库、迁移与 DAO](../../../../docs/standards/proxy-development-conventions.md#数据库迁移与-dao)

运营预置 `gateways` 与 `gateway_tokens`，令牌原文为至少 32 字节随机值的无填充 base64url 编码。同一网关同一范围最多保留两个有效令牌。数据库只写编码后令牌的 SHA-256 二进制摘要，采集器通过 HTTPS 将令牌放在 `Authorization: Bearer …` 中。可计量批次当前返回 503，网关保留文件和未确认偏移。

PostgreSQL 集成用例在独立临时 schema 中应用迁移，通过真实 HTTP 路由验证开户、上报、生命周期和回放。 [集成用例配置](../../../../docs/standards/proxy-development-conventions.md#pr-检查与业务回归)

网关生命周期使用同一 `report` 令牌调用 `POST /v1/reporting/lifecycle`。应用 `00003_gateway_lifecycle.sql` 后，可用 `gateway_boot_lifecycle` 核对心跳、失联和退出状态。
