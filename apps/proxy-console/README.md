# proxy-console

一期控制台采用 Vue 3、TypeScript、Vite 和 Vue Router。租户成员登录后可以切换有权访问的租户，查询上海自然日的用量和额度。

## 本地运行

在本目录运行 `make init` 安装依赖，运行 `make run` 启动开发服务器，运行 `make build` 检查类型并打包到 `dist/`。开发服务器默认将 `/v1/` 请求转发给 `http://127.0.0.1:8000`。可在启动前设置 `PROXY_CONTROL_URL` 指向本地控制面。

## 提取 Mock 演示

在本地演示数据库应用控制面的 Goose 迁移。使用控制面配置示例中的 `alpha`、`beta` 租户和各自的管理员 `alice`、`bob`，填写本地密码摘要并启用用量数据库。向该数据库导入 [mock_supplier_usage.sql](../proxy-control/demo/mock_supplier_usage.sql)。

分别登录两个管理员账号，在「代理提取演示」输入数量并点击「提取代理」。首次导入当天，Alpha 的模拟日用量为 100 MB，Beta 为 200 MB。重跑同一 SQL，PostgreSQL 返回 `INSERT 0 0`，刷新控制台后两个租户的总量保持原值。样例日期取首次导入时的上海自然日。

## 构建与部署

`npm run build` 会运行 TypeScript 检查并生成 `dist/`。在同一个 HTTPS 站点托管静态产物和以下控制台 API，API 转发保留路径和查询参数。前端页面路径回落到 `index.html`，未匹配的 API 返回错误状态。

| 方法 | 控制台 API |
| --- | --- |
| POST | `/v1/access/login` |
| GET | `/v1/access/tenants` |
| GET | `/v1/tenants/{tenant_id}/usage/daily` |
| GET | `/v1/tenants/{tenant_id}/balance` |
| POST | `/v1/tenants/{tenant_id}/proxy-extractions` |

正式环境复用哪个 HTTPS 入口在部署时确认。参阅[开发规范](../../docs/standards/proxy-development-conventions.md#控制台技术栈)和[控制面 API](../../api/proxycontrol/openapi.yaml)。
