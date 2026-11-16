# #39 皇冠采购与续费契约验收

2026-11-16（上海时间）｜已完成测试采购及两条明细的批量续费成功路径；采购异常与续费混合结果待补。[工单 #39](https://github.com/DuanJinTaoToday/proxy/issues/39) · [首版 Spec #36](https://github.com/DuanJinTaoToday/proxy/issues/36) · [仓库接入资料](../../integrations/ipcrown-static-ip.md)

## 环境与实际只读结果

皇冠文档列出 `http://test.ipcrown.com` 为测试地址，仓库原型使用 `http://www.ipcrown.com` 查询正式目录，原型的成交仅为本地模拟。仓库记录 2026-11-09 验证过正式目录。初次探测时，进程、用户和机器环境均未配置原型所需的 `IPCROWN_PARTNER_ID` / `IPCROWN_API_SECRET`，未取得可测试的主订单 ID 和明细 ID。测试域名的余额隔离及扣费规则尚待确认。

随后使用用户提供的凭证签名查询静态独享目录。正式地址的 nodes-and-continents 返回 HTTP 200、code=0 和 20 个节点。测试地址的 nodes 及 nodes-and-continents 返回 HTTP 401、code=-401，后者明确提示合作伙伴 ID 无效或已被禁用。默认网络路径和直连结果一致。当前凭证已验证可读正式目录，测试账号仍待确认。[测试地址](https://www.yuque.com/ludeng-4v9rr/egw6bz/lq4d0lxmbgb1izii#hE93A)

本轮对测试地址的 `nodes`、`order/list`、`order/detail`、`user/info`，以及正式地址的前三个接口，均发出匿名 HTTP POST，只检查访问与鉴权。所有路径得到同一结果。两台主机的 HTTPS 探测均在 TLS 握手阶段失败，正式或共享凭证需要通过供应商确认的安全通道使用。

```text
POST /open/api/nodes
Content-Type: application/json

{}

HTTP 401
{"code":-401,"msg":"请在 Header 中传递 X-Partner-ID"}
```

`nodes`、`nodes-and-continents`、`order/list`、`order/detail`、`user/info`、`ip/check-status` 用于查询。`order/place` 和 `order/renew` 会变更交付或到期状态并可能扣减供应商余额。初次探测未调用写接口。公开文档另列出定制 IP 和按号段下单入口，均在本次测试范围之外。

### 皇冠研发提供的测试账号

使用新测试账号向 `http://test.ipcrown.com` 发起签名查询，三个接口均返回 HTTP 200、`code=0`。同一测试域名的 HTTPS 仍在握手阶段失败。未向正式地址发送新账号的请求，测试余额与正式账务的隔离及实际费用待皇冠确认。[研发提供的 Apipost 文档](https://docs.apipost.net/docs/detail/6dd2623fdc70000?target_id=1ce46b3e30c045&locale=zh-cn)

以下为同次只读实测的脱敏节选。省略签名、时间戳和无关字段。

```text
nodes 请求       {"cid":200001,"page":1,"limit":100}
nodes 响应节选   {"code":0,"total":20,"data":[{"id":3,"max":146,"prices":[{"price_type":"yp1","price":50}]}]}

user/info 请求   {}
user/info 响应   {"code":0,"data":{"integral":"0.00","price_config":[]}}
```

`user/info` 节选中的 `price_config` 仅保留 `cid=200001`、`price_type=yp1` 的条目，实际响应中没有满足条件的条目。

```text
order/list 请求  {"cid":200001,"pageIndex":1,"pageSize":20}
order/list 响应  HTTP 200，code=0，count=0
```

充值前没有可供详情查询或续费的静态独享订单。最低月卡报价为 50 元/IP，选择节点 3 一次采购 2 个 `yp1` 的目录合计为 100 元。若皇冠确认该账号只扣隔离测试积分，实际支出预计为 0 元。续费报价需单独确认。若续费也按 50／IP 计，采购及一次两项续费合计预计 200 测试账面单位，必要时追加一次单 IP 续费预计合计 250。上述为充值前的基线。皇冠随后确认测试余额与正式余额隔离，并补充测试额度。

## 采购

| 工单验收项 | 文档确认与证据缺口 |
| --- | --- |
| 报价和库存 | `nodes` 的 `id` 对应下单 `nid`，`max` 是可购量，节点内 `total` 是总量。按 `prices[].price_type` 选租期，记录 `buytime` 和精确十进制 `price`（元）。目录样例的年卡单价与周期倍数说明有差异。实际费用用 `detail.total_rmb` 和采购前后的 `user/info.data.integral` 核对，本次测试采购已核对订单金额与余额差额一致，其他情形待验证。 |
| 成功与交付 | `place` 示例返回 `data.order_id`、`data.list[]` 和余额 `integral`。用主订单 ID 查 `detail`，以明细 `id` 对齐 `proxy_list[]`，核对逐条 IP、端口、凭证及 `stoptime`。文档的 `status=1/2` 分别表示正常和退订。已用下文测试订单核对两条交付及金额。时间字段的供应商时区、代理实际连通性待验证。 |
| 复查后拒单 | 文档无库存预留接口，明确列出 `code=-5`（库存不足）。没有实测复查与下单之间的库存竞争，也没有验证该码在多行请求中是否保证零交付和零扣费。请供应商提供受控库存变更或脱敏拒单及余额样例。 |
| 部分交付 | 可用请求量与按 `nid` 分组、以明细 `id` 去重后的交付计数观察数量差，已知主订单 ID 时继续查 `detail`。当前没有证据能从数量差判定剩余项已经终结。请供应商提供部分交付及后续查单样例，平台受控测试覆盖已交付部分立即可用、剩余金额待确认及退款。 |
| 超时或结果未知 | 已知主订单 ID 查 `detail`。响应丢失时 `list` 仅能以 `cid`、日期、`orderno` 等缩小排查范围，文档无平台请求号查询或业务幂等键。匹配不唯一时交运营核实，避免补发。用受控响应分别模拟未送达和上游已受理但响应丢失，确认本地都先待核实。 |

[目录](https://www.yuque.com/ludeng-4v9rr/egw6bz/lq4d0lxmbgb1izii#QHLr2) · [采购](https://www.yuque.com/ludeng-4v9rr/egw6bz/lq4d0lxmbgb1izii#WiFKJ) · [订单详情](https://www.yuque.com/ludeng-4v9rr/egw6bz/lq4d0lxmbgb1izii#Qz5Z2) · [订单列表](https://www.yuque.com/ludeng-4v9rr/egw6bz/lq4d0lxmbgb1izii#fknl8) · [用户信息](https://www.yuque.com/ludeng-4v9rr/egw6bz/lq4d0lxmbgb1izii#ZRNEG) · [错误码](https://www.yuque.com/ludeng-4v9rr/egw6bz/lq4d0lxmbgb1izii#MquN1)

### 测试采购实测

用户授权在节点 3 的 `yp1` 单价为 50 元时采购 2 个 IP。提交前复核 `max=146`、`price=50`，账户 `integral="2000.00"`，静态独享订单数 0。仅调用一次 `order/place`。以下为同一笔实际测试采购的脱敏节选，主订单和明细 ID 使用前后一致的代号，省略实际 IP、连接凭证和签名。

```text
place 请求    {"cid":200001,"time":"yp1","list":[{"nid":3,"number":2}]}
place 响应    HTTP 200，code=0，order_id=ORDER-A，integral=1900
             list=[{id:ITEM-A,nid:3,port:43000,stoptime:"2026-12-16 11:04:45"},
                   {id:ITEM-B,nid:3,port:43000,stoptime:"2026-12-16 11:04:45"}]
list 响应     HTTP 200，code=0，count=1，包含 ORDER-A
detail 请求   {"order_id":"ORDER-A"}
detail 响应   HTTP 200，code=0，order_id=ORDER-A，total_rmb="100.00"
             posttime="2026-11-16 11:04:45"，proxy_list 含 ITEM-A 和 ITEM-B
             两项 status=1，starttime="2026-11-16 11:04:45"，stoptime="2026-12-16 11:04:45"
user/info    HTTP 200，code=0，integral="1900.00"
```

下单与详情的两条明细按 ID 核对，IP、端口、账密、节点及到期字段一致，并且是两个不同 IP。请求量 2、确认交付量 2。订单金额 100.00 与测试余额差额 100.00 一致。到期字段显示比开通时间晚 30 天，该字段未带时区。此次仅验证供应商交付数据，尚未实拨代理。两条明细随后用于下文的批量续费测试。

## 续费

`renew` 支持 `cid=200001`、`ids[]` 及统一的 `time`。成功示例的 `data[]` 每条仅展示明细 `id` 和新 `stoptime`，未给出逐项失败原因、续费操作编号或金额。公开接口中未找到独立的续费交易查询。可按原主订单 ID 查详情，并按原明细 ID 比对续费前后 `ip` 和 `stoptime`。下文实测确认批量两项均成功时，原订单详情能立即读回同一 IP 和顺延后的到期时间。缺少明细或到期未变化时仍需核实；读回原详情也无法单独证明某笔续费的扣费归属。[续费接口](https://www.yuque.com/ludeng-4v9rr/egw6bz/lq4d0lxmbgb1izii#dR0Q9) · [订单详情](https://www.yuque.com/ludeng-4v9rr/egw6bz/lq4d0lxmbgb1izii#Qz5Z2)

**#49 首版采用皇冠批量续费。** 两项全成功时，续费响应和原订单详情均能按 ID 核对保号与顺延。单 IP 串行备选与混合结果实测留待后续。接口返回不完整、超时或扣费有疑问时，将无法确认的明细保留待核实并交运营；期间禁止对同一 IP 重复提交。运营依据原订单详情和供应商核实结果逐项处理，未确认明细沿用 #36 的 24 小时转运营、72 小时关单退款规则。

Apipost 的续费失败示例返回 `code=400`、`data=[]`，仅展示整次请求的失败形态。文档未提供混合批次中每个失败 ID 的状态或扣费，也未提供独立续费查单接口。其描述对续费 6 小时限制的表述与语雀不同，两个文档均无起算实测证据。首版仍按 #36 仅接受到期前提交。[Apipost 续费接口](https://docs.apipost.net/docs/detail/6dd2623fdc70000?target_id=1cf1fc79b0c7ff&locale=zh-cn)

### 批量续费前只读复核

按原主订单查详情，两条测试明细均在节点 3、状态为 1，原到期字段均为 `2026-12-16 11:04:45`。订单的 `total_rmb="100.00"` 是历史采购金额，明细未返回续费价格。当前测试余额为 `1900.00`，目录的节点 3 月卡采购报价仍为 50 元/IP。账户信息未返回适用于 `cid=200001` 的 `yp1` 专属价格。已检查皇冠提供的 Apipost 文档，续费相关接口只有会提交交易的 `order/renew`，未找到只读续费报价接口，也未找到续费直接使用同节点目录价的说明。

### 批量续费实测

用户授权直接使用隔离测试额度，以续费前后余额差核对实扣。按原订单 ORDER-A 查回两条明细并保存原 IP 和到期时间，随后仅提交一次批量续费。以下为脱敏实测节选，沿用采购明细代号。

```text
user/info 前   HTTP 200，code=0，integral="1900.00"
renew 请求     {"cid":200001,"ids":["ITEM-A","ITEM-B"],"time":"yp1"}
renew 响应     HTTP 200，code=0
              data=[{id:ITEM-A,stoptime:"2027-01-15 11:04:45"},
                    {id:ITEM-B,stoptime:"2027-01-15 11:04:45"}]
detail 请求    {"order_id":"ORDER-A"}
detail 响应    HTTP 200，code=0，proxy_list 含 ITEM-A 和 ITEM-B
user/info 后   HTTP 200，code=0，integral="1800.00"
```

| 明细 | IP 比对 | 原到期 | 查单新到期 | 顺延 | 查单状态 |
| --- | --- | --- | --- | --- | --- |
| ITEM-A | 保留原 IP | 2026-12-16 11:04:45 | 2027-01-15 11:04:45 | 30 天 | 1 |
| ITEM-B | 保留原 IP | 2026-12-16 11:04:45 | 2027-01-15 11:04:45 | 30 天 | 1 |

两项查单到期时间均与续费响应一致，按原到期时间顺延 30 天。全批实际扣减 100.00 测试额度，两条均成功，平均 50.00/IP。响应和查单未提供逐条续费扣费字段，因此这笔实测仅确认批次总额。此次未制造混合成功或超时，也未执行单 IP 续费。

## 脱敏契约样例

以下为根据官方文档重编的字段样例，并非同一笔实测交易。省略鉴权、响应时间戳和无关字段。标识、地址、时间和金额均为示意，金额不可作为当前报价。

```text
nodes 请求       {"cid":200001,"nid":101}
nodes 响应       {"code":0,"data":[{"id":101,"max":3,"prices":[{"buytime":1,"price_type":"yp1","price":10}]}]}

place 请求       {"cid":200001,"list":[{"nid":101,"number":1}],"time":"yp1"}
place 成功示例   {"code":0,"data":{"order_id":201,"list":[{"id":301,"nid":101,"ip":"192.0.2.10","port":1080,"p_user":"[redacted]","p_pass":"[redacted]","stoptime":"2026-12-16 10:00:00"}]}}
place 拒单示意   {"code":-5,"msg":"库存不足"}

detail 请求      {"order_id":201}
detail 响应      {"code":0,"data":{"order_id":201,"total_rmb":"10.00","proxy_list":[{"id":301,"ip":"192.0.2.10","stoptime":"2026-12-16 10:00:00","status":1}]}}

renew 单项请求   {"cid":200001,"ids":[301],"time":"yp1"}
renew 成功示例   {"code":0,"data":[{"id":301,"stoptime":"2027-01-15 10:00:00"}]}
```

## #48 / #49 实现边界

- **#48** 报价、正常采购、按主订单 ID 查详情、逐明细交付及测试余额扣费已在本次成功路径验证。提交前复核报价和库存，按已确认交付明细结算，按主订单 ID 复查并以明细 ID 去重。异常交易的余额与订单金额对应关系、缺货原子性、未交付项终态和无引用恢复均需补证，未知时保持待核实。
- **#49** 可按原明细 ID 建立独立锁价及结算项，保留续费前 IP、到期时间和原主订单 ID。核对成功后更新同一个分配。未知期间禁止对同一 IP 重复提交。本次验证了批量全成功时的逐项查单、保号和顺延，以及批次总成本。批量混合结果的逐项失败和扣费归属仍待验证。#49 提交前需逐 IP 锁定当前续费价，现有资料尚未确认续费的只读报价来源或目录价等价规则。批次平均扣费只能用于对账，续费报价口径由 #49 确认并在 #52 验收。

## 后续验收

采购和批量续费的正常路径已通过。首版采用已验证的批量续费路径，异常订单交运营核实。采购异常、续费混合结果、逐项扣费及单 IP 串行实测仍有证据缺口。需要复测时，优先请皇冠提供受控样例或同一隔离账号下可查的失败明细。首版商业链路开售验收归 #52。
