# GOST 本机认证中继

部署时从 `auth.example.yaml` 渲染原版 GOST 配置，填入本机令牌和部署所需的监听、上游参数。GOST 的 SOCKS5 handler 必须引用配置中唯一注册的 HTTP 认证器。让网关 Go 进程用 `-gost-config` 校验并使用与 GOST 相同的已渲染配置，再启动 GOST。

- `PROXY_GATEWAY_CONTROL_URL` 指向控制面的 HTTPS 地址。
- `PROXY_GATEWAY_LOCAL_AUTH_TOKEN` 仅用于 GOST 到本机中继。渲染后的配置含本机令牌，应限制读取权限。
- `PROXY_GATEWAY_AUTH_TOKEN` 是网关向控制面加载、刷新状态的逐网关令牌。
- `PROXY_GATEWAY_REPORT_TOKEN` 是采集器上报的独立令牌。

中继默认监听 `127.0.0.1:8091`。GOST 插件总超时示例为 500 ms，中继向控制面请求的总超时为 200 ms。凭据与额度分别从最近一次有效同步起最多使用 30 秒。额度快照还受最近的授予生效或到期时间约束。

中继通过本机令牌保护的 `GET /v1/gateway/auth/metrics` 提供认证结果、缓存命中和延迟区间计数。真实 Recorder 上报随 #5 联调，积压时暂停额度续期和恢复追平随 #14 验收。
