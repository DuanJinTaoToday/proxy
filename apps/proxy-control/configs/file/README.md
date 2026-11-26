# 控制面租户登录与授权

在 `server.local.yaml` 中配置 `data.repo.proxy_auth.database_dsn` 或 `data.repo.usage.database_dsn`，并对该数据库执行下文的完整 Goose `up`。同时启用认证和用量时，两项配置使用同一 DSN。客户注册后自动创建租户、唯一管理员、零余额钱包及零流量账户。运营通过数据库初始化，使用独立登录入口。 [首版商业交付 Spec](../../../../docs/features/proxy-first-release/proxy-first-release-spec.md)

| HTTP 接口 | 用途 |
| --- | --- |
| `POST /v1/access/register-code` | 提交 `email` 申请验证码。公开回执不区分邮箱是否已开户。 |
| `POST /v1/access/register` | 提交 `email`、`password` 和 `verification_code`。密码至少 12 位。 |
| `POST /v1/access/login` | 提交 `email` 和 `password`，获取租户管理员会话。 |
| `GET /v1/tenant/account` | 通过会话读取所属租户及已验证邮箱。 |
| `POST /v1/access/forgot-password` | 提交 `email` 申请通过原邮箱找回密码，使用统一公开回执。 |
| `POST /v1/access:reset-password` | 提交邮件 `token` 和新 `password`，完成后旧会话失效。 |
| `POST /v1/operations/access/login` | 独立的运营账号登录，提交 `username` 和 `password`。 |
| `POST /v1/operations/tenants/{tenant_id}:recover-email` | 运营核实租户归属后提交新的 `email`。需要 `manage_tenant_accounts`。 |
| `POST /v1/access:recover-email` | 提交邮件 `token` 和新 `password`，验证新邮箱并撤销旧会话。 |
| `POST /v1/operations/tenants/{tenant_id}:reset-password` | 原邮箱无法使用时保留的运营兜底方式。运营核实归属后可直接提交 `password`。需要 `manage_tenant_accounts`。 |

邮箱键去掉首尾空白并转为小写，保留加号和点号。邮件发往实际验证的地址。注册验证码有效 10 分钟，连续输错 5 次即失效。重置和恢复链接有效 30 分钟，重新申请会使对应旧链接失效。两个公开发信入口合计每个邮箱每 15 分钟最多 5 次，每个来源 IPv4 地址或 IPv6 /64 每小时最多 60 次。走 HTTPS 入口时，按下文配置可信代理以识别来源。运营可按主账号名或注册邮箱搜索租户。

在受保护的配置源设置 `biz.email_auth`。填入客户控制台的 HTTPS origin、所有控制面实例共享的 32 字节随机 `code_key`（无填充 base64url）、SMTP `smtp_address`（`host:port`）和发件邮箱 `from_address`。SMTP relay 需支持 STARTTLS，经授权时同时配置 `smtp_username` 和 `smtp_password`。隔离的本地测试使用 Mailpit，只有该类测试环境可设置 `allow_plaintext: true`。发信配置为空时，已有邮箱账户仍可登录。启用开户、找回和运营邮件恢复前应核对真实 SMTP 投递。命令 `docker compose -f deploy/test/compose.yaml port mailpit 8025` 可获取本地测试邮箱的 Web 入口。

迁移 `00018_email_access.sql` 保留既有租户 ID、主账号名和全部账务与订单关系。已知的本地测试账户迁移为 `alpha@example.test`、`beta@example.test` 和 `chenxl-studio@example.test`。其余没有验证邮箱的旧租户可由运营按主账号名查找并协助验证新邮箱。新账户没有历史手机号，创建新账户后需执行前向修复而无法降回原有手机号必填模式。

运营账号由受权部署方写入 `operators` 和 `operator_permissions`。给需要的账号分别授予 `read_operations`、`manage_billing` 或 `manage_tenant_accounts`。密码摘要格式为 `<16 字节随机盐的 hex>:<32 字节摘要的 hex>`，使用 PBKDF2-HMAC-SHA256，迭代 600000 次。测试初始化示例见 [本地 Docker 测试环境](../../../../docs/operations/local-test-environment.md)。

网关认证和商业计量等待首版子账号与流量链路接入。当前可验证开户、Recorder 水位、生命周期和回放。

## 当前访问 IP 归属地

租户登录后可请求 `GET /v1/tenant/access-location`。返回本次请求的 `ip`、国家或地区名称 `location` 和 `is_mainland_china`。`Accept-Language: zh-CN` 返回中文，默认返回英文。只有国家代码为 `CN` 时标记为中国大陆，`HK`、`MO` 和 `TW` 独立识别。归属地未知时 `location` 为空、`is_mainland_china` 为 `false`，来源 IP 无法确定时 `ip` 也为空。

在 `server.http.trusted_proxy_cidrs` 中配置直接 HTTPS 入口及转发链路中由部署方控制的代理地址，固定 IPv4 和 IPv6 地址分别使用 `/32` 和 `/128`。限制对控制面监听端口的访问，避免其他容器或主机使用这些地址。公网入口必须丢弃客户传入的 `X-Forwarded-For` 并用直接对端 IP 写入新值，后续可信代理把各自的直接对端附加到列表右侧。控制面先验证连接对端，再从右往左跳过可信代理，以第一个非可信地址为来源。连接对端不可信时仅使用直接对端 IP。可信入口未提供可识别的来源时，接口返回未知。

将 [DB-IP IP to Country Lite](https://db-ip.com/db/download/ip-to-country-lite) 的 CSV 或 `.csv.gz` 文件以只读方式挂载给控制面，将 `data.repo.ip_country_file` 指向挂载后的路径。库包含 IPv4 和 IPv6 的国家或地区范围，按 IP 顺序存放。控制面在启动时加载到内存，请求期间只查询内存。路径为空、文件无法读取或解析时，接口继续返回来源 IP，归属地显示未知。DB-IP Lite 每月更新，数据以 [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/) 发布，控制台在卡片上保留归属链接。

每次更新从 DB-IP 下载新月份的完整 CSV 归档，解压后按下载页发布的校验值核对 CSV，保存为新版本文件。将挂载和配置切换到新文件，再重启控制面。用已登录租户会话核对接口及启动日志，必要时把配置切回已验证的文件并重启。

## Recorder 上报与租户用量

在 `server.local.yaml` 中填写 `data.repo.usage.database_dsn`，并按需调整 `biz.usage` 的上报限制。

用量归档和流量额度结算由 `MaintainUsage` 调度。 [Cron 配置与热更新](../../../../docs/standards/proxy-development-conventions.md#cron)

从仓库根目录执行 Goose 迁移。`00002_usage.sql` 依赖 `00001_proxy_auth.sql` 的网关基础表，按版本顺序迁移。用量归档按上海自然日维护分钟分区，运行账号需要相应的 DDL 权限以及计量表的读写权限。

```sh
./.tools/bin/goose -dir apps/proxy-control/migrations postgres "<PostgreSQL DSN>" up
```

生成模型时，数据库账号需能通过 PostgreSQL 的 `information_schema` 读取主键约束，例如表所有者通过 `default_transaction_read_only=on` 建立只读连接。 [数据库、迁移与 DAO](../../../../docs/standards/proxy-development-conventions.md#数据库迁移与-dao)

运营预置 `gateways` 与 `gateway_tokens`，令牌原文为至少 32 字节随机值的无填充 base64url 编码。同一网关同一范围最多保留两个有效令牌。数据库只写编码后令牌的 SHA-256 二进制摘要，采集器通过 HTTPS 将令牌放在 `Authorization: Bearer …` 中。合格记录按输入和输出增量总字节扣减，余额、子账号累计、用量汇总和连续水位一同提交。批次写入失败时，网关保留文件和未确认偏移供后续重放。

PostgreSQL 集成用例在独立临时 schema 中应用迁移，通过真实 HTTP 路由验证开户、上报、生命周期和回放。 [集成用例配置](../../../../docs/standards/proxy-development-conventions.md#pr-检查与业务回归)

网关生命周期使用同一 `report` 令牌调用 `POST /v1/reporting/lifecycle`。应用 `00003_gateway_lifecycle.sql` 后，可用 `gateway_boot_lifecycle` 核对心跳、失联和退出状态。


### 代理子账号

启用客户子账号时，在本地或受保护的配置源中设置 `biz.subaccounts.password_key`、`gateway_host` 和
`gateway_port`。 `explicit_sidtime` 默认关闭。完成验收后，先在网关开启同名字段，再在控制面开启以显示配置入口。密钥使用独立的 32 字节随机值，以无填充 base64url 编码。所有控制面实例使用同一密钥，
并持久备份，以便解密已有子账号密码。地址和端口填写客户实际使用的 SOCKS5 监听地址。

网关通过已分配的 `auth` 范围令牌加载凭据、刷新状态和协调普通粘性会话。显式 SID 需要悟空出口池模式。
会话确认由首次成功的上游 SOCKS5 CONNECT 触发，平台使用现有 `3600` 参数和一小时平台会话窗口。
供应商实际时长和到期行为继续由 #40 验证。

### 皇冠静态 IP

在受保护的本地配置源设置 `biz.ipcrown.base_url`（仅填服务地址，程序追加 `/open/api`）、`partner_id`、`api_secret` 和 `password_key`。正式环境使用供应商确认的安全通信路径。密码加密密钥使用独立的 32 字节随机值，以无填充 base64url 编码。所有控制面实例使用同一密钥并持久备份。暂停皇冠接入时，保留 `password_key`，让已交付的 IP 继续可查。

启用租户换号时，为 `biz.ipcrown.exchange_probe_address` 配置可从皇冠 SOCKS5 出口访问的受控目标 `host:port`，并保留配置模板中每 5 秒执行的 `ProbeStaticExchanges`。目标只需接受 TCP 连接。换号确认后使用新代理参数拨测，每次限时 3 秒，最多 4 次，相邻尝试至少间隔 15 秒。达到预算后在原换号记录显示 `support_required`。停用新换号提交可清空拨测地址，已提交记录仍可查询。保留此地址及调度，才能继续推进确认成功的拨测。

运营控制台「待核实静态 IP 订单」显示超过 24 小时仍未结束的订单。已有皇冠订单号时查询并同步新增交付，缺少订单号时由运营在皇冠侧核实后填写。核实前避免重发采购。超过 72 小时可关闭仍未确认的剩余数量并释放对应预扣，填写核实依据。后续补发交运营处理。 [皇冠契约验证](../../../../docs/features/proxy-first-release/issue-39-ipcrown-contract.md)
