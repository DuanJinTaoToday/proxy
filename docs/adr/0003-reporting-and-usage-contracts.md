# 0003 网关上报、认证准入、用量查询与计费管理接口

状态：已采纳 · 日期：2026-11-12 · 相关任务：[#3](https://github.com/DuanJinTaoToday/proxy/issues/3)、[#4](https://github.com/DuanJinTaoToday/proxy/issues/4)、[#5](https://github.com/DuanJinTaoToday/proxy/issues/5)、[#7](https://github.com/DuanJinTaoToday/proxy/issues/7)、[#8](https://github.com/DuanJinTaoToday/proxy/issues/8)

> 部分条款由 [ADR 0004 一期计费入库与运营追溯](0004-billing-ingestion-and-audit.md#取代条款) 取代，其余决定继续适用。

> 认证部署与准入时效的现行约定见 [ADR 0005 一期代理认证可用性](0005-proxy-auth-availability.md#取代条款)。

## 背景

[开发规范](../standards/proxy-development-conventions.md#接口与依赖)要求平台自有接口以 Protobuf 定义，归 `api/proxycontrol/v1/`，用 `google.api.http` 声明 HTTP 路由。GOST 认证插件遵循 GOST 自有协议。表结构与计费口径见 [ADR 0001](0001-usage-data-model.md)，调用方身份见 [ADR 0002](0002-service-and-proxy-identity.md)。

## 决定

### 通用约定

- JSON 编解码沿用 Kratos 的 protojson 设置，与已发布的 `access.proto` 保持一致。`uint64` 和 `int64` 字节数在 JSON 中按 protojson 规则编码为字符串。
- 日期使用 `YYYY-MM-DD` 字符串，按 `Asia/Shanghai` 解释，区间两端都包含。时间使用 RFC 3339 时间戳，时间区间为左闭右开。
- 错误使用 Kratos 错误，`reason` 为大写下划线常量，调用方按 HTTP 状态和 `reason` 判断，不解析 `message`。
- 列表接口使用 `page_size`（默认 100，最大 500）和不透明的 `page_token`，内部按键集分页。

| 文件 | 服务 | 调用方 |
| --- | --- | --- |
| `reporting.proto` | `ReportingService` | 采集器，`report` 范围的网关令牌 |
| `gateway_auth.proto` | `GatewayAuthService` | GOST 认证插件，`auth` 范围的网关令牌 |
| `usage.proto` | `UsageService` | 控制台，租户成员和平台运营 |
| `billing.proto` | `BillingService` | 控制台，平台运营 |

### `reporting.proto`：网关上报

| RPC | 路由 | 说明 |
| --- | --- | --- |
| `SubmitBatch` | `POST /v1/reporting/batches` | 提交同一启动代次的一批记录，整批事务提交后返回成功。 |
| `ReportUnfinishedSessions` | `POST /v1/reporting/boots/{boot_id}/unfinished-sessions` | 旧代次回放确认完成后，分批上报缺少 `final` 的会话。按会话 ID 幂等。 |
| `CompleteReplay` | `POST /v1/reporting/boots/{boot_id}/replay-complete` | 未结束会话全部上报后，通知控制面结束该代次的缺口核对。幂等。 |

```proto
message SubmitBatchRequest {
  string gateway_id = 1;
  string boot_id = 2;
  repeated RecorderLine records = 3; // 按源文件顺序排列
}

message RecorderLine {
  string file_id = 1;
  uint64 file_offset = 2; // 记录在文件中的起始字节偏移
  string line = 3;        // Recorder 写入的一整行 JSON，不含分隔符
}

message SubmitBatchResponse {
  uint32 billable = 1;     // 计量
  uint32 not_billable = 2; // 不满足合格会话规则
  uint32 quarantined = 3;  // 隔离
  uint32 duplicates = 4;   // 低于文件水位，已跳过
}

message ReportUnfinishedSessionsRequest {
  string gateway_id = 1;
  string boot_id = 2;
  repeated UnfinishedSession sessions = 3;
}

message UnfinishedSession {
  string session_id = 1;
  string client_id = 2;
  google.protobuf.Timestamp last_recorded_at = 3;
}

message ReportUnfinishedSessionsResponse {}

message CompleteReplayRequest {
  string gateway_id = 1;
  string boot_id = 2;
  uint32 unfinished_session_count = 3; // 控制面核对已收到的数量
}

message CompleteReplayResponse {}
```

- 采集器原样上报 Recorder 行，控制面负责解析、合格会话判定和计费，规则升级不需要同步升级采集器。
- 单批记录数和请求体字节数的上限由控制面配置，开发默认 1,000 条和 1 MiB。未结束会话每次最多 1,000 个。
- 单条记录无法解析或无法归集时随批隔离，不拒绝整批。
- `CompleteReplay` 的数量与已收到的未结束会话不一致时返回 `409 UNFINISHED_SESSIONS_MISMATCH`，采集器重新上报后再通知。

| 响应 | `reason` | 采集器处理 |
| --- | --- | --- |
| `200` | — | 已确认，推进连续偏移或进入下一步。 |
| `400` | `INVALID_BATCH` | 请求结构不合法，例如缺少 `boot_id`、空批或偏移乱序。永久拒绝，保留文件并告警。 |
| `401` | `GATEWAY_UNAUTHENTICATED` | 令牌缺失、无效或已吊销。保留文件，退避重试并告警，令牌更新后恢复。 |
| `403` | `GATEWAY_FORBIDDEN` | 网关停用，或 `gateway_id` 与令牌不一致。永久拒绝，保留文件并告警。 |
| `409` | `UNFINISHED_SESSIONS_MISMATCH` | 重新上报未结束会话。 |
| `413` | `BATCH_TOO_LARGE` | 缩小批次后重发同一批记录。 |
| `429`、`5xx`、超时、连接失败 | — | 未确认。保留文件，从未确认位置退避重发。 |

### `gateway_auth.proto`：认证与准入

现行认证可用性约定见 [ADR 0005 一期代理认证可用性](0005-proxy-auth-availability.md#取代条款)。

| RPC | 路由 | 说明 |
| --- | --- | --- |
| `Authenticate` | `POST /v1/gateway/auth` | 请求 `{service, username, password, client}`，通过时返回 `200` 和 `{ok: true, id: <client_id>}`，拒绝时返回 `200` 和 `{ok: false}`。令牌无效返回 `401`，GOST 按认证失败处理。 |

- 依次检查凭据存在且启用、密码正确、未过期、`client` 的来源地址在白名单内、租户可用额度不低于准入门槛。任一不满足即拒绝。
- GOST 协议只返回 `ok` 和 `id`，拒绝原因写入控制面认证日志，取值为 `INVALID_CREDENTIAL`、`CREDENTIAL_DISABLED`、`CREDENTIAL_EXPIRED`、`SOURCE_NOT_ALLOWED`、`QUOTA_INSUFFICIENT`。日志不记录密码。
- 准入判断读取控制面的可用额度缓存，见 ADR 0001。

### `usage.proto`：用量与额度查询

租户接口按路径中的 `tenant_id` 检查 `PERMISSION_READ_TENANT_USAGE`，运营接口检查 `PERMISSION_READ_OPERATIONS`。两类接口使用不同路由，调用同一组查询函数。

| RPC | 路由 | 权限 |
| --- | --- | --- |
| `GetTenantDailyUsage` | `GET /v1/tenants/{tenant_id}/usage/daily` | 租户成员 |
| `GetTenantBalance` | `GET /v1/tenants/{tenant_id}/balance` | 租户成员 |
| `QueryUsage` | `GET /v1/operations/usage` | 平台运营 |
| `ListRecordAnomalies` | `GET /v1/operations/anomalies` | 平台运营 |
| `ListUsageGaps` | `GET /v1/operations/gaps` | 平台运营 |

```proto
message DailyUsageRequest {
  string tenant_id = 1;
  string start_date = 2;
  string end_date = 3;
}

message UsageAmount {
  uint64 input_bytes = 1;    // 上行实际字节
  uint64 output_bytes = 2;   // 下行实际字节
  uint64 total_bytes = 3;
  uint64 billable_bytes = 4; // 计费字节
}

message DailyUsage {
  string date = 1;
  UsageAmount amount = 2;
}

message DailyUsageResponse {
  repeated DailyUsage days = 1;                   // 只含有用量的日期，按日期升序
  google.protobuf.Timestamp last_received_at = 2; // 租户最近入库时间，无数据时为空
}

message TenantBalance {
  string tenant_id = 1;
  int64 available_bytes = 2;        // 可为负
  uint64 overdraft_bytes = 3;
  uint64 admission_threshold_bytes = 4;
  uint64 warning_threshold_bytes = 5;
  google.protobuf.Timestamp warning_since = 6;
  repeated TrafficGrant grants = 7; // 当前有效的授予
}

message TrafficGrant {
  string grant_id = 1;
  uint64 granted_bytes = 2;
  uint64 used_bytes = 3;
  google.protobuf.Timestamp starts_at = 4;
  google.protobuf.Timestamp expires_at = 5;
  string source = 6;
}

enum UsageGranularity {
  USAGE_GRANULARITY_UNSPECIFIED = 0;
  USAGE_GRANULARITY_MINUTE = 1;
  USAGE_GRANULARITY_HOUR = 2;
  USAGE_GRANULARITY_DAY = 3;
}

message QueryUsageRequest {
  string tenant_id = 1;                    // 必填
  string gateway_id = 2;                   // 可选，空表示所有网关
  google.protobuf.Timestamp start_time = 3;
  google.protobuf.Timestamp end_time = 4;
  UsageGranularity granularity = 5;
  bool split_by_gateway = 6;
  bool split_by_grant = 7;                 // 按授予拆分，欠费部分的 grant_id 为空
  int32 page_size = 8;
  string page_token = 9;
}

message UsageBucket {
  google.protobuf.Timestamp start_time = 1;
  string gateway_id = 2;
  string grant_id = 3;
  UsageAmount amount = 4;
}

message QueryUsageResponse {
  repeated UsageBucket buckets = 1;
  string next_page_token = 2;
}
```

- 日用量的日期区间最长 93 天。`QueryUsage` 的时间区间按粒度限制：分钟不超过 24 小时，小时不超过 31 天，天不超过 366 天。分钟和小时粒度只能查询近 60 天，更早的数据只按天提供。超出返回 `400 INVALID_TIME_RANGE`。
- `ListRecordAnomalies` 和 `ListUsageGaps` 按发现时间倒序分页，可按网关、代次、租户和类型过滤。
- 一期用量只包含 `tunnel` 来源。API 提取的 Mock 用量随 08 以独立来源加入，默认查询不混入。

### `billing.proto`：计费管理

所有接口检查新增的 `PERMISSION_MANAGE_BILLING`，只授予平台运营。读取接口同时接受 `PERMISSION_READ_OPERATIONS`。

| RPC | 路由 | 说明 |
| --- | --- | --- |
| `CreateRecharge` | `POST /v1/operations/tenants/{tenant_id}/recharges` | 充值并创建授予。请求含 `external_ref`、`granted_bytes`、`starts_at`、`expires_at`、`note`。同一 `external_ref` 重复提交返回原结果。 |
| `AdjustBalance` | `POST /v1/operations/tenants/{tenant_id}/adjustments` | 补增或扣减额度。请求含 `external_ref`、`mode`（`ADD`/`SUBTRACT`）、`bytes`、`grant_id`（扣减时必填）、`reason`。 |
| `ListBalanceLedger` | `GET /v1/operations/tenants/{tenant_id}/ledger` | 分页查询额度流水。 |
| `ListTrafficGrants` | `GET /v1/operations/tenants/{tenant_id}/grants` | 分页查询授予，含已到期和已吊销。 |
| `GetBillingSettings` | `GET /v1/operations/billing/settings/{scope_type}/{scope_id}` | 读取某一作用域的门槛与阈值，并返回对指定租户生效的值。 |
| `UpdateBillingSettings` | `PUT /v1/operations/billing/settings/{scope_type}/{scope_id}` | 设置或清除准入门槛和提醒阈值。生效的提醒阈值不高于准入门槛时返回 `400 INVALID_THRESHOLDS`。 |
| `CreateBillingRate` | `POST /v1/operations/billing/rates` | 追加倍率版本。`effective_from` 不得早于当前时间，避免改变已入库记录的计费。 |
| `ListBillingRates` | `GET /v1/operations/billing/rates` | 按作用域查询倍率版本。 |
| `CreateTenantGroup` | `POST /v1/operations/billing/groups` | 创建租户分组。 |
| `SetTenantGroup` | `PUT /v1/operations/tenants/{tenant_id}/group` | 设置或清除租户所属分组。 |

- `scope_type` 取 `global`、`group`、`tenant`，`global` 的路由使用 `_` 作为 `scope_id`。
- 修改分组只影响之后入库的记录所用倍率，已入库的汇总不重算。
- 扣减量超过授予剩余时，超出部分计入欠费，并在响应和流水中返回实际结果。

## 备选方案

- **采集器解析 Recorder 行后按字段上报。** 请求更紧凑，但合格会话规则和计费口径会分散到网关和控制面两处，升级时要同步发布。
- **单条记录不合法时整批拒绝。** 一条坏行会阻塞整个代次的后续上报。
- **回放完成通知直接携带全部未结束会话。** 异常退出时可能有大量长连接，单个请求过大。分批上报后用数量核对完成。
- **认证插件只校验凭据，额度拦截另走网关限流。** 原版 GOST 无法按租户额度在本地限流。准入判断放在认证中实现成本最低。
- **租户与运营共用一个用量路由，靠令牌区分。** 权限绑定依赖身份分支，容易越权。

## 影响

- 03 新增 `reporting.proto` 的 `SubmitBatch`、`usage.proto` 和 `billing.proto`。02 新增 `gateway_auth.proto`。06 实现运营查询、异常和缺口列表。07 实现 `ReportUnfinishedSessions` 和 `CompleteReplay`。
- `access.proto` 的 `Permission` 新增 `PERMISSION_MANAGE_BILLING = 4`。
- 这些接口发布后按开发规范的兼容规则演进，已发布字段编号不复用。
