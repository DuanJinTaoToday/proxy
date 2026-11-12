# GOST 网关启动和 Recorder 采集

网关 Go 进程负责本机认证中继、GOST 子进程和 Recorder 采集。将原版 GOST v3.3.3 与网关程序放在同一宿主机。用 `auth.example.yaml` 对接受控目标，用 `wukong.example.yaml` 对接一个已验证的悟空入口。

```sh
proxy-gateway -gost-binary ./gost -gost-config ./wukong.example.yaml -spool ./spool
```

- `PROXY_GATEWAY_ID` 是运营预置的网关实例 ID。
- `PROXY_GATEWAY_CONTROL_URL` 指向控制面的 HTTPS 地址，私有 CA 可通过 `PROXY_GATEWAY_CONTROL_CA_FILE` 提供。
- `PROXY_GATEWAY_LOCAL_AUTH_TOKEN` 仅用于 GOST 到本机中继。网关生成的临时 GOST 配置含凭据，进程退出时清理。
- `PROXY_GATEWAY_AUTH_TOKEN` 仅用于向控制面加载、刷新认证状态。
- `PROXY_GATEWAY_REPORT_TOKEN` 仅用于批次和生命周期上报。
- 悟空模板使用 `WUKONG_ADDRESS`、`WUKONG_USERNAME` 和 `WUKONG_PASSWORD`。每次启动渲染一个 9 位数字 SID。当前单入口配置的静态上游由该网关的连接共用。

中继默认监听 `127.0.0.1:8091`。GOST 插件总超时为 500 ms，中继向控制面请求的总超时为 200 ms。凭据与额度分别从最近一次有效同步起最多使用 30 秒。积压时暂停额度续期和恢复追平由 #14 接续。

包装器为每次 GOST 启动独占一个 `<spool>/<boot_id>` 目录，预先创建 `sessions.jsonl`。采样与扫描默认每秒一次，批次上限 1,000 条和 1 MiB，HTTP 超时 3 秒，可通过命令行配置。重试退避从 1 秒增至 30 秒。当前代次每 5 秒报告心跳，退出时间持久化到 `exited_at`，控制面确认后记为 `exit.confirmed`。GOST 异常退出时包装器尝试报告退出并返回失败，外部进程管理器可据此重启网关。重启后继续采集旧代次。

控制面整批确认后才更新同目录的 `confirmed.offset`。非法 UTF-8 和已关闭代次的无换行残尾停在原偏移并留证，其他代次继续采集。磁盘使用率 80% 时告警，90% 时升级告警。网关保留 Recorder 文件供补报和追查。

可选验收环境变量为 `PROXY_GOST_TEST_BINARY`、`PROXY_GATEWAY_TEST_BINARY`、`PROXY_CONTROL_TEST_DSN` 和上面的 `WUKONG_*`。原版 GOST 测试覆盖受控的成功、空连接、拒绝、超时和策略拒绝。悟空测试通过原版 GOST 访问 HTTPS 目标，再查询控制面真实上报的租户日用量。
