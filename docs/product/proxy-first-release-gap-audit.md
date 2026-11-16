# Proxy 首版商业交付差异审计

审计基线为本地 `main` 的 `ab6f39e`，以[已确认的首版产品决策](proxy-first-release-decisions.md)为业务口径。以下对照旧票的验收条款、仓库代码及已有验收记录。本页保留发布前的实现差距和处置建议。现行交付范围见[首版商业交付 Spec](../features/proxy-first-release/proxy-first-release-spec.md)。旧 #1 的关闭及双向关联已在 [#36 发布](https://github.com/DuanJinTaoToday/proxy/issues/36)时完成。

## 旧票 → 基线实现 → 首版差异

| 旧票 | 基线实现与证据 | 首版影响和后续工作 |
| --- | --- | --- |
| [#1 一期 Spec](https://github.com/DuanJinTaoToday/proxy/issues/1) | 审计时为 Open，带 `ready-for-agent`。目标是隧道计量、额度、提取 Mock 和静态 IP 建模。 | **替换**。新 Spec 覆盖两类商品、钱包、运营及各自开售验收。已由 #36 取代，双向关联后移除就绪标签并关闭。 |
| [#2 登录与授权](https://github.com/DuanJinTaoToday/proxy/issues/2) | [Access 契约](https://github.com/DuanJinTaoToday/proxy/blob/ab6f39eb455da45b2b92ba48e1e4c2bdf42561f1/api/protos/proxycontrol/v1/access.proto)支持登录、查询租户及鉴权，基线使用预置用户和成员关系。 | **复用鉴权边界，新增开户流程**。规范化 +86 手机号并确保唯一，注册、租户、唯一管理员和零余额一起初始化。控制台密码至少 12 位，忘记密码由运营核实后重置。首版客户流程收敛到自己的租户，保留运营权限隔离。 |
| [#3 隧道凭据](https://github.com/DuanJinTaoToday/proxy/issues/3) | [Credentials 契约](https://github.com/DuanJinTaoToday/proxy/blob/ab6f39eb455da45b2b92ba48e1e4c2bdf42561f1/api/protos/proxycontrol/v1/credentials.proto)只有发放和停用。后端生成账密，密码仅发放时返回，认证使用哈希与稳定 clientID。 | **改造**。复用稳定身份及认证同步，补子账号命名、固定 10 个、备注、启停、设置及查看密码、累计上限。代理密码需新增可恢复的受控存储与租户授权，避免日志和网关同步泄露明文。改密和禁用仅影响同步后的新连接，旧连接继续计量。累计上限判断与历史归属均绑定稳定子账号身份。 |
| [#4 入库](https://github.com/DuanJinTaoToday/proxy/issues/4)、[#5 Recorder 上报](https://github.com/DuanJinTaoToday/proxy/issues/5)、[#8 回放](https://github.com/DuanJinTaoToday/proxy/issues/8) | [技术方案与实现入口](../features/proxy-phase-one/proxy-phase-one-architecture.md)已有可信身份归属、文件水位、批次入库、幂等补报、归档和可能缺口识别。 | **复用链路，改计费和聚合维度**。总字节按 1:1 扣减，子账号累计用量和按日实扣流量要覆盖补报与归档。已有 Recorder 输入输出字段可继续用于内部诊断。复跑重复提交、跨日、失败连接、异常退出和子账号归属。 |
| [#6 租户日用量](https://github.com/DuanJinTaoToday/proxy/issues/6) | [用量查询](https://github.com/DuanJinTaoToday/proxy/blob/ab6f39eb455da45b2b92ba48e1e4c2bdf42561f1/apps/proxy-control/internal/biz/usage/query.go)与 [控制台](https://github.com/DuanJinTaoToday/proxy/blob/ab6f39eb455da45b2b92ba48e1e4c2bdf42561f1/apps/proxy-console/src/UsageDashboard.vue)已有租户日用量和剩余额度。 | **改造**。默认整租户，按日期、子账号筛选，含禁用账号。展示按日实际扣减流量、更新时间，按当前筛选导出 CSV。统一十进制 GB。低余额提醒改为管理员设阈值、每轮触发一次，余额回到阈值以上后重新触发。 |
| [#7 运营追溯](https://github.com/DuanJinTaoToday/proxy/issues/7) | [用量/异常契约](https://github.com/DuanJinTaoToday/proxy/blob/ab6f39eb455da45b2b92ba48e1e4c2bdf42561f1/api/protos/proxycontrol/v1/usage.proto)已有运营查询、异常和缺口 API，旧验收包含授予及倍率追溯。 | **复用追溯，新增商业运营入口**。保留计量对账及权限隔离。运营支持按主账号名或手机号查租户、看钱包及订单；计费管理角色负责钱包充值、套餐维护和开售开关。 |
| [#9 API 提取 Mock](https://github.com/DuanJinTaoToday/proxy/issues/9) | 审计时控制台含「代理提取演示」，控制面含可调用的提取路由。 | **退出客户首版**。删除页面、路由、实现和测试数据。 |
| [#10 分机验收](https://github.com/DuanJinTaoToday/proxy/issues/10) | 有[真实悟空双租户和受控故障验收记录](../features/proxy-phase-one/issue-10-cross-host-acceptance.md)。[网关路由](https://github.com/DuanJinTaoToday/proxy/blob/ab6f39eb455da45b2b92ba48e1e4c2bdf42561f1/apps/proxy-gateway/internal/data/wukong/route.go)按租户分配上游 SID，入口池已有健康检查。 | **复用出口池，改造会话映射并重验**。默认会话和显式 SID 均按子账号隔离，认证与 Recorder 归属使用同一稳定身份。现行口径按 [#53](https://github.com/DuanJinTaoToday/proxy/issues/53) 验收 sidtime 透传、同一会话的入口选择及入口不可用时的处理。实际时长、到期处理和出口 IP 由悟空决定，证据缺口见 [#40 验收记录](../features/proxy-first-release/issue-40-wukong-sidtime-acceptance.md)。普通接入可独立验收。 |
| [#11 容量基线](https://github.com/DuanJinTaoToday/proxy/issues/11) | [devbox 报告](../features/proxy-phase-one/issue-11-devbox-capacity.md)中 500 条已建会话达标，1,000 条首轮 P99 为 462.01 ms，超过 250 ms 阈值。 | **保留基线，目标环境复测**。沿用集中关闭时新连接成功率、P99、final 完整性三项口径，复核认证与计量变更后的结果。 |
| [#12 静态 IP 和流量领域模型](https://github.com/DuanJinTaoToday/proxy/issues/12) | 有[领域模型](../features/proxy-phase-one/proxy-phase-one-billing-domain.md)、[皇冠接入资料](../integrations/ipcrown-static-ip.md)与[模拟购买原型](https://github.com/DuanJinTaoToday/proxy/blob/ab6f39eb455da45b2b92ba48e1e4c2bdf42561f1/prototypes/ipcrown/server.py)，基线应用未找到静态订单、交付或续费入口。 | **新增商业链路**。复用报价/库存查询经验，补提交复核、锁价预扣、部分交付和实际数量结算、钱包退款、逐 IP 到期提醒与 TXT 导出、到期前单个/批量保号续费。异常交运营，保留可核实的订单和款项状态。 |
| [#14 授予、倍率和准入](https://github.com/DuanJinTaoToday/proxy/issues/14) | [Billing 契约](https://github.com/DuanJinTaoToday/proxy/blob/ab6f39eb455da45b2b92ba48e1e4c2bdf42561f1/api/protos/proxycontrol/v1/billing.proto)的充值是流量授予；[额度](https://github.com/DuanJinTaoToday/proxy/blob/ab6f39eb455da45b2b92ba48e1e4c2bdf42561f1/apps/proxy-control/internal/biz/usage/account.go)已支持超额与入账抵欠，[网关](https://github.com/DuanJinTaoToday/proxy/blob/ab6f39eb455da45b2b92ba48e1e4c2bdf42561f1/apps/proxy-gateway/internal/biz/authrelay/relay.go)按配置门槛判断。 | **复用抵欠、事务及同步，替换商业入口和准入规则**。首版按余额严格大于 0 且子账号已启用、未达累计上限准入。覆盖网关同步、冷缓存、断连恢复及 0 字节边界。客户购买的流量长期有效；固定 1:1，无上线前业务用量迁移。 |

#2–#12、#14 均已关闭，可作为实现与证据索引。上表后续工作由新 Spec 派生差量票。

## 没有对应旧实现的首版工作

- **钱包和订单**：新增独立金额余额及流水，计费运营手动充值，记操作人与变动前后值。流量订单锁价、扣钱包、授予流量和订单成功须一起生效；余额不足返回差额，不生成待支付订单。静态订单需要可核对预扣、逐项结算与退款。现有流量流水可供设计参考，金额与流量各自记账。
- **商品和客户入口**：新增动态套餐配置及操作记录，默认 5/50/200 GB、一单一份。新增官网报价、登录后恢复购买选择、账户概览、代理接入配置、统一订单记录、钱包流水和「我的 IP」。两类商品各自验收，由计费运营分别开售。
- **皇冠真实交易**：开售前须验证报价库存复核、真实购买和代理连通、保号续费与从原到期时间顺延、扣退款对账。批量先验证逐 IP 结果，如无法逐项确认，再验证单 IP 串行续费和独立结算。24/72 小时的异常处理由运营执行。

## Spec 切换时的文档处置记录

| 文档 | 切换计划 |
| --- | --- |
| 旧一期 Spec、一期简报和旧产品对齐（见基线 `ab6f39e`） | 合并仍有效的口径与供应商约束，换掉旧范围。旧 Spec 按仓库约定从工作树移除，可从 Git 历史追溯。 |
| [技术方案](../features/proxy-phase-one/proxy-phase-one-architecture.md)、[计费领域模型](../features/proxy-phase-one/proxy-phase-one-billing-domain.md)、[ADR 0001–0005](../adr/README.md) | 标出被替代的成员模型、只保存凭据哈希、倍率、授予到期、可配置准入门槛及协议条款。保留计量事实、回放、审计和认证故障边界；新 ADR 引用具体被取代条款。 |
| [开发任务与验收记录](../features/proxy-phase-one/proxy-phase-one-development.md)、[#10 分机记录](../features/proxy-phase-one/issue-10-cross-host-acceptance.md)、[#11 压测](../features/proxy-phase-one/issue-11-devbox-capacity.md) | 保留历史环境、结果和复跑入口。将旧任务顺序标为历史，新的验收案例覆盖首版规则。 |
| [AGENTS](../../AGENTS.md)、[README](../../README.md)、[开发规范](../standards/proxy-development-conventions.md)、[控制台 AGENTS](https://github.com/DuanJinTaoToday/proxy/blob/ab6f39eb455da45b2b92ba48e1e4c2bdf42561f1/apps/proxy-console/AGENTS.md)、[术语](../../GLOSSARY.md)、[产品决策](proxy-first-release-decisions.md) | 切换现行入口、定义和交叉链接，同时检查控制台 README、接口示例及部署配置中的旧用法。开发规范中的测试和发布要求继续适用。 |

## 后续开发顺序建议

先打通开户、权限、运营钱包和商品开售状态。动态链路接着完成「钱包购买 → 流量到账 → 子账号接入 → 实扣与恢复」的端到端切片，再补查询导出、提醒和可选的 `sidtime`。静态链路可并行验证皇冠契约，钱包就绪后再串通报价、部分交付与手动续费。两条链独立验收。

本轮基于代码静态核对和已存验收记录，未重跑集成测试、压测或供应商交易。
