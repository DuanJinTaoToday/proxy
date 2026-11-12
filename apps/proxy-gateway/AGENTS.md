# proxy-gateway

本指南适用于 `apps/proxy-gateway/`。网关交付原版 GOST 和 Go 采集器，负责 SOCKS5 转发、本机认证中继及 Recorder 记录上报。

| 目录 | 职责 |
| --- | --- |
| `collector/` | 采集器进程入口、控制面连接、GOST 配置渲染与子进程管理。 |
| `internal/recorder/` | Recorder 文件读取、批量上报、确认偏移持久化及生命周期和监控逻辑。 |
| `internal/authrelay/` | GOST 认证中继、控制面状态刷新和认证指标。 |
| `gost/` | GOST 配置模板和上游产物相关资料。配置变量见 [GOST 配置说明](gost/README.md)。 |
| `bin/`、`dist/` | 分别存放构建产物和发布归档。平台目录及归档命名见[开发规范](../../docs/standards/proxy-development-conventions.md)。 |

`collector/` 装配 Recorder 和认证中继，两个模块通过各自的接口与控制面交互。共享协议使用 [控制面 API](../../api/proxycontrol/v1/)。Recorder 文件和确认偏移由运行环境管理，升级或回滚时保留。

修改采集、重放或认证同步时，阅读[一期技术方案](../../docs/features/proxy-phase-one/proxy-phase-one-architecture.md)。涉及文件系统或持久化时同时检查现有 Windows 和 Unix 实现。采集器构建和检查使用 `make build`、`make lint` 和 `go test ./...`。
