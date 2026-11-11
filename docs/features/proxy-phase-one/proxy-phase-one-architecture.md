# Proxy 一期技术方案

用量链路由原版 GOST 文件 Recorder、本机采集器和 `proxy-control` 组成。控制台通过 `proxy-control` 查询租户用量和运营明细。开发任务、交付边界和验收口径以一期 Spec 为准。 [一期 Spec](proxy-phase-one-spec.md) · [开发任务](proxy-phase-one-development.md)

## 部署与数据流

```mermaid
flowchart LR
    Client[企业 SOCKS5 客户端] --> GOST
    GOST --> Wukong[悟空隧道] --> Target[目标服务]
    GOST -->|认证插件| Control[proxy-control]
    subgraph Gateway[proxy-gateway 所在机器]
        GOST[原版 GOST v3.3.3] -->|interim / final| Files[(本机 Recorder 文件)]
        Files --> Collector[网关侧采集器]
    end
    Collector -->|每秒批量 HTTP| Control
    Control --> Store[(用量与检查点)]
    Console[租户与运营控制台] -->|查询| Control
```

- `proxy-gateway` 使用 GOST 的 SOCKS5 TCP CONNECT 及悟空上游配置。平台认证插件校验凭据并返回稳定的 `clientID`，采集时以可信映射确定租户。文件 Recorder 提供计量数据，Observer 提供运行监控。 [悟空接入记录](../../integrations/wukong-socks5-tunnel.md)
- 采集器跟随网关部署，只读本机 Recorder 文件。`proxy-control` 可以部署在其他机器，通过受服务身份认证的 HTTP 接口接收批次。`proxy-control` 负责入库、用量汇总与权限校验。
- API 提取和上游流量查询使用独立的 Mock 适配器。悟空提取服务部署后，客户直连 `IP_List` 中的 SOCKS5 入口，`proxy-control` 再按租户从悟空拉取用量。静态 IP 和计费规则先做领域建模。 [产品对齐](../../product/proxy-product-alignment.md)

使用 SOCKS5 用户名和密码认证时，握手会以明文携带凭据。目标站点的 HTTPS 只保护客户端与目标之间的应用流量。对外开放前，确认首批客户端与网关之间可用的传输保护方式。 [RFC 1929](https://www.rfc-editor.org/rfc/rfc1929.html#section-3)

## 计量、上报与回放

1. **写文件。** GOST 为活跃会话写 `interim`，会话正常结束时写 `final`。原始快照保留累计 `inputBytes`、`outputBytes`。成功建立的会话可计入 SOCKS5 协议字节。采集规则按固定版本验证成功、空连接、上游拒绝、超时和网关策略拒绝，未成功建立的连接不计用量。
2. **读文件并组批。** 采集器每秒检查完整记录，有数据才发，限制批次条数和字节数。同一网关启动代次按文件及记录顺序发送，上一批确认后发送下一批。记录 ID 由网关实例、启动代次、稳定文件标识和记录起始偏移组成，文件轮转及重放时沿用原标识。
3. **提交后确认。** `proxy-control` 在整批事务中保存原始记录、检查点与认可增量，事务提交后返回成功。采集器只推进连续已确认的持久化偏移。超时、停机、提交失败或确认丢失时保留源文件并退避重放。已关闭且所有记录均已确认的轮转文件可按保留策略清理。
4. **去重与归日。** 控制面按记录 ID 去重，按 `(gateway_id, boot_id, sessionID)` 保存会话累计检查点。相同累计的增量为零，累计增长取差值，`recordIndex` 用于追溯。零增量 `final` 仍可关闭会话。增量按本次 Recorder 记录时间归入 `Asia/Shanghai` 自然日，跨午夜归到后一个采样所在日，回放沿用原采样时间。

## 查询、故障与容量

- 租户侧仅可查本租户的上行、下行、合计和最近入库时间。运营通过同一聚合查询，能追到原始采样、网关实例、启动代次、会话和认可增量。原始明细仅保留计量和追溯所需字段。
- 网关侧监控 Recorder 写入错误、未确认积压和磁盘水位。进程外生命周期信号用于发现异常退出。旧代次文件补报并确认完成后，仍缺 `final` 的会话显示可能缺口，缺失字节数记为未知。已写入文件的数据须通过 GOST 进程重启回放验收，宿主机掉电及磁盘丢失另行验证。
- 单实例连接上限取目标环境通过验收的最高已测负载。集中关闭期间需同时验证新连接成功率、成功连接端到端 P99 和 Recorder 记录完整性，并记录文件读齐耗时。具体窗口、发起速率、超时和阈值在压测前约定。 [验收与压测口径](proxy-phase-one-spec.md#验收与测试入口)
- 排查容量瓶颈时，按实例记录有效吞吐、建连速率、最忙 CPU 核、socket 内存和文件描述符余量。按实际源 IP 与上游 IP:port 组合核对出站临时端口，关注热点上游、共享 NAT 和 TIME_WAIT。 [Linux IP sysctl](https://docs.kernel.org/networking/ip-sysctl.html) · [Linux 网络栈扩展](https://docs.kernel.org/networking/scaling.html)
