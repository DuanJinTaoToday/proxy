# AGENTS.md

本仓库用于开发多租户代理服务。平台可接入多个第三方 SOCKS 代理供应商，对客户提供统一的产品、接入方式、权限与计费。

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
