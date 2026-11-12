# proxy-gateway

本指南适用于 `apps/proxy-gateway/`。网关交付原版 `gost` 和自研 Go 服务 `proxy-gateway`。GOST 负责 SOCKS5 转发，`proxy-gateway` 启动并管理 GOST 子进程，运行本机认证中继和 Recorder 采集上报。

## 目录

| 目录 | 职责 |
| --- | --- |
| `cmd/` | 服务入口、启动参数、配置读取、依赖组装和进程启停。 |
| `internal/server/`、`internal/service/` | 装配本机 HTTP 服务，处理 GOST 认证协议和指标请求。 |
| `internal/biz/authrelay/` | 认证决策、缓存和状态刷新。 |
| `internal/biz/recorder/` | 采集调度、批次确认与重放、GOST 代次状态上报。 |
| `internal/data/control/` | 控制面 HTTP 客户端和传输配置。 |
| `internal/data/recorder/` | Recorder 文件、确认偏移、退出标记和磁盘监控。 |
| `internal/data/gost/` | 渲染 GOST 配置，管理子进程并记录退出状态。 |
| `internal/pkg/authmetrics/` | 认证中继各层共享的指标。 |
| `gost/` | GOST 配置模板和上游产物相关资料。配置变量见 [GOST 配置说明](gost/README.md)。 |
| `bin/`、`dist/` | 分别存放构建产物和发布归档。平台目录及归档命名见[开发规范](../../docs/standards/proxy-development-conventions.md)。 |

按 `server → service → biz` 组织请求处理，`data` 实现业务接口，由 `cmd` 注入。新增网关业务按能力在 `internal/biz/` 下分包。网关与控制面通过[控制面 API](../../api/proxycontrol/v1/)交互。升级或回滚时保留 Recorder 文件和确认偏移。

修改采集、重放或认证同步时，阅读[一期技术方案](../../docs/features/proxy-phase-one/proxy-phase-one-architecture.md)。涉及文件系统或持久化时同时检查现有 Windows 和 Unix 实现。`make build` 生成 `bin/proxy-gateway`（Windows 为 `bin/proxy-gateway.exe`），检查使用 `make lint` 和 `go test ./...`。设置 `PROXY_GOST_TEST_BINARY` 为 GOST v3.3.3 可执行文件的绝对路径后，测试会覆盖真实 GOST 认证、采集和子进程退出。
