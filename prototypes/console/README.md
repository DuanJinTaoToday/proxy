# 控制台交互原型

在仓库根目录安装控制台依赖后启动原型预览服务。

```sh
npm --prefix apps/proxy-console ci
npm --prefix apps/proxy-console exec -- vite --config vite.prototypes.config.mjs
```

在 Vite 输出的本地地址打开对应页面。

- `/tenant-console-prototype.html` 预览租户控制台布局和示例账户状态。
- `/my-ips-prototype.html` 预览静态 IP 列表和详情。
- `/subaccounts-prototype.html` 预览代理子账号创建流程。

原型使用演示数据，独立于正式控制台的生产构建。

## 客户控制台高保真原型

`static-residential-ip/` 是独立的 Vue 3 + Element Plus 工程，覆盖客户控制台全部侧栏页面：仪表板、动态住宅代理-GB、静态住宅代理、代理子账号、钱包和我的账户，以及顶栏的通知、语言和账号菜单。余额、订单、流水、IP 和子账号共用一份浏览器内状态，购买、续费会实时写入订单和流水。

```sh
npm --prefix prototypes/console/static-residential-ip ci
npm --prefix prototypes/console/static-residential-ip run dev
```

打开 `http://127.0.0.1:4188/`。网址后加 `?ip=cn` 可预览中国大陆网络下的提示。`npm run build` 会先做类型检查。

静态 IP 的地区、库存、租期和价格来自皇冠正式 API 的目录快照，存于 `src/catalog.ts`。重新查询后把快照写入仓库根目录 `.scratch/ipcrown-catalog.json`，再运行 `pwsh prototypes/console/static-residential-ip/scripts/gen-catalog.ps1` 重新生成。快照和生成文件均不含凭证。动态套餐价格、网关地址和用量为示例值。
