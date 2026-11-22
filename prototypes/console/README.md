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

`static-residential-ip/` 是独立的 Vue 3 + Element Plus 工程，覆盖客户控制台全部侧栏页面：仪表板、动态住宅代理-GB、静态住宅代理、代理子账号、钱包和我的账户，以及顶栏的通知、语言和账号菜单。余额、订单、流水、IP 和子账号共用一份状态，购买、续费会实时写入订单和流水。

```sh
npm --prefix prototypes/console/static-residential-ip ci
npm --prefix prototypes/console/static-residential-ip run dev
```

打开 `http://127.0.0.1:4188/`。网址后加 `?ip=cn` 可预览中国大陆网络下的提示。`npm run build` 会先做类型检查。

同一工程的 `/operator.html` 是运营后台原型：工作台（确认中的静态订单核实与关闭退款）、租户查找与详情（钱包充值、重置登录密码）、订单、钱包流水、商品与定价（开售开关、流量包调价和上下架）和用量异常。顶栏可切换「计费管理」和「客服运营」演示角色，后者只能查看资金相关页面。

两个入口共用一份演示数据，保存在浏览器 localStorage，在两个标签页分别打开时会实时同步，例如运营充值后客户钱包立即到账、下架某档流量包后客户价格页随之隐藏。运营后台账号菜单里的「重置演示数据」恢复初始状态。

同一工程的 `/home.html` 是官网首页原型，供评审首屏方案。网址参数 `?variant=D|A|B|C` 切换方案，默认 D 组合方案：在连续深色底上依次讲选型、接入和查账管理，地图首屏接一组动态与静态的双栏价格对比，C 的实时终端做接入主画面，再用一张控制台局部展示用量和子账号，画面数据均标为示例。A、B、C 保留为单独方案（数据冲击、双商品分屏、实时控制台）供对照，`&hero=map|globe` 切换点阵地图或可拖拽地球动效，底部黄色切换条和键盘 ←、→ 也能切换。切换条的「开售」直接改写共用的商品开售开关，评审后请切回「两个都开售」。首屏数字、客户评价和 Telegram 联系方式为编造或占位，集中在 `src/home/home-demo.ts`，上线前替换。

静态 IP 的地区、库存、租期和价格来自皇冠正式 API 的目录快照，存于 `src/catalog.ts`。重新查询后把快照写入仓库根目录 `.scratch/ipcrown-catalog.json`，再运行 `pwsh prototypes/console/static-residential-ip/scripts/gen-catalog.ps1` 重新生成。快照和生成文件均不含凭证。动态套餐价格、网关地址和用量为示例值。
