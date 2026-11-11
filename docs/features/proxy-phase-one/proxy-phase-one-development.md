# Proxy 一期开发任务与验收顺序

一期 11 项任务、进度和阻塞关系以 [GitHub Issue #1 的子任务](https://github.com/DuanJinTaoToday/proxy/issues/1)为准。

先打通一个测试租户的真实隧道用量，再补足跨机器故障恢复、租户自查和运营追溯。API 提取和流量查询的 Mock 可以并行准备。 [一期 Spec](proxy-phase-one-spec.md) · [一期技术方案](proxy-phase-one-architecture.md)

联调从一个网关、一个本地采集器和一个批量 HTTP 接口开始。Mock 适配器和样本可并行准备，真实 API 联调按[悟空接口与数据清单](../../product/proxy-product-alignment.md#需要老板推动悟空提供的接口与数据)推进。

验收证据分两类。受控 SOCKS5 上游用于核对字段、累计差值、故障恢复和跨日。真实悟空入口用于验证部署后从客户端经网关到用量页面的全链路。直接连接悟空的实测可作为联调起点。[悟空隧道接入记录](../../integrations/wukong-socks5-tunnel.md)
