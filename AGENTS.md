# AGENTS.md

本仓库用于开发多租户代理服务。平台可接入多个第三方 SOCKS 代理供应商，对客户提供统一的产品、接入方式、权限与计费。

## 仓库目录

仓库使用根目录 `go.mod`。按部署单元组织 `apps/`，修改应用前阅读该应用的 `AGENTS.md`。应用规则补充根目录规则，具体目录和命令以应用指南为准。

| 目录 | 职责 |
| --- | --- |
| `apps/` | [控制面](apps/proxy-control/AGENTS.md)、[网关](apps/proxy-gateway/AGENTS.md)和[控制台](apps/proxy-console/AGENTS.md)的源码、配置和构建入口。 |
| `api/<服务名>/<版本>/` | 按接口提供方和版本维护平台 Protobuf 契约及其生成物。 |
| `pkg/` | 放多个应用实际复用的 Go 包，出现复用时再创建。应用私有实现留在所属应用。 |
| `build/`、`third_party/` | 共享构建工具和第三方协议定义及其许可证。 |
| `docs/`、`prototypes/` | 项目文档和独立验证原型。部署单元的实现归 `apps/`。 |

跨应用通过 API 契约交互。Go 应用仅引用公开契约和共享包，应用的 `internal/` 由所属应用使用。本地覆盖配置和调试产物如需保留，可放在应用的 `local/`，并配置 Git 忽略。

## 项目文档

一期开发与验收参阅[一期 Spec](docs/features/proxy-phase-one/proxy-phase-one-spec.md)和[开发任务](docs/features/proxy-phase-one/proxy-phase-one-development.md)。设计、实现或排查用量链路时参阅[一期技术方案](docs/features/proxy-phase-one/proxy-phase-one-architecture.md)。讨论产品形态与供应商时参阅[产品对齐](docs/product/proxy-product-alignment.md)。

接入或排查悟空 SOCKS5 隧道时，参阅[悟空隧道转发接入](docs/integrations/wukong-socks5-tunnel.md)。

调整项目结构、构建、代码生成、检查或发布流程时，参阅[开发规范](docs/standards/proxy-development-conventions.md)。

替换 Spec 时，把仍有效的约定并入现行文档，更新引用，并从工作树移除旧稿。需要追溯时查阅 Git 历史。

## Agent skills

### Issue tracker

Issues are tracked in `DuanJinTaoToday/proxy` on GitHub. See `docs/agents/issue-tracker.md`.

### Triage labels

Use the five default triage labels. See `docs/agents/triage-labels.md`.

### Domain docs

The monorepo uses a single context across all three deployment units. See `docs/agents/domain.md`.
