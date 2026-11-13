# Proxy

多租户代理服务。平台接入多个第三方 SOCKS 代理供应商（一期为悟空），向客户提供统一的产品、接入方式、权限与计费。

一期目标是打通「隧道用量与流量额度」闭环：租户用平台凭据经网关访问目标，网关采集用量并上报，控制面按租户归集、结算额度，额度不足时拒绝新连接。

## 仓库结构

| 目录 | 说明 |
| --- | --- |
| [apps/proxy-control](apps/proxy-control/README.md) | 控制面：租户登录与授权、代理凭据、用量归集、流量额度、归档 |
| [apps/proxy-gateway](apps/proxy-gateway/README.md) | 网关：基于原版 GOST 的 SOCKS5 接入、认证中继、Recorder 采集与上报 |
| [apps/proxy-console](apps/proxy-console/README.md) | 控制台：租户与运营前端（尚未开始） |
| `api/protos/proxycontrol` | 控制面 Protobuf 接口定义 |
| `api/proxycontrol` | 生成的 Go 绑定与 OpenAPI 文档 |
| `pkg` | 公共库：`client`、`cron`、`log` |
| `build` | 构建工具（protoc 安装、工具链版本） |
| `third_party` | 第三方 proto（googleapis） |
| `prototypes` | 一次性原型，不属于交付物 |
| `docs` | Spec、技术方案、ADR、集成与运维文档 |

三个应用作为独立部署单元，共用同一个 Go module（`github.com/DuanJinTaoToday/proxy`）和同一份领域语境（见 [GLOSSARY.md](GLOSSARY.md)）。

## 快速开始

前置：Go（版本见 `go.mod`）、PostgreSQL（控制面）、原版 GOST v3.3.3（网关）。

```sh
# 安装 protoc、Go / HTTP / OpenAPI 生成插件、golangci-lint、goose、wire 到 ./.tools
make init
```

各应用的构建、配置与启动见对应目录的 README。通用约定是：复制 `configs/file/server.yaml` 为 `server.local.yaml` 并填写，复制 `configs/init.yaml` 为 `configs/file/init.local.yaml`，再运行 `make build` 与 `./bin/<应用名> -conf ./configs/file/init.local.yaml`。详见[开发规范](docs/standards/proxy-development-conventions.md#本地启动)。

## 文档

- [一期 Spec](docs/features/proxy-phase-one/proxy-phase-one-spec.md) 与[开发任务](docs/features/proxy-phase-one/proxy-phase-one-development.md)：范围与验收
- [一期技术方案](docs/features/proxy-phase-one/proxy-phase-one-architecture.md)：用量链路设计
- [ADR](docs/adr/README.md)：用量模型、身份、上报契约、计费入库、认证可用性等决策
- [开发规范](docs/standards/proxy-development-conventions.md)：分层、Wire、配置与 Cron、数据库与 DAO、构建、PR 检查、发布
- [产品对齐](docs/product/proxy-product-alignment.md)
- [悟空 SOCKS5 隧道接入](docs/integrations/wukong-socks5-tunnel.md)、[GOST 文件 Recorder](docs/integrations/gost-file-recorder.md)
- [AGENTS.md](AGENTS.md)：面向 AI 协作者的项目指引
