# AGENTS.md

本仓库用于开发多租户 SOCKS5 代理平台，集中管理三个部署单元的代码和文档。

- `management-console` 提供用户控制台和管理后台，通过 `proxy-control` 的 API 执行业务操作。
- `proxy-control` 管理用户、租户、API Key、代理策略、用量和计费。
- `proxy-gateway` 接入 SOCKS5 连接，执行鉴权、上游转发和流量计量。

架构背景见[梳理多租户代理架构](chatgpt-conversation://6af07b80-636c-83e9-ba22-5752b82ad7ec)。

接入或排查悟空 SOCKS5 隧道时，参阅[悟空隧道转发接入](docs/wukong-socks5-tunnel.md)。

## Agent skills

### Issue tracker

Issues are tracked in `DuanJinTaoToday/proxy` on GitHub. See `docs/agents/issue-tracker.md`.

### Triage labels

Use the five default triage labels. See `docs/agents/triage-labels.md`.

### Domain docs

The monorepo uses a single context across all three deployment units. See `docs/agents/domain.md`.
