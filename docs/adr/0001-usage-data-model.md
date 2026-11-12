# 0001 一期用量计费与凭据数据模型

状态：已采纳 · 日期：2026-11-12 · 相关任务：[#3](https://github.com/DuanJinTaoToday/proxy/issues/3)、[#4](https://github.com/DuanJinTaoToday/proxy/issues/4)、[#7](https://github.com/DuanJinTaoToday/proxy/issues/7)、[#8](https://github.com/DuanJinTaoToday/proxy/issues/8)

> 部分条款由 [ADR 0004 一期计费入库与运营追溯](0004-billing-ingestion-and-audit.md#取代条款) 取代，其余决定继续适用。

## 背景

一期需要回答两个问题：租户在某个时间段、在哪个网关实例上用了多少流量，以及租户的流量额度是否已经用完。两者的时效要求不同。

- **计费汇总**用于扣减租户额度、拦截新连接，防止超卖，需要随每批上报实时更新。
- **会话明细**用于查看详细账单和排查问题，可以晚到。一期从网关本地的 Recorder 文件查询，下一阶段写入 ClickHouse 这类明细库。

GOST 只按会话写记录：会话存续期间每个采样周期写一条 `interim`，没有新流量的周期跳过，会话结束时写一条 `final`。`go-gost/x v0.28.1` 的每条会话记录除累计值外，还带 `inputBytesDelta` 和 `outputBytesDelta`。增量相对上一条成功写入的记录计算，写入失败时 GOST 不推进游标，未写入的部分并入下一条记录的增量。 [会话记录源码](https://github.com/go-gost/x/blob/v0.28.1/recorder/session_record.go)

代理连接开始时无法预估会传多少字节，计量也要等采样上报后才到达控制面，因此不能像按请求计费那样在连接开始前预扣额度。本方案在新连接准入时比较可用额度与门槛，在每批入库时按记录结算，并允许有界欠费。计费汇总是扣减依据，必须与扣减在同一事务提交。

## 决定

### 计费口径

- 每条合格记录的计费字节为 `ceil((上行增量 × 上行倍率 + 下行增量 × 下行倍率) / 1000)`。倍率以千分数整数保存，`1200` 表示 1.2 倍，向上取整避免少计。上行对应 Recorder 的 `inputBytes`，下行对应 `outputBytes`。
- 倍率、准入门槛和提醒阈值由运营配置，按租户、租户分组、全局的顺序取第一个已配置的值。默认倍率上下行均为 `1000`，默认准入门槛 10 MiB，即 10,485,760 计费字节。默认提醒阈值 1000 MiB，即 1,048,576,000 计费字节。生效的提醒阈值必须高于生效的准入门槛。
- 倍率按生效时间分版本，只追加新版本。每条记录使用其 `recorded_at` 时生效的倍率，回放和晚到记录的计费结果不变。
- 租户的流量额度来自流量授予，例如充值 100 MB 即增加一笔 104,857,600 计费字节的授予。授予可以设生效时间和到期时间，不设到期时间表示长期有效。
- 每条记录按 `recorded_at` 分配到当时有效的授予，先到期的先用，到期时间相同的按创建顺序。没有可用授予时，记为欠费。
- **允许有界欠费。** 已建立的连接继续计量，额度用尽后的计费字节计入欠费，不中断连接。新充值的授予先抵扣欠费。
- **准入拦截。** 租户可用额度为有效授予的剩余计费字节之和减去欠费。可用额度低于准入门槛时，认证插件拒绝新连接。
- 超卖上限约为租户吞吐乘以“GOST 采样周期加上报、入库和缓存刷新延迟”，再加上额度耗尽前已建立的连接在关闭前继续产生的流量。采样周期越短，窗口越小。

### 约定

- 时间列使用 `timestamptz`，按 UTC 写入。分钟和日期按 `Asia/Shanghai` 由记录的 `recorded_at` 计算，跨分钟、跨午夜的增量归入后一个采样所在的分钟和日期，补报沿用原时间。
- 字节和计费字节使用 `bigint`。授予、欠费、流水和汇总中的计费字节使用同一单位。控制台和运营配置中的 MB 均按 1,048,576 字节换算。
- 租户 ID 沿用配置中的字符串 ID，暂不建租户表和外键。成员关系改存数据库时再补外键。
- 配置作用域用 `(scope_type, scope_id)` 表示，`scope_type` 取 `global`、`group` 或 `tenant`，`global` 的 `scope_id` 为空字符串。
- 去重靠文件水位，不保存每条记录。增量直接取 GOST 记录的增量字段，不保存会话检查点。

### 表

**采集控制**

| 表 | 主键 | 用途与关键列 |
| --- | --- | --- |
| `gateways` | `id` | 网关实例。`name`、`status`（`active`/`disabled`）、`created_at`、`disabled_at`。由运营预置。 |
| `gateway_tokens` | `id` | 网关服务令牌。`gateway_id`、`scope`（`report`/`auth`）、`token_hash`（唯一）、`created_at`、`revoked_at`。见 [ADR 0002](0002-service-and-proxy-identity.md)。 |
| `gateway_boots` | `(gateway_id, boot_id)` | 启动代次。`first_seen_at`、`last_record_at`、`replay_completed_at`。退出检测列随生命周期设计补充。 |
| `ingest_cursors` | `(gateway_id, boot_id, file_id)` | 文件水位。`confirmed_offset` 为已确认的最后一条记录的结束偏移，`updated_at`。起始偏移小于水位的记录视为重复并跳过。 |
| `proxy_credentials` | `client_id` | 代理凭据，`client_id` 即返回给 GOST 的稳定标识。`tenant_id`、`username`（唯一）、`secret_hash`、`status`（`active`/`disabled`）、`expires_at`（可空）、`allowed_source_cidrs`（`cidr[]`，空表示不限制）、`created_by`、`created_at`、`disabled_at`。`tenant_id` 写入后不可修改，重新发放创建新行。 |

**计费配置**

| 表 | 主键 | 用途与关键列 |
| --- | --- | --- |
| `tenant_groups` | `id` | 租户分组。`name`、`created_at`。 |
| `tenant_group_members` | `tenant_id` | 租户所属分组，每个租户最多一个分组。`group_id`、`updated_by`、`updated_at`。 |
| `billing_settings` | `(scope_type, scope_id)` | 准入门槛与提醒阈值。`admission_threshold_bytes`、`warning_threshold_bytes`（均可空，空表示沿用上一级）、`updated_by`、`updated_at`。迁移时写入全局准入门槛 10,485,760 和提醒阈值 1,048,576,000。写入时校验受影响租户的生效提醒阈值高于生效准入门槛。 |
| `billing_rates` | `id` | 倍率版本。`scope_type`、`scope_id`、`upload_ratio_milli`、`download_ratio_milli`、`effective_from`、`created_by`、`created_at`。唯一约束 `(scope_type, scope_id, effective_from)`。迁移时写入全局 `1000/1000`。已生效的版本不修改。 |

**额度**

| 表 | 主键 | 用途与关键列 |
| --- | --- | --- |
| `traffic_grants` | `id` | 流量授予。`tenant_id`、`granted_bytes`、`used_bytes`、`overdraft_offset_bytes`、`starts_at`、`expires_at`（可空）、`status`（`active`/`revoked`）、`source`（`recharge`/`adjustment`）、`created_by`、`created_at`。约束 `used_bytes <= granted_bytes`。索引 `(tenant_id, expires_at)`。 |
| `tenant_balances` | `tenant_id` | 租户额度状态。`overdraft_bytes`、`warning_since`（可空）、`last_received_at`、`updated_at`。充值、调整和入库时在锁内更新。 |
| `balance_ledger` | `id` | 额度流水，只追加。`tenant_id`、`kind`、`grant_id`（可空）、`delta_bytes`（带符号）、`available_before`、`available_after`、`external_ref`、`reason`、`operator`、`created_at`。唯一约束 `(tenant_id, external_ref)`。日常消耗不写流水，由 `usage_minutely` 承担。 |

`balance_ledger.kind` 取以下值。

| 值 | 产生方式 |
| --- | --- |
| `recharge` | 充值创建授予。同一 `external_ref` 重复提交时返回已有结果，不重复入账。 |
| `adjust_add` | 运营补增额度，创建 `source=adjustment` 的授予。 |
| `adjust_subtract` | 运营扣减额度，减少指定授予的 `granted_bytes`，最多扣到 `used_bytes`。不足部分计入欠费。 |
| `overdraft_offset` | 新授予抵扣欠费，与充值或补增在同一事务内写入。 |
| `grant_expire` | 授予到期时剩余额度失效。由定时任务补记，准入和结算按时间直接排除过期授予，不依赖这条流水。 |

**用量汇总**

| 表 | 主键 | 用途与关键列 |
| --- | --- | --- |
| `usage_minutely` | `(tenant_id, gateway_id, minute_start, grant_id, rate_id)` | 分钟级计费汇总。`input_bytes`、`output_bytes`、`billable_bytes`、`record_count`、`last_received_at`。`grant_id` 为 `0` 表示欠费部分，`rate_id` 为所用倍率版本。按天分区，保留 60 天。 |
| `usage_daily` | `(tenant_id, gateway_id, usage_date, grant_id, rate_id)` | 日级计费汇总，列与分钟级相同，永久保留。超过 60 天的分钟级分区先在一个事务内汇总写入日表，再删除分区。归档幂等，同一日期重复归档不重复累加。 |

**异常与缺口**

| 表 | 主键 | 用途与关键列 |
| --- | --- | --- |
| `record_anomalies` | `id` | 异常留证。`gateway_id`、`boot_id`、`file_id`、`file_offset`、`kind`、`detail`（JSONB，仅含计量和追溯字段）、`detected_at`、`resolved_at`。 |
| `usage_gaps` | `id` | 可能缺口。`gateway_id`、`boot_id`、`session_id`（代次级缺口为空）、`client_id`、`tenant_id`、`from_at`、`detected_at`、`resolved_at`。 |

### 入库事务

每批在一个 PostgreSQL 事务内完成，提交后才返回成功确认。

1. 锁定本批涉及文件的 `ingest_cursors` 行，跳过起始偏移小于水位的记录。
2. 解析每条记录并判定处置：
   - **计量**：满足合格会话规则，`client_id` 能解析到租户。
   - **不计量**：不满足合格会话规则，例如 `network` 不是 `tcp`、缺少 `clientID`/`src`/`dst` 或 `err` 非空。不写汇总，只计入采集监控指标。
   - **隔离**：能识别来源但无法归集，例如未知 `client_id`、无法解析的内容、增量大于同一条记录的累计值。写入 `record_anomalies`，不计量。
3. 按租户 ID 升序锁定涉及租户的 `tenant_balances` 和有效 `traffic_grants`，避免多网关并发入库时死锁。
4. 按批内顺序，为每条计量记录取倍率、计算计费字节，并分配到授予或欠费。
5. 在内存中按 `usage_minutely` 主键合并，再用 `INSERT ... ON CONFLICT DO UPDATE` 累加。
6. 更新授予的 `used_bytes`、租户的 `overdraft_bytes`、`warning_since` 和 `last_received_at`、各文件水位及 `gateway_boots.last_record_at`。
7. 提交后刷新涉及租户的可用额度缓存。

每批的写入行数取决于涉及的文件、租户、分钟和倍率版本数，与连接数无关。

### 充值、调整与准入

- 充值和调整在一个事务内锁定 `tenant_balances`，变更授予，写入流水。新授予先抵扣欠费：抵扣量写入 `overdraft_offset_bytes` 并计入 `used_bytes`，同时减少 `overdraft_bytes`，另写一条 `overdraft_offset` 流水。提交后刷新可用额度缓存。
- `可用额度 = Σ(granted_bytes - used_bytes)（当前有效的授予） - overdraft_bytes`。到期和吊销的授予在计算时排除。
- 控制面在进程内缓存每个租户的可用额度，入库、充值和调整提交后刷新，并设短期过期作为兜底。认证插件的准入判断读取缓存。一期控制面单实例部署，多实例部署时改用共享缓存。
- 认证插件依次检查凭据状态、到期时间、来源地址白名单和租户可用额度，任一不满足即拒绝。
- 可用额度低于提醒阈值时设置 `warning_since`，回到阈值以上时清空。一期在控制台展示，通知渠道后续加入。

### 缺口识别

控制面不保存会话状态，缺口由采集器识别。旧启动代次的已写记录全部获得确认后，采集器从本地文件找出有 `interim` 而没有 `final` 的会话，上报会话 ID、`client_id` 和最后采样时间，再发送回放完成通知。控制面据此写入 `usage_gaps`。旧代次没有任何会话记录时，写入代次级缺口。缺失字节量未知。

### 查询与对账

- 计费汇总承担用量查询和对账，也是后续出账的依据。实时扣减和准入只依赖授予与余额表，不读取计费汇总。
- 租户和运营的用量查询对计费汇总求和，可按网关实例、时间段和日期汇总。近 60 天可按分钟或小时查询，更早的数据按日查询。上下行为实际字节，计费字节单列。
- 最近入库时间取 `tenant_balances.last_received_at`。
- 授予已用量满足 `used_bytes = Σ billable_bytes（该授予） + overdraft_offset_bytes`。欠费满足 `overdraft_bytes = Σ billable_bytes（grant_id = 0） - Σ overdraft_offset_bytes`。求和范围为日级汇总加上尚未归档的分钟级汇总。授予总量的变化可由 `balance_ledger` 逐笔复核。
- 单条会话和原始采样的明细由运营从网关 Recorder 文件按保留期查询。一期不向租户提供会话明细。

## 备选方案

- **每条记录入库，按会话检查点计算增量。** 可在数据库内追溯到每个采样，但短连接多时每天数千万行，且不服务于扣减。明细改由网关文件承担，下一阶段写入 ClickHouse。
- **按小时汇总后扣减。** 无法区分流量包到期或额度耗尽前后的用量，扣减延迟也会放大超卖。本方案按记录时间逐条分配，汇总粒度只影响查询。
- **计费汇总在内存中累加后定时落库。** new-api 的统计数据这样做，但进程崩溃会丢失已确认的计量，也无法保证重放只计一次。
- **逐连接预扣额度。** 连接开始时无法预估字节数，原版 GOST 也不能在网关本地按额度断开连接。用准入门槛和有界欠费代替。
- **只按租户覆盖倍率。** 租户多时运营需要逐个配置。增加租户分组一级，与 new-api 的分组倍率一致。
- **控制面保存未结束会话以识别缺口。** 长连接的每次采样都要读写会话行。采集器本身能读文件，由它在回放完成时识别更简单。

## 影响

- 03 建立采集控制、计费配置、额度、流水和汇总表，实现入库事务、缓存刷新和分钟级汇总的归档任务。02 建立 `proxy_credentials`，认证中加入凭据限制和准入判断。07 实现未结束会话上报、`usage_gaps` 和代次生命周期。
- 访问控制新增运营的计费管理权限，用于充值、调整和修改计费配置，读取运营数据的权限不包含它。
- 验收：在真实 Recorder 文件上确认增量字段的语义，包括写入失败后的并入行为；用 `130/214`、`180/294` 样例验证次日增加 `50/80`；用 1.2 倍上行倍率、100 MB 授予和 10 MiB 门槛验证欠费、抵扣和拦截；重复提交同一充值单号只入账一次。
- 迁移按[开发规范](../standards/proxy-development-conventions.md#数据库迁移与-dao)使用 Goose，模型由 GORM Gen 生成。
