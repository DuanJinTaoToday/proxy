# proxy-console

一套 Vue 应用承载官网（`/zh-CN/`、`/en/`）、用户控制台（`/console`）和运营控制台（`/ops`）。技术栈为 Vue 3、TypeScript、Vite、Vue Router、Element Plus 和 Vue I18n。 [前端规范](../../docs/standards/proxy-console-frontend.md) · [前端设计与实施计划](../../docs/features/proxy-first-release/proxy-console-frontend-design.md)

## 本地运行

在本目录运行 `make init` 安装依赖，`make run` 启动开发服务器，`make build` 依次运行类型检查、翻译检查并打包到 `dist/`。开发服务器把 `/v1/` 和 `/public/` 请求转发给 `http://127.0.0.1:8000`，启动前可设置 `PROXY_CONTROL_URL` 指向其他控制面地址。`npm run preview` 用构建产物启动本地服务，可直接刷新深链验证路由回退。

## 目录

| 路径 | 内容 |
| --- | --- |
| `src/app/` | 应用装配、路由（含页签容器和建设中占位）、导航配置、语言注册、错误翻译和三套布局 |
| `src/pages/` | 路由页面，`public/`、`console/`、`ops/` 分别归属三个入口 |
| `src/features/<module>/` | 业务模块：组件、会话等状态、业务规则和本模块的 `locales/` 翻译。`auth` 负责登录注册、会话、路由守卫所需的判断和账号菜单 |
| `src/api/` | 统一请求入口 `request()` 与各端点调用 |
| `src/shared/ui/` | 跨业务复用的界面组件 |
| `src/styles/` | 设计 token、Element Plus 映射和基础样式 |
| `scripts/check-i18n.mjs` | 检查中英文键一致、源码引用的键存在 |

## 部署

同一个 HTTPS 站点托管 `dist/` 并转发控制面 API，前端页面路径回落到 `index.html`。测试环境配置见 `deploy/test/Caddyfile`。

## 会话

控制面以 Bearer 令牌鉴权。登录后令牌、到期时间和登录手机号保存在浏览器 `localStorage`（键 `proxy-console.session`），刷新和新标签页保持登录，令牌到期或被控制面拒绝时清除。账号接口不返回手机号，顶栏的脱敏手机号取自登录输入。存放在 `localStorage` 的令牌可被页面内运行的脚本读取，因此不引入第三方脚本，并保持严格的内容安全策略。
