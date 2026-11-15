# 住宅代理竞品功能尽调

对照辣椒 HTTP、711Proxy 和 kookeey，重点覆盖动态住宅 GB 与静态住宅 IP。辣椒 HTTP 用于参考客户购买、提取和到期管理，711Proxy 用于参考凭证限额及企业流量分发，kookeey 补充套餐组合及开发者 API。记录来自 Chrome 实际页面和官方公开文档。价格为页面快照，供比较套餐结构。

## Chrome 已登录控制台走查

走查日期为 2026-11-13。以下“看见”表示页面出现对应入口、字段或说明。账户未持有可用于本次验证的付费资源，支付、发票开具、真实流量扣减、静态 IP 交付和续费结果均未实测。

| 用户任务 | 辣椒 HTTP | 711Proxy | kookeey |
| --- | --- | --- | --- |
| 从首页选择商品 | 首页把静态按个数、动态常规/企业、获取代理、帮助中心和钱包充值放在导航。静态、动态可分别进入购买和接入页面。[官网](https://www.lajiaohttp.com/) | 首页按商品拆分动态 GB 与静态住宅，提供价格与资源入口；FAQ 解释高级流量分配、静态续期和认证方式。[官网](https://www.711proxy.com/zh-CN) | 首页提供定价、代理产品、登录和试用入口，强调动态轮转与静态 ISP。[官网](https://www.kookeey.com/) |
| 买动态流量并开始连接 | 套餐中心有常规和企业两档。控制台分账密模式和 API 白名单模式。账密页显示余量和有效期，提供地区、会话类型、粘性时长 1–120 分钟、协议及连接示例。API 页有生成链接和白名单列表。[套餐](https://center.lajiaohttp.com/coral/BuyPackage) · [API 白名单](https://center.lajiaohttp.com/coral/flow-white-model) | 常规流量页合并余量、购买、兑换、账密、API、地区、会话类型和代理配置生成。账密可叠加来源 IP 白名单。高级流量页面要求先分配给账户或生成 CDKey，再转入常规流量使用。[常规 GB](https://dashboard.711proxy.com/zh-CN/residential-regular-gb) · [账户分配](https://dashboard.711proxy.com/zh-CN/residential-enterprise-assign) · [CDKey](https://dashboard.711proxy.com/zh-CN/residential-enterprise-ip) | GB 套餐卡同时展示流量、期限、独享端口数、独立子账密数和优惠券下单入口。动态产品页分购买、提取、独享端口和流量统计。独享端口在动态商品内，页面仍展示 GB 余量。[动态控制台](https://www.kookeey.net/clientarea/#/dynamic/buy) |
| 管理凭据和流量分发 | 观察到动态白名单与独立 CDK 页。CDK 可填写分配 GB、生成条数，列表有兑换账号及记录入口。已走查页面未证实控制台成员邀请或角色权限。[CDK](https://center.lajiaohttp.com/coral/exchangecdk) | 子账户页说明系统生成代理用户，客户可新增连接代理的用户。列表包含总流量上限/已用、每日流量上限/已用及状态。高级账户分配页显示已分配和已消耗 GB。未证实这些代理子账户可以登录控制台或拥有财务角色。[子账户](https://dashboard.711proxy.com/zh-CN/sub-user) · [账户分配](https://dashboard.711proxy.com/zh-CN/residential-enterprise-assign) | 套餐卡确认独立子账密数，成员登录、组织隔离及财务角色权限未证实。[动态控制台](https://www.kookeey.net/clientarea/#/dynamic/buy) |
| 买到并管理静态 IP | 套餐页展示原生/广播 IP、国家地区、7/30/90 天、每 IP 报价。控制台显示可提取余量，提示购买后在提取区选国家、城市和数量，另有 IP 段筛选。列表支持搜索、批量编辑/删除/续费、自动续费开关、续费记录、导出、到期前 5 天短信提醒。[套餐](https://center.lajiaohttp.com/coral/BuyPackage) · [静态控制台](https://center.lajiaohttp.com/coral/static-account-model) | 控制台以“我的代理 IP”为中心，展示国家、剩余天数、到期时间、分配时间、类型、状态、自动续费和备注。支持批量续费、备注、导出，页面提示钱包充值可用于自动续费。页面同时有“可恢复 IP”指标和“到期后无法续期”提示，恢复资格及能否保留同 IP 待核实。[静态控制台](https://dashboard.711proxy.com/zh-CN/static-residential-proxies) | 购买表单有国家、独享、交付协议、带宽、UDP、租期、数量和余额自动续费选项，静态 ISP 与静态数据中心各有入口。[静态购买](https://www.kookeey.net/clientarea/#/static/buy) |
| 查用量和交易 | 动态页有低流量短信阈值；统计页有日期与使用流量 MB，导航有订单发票和问题反馈。[数据统计](https://center.lajiaohttp.com/coral/DataTotal) | 常规 GB 数据统计提供更新时间和下载报表，静态页有续费数量折扣，导航有钱包和账户入口。[常规 GB](https://dashboard.711proxy.com/zh-CN/residential-regular-gb) · [静态控制台](https://dashboard.711proxy.com/zh-CN/static-residential-proxies) | 动态页有剩余流量、端口用量、动态流量统计和提醒设置，购买页有余额入口和优惠券文案。优惠券的适用范围、叠加和退款后退回规则未实测。[动态控制台](https://www.kookeey.net/clientarea/#/dynamic/buy) |

## 对 proxy 最终形态的建议

这些是尽调时记录的设计选项。首版范围见[已确认的产品决策](proxy-first-release-decisions.md)。

| 决策 | 建议 |
| --- | --- |
| 首页与客户控制台 | 首页只呈现两个商品，分别讲清计费单位、可购地区、交付方式、定价和使用教程，主按钮进入对应商品购买或接入页。控制台按两条客户任务组织。动态侧是余量、接入配置、用量及提醒；静态侧是选规格购买、我的 IP、到期和续费。订单、发票、优惠券及成员设置放在共享区域。平台运营侧沿用独立授权范围，保留按租户追溯。 |
| 动态住宅 GB | 先确定客户怎么买和怎么扣，包括套餐规格、流量有效期、多个流量包扣减顺序、余额不足和统计延迟。隧道与 API 提取可作为同一商品的两种接入方式，分别展示计量来源。真实直连流量能否归到租户，取决于悟空用量数据的租户标识和粒度。 |
| 静态住宅 IP | 优先评估按地区、类型、数量与租期购买。当前对齐文档已选皇冠下单后交付 IP，控制台和接口查询已购 IP。辣椒的“先买可提取数量，再选库存提取”可作为对照，只有确认客户确实需要延迟分配或多次提取时，再评估是否增加这一业务。到期提醒可先规划，保号续费、自动续费及恢复需先验证皇冠能力。 |
| 权限 | 把租户、控制台成员、代理凭证、提取 API 访问分别建模。租户持有流量、IP 和订单；成员权限决定谁能管理资源、购买和看账；代理凭证可按应用分发、禁用、限额和追踪；提取 API 走既有产品授权。竞品页面提供的是连接身份和用量管理的参考，组织角色需按自家客户协作场景定。 |
| 优惠、赠送和分发 | 区分订单优惠券、套餐赠送 GB、静态续费阶梯折扣、钱包余额和 CDK 流量分发。先定义单商品结算与入账，再决定是否上优惠券。首版如需要促销，可先验证限定商品、有效期、单次使用、退款回退与防重复核销。客户有真实跨账户分发需求时，再讨论企业流量池及 CDK。 |

建议评审优先定三件事。第一，动态两种接入方式是否共用可消费余额，以及如何向租户展示两条用量来源。第二，静态下单后直接交付的库存、缺货、部分交付和续费责任。第三，首批客户需要的是多个可限额的代理凭证，还是多个能分别登录控制台的成员。这三项决定账户、权益和控制台结构，优惠玩法可以在这些边界确定后再排期。

## 官方公开资料补充

### 动态住宅按 GB

| 供应商 | 小额起购和常见档位 | 大额和企业档位 | 有效期和分发 |
| --- | --- | --- | --- |
| 711Proxy | 30 天页面快照显示 5 GB US$15、40 GB US$56、100 GB US$117、250 GB US$215。100 GB 卡片同时显示 US$0.98/GB，按总价除以容量为 US$1.17/GB，需以结算核实。 | 企业 525 GB 实到 US$400、1,050 GB US$680、2,100 GB US$1,300、5,250 GB US$3,000。5,250 GB 实算约 US$0.57/GB，US$0.55/GB 属于询价口径。 | 常规支持 30、90、180 天，到期冻结，购买新 GB 包可解冻。企业未分配流量长期有效，直接分配给成员的包有效 90 天，也可生成 CDKey。 [常规定价](https://www.711proxy.com/pricing/regular/residential-proxies-gb)、[企业定价](https://www.711proxy.com/pricing/enterprise/residential-proxies-gb)、[有效期 FAQ](https://docs.711proxy.com/faq/residential-proxies/how-much-do-rotating-residential-proxies-cost)、[企业计划](https://docs.711proxy.com/features/enterprise-plan) |
| 辣椒HTTP | 官方案例页较新快照显示 2 GB ¥19.80、10 GB ¥99、40 GB ¥352、100 GB ¥690、200 GB ¥1,000，均为 60 天。较早快照同页显示 2 GB 首购 ¥24/常规 ¥45、10 GB ¥160、40 GB ¥396、100 GB ¥760，结算价待核。 | 较新快照显示 500 GB ¥2,400（60 天）、1 TB ¥4,500 和 2 TB ¥8,800（90 天）、3 TB ¥12,000 和 5 TB ¥17,500（永久）。较新宣传最低 ¥3.5/GB，较早版本显示 ¥3.8/GB。 | 企业级支持 CDK，永久有效与具体规格绑定。正式购买页静态内容可见结算框和 FAQ，未显示当前完整档位，表中价格取案例页快照。 [官方案例页](https://www.lajiaohttp.com/cases/proxy_land/)、[动态购买页](https://www.lajiaohttp.com/package/residential/) |
| kookeey | 1 GB ¥38（30 天）、10 GB ¥280（180 天）、100 GB ¥2,000（365 天）。 | 1 TB ¥15,360（页面称长期有效）、5 TB ¥51,200（不限时长）。定价按 1 TB = 1,024 GB 折算为 ¥15/GB、¥10/GB。 | 档位卡列出 10 个独立子账密，支持定制上限。公开 API 中的套餐期限示例与零售定价页存在差异，应核对线上零售和分发是否使用相同套餐及期限。 [动态定价](https://www.kookeey.com/pricing/residential-proxies)、[API 文档](https://www.kookeey.com/apidoc) |

### 静态住宅按 IP

| 供应商 | 价格和期限 | 选购和续费 |
| --- | --- | --- |
| 711Proxy | 公示最低 US$3.50/IP，官方文章将其写为 US$3.50/IP/月，同一产品页 FAQ 另有 US$5/月/IP 最低充值的描述。公开下单组件提供 30 天和 90 天；可售国家、类型及各行价格需在购买器确认。 | 独享、原生或广播住宅资源，可按地区选购。公开价格页列出 5–9 / 10–49 / 50–199 / 200–499 个 IP 分别减 2% / 5% / 10% / 20%，500 个起询价。支持手动和自动续费，文档说明到期前续订可延续原 IP。 [静态产品页](https://www.711proxy.com/static-residential-proxies)、[静态定价](https://server.711proxy.com/pricing/regular/static-residential-proxies)、[每月价格口径](https://www.711proxy.com/blog/data-collection-stalls-residential-proxy-setup.html)、[自动续费](https://docs.711proxy.com/proxies/static-residential-proxies/manage-auto-renewal) |
| 辣椒HTTP | 7、30、90 天，按国家、原生/广播和时长报价。静态页面按 7、30、90 天呈现多组价格，前两组示例为美国原生 ¥15.8 / ¥29.8，日本原生 ¥19.8 / ¥39.8；建议在可交互购买器确认时长映射。页面另有任意地区 ¥9.9 的活动入口，适用范围待核。 | 公开购买页标明独享固定住宅 IP、不限流量，支持在后台管理已购线路。 [静态购买页](https://www.lajiaohttp.com/static_isp/)、[静态使用教程](https://www.lajiaohttp.com/help_center/detail/7/) |
| kookeey | 静态 ISP 按 IP 包月且不限流量。公开定价组件美国/SOCKS5/关闭 UDP 的默认合计显示 ¥25，旁边有 ¥10 平台无门槛券提示，页面文本无法确认时长和数量默认值及扣券前后关系。API 文档以 30 个自然日为标准月。 | 购买器展示国家、协议、独享、UDP、时长、数量，部分协议和参数按线路适用。控制台支持启停、续订和自动续订，线路级续订设置优先于系统设置。 [静态 ISP 定价](https://www.kookeey.com/pricing/isp-proxies)、[线路管理](https://help.kookeey.com/docs/jolv40)、[API 文档](https://www.kookeey.com/apidoc) |

### 结算、权限与 API

#### 711Proxy

- 动态 GB 的文档流程为登录控制台、选择流量包、核对订单和付款；文档列出信用卡、加密货币、PayPal、支付宝、钱包余额。静态从控制台进入定价，按地区和数量配置订单，核对时长和总价；文档列出信用卡、加密货币、支付宝和钱包余额。结算可提交优惠码，账户页可管理优惠券和查看交易。 [动态下单](https://docs.711proxy.com/proxies/residential-proxies-gb/editor)、[静态下单](https://docs.711proxy.com/proxies/static-residential-proxies/order-proxies)、[优惠券](https://docs.711proxy.com/account-settings/coupons)、[账户后台](https://docs.711proxy.com/account-settings/account-dashboard)
- 代理子用户持独立账密，可设 GB/MB/KB 或 IP 次数限额，支持启停、改密和删除，删除会归还未用配额。动态统计可按子用户和时间筛选，文档称一般每小时更新。主账号还可设余额提醒和主、子账号合计日限额。 [子用户](https://docs.711proxy.com/proxies/proxy-configuration-parameters/sub-users)、[用量统计](https://docs.711proxy.com/proxies/residential-proxies-gb/interactive-blocks)、[用量设置](https://docs.711proxy.com/proxies/residential-proxies-gb/images-and-media)
- 企业成员通过专属邀请链接注册后可被分配流量，CDKey 可按量生成并查看兑换状态。动态接入另有白名单提取 API 或子用户账密生成代理字符串；文档称提取列表是代理接入地址，实际出口需要通过代理访问验证。公开资料未确认程序化购买、子用户 CRUD、企业分发或用量查询 API。 [企业计划](https://docs.711proxy.com/features/enterprise-plan)、[企业项目](https://www.711proxy.com/enterprise-program)、[动态代理配置](https://docs.711proxy.com/proxies/residential-proxies-gb/markdown)

#### 辣椒 HTTP

- 公开结算框展示套餐、数量、原价、优惠、实付，静态还展示国家及有效期。提供优惠码输入和优惠券选择，列出支付宝、微信、余额三种支付方式，订单支付时限 60 分钟。动态使用教程要求实名认证。条款默认限制活动优惠叠加，明确允许的活动例外。 [动态购买页](https://www.lajiaohttp.com/package/residential/)、[静态购买页](https://www.lajiaohttp.com/static_isp/)、[动态使用教程](https://www.lajiaohttp.com/help_center/detail/3/)、[服务协议](https://www.lajiaohttp.com/terms-service/)
- 动态账密可建子账号并设流量限额，获取代理时选择子账号、地区、协议、轮换或粘性会话。案例页展示周期报表、分项流量、筛选及导出，并宣传流量告警和续费管理。子账号上限、用量更新时间、导出字段及 CDK 回收规则待确认。 [动态使用教程](https://www.lajiaohttp.com/help_center/detail/3/)、[后台介绍](https://www.lajiaohttp.com/cases/proxy_land/)
- 公开 API 文档覆盖 IP 白名单提取、国家或地区、数量、轮换或粘性、HTTP(S)/SOCKS5、TXT/JSON。官方示例中的域名及部分参数拼写存在差异，应以控制台生成链接为准。公开资料未确认订单、用量、子账号和 CDK 管理 API。 [API 文档](https://www.lajiaohttp.com/help_center/detail/20/)、[账密控制台](https://center.lajiaohttp.com/coral/flow-account-model)、[白名单控制台](https://center.lajiaohttp.com/coral/flow-white-model)

#### kookeey

- 静态公开购买器展示报价和优惠券。官方手册列出支付宝和微信充值，线路支付后在已购线路明细显示处理状态和接入信息。动态可购买流量后配置代理，也可生成独享动态线路。实名认证页称认证权益包括流量、券和折扣，并注明未认证仍可使用基础服务，港澳台动态资源需认证。 [静态定价](https://www.kookeey.com/pricing/isp-proxies)、[使用说明](https://help.kookeey.com/docs/B5EL4Z)、[生成动态线路](https://help.kookeey.com/docs/thXQmn)、[实名认证](https://www.kookeey.com/verification)
- 签名 API 文档覆盖子账号列表、静态地区库存及价格、下单和支付确认、动态余额、流量分发及用量查询。静态线路支付后异步分配。动态分发以临时单加确认执行，扣母账号流量并增加子账号流量。文档列出 `/tdetail` 按天或小时查用量和子账号筛选、`/tdetailex` 导出 CSV、`/package` 查余额与到期时间。对接前需核实 reseller 开通条件、分发范围、订单幂等及报表延迟。 [API 文档](https://www.kookeey.com/apidoc)、[服务协议](https://www.kookeey.com/legalterms)
