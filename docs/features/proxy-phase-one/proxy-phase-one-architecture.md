# Proxy 用量采集、上报与认证技术方案

本文记录动态隧道技术基线。清理旧业务后，GOST、Recorder 文件采集、生命周期、文件水位和回放机制继续保留，旧凭据签发和计费入库路径已移除。首版产品行为和开售验收见[首版商业交付 Spec](../proxy-first-release/proxy-first-release-spec.md)。旧一期复跑步骤和结果见[历史验收索引](proxy-phase-one-development.md)。

## 部署与数据流

```mermaid
flowchart LR
    Client[企业 SOCKS5 客户端] --> GOST
    GOST --> Wukong[悟空隧道] --> Target[目标服务]
    subgraph Gateway[proxy-gateway 所在机器]
        GOST -->|认证插件| Relay[本机认证中继]
        GOST[原版 GOST v3.3.3] -->|interim / final| Files[(本机 Recorder 文件)]
        Files --> Collector[网关侧采集器]
    end
    Relay -->|加载和刷新认证状态| Control[proxy-control]
    Collector -->|每秒批量 HTTP| Control
    Control --> Store[(计费汇总、额度与水位)]
    Console[租户与运营控制台] -->|查询| Control
```

- `proxy-gateway` 使用 GOST 的 SOCKS5 TCP CONNECT 及悟空上游配置。平台认证插件校验凭据并返回稳定的 `clientID`，采集时以可信映射确定租户。文件 Recorder 提供计量数据，Observer 提供运行监控。 [悟空接入记录](../../integrations/wukong-socks5-tunnel.md)
- 采集器跟随网关部署，只读本机 Recorder 文件。`proxy-control` 可以部署在其他机器，通过受服务身份认证的 HTTP 接口接收批次。`proxy-control` 负责入库、用量汇总与权限校验。
- 首版客户入口提供动态 SOCKS5 隧道。旧 API 提取及用量查询 Mock 已从控制面、数据库和控制台移除。

使用 SOCKS5 用户名和密码认证时，握手会以明文携带凭据。目标站点的 HTTPS 只保护客户端与目标之间的应用流量。对外开放前，确认首批客户端与网关之间可用的传输保护方式。 [RFC 1929](https://www.rfc-editor.org/rfc/rfc1929.html#section-3)

## 计量、上报与回放

1. **写文件。** GOST 为活跃会话写 `interim`，会话正常结束时写 `final`。每条记录带累计 `inputBytes`、`outputBytes` 和相对同一会话上次 Recorder 写成功的增量。失败的 `interim` 增量会并入该会话下一次成功写入，部分写入可能留下残片并污染下一行，失败的 `final` 不重试。成功建立的会话可计入 SOCKS5 协议字节。采集规则按固定版本验证成功、空连接、上游拒绝、超时和网关策略拒绝，未成功建立的连接不计用量。
2. **读文件并组批。** 采集器每秒检查完整记录，有数据才发，限制批次条数和字节数。同一网关启动代次按文件及记录顺序发送，上一批确认后发送下一批。记录 ID 由网关实例、启动代次、稳定文件标识和记录起始偏移组成，文件轮转及重放时沿用原标识。
3. **提交后确认。** `proxy-control` 在整批事务中扣减额度、累加计费汇总并推进文件水位，事务提交后返回成功。偏移不连续或记录跨越已确认水位时整批拒绝。未知 clientID、无法解析的记录及增量大于累计值的记录随批隔离留证。采集器只推进连续已确认的持久化偏移。超时、停机、提交失败或确认丢失时保留源文件并退避重放。已关闭且所有记录均已确认的代次文件可按保留策略清理。
4. **去重与归属。** 控制面按文件水位去重，直接使用 Recorder 增量，不保存会话检查点。增量按本次记录时间归入 `Asia/Shanghai` 的分钟和自然日，跨午夜归到后一个采样所在日，回放沿用原采样时间。旧一期按历史倍率与流量授予规则计算计费字节，详见 [ADR 0004](../../adr/0004-billing-ingestion-and-audit.md)。首版改为 Recorder 总字节 1:1 扣减。

## 查询、故障与容量

- 旧一期租户可查看上海自然日用量。运营通过同一计量事实按租户、网关实例及时间段复核，会话和原始采样到网关文件查询。分钟汇总保留 60 天，之后归档为日汇总。Recorder 源文件须已关闭并全部确认，至少保留 14 天，并满足缺口核对证据要求后清理。禁止将凭据或业务报文写入用量库。
- 网关侧监控 Recorder 写入错误、未确认积压和磁盘水位。进程外生命周期信号用于发现异常退出。旧代次文件补报并确认完成后，采集器上报仍缺 `final` 的会话，控制面显示可能缺口，缺失字节数记为未知。已写入文件的数据须通过 GOST 进程重启回放验收，宿主机掉电及磁盘丢失另行验证。原版 HTTP Recorder 曾在 `final` 返回 503 后未见重发，生产回放以本机文件为来源。 [Recorder 实测](../../integrations/gost-file-recorder.md)
- 单实例连接上限取目标环境通过验收的最高已测负载。集中关闭期间需同时验证新连接成功率、成功连接端到端 P99 和 Recorder 记录完整性，并记录文件读齐耗时。具体窗口、发起速率、超时和阈值在压测前约定。 [验收与压测口径](issue-11-devbox-capacity.md#集中关闭压测口径与-windows-对照)
- 排查容量瓶颈时，按实例记录有效吞吐、建连速率、最忙 CPU 核、socket 内存和文件描述符余量。按实际源 IP 与上游 IP:port 组合核对出站临时端口，关注热点上游、共享 NAT 和 TIME_WAIT。 [Linux IP sysctl](https://docs.kernel.org/networking/ip-sysctl.html) · [Linux 网络栈扩展](https://docs.kernel.org/networking/scaling.html)

## 已采纳的设计

数据模型、身份机制和接口契约见 [ADR 0001 一期用量计费与凭据数据模型](../../adr/0001-usage-data-model.md)、[ADR 0002 网关服务身份、代理凭据与控制台会话](../../adr/0002-service-and-proxy-identity.md)、[ADR 0003 网关上报、认证准入、用量查询与计费管理接口](../../adr/0003-reporting-and-usage-contracts.md) 和 [ADR 0004 一期计费入库与运营追溯](../../adr/0004-billing-ingestion-and-audit.md)。

认证中继、离线准入与恢复追平见 [ADR 0005 一期代理认证可用性](../../adr/0005-proxy-auth-availability.md)。

控制台的框架、同源部署和 API 客户端见[开发规范的控制台技术栈](../../standards/proxy-development-conventions.md#控制台技术栈)。

## 已验证的实现约定

### 文件 Recorder

- **启动路径与稳定标识。** 由网关的进程外启动包装器每次生成新 `boot_id`，独占创建代次目录，验证路径可写后渲染绝对路径 `<spool>/<boot_id>/sessions.jsonl`，用 `gost -C` 启动。每代一个文件，`file_id` 固定为 `sessions`。采集器按 `(gateway_id, boot_id, file_id)` 和字节偏移恢复，文件打开告警纳入就绪检查。
- **文件切换与异常行。** 原版 GOST 的原生轮转可能把 JSON 和 LF 写到两个文件。一期关闭原生轮转，随进程代次换文件，运行期间保持活跃文件路径和内容前缀不变。采集器将以 LF 结束且编码有效的 UTF-8 行原样上报，由控制面在批次事务内隔离无法解析的行。非法 UTF-8 和旧代退出后的无 LF 残尾保留原始字节，采集器保留原偏移并告警，其他代次继续采集。文件清理遵循 [ADR 0004](../../adr/0004-billing-ingestion-and-audit.md#6-recorder-文件追查与保留)。以磁盘水位监控和计划排空、重启限制单文件增长。
- **网关时钟。** 宿主启用时间同步并监控偏移。`time` 按 RFC 3339 解析，接受最多 9 位小数与实际 UTC 偏移，分钟和自然日统一按 `Asia/Shanghai` 归属。 [Recorder 实测与配置](../../integrations/gost-file-recorder.md)

### 上报与恢复

- **生命周期。** 进程外包装器每 5 秒用 `report` 令牌调用 `POST /v1/reporting/lifecycle`。GOST 退出后先持久化退出时间，再上报。控制面按网关和代次幂等记录，15 秒未收到心跳视为失联。失联与已确认退出分开表示。
- **偏移。** 每个代次的 `confirmed.offset` 与 Recorder 同目录。整批确认后刷新临时文件、原子替换确认位置。确认丢失或落盘前退出时，从旧偏移重发。同盘故障可能同时丢失文件和偏移。
- **开发默认值。** 采样和扫描周期 1 秒，单批 1,000 条或 1 MiB，HTTP 总超时 3 秒，重试退避 1 至 30 秒。磁盘使用率 80% 告警、90% 升级。采集器保留未确认文件并报告积压。
## 首版待验证

- 按日查询和导出实际扣减的流量，支持按子账号筛选及查询已禁用账号的历史用量。

- [悟空接入](../../integrations/wukong-socks5-tunnel.md)验证子账号会话映射与 `sidtime` 语义。现有租户 SID 和入口池联调结果见[分机验收](issue-10-cross-host-acceptance.md)。
- 对外开放前，确认 SOCKS5 凭据的客户端到网关传输保护方式，并在目标环境验证容量、采样延迟与缓存更新边界。
