# Proxy

多租户代理服务。首版商业交付包含悟空动态住宅 GB 和皇冠静态住宅 IP。客户自助开户，由运营为租户钱包充值，客户从钱包购买商品。动态住宅通过 SOCKS5 隧道接入，静态住宅 IP 按实际交付数量结算并支持到期前手动保号续费。两类商品独立验收和开售。 [首版商业交付 Spec #36](https://github.com/DuanJinTaoToday/proxy/issues/36)

当前工作区已有手机号注册和登录、动态隧道接入、Recorder 采集上报及租户用量查询。开售仍须按 Spec 验收。首版规划时的实现差距见[差异审计](docs/product/proxy-first-release-gap-audit.md)。

## 本地 Docker 测试环境

启动 Docker Desktop 后，在仓库根目录运行 `make test-env`，即可搭建或更新本地环境。控制台为 `https://localhost:8443`，SOCKS5 为 `127.0.0.1:1080`。测试账号、部署配置和命名规则见[本地 Docker 测试环境](docs/operations/local-test-environment.md)。

## 仓库结构

| 目录 | 说明 |
| --- | --- |
| [apps/proxy-control](apps/proxy-control/README.md) | 控制面：租户登录与授权、代理凭据、用量归集、流量额度、归档 |
| [apps/proxy-gateway](apps/proxy-gateway/README.md) | 网关：基于原版 GOST 的 SOCKS5 接入、认证中继、Recorder 采集与上报 |
| [apps/proxy-console](apps/proxy-console/README.md) | 控制台：手机号注册、登录与租户用量页面，商业页面待实现 |
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

各应用的构建、配置与启动见对应目录的 README。通用约定是：复制 `configs/file/server.yaml` 为 `server.local.yaml` 并填写，复制 `configs/init.yaml` 为 `configs/file/init.local.yaml`，再运行 `make build` 与 `./bin/<应用名> -conf ./configs/file/init.local.yaml`。详见[开发规范](docs/standards/proxy-development-conventions.md#启动与-wire)。

## 文档

- [首版商业交付 Spec](docs/features/proxy-first-release/proxy-first-release-spec.md) · [历史验收索引](docs/features/proxy-phase-one/proxy-phase-one-development.md)
- [用量技术方案](docs/features/proxy-phase-one/proxy-phase-one-architecture.md)：用量链路设计
- [ADR](docs/adr/README.md)：用量模型、身份、上报契约、计费入库、认证可用性等决策
- [开发规范](docs/standards/proxy-development-conventions.md)：分层、Wire、配置与 Cron、数据库与 DAO、构建、PR 检查、发布
- [首版产品决策](docs/product/proxy-first-release-decisions.md)
- [悟空 SOCKS5 隧道接入](docs/integrations/wukong-socks5-tunnel.md) · [皇冠静态 IP 接入](docs/integrations/ipcrown-static-ip.md) · [GOST 文件 Recorder](docs/integrations/gost-file-recorder.md)
- [AGENTS.md](AGENTS.md)：面向 AI 协作者的项目指引
