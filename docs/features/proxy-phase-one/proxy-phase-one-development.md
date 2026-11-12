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

## Issue #8 验收记录

2026-11-13 从 `origin/main` 的 `2cdf0a7` 开分支。沿用 Issue #5 的 Windows Docker Desktop 三容器拓扑，使用 Linux amd64 原版 GOST v3.3.3、控制面容器内的 Caddy HTTPS 和 PostgreSQL 17。Recorder 仅位于网关容器，SOCKS5 目标换成同容器内的受控 TCP echo 服务。

| 验收项 | 实测结果 |
| --- | --- |
| 关闭前可查用量 | 使用平台凭据通过 SOCKS5 先关闭一条连接，再保持第二条连接打开并收发 128 字节。第二条连接已写入 `interim`，尚无 `final`。租户日用量接口返回输入 343、输出 229、计费 572 字节，与当时已确认的 Recorder 增量一致。确认偏移为 1,080。 |
| 停机、提交后响应丢失和恢复 | 暂停控制面容器，经仍打开的连接收发 4,096 字节。Recorder 写到 1,628，网关确认偏移保持 1,080。恢复控制面后，Caddy 在收到上游 HTTP 200 后断开响应。数据库水位已到 1,628，网关仍为 1,080。恢复正常响应后，网关重报并追到 1,628。租户日用量保持输入 4,439、输出 4,325、计费 8,764 字节。 |
| 确认偏移丢失后的重放 | 停止网关后将旧代次偏移重置到 0，模拟收到确认后、持久化前退出时的磁盘状态。网关重启后按原记录 ID 重放，确认偏移重新达到 1,628。再次提交相同的 3 条记录，控制面返回 `duplicates=3`，日用量保持 8,764 字节。自动化用例另外在收到确认后的偏移保存位置注入失败并验证重启重放。 |
| GOST 异常退出和可能缺口 | 对 GOST 发 `SIGKILL`，控制面收到代次 `FW5LGL77GOWOR5FTTPODL5SHTK` 的退出上报。网关重启并补报后，`CompleteReplay` 核对到 1 条可能缺口，对应缺少 `final` 的会话 `WLP5TBP7E6SSW765ERKV42H3R2`。已有 `final` 的会话未产生缺口。第二个代次 `DTJBNZB3NNBNS2BQNTHHHLLXZN` 的 Recorder 为空，强制终止仍检测到退出，重启后登记 1 条代次级可能缺口。缺失字节量未知。 |
| 文件换代和保留 | 每次重启创建新的 Recorder 路径。旧代次核对完成后仍保留源文件。将首个旧代次的本地回放确认标记改为 15 天前，采集器删除该文件，保留刚完成核对的空文件。自动化用例验证仍在运行和残尾未确认的旧文件均不会清理。 |

保留期从本地回放确认标记写入时算起。旧代次只有无 LF 残尾时，自动化验证采集器核对已确认前缀并保留源文件。仅有隔离记录的旧代次也能登记代次级可能缺口。非法 UTF-8 沿用现有停偏移和留证规则。磁盘水位和写入失败沿用现有监控，本轮未注入磁盘写满、宿主机掉电或网关与 GOST 同时被强制终止。
