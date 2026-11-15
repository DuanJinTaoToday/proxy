# proxy-console

- 调整前端目录、构建入口或 API 调用时，阅读[开发规范中的应用分层约定](../../docs/standards/proxy-development-conventions.md#应用分层与测试归属)。
- 调整租户或运营功能时，参阅[首版商业交付 Spec](../../docs/features/proxy-first-release/proxy-first-release-spec.md)和[控制面 API](../../api/proxycontrol/openapi.yaml)。

## 控制台界面

- 新建或改版控制台页面时，参考 [711Proxy 控制台](https://dashboard.711proxy.com/zh-CN) 的信息架构、导航和业务流程，功能范围与字段以本项目 Spec 和 API 为准。
- 视觉统一采用 [shadcn-vue dashboard-01](https://www.shadcn-vue.com/blocks) 风格。复用现有 Vue 3 + Vite 实现和控制台组件，沿用可折叠侧栏、紧凑页头、中性配色、轻边框卡片和表格。以[租户钱包](src/TenantWallet.vue)为项目内样板，按业务需要安排卡片、图表和操作。
- 实现页面时，优先复用 shadcn-vue 官方已有的 block 和组件，再按本项目的业务逻辑、字段和 API 适配。
- 列表和统计使用真实 API，标明统计范围。界面改版保留现有操作及加载、空数据和错误状态，并检查桌面与窄屏布局。
