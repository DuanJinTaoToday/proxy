# 控制面租户登录与授权

在 `apps/proxy-control/` 按开发规范准备本地配置并启动服务。在 `server.local.yaml` 中启用 `biz.access` 所需配置，并为每个账号填写独立的 `password_hash`。 [本地启动](../../../../docs/standards/proxy-development-conventions.md#本地启动)

可以在可信终端运行以下命令生成摘要。

```sh
python -c "import getpass, hashlib, secrets; salt = secrets.token_bytes(16); key = hashlib.pbkdf2_hmac('sha256', getpass.getpass().encode(), salt, 600000); print(salt.hex() + ':' + key.hex())"
```

| HTTP 接口 | 用途 |
| --- | --- |
| `POST /v1/access/login` | 用 JSON 的 `username`、`password` 换取 `access_token` 和 `expires_in_seconds`。 |
| `GET /v1/access/tenants` | 返回当前身份的有效租户列表。 |
| `GET /v1/access/tenants/{tenant_id}` | 验证并返回选中的租户。控制台保存选择，每个后续业务请求仍需独立授权。 |
| `POST /v1/access/check` | 用 JSON 的 `permission` 和 `tenant_id` 检查当前身份。普通 JSON 的权限值为 1（读租户用量）、2（管理代理凭据）、3（读平台运营数据）。权限 3 省略 `tenant_id`。 |

登录后通过 `Authorization: Bearer <access_token>` 传递令牌。无有效身份返回 401，无权限返回 403。租户管理员只获得其有效成员关系内的权限。业务接口调用 `access.Service.Require`，传入本次实际操作的租户和服务端指定的权限。

默认配置允许启动健康检查，但没有预置登录账号。预置成员关系由启动配置加载，变更后重启控制面生效。会话有效期为八小时，签名密钥随进程启动生成，因此重启后需要重新登录。对外访问由部署入口提供 HTTPS 和登录频率限制。

## Recorder 上报与租户用量

在 `server.local.yaml` 中填写 `data.repo.usage.database_dsn`，并按需调整 `biz.usage` 的上报限制。

用量归档和流量额度结算由 `MaintainUsage` 调度。 [Cron 配置与热更新](../../../../docs/standards/proxy-development-conventions.md#cron)

从仓库根目录执行 Goose 迁移。`00002_usage.sql` 依赖 `00001_proxy_auth.sql` 的网关、凭据和额度基础表，按版本顺序迁移。用量服务按上海自然日创建和删除分钟分区，运行账号需要相应的 DDL 权限以及计量表的读写权限。

```sh
./.tools/bin/goose -dir apps/proxy-control/migrations postgres "<PostgreSQL DSN>" up
```

生成模型时，数据库账号需能通过 PostgreSQL 的 `information_schema` 读取主键约束，例如表所有者通过 `default_transaction_read_only=on` 建立只读连接。 [数据库、迁移与 DAO](../../../../docs/standards/proxy-development-conventions.md#数据库迁移与-dao)

运营预置 `gateways` 与 `gateway_tokens`，令牌原文为至少 32 字节随机值的无填充 base64url 编码。同一网关同一范围最多保留两个有效令牌。数据库只写编码后令牌的 SHA-256 二进制摘要，采集器通过 HTTPS 将令牌放在 `Authorization: Bearer …` 中。`SubmitBatch` 通过 #3 的 `proxy_credentials.client_id → tenant_id` 映射归集，映射在凭据停用后仍可查询。

两个租户接口沿用控制台会话令牌。日用量的起止日期使用上海自然日，首尾日期都包含在内，最多 93 天。日用量和余额响应按 Protobuf JSON 编码，64 位字节数字段用字符串表示。

PostgreSQL 集成用例在独立临时 schema 中应用迁移，通过真实 HTTP 路由验证凭据发放、停用及用量归集。 [集成用例配置](../../../../docs/standards/proxy-development-conventions.md#pr-检查与业务回归)

## 代理凭据与网关认证同步

应用 `00001_proxy_auth.sql` Goose 迁移，在 `server.local.yaml` 中填写 `data.repo.proxy_auth.database_dsn`。

租户管理员使用控制台会话令牌发放和停用代理凭据。发放响应仅交付一次密码。

- `POST /v1/tenants/{tenant_id}/proxy-credentials` 接收可选的 `expires_at` 和 `allowed_source_cidrs`。
- `POST /v1/tenants/{tenant_id}/proxy-credentials/{client_id}:disable` 幂等停用并保留历史租户归属。
- `POST /v1/gateway/auth-state/load` 由持有逐网关 `auth` 令牌的中继调用。校验密码后返回该网关专用的刷新引用。
- `POST /v1/gateway/auth-state/refresh` 每次接受最多 100 个曾签发给该网关的引用。状态响应携带密码校验值，只能经 HTTPS 传输。

运营预置 `gateways` 和 `gateway_tokens`。令牌为至少 32 字节随机值的无填充 base64url 编码，数据库保存令牌字符串的 SHA-256 二进制摘要。轮换使用同一范围的另一个 `rotation_slot`，随后吊销旧令牌。余额来自已生效且完成激活的授予减去欠费，缺少额度时按准入门槛拒绝。充值、调整和上报积压期间的额度续期联动见 #14。

网关生命周期使用同一 `report` 令牌调用 `POST /v1/reporting/lifecycle`。应用 `00003_gateway_lifecycle.sql` 后，可用 `gateway_boot_lifecycle` 核对心跳、失联和退出状态。
