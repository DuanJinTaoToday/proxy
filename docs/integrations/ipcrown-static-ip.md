# IPCrown 静态独享 IP 接口核查

IPCrown v2.0.0 文档提供节点库存查询、直接采购、订单详情及列表。下单成功的示例在同一响应内返回代理连接信息。文档未说明库存刷新延迟、独立锁库存或预留、客户端幂等键、异步交付回调及部分交付规则。2026-11-13 只读复核了供应商文档。2026-11-09 已验证生产目录接口；真实扣款、订单查询、交付和代理连通性仍待实测。 [CrownIP 接口文档](https://www.yuque.com/ludeng-4v9rr/egw6bz/lq4d0lxmbgb1izii)

## 库存与价格

- `POST /open/api/nodes` 传 `cid=200001` 查询独享产品。可按 `nid`、大洲、国家、省份、ISP 和代理分类筛选，分页默认 20 条、最多 100 条。
- 单个节点的 `id` 对应采购请求的 `nid`。`data[].total` 为节点全部数量，`data[].max` 为可用数量，文档以 `max > 0` 表示可下单。响应根级 `total` 是列表记录总数。
- `prices[]` 提供 `buytime`、`label`、`price_type` 与单位为元的 `price`。示例中的年卡金额与同页周期倍数说明存在差异，目录和实际采购金额的对应关系仍需验证。响应 `timestamp` 是接口响应时间戳，文档未提供独立的库存更新时间或库存延迟承诺。 [获取节点列表](https://www.yuque.com/ludeng-4v9rr/egw6bz/lq4d0lxmbgb1izii#QHLr2)

## 预留与采购

- 已公开的目录和采购接口没有独立的预留请求、库存锁令牌或有效期。供应商也提供指定 IP 列表和下单接口，列表给出资源标识，购买通过单独的下单请求提交，文档未描述查询后会锁住所选 IP。 [接口概览](https://www.yuque.com/ludeng-4v9rr/egw6bz/lq4d0lxmbgb1izii#LlSps) · [定制 IP 列表](https://www.yuque.com/ludeng-4v9rr/egw6bz/lq4d0lxmbgb1izii#KaElh) · [定制 IP 下单](https://www.yuque.com/ludeng-4v9rr/egw6bz/lq4d0lxmbgb1izii#VywGX)
- `POST /open/api/order/place` 必填 `cid=200001`、`list[].nid`、`list[].number` 和 `time`。静态独享从月卡起售，文档列出 `yp1`、`yp3`、`yp6`、`np`，并说明支持 `yp1` 到 `yp99`。可选项为 `pid`（独享可省略）、`filter_sids`、`ipsegment`、`p_user`、`p_pass`。鉴权使用 `X-Partner-ID` 和签名参数 `timestamp`、`nonce`、`sign`。重复 `nonce` 会被拒绝，文档没有承诺业务订单去重。 [下单购买](https://www.yuque.com/ludeng-4v9rr/egw6bz/lq4d0lxmbgb1izii#WiFKJ) · [签名认证](https://www.yuque.com/ludeng-4v9rr/egw6bz/lq4d0lxmbgb1izii#DxLxu) · [随机数要求](https://www.yuque.com/ludeng-4v9rr/egw6bz/lq4d0lxmbgb1izii#PgBZE)
- 业务错误含 `-4` 余额不足、`-5` 库存不足、`-8` 并发限制。文档明确采购、续费和调换不支持并发，并列出查询 60 次/分钟、下单 10 次/分钟及超限 HTTP 429。文档未说明并发限制按账户还是其他维度生效。 [业务错误码](https://www.yuque.com/ludeng-4v9rr/egw6bz/lq4d0lxmbgb1izii#MquN1) · [并发与频率限制](https://www.yuque.com/ludeng-4v9rr/egw6bz/lq4d0lxmbgb1izii#KQ87N)

## 订单查询与交付

- 下单成功示例给出 `data.order_id` 和 `data.list[]`，明细含订单明细 ID `id`、供应商服务器 ID `sid`、节点 `nid`、代理 ID `proxyid`、`ip`、`port`、`p_user`、`p_pass` 和到期时间 `stoptime`。同一响应含账户值 `integral`。成功示例支持在拿到响应后登记连接信息，实际交付时延、采购数量与返回明细的完整性仍需实测。 [下单购买](https://www.yuque.com/ludeng-4v9rr/egw6bz/lq4d0lxmbgb1izii#WiFKJ)
- `POST /open/api/order/detail` 按供应商返回的 `order_id` 查询。示例包含 `orderno`、金额 `total_rmb`、下单时间 `posttime`、到期时间和 `proxy_list[]`。代理明细包含连接信息，`status=1` 表示正常、`status=2` 表示退订。另有 `POST /open/api/ip/check-status`，可按 IP 查询供应商状态。代理连接仍需从实际接入路径验证。 [查询订单详情](https://www.yuque.com/ludeng-4v9rr/egw6bz/lq4d0lxmbgb1izii#Qz5Z2) · [检查 IP 状态](https://www.yuque.com/ludeng-4v9rr/egw6bz/lq4d0lxmbgb1izii#hkUuH)
- `POST /open/api/order/list` 按 `cid` 分页，可按供应商订单编号 `orderno`、日期 `stime` / `etime` 和 `pid` 筛选。若采购响应丢失且平台没有供应商订单标识，列表可用于人工排查，文档未提供按平台请求号唯一匹配的方法。由此推断，采购超时应视为结果未知，平台端幂等只能约束自身请求，采购补发前仍需确认供应商侧结果。 [订单列表](https://www.yuque.com/ludeng-4v9rr/egw6bz/lq4d0lxmbgb1izii#fknl8) · [下单购买](https://www.yuque.com/ludeng-4v9rr/egw6bz/lq4d0lxmbgb1izii#WiFKJ)
