# proxy-control

控制面服务（Go）。负责：

- 租户登录、租户选择与权限检查（`internal/proxyauth`）
- 代理凭据发放与停用，向网关提供认证与额度同步
- 接收网关 Recorder 批次上报，按租户、网关、分钟归集计费汇总（`internal/usage`）
- 流量额度、倍率、准入门槛与欠费结算，用量归档（Cron 任务 `MaintainUsage`）
- 租户与运营的用量、余额查询接口

## 目录

| 路径 | 说明 |
| --- | --- |
| `cmd` | 入口与 Wire 装配 |
| `internal/biz`、`service`、`server`、`data` | 分层实现 |
| `internal/conf` | 配置 Protobuf 与生成代码 |
| `configs` | `init.yaml` 与 `file/server.yaml` 配置模板 |
| `migrations` | Goose 数据库迁移 |
| `integration` | PostgreSQL 集成用例 |

接口定义位于仓库根目录 [`api/protos/proxycontrol/v1`](../../api/protos/proxycontrol/v1/)。生成的 Go 绑定位于 [`api/proxycontrol/v1`](../../api/proxycontrol/v1/)，接口文档见 [`openapi.yaml`](../../api/proxycontrol/openapi.yaml)。

## 构建与启动

在本目录执行：

```sh
cp configs/file/server.yaml configs/file/server.local.yaml   # 填写本地配置
cp configs/init.yaml configs/file/init.local.yaml            # source.file 指向 ./configs/file/server.local.yaml
make build
./bin/proxy-control -conf ./configs/file/init.local.yaml
```

Windows 下可执行文件带 `.exe` 后缀。

数据库迁移（在仓库根目录，需先 `make init`）：

```sh
./.tools/bin/goose -dir apps/proxy-control/migrations postgres "<PostgreSQL DSN>" up
```

## 常用命令

| 命令 | 说明 |
| --- | --- |
| `make build` | 构建到 `bin/proxy-control` |
| `make api` | 生成控制面 Protobuf Go 代码、Kratos HTTP 绑定和 OpenAPI 文档 |
| `make pb` | 运行 `make api` 和 `make config`，同时生成 API 和应用配置的产物 |
| `make dao` | 生成 DAO 模型 |
| `make wire` | 重新生成依赖注入 |
| `make lint` | 运行 golangci-lint |

## 文档

- [配置说明：租户登录、授权与用量上报](configs/file/README.md)
- [开发规范](../../docs/standards/proxy-development-conventions.md)
- [一期技术方案](../../docs/features/proxy-phase-one/proxy-phase-one-architecture.md)
- [AGENTS.md](AGENTS.md)
