# Proxy 一期开发任务与验收顺序

一期 12 项任务、进度和阻塞关系以 [GitHub Issue #1 的子任务](https://github.com/DuanJinTaoToday/proxy/issues/1)为准。

先打通一个测试租户的真实隧道用量，再补足跨机器故障恢复、租户自查和运营追溯。API 提取和流量查询的 Mock 可以并行准备。 [一期 Spec](proxy-phase-one-spec.md) · [一期技术方案](proxy-phase-one-architecture.md)

联调从一个网关、一个本地采集器和一个批量 HTTP 接口开始。Mock 适配器和样本可并行准备，真实 API 联调按[悟空接口与数据清单](../../product/proxy-product-alignment.md#需要老板推动悟空提供的接口与数据)推进。

验收证据分两类。受控 SOCKS5 上游用于核对字段、Recorder 增量字段、故障恢复和跨日。真实悟空入口用于验证部署后从客户端经网关到用量页面的全链路。直接连接悟空的实测可作为联调起点。[悟空隧道接入记录](../../integrations/wukong-socks5-tunnel.md)

## Issue #5 验收记录

### 本地受控验证

2026-11-12 在 Windows amd64 上，基于结构调整后的 `main`（`6dd61f4`）验证。

| 验收项 | 结果 |
| --- | --- |
| 连接结果与计量 | `TestGOSTWritesAndCollectorReportsRealRecorderLines` 使用原版 GOST v3.3.3，确认带载荷和空连接各产生一条成功记录，拒绝、超时和策略拒绝各产生一条失败记录。`TestRecorderOnlyMetersEstablishedSOCKSConnections` 向使用 PostgreSQL 17 的控制面提交同类测试样本，确认只汇总两条成功记录的增量，输入 14 字节、输出 16 字节。 |
| Recorder 增量与写入失败 | 重跑 [Recorder 原型](../../integrations/gost-file-recorder.md#复现)。官方二进制的两次中间采样和一次结束采样增量为 `45/44 → 64/64 → 0/0`，累计为 `109/108`。同版本 Recorder 在写失败且未留下字节后，下一条增量为 `80/94`，等于两次成功记录的累计差。部分写入导致粘行、`final` 写失败后无法补写也已复现。 |
| 分批、确认和记录标识 | `TestCollectorHonorsBatchLimits` 分别验证条数和编码字节上限、按起始偏移顺序提交及无数据时不发批次。`TestCollectorResumesFromDurableAcknowledgement` 验证确认丢失后重放同一前缀、整批确认后按持久化偏移继续、残行补齐 LF 后才上报。默认每秒扫描，每次扫描每代最多确认一批，记录由 `gateway_id`、`boot_id`、`file_id` 和起始偏移定位。 |

心跳、退出事件、确认偏移及扫描参数见[网关采集说明](../../../apps/proxy-gateway/gost/README.md)。确认偏移写入同代次目录的临时文件，经 `Sync` 后替换 `confirmed.offset`。Unix 随后同步目录，Windows 使用 `MoveFileEx` 的 write-through 标志。Recorder 与偏移同盘，磁盘丢失会同时影响源文件和本地确认位置。生产多租户 SID 策略及入口池健康检查仍见[待定设计](proxy-phase-one-architecture.md#待定设计)。

批次请求当前用 `encoding/json` 编码，记录偏移在请求中为 JSON 数字。控制面的 `protojson` 解码器可接收该表示，本次批次大小测试按网关实际发送的格式验证。[ADR 0003](../../adr/0003-reporting-and-usage-contracts.md#通用约定)约定使用 `protojson`，批次请求编码仍需对齐。

### 本机接入真实悟空

`TestWukongGatewayReportsRealRecorderUsage` 使用本机网关进程、原版 GOST v3.3.3、本机控制面 HTTPS 测试服务和本机 PostgreSQL 17。通过平台发放的 SOCKS5 凭据经悟空访问 `https://api.ipify.org` 并收到 HTTP 200。控制面查询得到测试租户输入 1,792 字节、输出 4,142 字节、计费 5,934 字节，与 Recorder 一致。第二个测试租户无用量。控制面通过 HTTPS 接收记录，测试进程读取网关 Recorder 文件用于核对结果。

### 本机 Docker 容器联调

2026-11-12 在 Windows Docker Desktop 的 Linux amd64 环境验证，代码基线为 `d6298de`。网关、控制面和 PostgreSQL 17 分别运行在三个容器中。网关启动原版 GOST v3.3.3，通过 Docker 网络连接控制面容器内的 Caddy HTTPS 入口，并使用该入口的 CA 证书校验 TLS。Recorder 目录位于网关容器内，控制面未挂载该目录。

| 检查项 | 实测结果 |
| --- | --- |
| 真实悟空转发 | Windows 客户端使用平台发放的 SOCKS5 凭据，经网关和悟空访问 `https://api.ipify.org`，返回 HTTP 200 和有效出口 IP。 |
| Recorder 与上报确认 | `boot_id=LUEEVPNU3JHISLIRTY6C447345`，`file_id=sessions`。测试流量最后一条记录的结束偏移为 2,050 字节。控制面 HTTPS `POST /v1/reporting/batches` 返回 HTTP 200，网关 `confirmed.offset` 和控制面 `ingest_cursors.confirmed_offset` 均为 2,050。 |
| 租户日用量 | 上海自然日 2026-11-12 的基线为空。请求结束后，Recorder 增量之和与控制面查询一致，输入 746 字节、输出 3,550 字节、计费 4,296 字节。 |

### 真实悟空分机联调

待验证。将网关和控制面运行在不同宿主机，Recorder 目录只保留在网关机。使用平台凭据经悟空访问目标，核对网关文件、控制面的 HTTPS 上报确认和租户日用量，并在验收记录中补齐拓扑和实测结果。
