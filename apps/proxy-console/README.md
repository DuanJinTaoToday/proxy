# proxy-console

控制台前端，面向租户与运营。目前该目录只有 [AGENTS.md](AGENTS.md)，尚未开始实现。

## 规划功能（一期）

- 租户：登录并选择租户，按日期范围查看每日上下行流量、计费流量、剩余额度与低额度提醒
- 运营：按租户、网关实例和时间段查询用量，查看可能的用量缺口，为租户充值额度并配置倍率与准入门槛

## 对接

后端为 [proxy-control](../proxy-control/README.md)，接口定义见 [`api/proxycontrol`](../../api/proxycontrol/v1/)。

## 文档

- [一期 Spec](../../docs/features/proxy-phase-one/proxy-phase-one-spec.md)
- [开发规范：应用分层与测试归属](../../docs/standards/proxy-development-conventions.md#应用分层与测试归属)
