# Proxy Console 前端规范

[设计方案与实施计划](../features/proxy-first-release/proxy-console-frontend-design.md)记录页面范围、导航和验收。本文约束 `apps/proxy-console` 的日常开发。

## 框架与目录

使用 Vue 3、TypeScript、Vite、Vue Router、Element Plus 和 Vue I18n。组件使用 Vue 单文件组件与 `<script setup lang="ts">`，通过类型化的 props 和 emits 定义接口。可复用的响应式行为使用 composable。使用路由名称、嵌套路由和动态导入组织页面。用 `vue-tsc` 检查模板和 TypeScript，Vite 负责构建。使用 npm 锁文件和 `npm ci` 复现依赖。

[Vue 单文件组件](https://vuejs.org/guide/scaling-up/sfc.html) · [Vue composable](https://vuejs.org/guide/reusability/composables.html) · [Vue Router 路由懒加载](https://router.vuejs.org/guide/advanced/lazy-loading.html) · [Element Plus 接入](https://element-plus.org/en-US/guide/quickstart.html)

| 路径 | 归属 |
| --- | --- |
| `src/app/` | 应用装配、路由、三套布局、语言注册和顶栏。 |
| `src/pages/{public,console,ops}/` | 路由页面和页面组合。每个页面使用对应入口的布局。 |
| `src/features/<module>/` | 业务组件、composable、业务类型和翻译。出现跨模块调用时，从模块入口导出所需接口。 |
| `src/api/` | HTTP 客户端、请求和响应类型、各端点调用与数据映射。控制面契约以仓库 `api/` 为准。 |
| `src/shared/ui/` | 稳定跨业务复用的界面组件。 |
| `src/shared/lib/` | 跨模块纯工具，例如金额及时间格式化。仅供单模块使用的函数留在模块内。 |
| `src/styles/` | 设计 token、默认主题、Element Plus 映射和基础样式。 |
| `src/assets/` | 应用自有图片和字体等静态资源。 |

目录随真实用途创建。页面可组合多个业务模块。模块之间只引用公开接口，禁止导入对方的内部文件或路由页面。共享层不依赖业务层。API 层不依赖页面、组件和翻译包。

## 模块边界

| 模块 | 职责 |
| --- | --- |
| `auth` | 登录、注册、退出、会话和可校验的站内回跳。路由守卫处理入口体验，控制面检查身份和权限。 |
| `users` | 当前账号资料、运营租户查找和租户详情。通过租户 ID 组合钱包与订单模块。代理子账号由动态代理模块管理。 |
| `orders` | 订单筛选、成交快照、交付进度与运营待处理订单。以控制面返回的状态为准。 |
| `settings` | 浏览器语言偏好及后续账号设置。认证凭据变更由 `auth` 执行，商品定价与开售归 `catalog`。 |
| `catalog`、`checkout` | 前者负责商品、报价与开售状态，后者负责购买意图、报价复核和确认提交。登录回跳只携带规格、数量和站内目标。 |
| `dynamic-proxy`、`static-ip` | 前者负责代理子账号、接入和用量，后者负责已交付 IP、复制、导出和续费。 |
| `wallet`、`notifications` | 前者负责余额、流水和运营充值，后者负责当前账号通知和提醒。 |

用组件状态保存临时交互，用路由 query 保存需要返回或分享的筛选条件。需要列表与详情共享服务端缓存时使用 TanStack Vue Query，并在退出或切换身份时清理会话数据。通过统一 `fetch` 入口处理认证、请求取消和错误。API 映射保留 64 位字节值的精度。业务错误用稳定错误码翻译。购买和充值由后端确认结果，前端保持原有幂等契约。 [Vue Query 默认行为](https://tanstack.com/query/latest/docs/framework/vue/guides/important-defaults)

## 组件复用

| 用途 | 首选 |
| --- | --- |
| 按钮、输入、表单、卡片 | `ElButton`、`ElInput`、`ElForm`、`ElCard` |
| 表格、分页、状态 | `ElTable`、`ElPagination`、`ElTag` |
| 弹窗、确认、提示 | `ElDialog`、`ElMessageBox`、`ElAlert`、`ElMessage` |
| 导航、页签、菜单 | `ElMenu`、`ElTabs`、`ElDropdown` 配合 Vue Router |

优先使用组件库公开的 props、slots、事件和主题变量。禁止复制组件库源码、逐个包装基础控件，或引入功能重复的组件库。共用顶栏放在 `app/layouts/`，其中的通知、语言和用户入口用 Element Plus 组合。骨架期先提供 `PageHeader` 和基于 `ElEmpty` 的页面空状态。余额卡、接入凭据、续费确认和充值确认在有真实调用方时封装，归各自业务模块。筛选、表格列和批量资格由业务模块定义。

## 设计 token 与主题

在 `styles/tokens.css` 定义 CSS 自定义属性。颜色用语义命名，页面直接消费角色值。初版使用浅色绿调，以下数值取自静态 IP 原型。调整时只修改主题定义或对应语义 token。

| 类别 | 初始规则 |
| --- | --- |
| 颜色 | 主操作 `#086950`，主操作文字 `#FFFFFF`，强调 `#B3F0D7`，画布 `#F4F6F5`，工作面 `#FFFFFF`，正文 `#172322`，次级文字 `#556562`，边框 `#DEE5E1`。控制台侧栏底色 `#142522`，选中项 `#294940`。定义选中、焦点、禁用及成功、警告、错误、信息各自的文字、背景和边框 token。 |
| 字体 | 系统无衬线字体和中英文回退。正文 14 px，辅助 12 px，页面标题 24 px。金额及用量用等宽数字，连接字符串用等宽字体。字号、字重与行高有对应 token。 |
| 间距与密度 | 4、8、12、16、24、32、48 px 间距阶梯。客户控件常规密度，运营控件紧凑密度。控件最小高度、布局边距和内容宽度用 token。 |
| 圆角与阴影 | 控件 6 px，卡片 10 px。卡片以细边框分区，浮层使用统一浮层阴影。圆角、边框宽度和阴影引用 token。 |

`styles/element-plus.css` 在组件库样式之后加载，将默认主题的语义 token 映射到 `--el-*`，包含实际用到的交互状态。页面样式不得硬编码颜色、字号、间距、圆角和阴影，也不得覆盖组件库内部选择器。允许在布局样式中集中定义响应式断点。将主题作用域挂在文档根元素，以覆盖 Dialog、Dropdown 等传送到正文的浮层。首版提供一套浅色主题，后续主题通过补充根级 token 值接入。 [Element Plus 主题定制](https://element-plus.org/en-US/guide/theming.html)

## 双语

- 应用语言为 `zh-CN` 和 `en`。官网用 `/zh-CN/…`、`/en/…`，根路径 `/` 默认进入中文首页。官网以 URL 为准。控制台用 `/console/…` 和 `/ops/…`，使用浏览器记住的偏好，缺省和无效值回到中文。
- 顶栏切换语言时更新偏好和文档 `lang`。官网跳到当前页面的对应语言路由，控制台保持当前 URL。保留规格、数量、筛选和编辑中的表单。应用、Element Plus 和日期控件共用同一个 locale。
- 页面标题、按钮、菜单、提示、空状态、校验和错误文案都用翻译键。业务模块维护本模块的中英文翻译，共用文案由应用层维护。教程和官网长文维护对应的双语内容。双语键缺失在检查阶段报出。
- 用完整句子、命名插值和复数规则处理文案。金额仍为人民币，用量仍采用十进制 GB，业务日仍按上海时区。翻译切换只改变呈现。
- 按钮随文字增长，操作组允许换行。卡片标题可换行，底部操作区对齐。窄屏筛选工具栏可折行，表格滚动限制在表格容器内。关键确认、错误和状态不得依赖截断或颜色才能理解。

[Vue I18n 语言切换](https://vue-i18n.intlify.dev/guide/essentials/scope) · [Element Plus 国际化](https://element-plus.org/en-US/guide/i18n.html)
