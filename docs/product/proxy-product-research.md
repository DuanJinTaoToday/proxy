# 多供应商代理产品尽调

调研日期为 2026-11-09。以 711Proxy、kookeey 为直接对照，以 Oxylabs、Webshare、Bright Data 验证其他市场形态。以下为公开产品文档和定价页观察，未登录验证下单，也未对供应商做连通性或质量实测。

## 市场产品形态

| 样本 | 商品和计费 | 客户如何接入 | 官方资料 |
| --- | --- | --- | --- |
| 711Proxy | 动态住宅同时有按 GB 和按成功提取 IP 的套餐。另有每日 IP 额度、可续期的静态住宅 IP，以及不限量套餐。 | 动态住宅 GB 提供白名单 API 取 `host:port`，也提供账密配置。文档明确说明列表中的 IP 是代理入口地址，实际出口另计。 | [GB 接入](https://docs.711proxy.com/cn/dai-li-chan-pin/dong-tai-zhu-zhai-dai-li-gb/markdown) · [按 IP 定价](https://www.711proxy.com/pricing/regular/residential-proxies-ip) · [商品分类](https://docs.711proxy.com/faq/general/what-types-of-ips-can-i-buy) |
| kookeey | 动态住宅与移动代理按流量售卖。静态 ISP 和静态数据中心按 IP 和租期售卖。 | 动态住宅支持提取与端口模式。手册中的部分提取配置使用固定网关和带地区、轮换参数的账密，另有 API 白名单流程。静态购买后交付线路列表。 | [定价](https://www.kookeey.com/pricing) · [安全策略](https://help.kookeey.com/docs/GT9LvG) · [API 提取示例](https://help.kookeey.com/docs/ying-dao-RPA-pei-zhi-shi-yong-jiao-cheng) · [静态线路](https://help.kookeey.com/docs/rdNEjv) |
| Oxylabs | 提供住宅、移动、机房及 ISP 等代理网络。独享 ISP 按所购 IP 分配资源。 | 住宅可用会话参数尝试维持出口。独享 ISP 可以按端口指定某个 IP，也能用轮换端口在已分配的 IP 池中切换。 | [会话控制](https://developers.oxylabs.io/help-center/products-and-features/how-to-use-session-control) · [独享 ISP](https://developers.oxylabs.io/help-center/getting-started/start-using-dedicated-isp-proxies) |
| Webshare | 动态住宅按 GB。静态代理套餐可配置 IP 数、流量和独享类型等。 | 提供 Direct、Rotating 和 Backbone。固定入口可以对应动态出口，静态代理列表也可以搭配轮换入口。 | [连接形态](https://help.webshare.io/en/articles/8375305-understanding-proxy-connection-types-direct-rotating-and-backbone) · [动态住宅](https://help.webshare.io/en/articles/15517620-what-are-the-rotating-residential-proxies) · [套餐配置](https://help.webshare.io/en/articles/16919504-how-can-i-change-my-proxy-plan) |
| Bright Data | 动态住宅有按 GB 即用即付与按月用量包。静态 ISP 同时可按 IP 或按 GB 购买。 | 住宅定价页列有地区筛选、会话控制和控制台 API，并注明经 Proxy Manager 支持 SOCKS5。 | [动态住宅定价](https://brightdata.com/pricing/proxy-network/residential-proxies) · [ISP 定价](https://brightdata.com/pricing/proxy-network/isp-proxies) |

## 对自有产品的含义

产品目录应区分四件事。

1. **出口资源**包括住宅、移动、ISP 和数据中心。共享或独享、国家或城市覆盖是资源属性。
2. **出口行为**包括自动轮换、限时粘性和固定出口。入口固定只说明客户端配置可以长期使用。对粘性时长、掉线、重连和续费后的出口变化，需要分别约定。
3. **交付方式**包括固定隧道入口、批量提取 API、控制台生成或导出代理配置。提取接口应写清交付的是可连接配置、限时绑定，还是独立分配的固定出口。部分客户工具可能只认裸 `IP:port`，应拿真实客户工具验证域名和账密兼容性。
4. **计费口径**包括按 GB、按有效分配次数、按 IP 和租期、按时间或端口订阅。按 GB 仍可提供提取接口。按 IP 则要明确重复提取、部分成功、到期和提前失效的处理。不限量套餐也要确认带宽、并发和公平使用约束。

建议优先验证一个**动态代理流量包**，先用同一套产品权益比较固定隧道入口和 API 批量交付的客户需求。已有客户明确需要批量处理限时独立线路时，再单独验证按次提取商品。静态独享需要可持续供货、续费保号和失效补偿，可作为另一条商品线评估。这些是候选方向，尚待客户需求和供应成本验证。

## 多供应商产品承诺

| 客户侧看到什么 | 内部需要掌握什么 |
| --- | --- |
| 平台域名、平台凭据、统一 API、稳定错误语义和统一账单 | 供应商接入地址、认证方式、产品标识、采购成本、库存和故障原因。内部可按租户、线路及供应商追溯 |
| 出口类型、地区、独享性、会话模式和有效期 | 哪些供应商实际符合每项承诺。候选不足时展示不可用或按已说明规则处理 |
| 轮换商品的入口和使用方式稳定 | 新连接可选择符合条件的其他供应商。固定出口、独享出口或尚在有效期内的粘性会话，要遵守原有出口承诺，切换导致出口变化时按约定报告或更换 |
| 自有计费单位及账单明细 | 分开保留客户计量和供应商成本。以两种口径对账，核算失败、重试、上游计量差异和闲置固定资源对毛利的影响 |

屏蔽的是供应商的接入参数和采购关系。目标站点仍能看到实际出口 IP，也可能通过 ASN 等信息推断网络归属。产品可以承诺客户接入配置统一，以及在符合商品条件的出口之间调度。

## 下一轮尽调

| 核实项 | 要拿到的证据 |
| --- | --- |
| 首批客户到底买什么 | 收集真实代理配置样例，确认要的是大量轮换、短期保号、长期独享，还是批量导入的便捷性。用真实客户端测试隧道和提取样例 |
| 每个供应商的能力和替代边界 | 对私有部署及拟接入的第三方分别记录实际协议、入口和出口格式、授权方式、地区、TTL、粘性、断线行为、限额和用量查询。选同一产品条件验证至少两路供给 |
| 按候选商品算成本 | 取得可比的采购报价和计量定义，用样本流量或提取任务计算单位成本，计入转发成本、上游失败收费、退款及余量过期 |
| 转售和售后责任 | 核实合同是否覆盖自有品牌转售、下游用途、地域、滥用处置与补偿。kookeey 公布专属转售 API，与推广返佣分列。711Proxy 公布企业转售项目，其企业条款同时要求转售获得事先书面许可。以实际签约条款为准。 [kookeey 合作伙伴](https://www.kookeey.com/partners/detail) · [711Proxy 企业项目](https://server.711proxy.com/enterprise-program) · [711Proxy 企业条款](https://enterprise.711proxy.com/policy/TermsOfService.html) |
