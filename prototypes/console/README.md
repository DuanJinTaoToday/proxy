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
