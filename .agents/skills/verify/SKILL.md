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
| 运营充值 → 租户看到到账 | 租户从钱包流水读取余额。运营查找同一租户，核对租户名、主账号、10.00 元金额、预计余额和本轮唯一原因，确认后获取流水号。租户刷新两次，每次核对余额增量、该编号的唯一流水以及流水上的金额和余额。 | [wallet-recharge.spec.mjs](../../../tests/verify/scenarios/wallet-recharge.spec.mjs) |
| 动态套餐开售 → 充值后购买 | 运营给 5 GB 定价并单独开售动态套餐。租户第一次购买时看到差额，订单和余额无变化。运营充值后，租户重新确认。刷新后核对钱包扣款、到账流量、唯一订单和关联流水。 | [traffic-purchase.spec.mjs](../../../tests/verify/scenarios/traffic-purchase.spec.mjs) |

新 Issue 优先复用已有场景。验收项覆盖新行为时，在 `tests/verify/scenarios/` 增加或扩展对应的用户路径，通过页面或公开接口核对业务效果。真实供应商、网关或其他依赖仅在相应场景实际接入并运行后计为已验证。

## 失败与交付

从输出的 Evidence 目录打开 `report/index.html`，定位第一个失败步骤，对照实际页面、响应和 Issue 预期。部署失败时查看终端中首个失败的构建或迁移命令。修复产品、验收脚本或环境中的具体问题，再运行同一命令重新构建、部署并复测受影响场景。保留原来的断言依据。遇到确实缺少的依赖或业务口径冲突，继续验收可执行部分并记录阻断。

运行结束后，启动器清理本轮 Compose 容器、卷和本次构建的镜像。证据保存在工作区 `.scratch/verify/<运行 ID>/`。交付时逐条列出验收条件、预期、实测和结论，链接报告及对应截图或业务结果。引用 `run.json` 中的提交、工作区状态、镜像和清理结果。发生过失败时说明修复内容与最终复跑结果。
