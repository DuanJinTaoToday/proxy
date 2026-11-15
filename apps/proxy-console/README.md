# proxy-console

控制台采用 Vue 3、TypeScript 和 Vite。客户从 `/` 注册或登录，查看钱包流水和订单。运营从 `/operations` 登录，按主账号名或手机号查找租户。具备 `manage_billing` 权限的运营可以核对并提交充值。 [首版商业交付 Spec](../../docs/features/proxy-first-release/proxy-first-release-spec.md)

## 本地运行

在本目录运行 `make init` 安装依赖，运行 `make run` 启动开发服务器，运行 `make build` 检查类型并打包到 `dist/`。开发服务器默认将 `/v1/` 请求转发给 `http://127.0.0.1:8000`。可在启动前设置 `PROXY_CONTROL_URL` 指向本地控制面。

## 本地构建与 API 路由

`npm run build` 会运行 TypeScript 检查并生成 `dist/`。在同一个 HTTPS 站点托管静态产物和以下控制台 API，API 转发保留路径和查询参数。前端页面路径回落到 `index.html`，未匹配的 API 返回错误状态。

| 方法 | 控制台 API |
| --- | --- |
| POST | `/v1/access/register` |
| POST | `/v1/access/login` |
| GET | `/v1/tenant/wallet`、`/v1/tenant/wallet/ledger`、`/v1/tenant/orders`、`/v1/tenant/orders/{order_id}` |
| POST | `/v1/operations/access/login`、`/v1/access/check` |
| GET | `/v1/operations/tenants`、`/v1/operations/tenants/{tenant_id}` 及其流水、订单查询子路径 |
| POST | `/v1/operations/tenants/{tenant_id}/wallet/recharge-quotes`、`/v1/operations/tenants/{tenant_id}/wallet/recharges` |

首版的客户部署需核对新版 API 契约和购买入口。参阅[开发规范](../../docs/standards/proxy-development-conventions.md#控制台技术栈)和[控制面 API](../../api/proxycontrol/openapi.yaml)。
