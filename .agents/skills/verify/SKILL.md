---
name: verify
description: Proxy 实现 Issue 的交付前验收，由 implement 或自然语言实现请求触发。按工单条件部署、执行用户流程、修复复跑并交付证据。
---

# Proxy 端到端验收

从 Issue 和父 Spec 摘出本票承诺的行为。逐条对应用户角色、操作入口、前置条件、可观察结果和执行环境。沿用已确认的预期。依赖下游的条目按[跨 Issue 验收](../../../docs/agents/issue-tracker.md#跨-issue-验收)标明执行票。

## 部署与运行

按验收条件选择入口。控制台行为用 Playwright 从登录走到业务结果。API、网关或命令行行为从对应调用方入口实测。为每条运行时路径部署本票 worktree 的代码及该路径需要的服务。纯文档或静态产物按票面约定检查交付物，记录检查结果。

控制台验收在待交付的 worktree 根目录执行以下命令。需要 Docker Linux 引擎、支持 `!override` 的 Compose（2.24.4 或更新）和 Node.js 22.12+。首次使用或依赖锁文件更新时先安装依赖和浏览器。

```sh
npm --prefix tests/verify ci
npm --prefix tests/verify run browser:install
npm --prefix tests/verify test
```

最后一条命令创建本次专用的 Compose 项目，从当前工作区构建控制台、控制面和迁移镜像，初始化 PostgreSQL，再从随机本地 HTTPS 端口运行 Playwright。项目名、数据卷和浏览器会话随本轮创建。可用 `docker info` 检查引擎，运行期间用 `docker ps --filter label=com.docker.compose.project=<本轮项目名>` 核对容器。浏览器须从正常入口登录。

## 场景

| 路径 | 操作与判断依据 | 自动化 |
| --- | --- | --- |
| 租户管理代理子账号 | 从客户控制台创建子账号，核对账号名和密码校验、服务端重名提示、SOCKS5 用户名及创建时间。编辑备注、设置和调高上限、取消限制，修改并复制密码，确认启用和禁用后搜索、筛选、刷新核对。禁用账号计入 10 个名额。验证列表加载失败后重试和窄屏布局。 | [console-subaccounts.spec.mjs](../../../tests/verify/scenarios/console-subaccounts.spec.mjs) |
| 前端骨架 → 路由、双语与版式 | 访问 `/` 进入中文首页，直接打开控制台和运营深链显示对应布局与菜单，页签可后退，未知路径显示未找到页。控制台切换语言保持 URL 并在刷新后保留，官网切换到对应语言路由。在 390 px 与 1280 px 下检查两种语言无横向溢出，窄屏抽屉导航可用。 | [console-skeleton.spec.mjs](../../../tests/verify/scenarios/console-skeleton.spec.mjs) |
| 注册、登录与会话 | 未登录访问控制台跳到登录页并带回跳；账密错误有提示，登录后回到原页，刷新保持登录。账号菜单显示主账号名和脱敏手机号，退出需确认，退出后控制台再次要求登录。新号码注册后直接进入控制台，站外回跳被拒；同号码（带或不带 +86）重复注册被拒。伪造凭据被拒绝时清理会话并回到登录页。 | [console-auth.spec.mjs](../../../tests/verify/scenarios/console-auth.spec.mjs) |
| 运营登录、权限与充值到账 | 正常登录运营深链接，核对退出和过期回跳，分别检查只读、计费和账号权限。计费运营通过两种查询找到租户，核对金额和预计余额，提交后丢弃首次回执并重试同一请求。独立客户浏览器刷新两次核对余额增量和唯一流水。 | [ops-auth.spec.mjs](../../../tests/verify/scenarios/ops-auth.spec.mjs)、[wallet-recharge.spec.mjs](../../../tests/verify/scenarios/wallet-recharge.spec.mjs) |
| 动态代理接入 | 从空状态进入子账号管理，创建并选择启用的账号。核对主机、端口、账密、连接串和 curl 命令的显示与复制。检查默认会话、SID 校验和中英文教程。 | [console-dynamic-setup.spec.mjs](../../../tests/verify/scenarios/console-dynamic-setup.spec.mjs) |
| 动态套餐开售 → 充值后购买 | 通过运营 API 配置报价、开售并充值。从租户控制台核对已上架档位、5 GB 档下架后的折扣、余额不足拦截和过期报价拒绝。成交后用租户 API 核对流量、订单和钱包流水。 | [traffic-purchase.spec.mjs](../../../tests/verify/scenarios/traffic-purchase.spec.mjs) |
| 购买结果未知 → 重试原提交 | 将首笔请求转发给实际服务，成交后丢弃浏览器回执。关掉原标签页并重新打开购买页，点击「重试原提交」。核对原请求、同一订单、一次扣款和一次流量到账。 | [traffic-purchase-retry.spec.mjs](../../../tests/verify/scenarios/traffic-purchase-retry.spec.mjs) |

新 Issue 优先复用已有场景。验收项覆盖新行为时，在 `tests/verify/scenarios/` 增加或扩展对应的用户路径，通过页面或公开接口核对业务效果。真实供应商、网关或其他依赖仅在相应场景实际接入并运行后计为已验证。

## 失败与交付

从输出的 Evidence 目录打开 `report/index.html`，定位第一个失败步骤，对照实际页面、响应和 Issue 预期。部署失败时查看终端中首个失败的构建或迁移命令。修复产品、验收脚本或环境中的具体问题，再运行同一命令重新构建、部署并复测受影响场景。保留原来的断言依据。遇到确实缺少的依赖或业务口径冲突，继续验收可执行部分并记录阻断。

运行结束后，启动器清理本轮 Compose 容器、卷和本次构建的镜像。证据保存在工作区 `.scratch/verify/<运行 ID>/`。交付时逐条列出验收条件、预期、实测和结论，链接报告及对应截图或业务结果。引用 `run.json` 中的提交、工作区状态、镜像和清理结果。发生过失败时说明修复内容与最终复跑结果。
