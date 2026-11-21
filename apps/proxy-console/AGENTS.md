# proxy-console

- 开发约定以 [Proxy Console 前端规范](../../docs/standards/proxy-console-frontend.md) 为准：目录与模块边界、Element Plus 复用、设计 token 和双语规则。页面范围、导航与实施阶段见[前端设计与实施计划](../../docs/features/proxy-first-release/proxy-console-frontend-design.md)。
- 页面结构和交互参照客户控制台高保真原型 `prototypes/console/static-residential-ip/`，字段、状态和错误以[首版 Spec](../../docs/features/proxy-first-release/proxy-first-release-spec.md) 和[控制面 API](../../api/proxycontrol/openapi.yaml) 为准。
- 调整构建入口或 API 转发时，阅读[开发规范中的应用分层约定](../../docs/standards/proxy-development-conventions.md#应用分层与测试归属)。
- 新增文案同时补齐 `zh-CN` 和 `en`，`npm run build` 会运行翻译检查。交付前按项目 [verify](../../.agents/skills/verify/SKILL.md) 在 `tests/verify/scenarios/` 补充用户路径。
