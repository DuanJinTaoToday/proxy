# GOST v3.3.3 文件 Recorder 实测

在 Windows amd64 上，用官方 GOST v3.3.3 发行版跑通本地 SOCKS5 CONNECT、原生轮转和独立代次路径。二进制的 Go 构建信息包含 `go-gost/x v0.28.1` 和 `lumberjack.v2 v2.2.1`。临界写入和写入故障由第二个程序直接调用同版本的未修改 Recorder，使用临时文件复现。 [官方发行版](https://github.com/go-gost/gost/releases/tag/v3.3.3)

## 复现

检出 [archive/gost-recorder](https://github.com/DuanJinTaoToday/proxy/tree/archive/gost-recorder/prototypes/gost-recorder) tag，从仓库根目录运行。原型需要 Python 3.8+、Go 1.26.3+ 和官方二进制，本次使用 Go 1.27.4 运行原型。Windows amd64 的发行包 SHA-256 为 `b00441624a66ebed94a2ddcc01ab2832069f053e6ca0abf19d7104b84fad1f78`，可用发行版的 `checksums.txt` 核对。

```powershell
go version -m C:\path\to\gost.exe
python prototypes/gost-recorder/run.py C:\path\to\gost.exe
Push-Location prototypes/gost-recorder
go run .
Pop-Location
```

- `run.py` 只绑定环回地址。它先在测试代次的活跃路径放入距 1 MiB 上限差 2 字节的样本，再让官方进程写首条采样，核对备份名称与文件身份。之后启动两个不配置 `rotation` 的代次，核对各自的独立路径和记录。最后把 Recorder 的目标设为目录，验证初始化打开失败时 SOCKS5 仍可转发且没有记录文件。
- `go run .` 用真实配置解析入口接入 lumberjack，把已经打开的文件恰好填到 1 MiB，检查换行符的位置。故障部分用可控的 `io.WriteCloser` 向真实临时文件分别写入零字节、JSON 前 13 字节或整段 JSON，再返回错误，之后恢复写入并检查原始字节、增量和行解析结果。实验不填满宿主磁盘。两个程序都会打印样本文件所在的临时目录。

## 原生轮转与文件身份

配置 `recorders[].file.rotation` 才走 lumberjack。已打开的文件在下一次 `Write` 会使总大小 **大于** `maxSize` 时先轮转，单位为 MiB，零值默认 100 MiB。首次写入已有文件时，若现有大小加本次写入大小 **大于等于** 上限也会轮转。单次写入自身大于上限会报错。没有按时间触发的轮转。`maxAge` 和 `maxBackups` 控制清理，`compress` 控制备份压缩。 [GOST 配置解析](https://github.com/go-gost/x/blob/v0.28.1/config/parsing/recorder/parse.go) · [lumberjack v2.2.1](https://github.com/natefinch/lumberjack/blob/v2.2.1/lumberjack.go)

轮转关闭活跃文件，在同目录改名为 `<stem>-YYYY-MM-DDTHH-mm-ss.mmm<ext>`，再用原路径创建新文件。备份名默认用 UTC，`localTime: true` 时用宿主本地时间，名称自身没有时区标记。官方二进制生成了 `sessions-2026-11-12T02-45-36.457.jsonl`。改名前后原文件的系统文件身份一致，新活跃文件的身份不同。

Recorder 加锁串行写每条记录，但 JSON 和 `sep: "\n"` 各调用一次 `Write`。lumberjack 在每次写入前决定轮转，成功轮转不会把一次 JSON 写入从中间切开。临界实测中，旧文件正好 1,048,576 字节，末尾是完整 JSON 且没有换行，新文件最初只有一个 `0a` 字节。逐文件只收 LF 结尾的行会漏掉旧文件最后一条。 [FileRecorder](https://github.com/go-gost/x/blob/v0.28.1/recorder/file.go#L55-L78)

同文件系统内改名可用系统文件身份跟踪，Linux 取 `(st_dev, st_ino)`，Windows 取卷与文件 ID。本次在 Windows 实测改名身份，Linux 行为依据同文件系统的改名语义。路径会复用，时间戳只到毫秒，Recorder 没有文件头。系统文件身份也可能在删除后复用，因此若将来启用原生轮转，采集器应按打开句柄取得身份，在保留文件期间把身份持久映射到文件 ID，从原字节偏移接着读，定期扫描补足通知遗漏。旧文件应先处理到 EOF，再处理新文件。完整 JSON 末尾和后继文件的首个 LF 必须作为同一逻辑行处理，记录 ID 锚定旧文件及 JSON 起始偏移。该做法还需要验证文件顺序、时间戳碰撞、跨文件确认及崩溃恢复。自动清理、压缩和 `copytruncate` 会改变这一跟踪前提。

## 增量与写入故障

同一 `sessionID` 的累计字节以最近一次 **Recorder 返回成功** 的累计值为基准。首次成功记录从零计算，正常连续记录的增量等于累计差。累加基准、`recordIndex` 和上次采样时间只在写入成功后推进。会话的计数若下降，序列化的累计值会钳制到已提交的值。 [Session.emit](https://github.com/go-gost/x/blob/v0.28.1/recorder/session.go#L111-L118) · [增量计算](https://github.com/go-gost/x/blob/v0.28.1/recorder/session_record.go#L34-L55)

| 场景 | 实测结果 |
| --- | --- |
| 官方二进制正常转发 | 累计为 `45/44 → 109/108 → 109/108`，对应增量为 `45/44 → 64/64 → 0/0`。最后一条是 `final`。 |
| 写 `130/214` 时失败且未留下字节 | 上次成功为 `100/200`，恢复后写 `180/294` 的增量为 `80/94`，`recordIndex=2`。如果 `130/214` 已成功，`180/294` 的增量才是 `50/80`。 |
| JSON 写了前 13 字节后失败 | 残片留在磁盘。下一条仍写 `80/94`，但和残片粘在同一物理行，整行不能解析。再下一行可解析，其增量只有新增的 `1/2`。 |
| JSON 写完，LF 写入失败 | 留下完整 JSON，恢复时另一段 JSON 紧接其后。两段都用 `recordIndex=2`，恢复记录已包含失败记录的增量。整行不能解析，分别计费会重计。再下一行可解析。 |
| `final` 写入失败 | 会话已结束，恢复后再次调用 `Finish` 不会重写，后续采样无法补入该增量。 |
| 关闭原生轮转时，启动打开文件失败 | 配置解析记录 warning 并使用返回成功的丢弃 writer。实测目标是目录时代理仍成功转发。恢复路径权限也需要重启才能换回文件 writer。 |

官方进程输出的第二条样本行：

```json
{"service":"lab","network":"tcp","remote":"127.0.0.1:58375","local":"127.0.0.1:58373","client":"127.0.0.1:58375","src":"127.0.0.1:58376","dst":"127.0.0.1:58372","host":"127.0.0.1:58372","clientIP":"","route":"127.0.0.1:58372","inputBytes":109,"outputBytes":108,"sid":"dbqigjtk1l19ui0q04c0","duration":1771687400,"time":"2026-11-12T10:45:37.4568944+08:00","sessionID":"RYWMPOII65GIAGMSTNHJM6EBA2","recordIndex":2,"phase":"interim","inputBytesDelta":64,"outputBytesDelta":64,"durationDelta":1000000000}
```

坏行示意，省略其他字段：

```text
{"service":""{"service":"","recordIndex":2,"inputBytesDelta":80,"outputBytesDelta":94,...}
{"recordIndex":2,"inputBytesDelta":30,...}{"recordIndex":2,"inputBytesDelta":80,...}
```

## 时间与一期配置

`time` 是本次采样或结束时间，Go JSON 编码输出 RFC 3339，允许最多 9 位小数，携带实际 UTC 偏移或 `Z`。实测官方进程输出 `+08:00`，受控样本输出了 9 位小数。备份名的 UTC 设置独立于记录时间。网关宿主启用时间同步并监控偏移，采集与控制面按带偏移的时间解析，控制面转成 `Asia/Shanghai` 做分钟和自然日归属。

一期按启动代次换文件，省略 GOST 的 `file.rotation`，也不在进程运行期间改名或截断活跃文件。启动包装器每次启动或重试都生成新 `boot_id`，独占创建 `<spool>/<boot_id>/`，以 GOST 的运行身份验证目标可写，渲染 `file.path` 为该目录下的绝对路径 `sessions.jsonl`，配置 `sep: "\n"`，再用 `gost -C <config>` 启动。源路径显式写入配置。打开文件告警须进入网关就绪检查和监控，因为 SOCKS5 端口可用无法证明 Recorder 可用。

每代只有一个源文件，`file_id = "sessions"`，以 `(gateway_id, boot_id, file_id)` 限定文件，起始字节偏移限定记录。采集器重新扫描代次目录，逐代从连续已确认的位置恢复。以 LF 结束且编码有效的 UTF-8 行原样上报，由控制面在批次事务内隔离无法解析的行。禁止拆开粘连 JSON 分别计费。非法 UTF-8 与旧代确认退出后的无 LF 残尾须保留原始字节，前者无法直接放进 `RecorderLine.line`，后者缺少完整行的分隔符。两类异常的确认与水位规则待定，确认前保留源文件和原偏移。已关闭的文件在全部确认后才清理。按代次换文件会使单文件随进程运行增长。以磁盘水位监控和计划排空、重启限制增长，连接切换及告警阈值仍需按部署验收确定。

已复现文件写入与格式行为。实际磁盘满、宿主机掉电、生产 Linux 采集器和 PostgreSQL 回放未在本原型中验证。文件 writer 返回成功只说明写调用成功，源码没有逐条 `fsync`。
