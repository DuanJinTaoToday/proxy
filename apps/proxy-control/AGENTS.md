# proxy-control

本指南适用于 `apps/proxy-control/`。控制面使用 Go、Kratos 和 Protobuf，管理租户访问权限、代理凭据、网关上报和用量。持久化使用 PostgreSQL、GORM 和 Goose。

## 目录

| 目录 | 职责 |
| --- | --- |
| `cmd/` | 进程入口、启动参数、配置加载和依赖组装。 |
| `internal/server/` | HTTP 服务装配、路由注册和中间件。 |
| `internal/service/` | API handler、协议参数与响应映射、用例调用。 |
| `internal/biz/<业务能力>/` | 用例、业务规则及其所需的 repo 接口。现有包为 `access/`、`proxyauth/`、`usage/`。 |
| `internal/data/` | 实现 repo，封装数据库和外部服务访问。生成的持久化模型位于 `internal/data/model/`。 |
| `internal/conf/protos/`、`internal/conf/` | 分别存放应用配置 proto 和生成的 Go 类型。 |
| `configs/` | 默认配置和示例。部署环境注入实际凭据。 |
| `bin/` | 构建产物，由构建创建并由 Git 忽略。 |
| `migrations/`、`tools/dao/` | Goose SQL 迁移和持久化模型生成工具。 |
| `internal/pkg/` | 多个应用内包实际复用的工具，按需创建。 |

调用从 `server` 到 `service`、`biz`，再经业务层定义的接口访问数据。Go 包依赖为 `server → service → biz` 和 `data → biz`，由 `cmd` 注入 `data` 实现。业务层使用自己的类型，协议类型在服务层转换，持久化模型在数据层转换。

计量写入、确认偏移和归档锁由同一个数据库事务保护。修改这些流程时，从 `internal/biz/usage/` 的用例追到 `internal/data/` 的事务实现。

## 生成与验证

- 修改 [控制面 API](../../api/proxycontrol/v1/) 或配置 proto 后，在本目录运行 `make api` 或 `make config`，同时提交受影响的生成物。`make pb` 会运行两者。
- 修改表结构时按[开发规范](../../docs/standards/proxy-development-conventions.md)编写迁移并运行 `make dao`。
- 构建和检查使用 `make build`、`make lint` 和 `go test ./...`。
