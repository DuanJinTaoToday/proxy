# AGENTS.md

本仓库用于开发多租户代理服务。平台可接入多个第三方 SOCKS 代理供应商，对客户提供统一的产品、接入方式、权限与计费。

## 项目文档

首版产品范围与验收以 [GitHub Spec #36](https://github.com/DuanJinTaoToday/proxy/issues/36) 为准，仓库内参阅[首版商业交付 Spec](docs/features/proxy-first-release/proxy-first-release-spec.md)。设计、实现或排查用量链路时参阅[用量技术方案](docs/features/proxy-phase-one/proxy-phase-one-architecture.md)。产品讨论参阅[首版产品决策](docs/product/proxy-first-release-decisions.md)。旧一期结果见[历史验收索引](docs/features/proxy-phase-one/proxy-phase-one-development.md)。

接入或排查悟空 SOCKS5 隧道时，参阅[悟空隧道转发接入](docs/integrations/wukong-socks5-tunnel.md)。

调整仓库结构、应用分层、启动与 Wire、配置与 Cron、数据连接与 DAO、构建、代码生成、PR 检查或发布流程，或替换 Spec 时，阅读[开发规范](docs/standards/proxy-development-conventions.md)的对应章节。涉及单个应用时，按需阅读[控制面](apps/proxy-control/AGENTS.md)、[网关](apps/proxy-gateway/AGENTS.md)或[控制台](apps/proxy-console/AGENTS.md)的文档入口。

## Agent skills

在 Proxy 使用 `to-spec` 或 `to-tickets` 时，先阅读开发规范中的 [`to-spec` 与 `to-tickets`](docs/standards/proxy-development-conventions.md#to-spec-与-to-tickets)。

收到“实现 Issue”或 `implement` 请求时，按 [Issue 实现与交付](docs/standards/proxy-development-conventions.md#issue-实现与交付) 执行，交付前主动运行项目级 [verify](.agents/skills/verify/SKILL.md)。逐条取得该 Issue 的验收证据后再报完成。

### Issue tracker

Issues are tracked in `DuanJinTaoToday/proxy` on GitHub. See `docs/agents/issue-tracker.md`.

### Triage labels

Use the five default triage labels. See `docs/agents/triage-labels.md`.

### Domain docs

The monorepo uses a single context across all three deployment units. See `docs/agents/domain.md`.
