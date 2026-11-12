# proxy-console

一期控制台采用 Vue 3、TypeScript、Vite 和 Vue Router。租户成员登录后可以切换有权访问的租户，查询上海自然日的用量和额度。

## 本地运行

在本目录运行 `npm ci`，再运行 `npm run dev`。开发服务器默认将 `/v1/` 请求转发给 `http://127.0.0.1:8000`。可在启动前设置 `PROXY_CONTROL_URL` 指向本地控制面。

## 构建与部署

`npm run build` 会运行 TypeScript 检查并生成 `dist/`。在同一个 HTTPS 站点托管静态产物和以下控制台 API，API 转发保留路径和查询参数。前端页面路径回落到 `index.html`，未匹配的 API 返回错误状态。

| 方法 | 控制台 API |
| --- | --- |
| POST | `/v1/access/login` |
| GET | `/v1/access/tenants` |
| GET | `/v1/tenants/{tenant_id}/usage/daily` |
| GET | `/v1/tenants/{tenant_id}/balance` |

正式环境复用哪个 HTTPS 入口在部署时确认。参阅[开发规范](../../docs/standards/proxy-development-conventions.md#控制台技术栈)和[控制面 API](../../api/proxycontrol/v1/)。
