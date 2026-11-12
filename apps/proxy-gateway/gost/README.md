# GOST 网关启动和 Recorder 采集

网关 Go 进程负责本机认证中继、GOST 子进程和 Recorder 采集。将原版 GOST v3.3.3 与网关程序放在同一宿主机。用 `auth.example.yaml` 对接受控目标，用 `wukong.example.yaml` 对接一个已验证的悟空入口。

在 `apps/proxy-gateway/` 按开发规范准备本地配置并启动网关。下列字段填在 `server.local.yaml`。 [本地启动](../../../docs/standards/proxy-development-conventions.md#本地启动)

- `biz.gateway_id` 是运营预置的网关实例 ID。`server.http.addr` 是认证中继监听地址。
- `data.gost.binary`、`data.gost.config` 和 `data.repo.recorder.spool` 分别指向 GOST 可执行文件、配置模板和 Recorder 目录。
- `data.acl.control.url` 指向控制面的 HTTPS 地址。使用私有 CA 时填写 `data.acl.control.ca_file`。`data.acl.control.auth_token` 用于加载和刷新认证状态，`data.acl.control.report_token` 用于批次和生命周期上报。
- `data.gost.local_auth_token` 用于 GOST 到本机中继，对应模板的 `${LOCAL_AUTH_TOKEN}`。网关生成的临时 GOST 配置含凭据，进程退出时清理。
- 使用悟空模板时，将 `data.gost.config` 指向 `wukong.example.yaml`，填写 `data.gost.wukong` 下的 `address`、`username` 和 `password`。每次启动渲染一个 9 位数字 SID。当前单入口配置的静态上游由该网关的连接共用。

升级已有网关配置时，将原 `data.control` 和 `data.recorder` 两段分别移到 `data.acl.control` 和 `data.repo.recorder`。保持 `spool` 指向原有目录，以继续采集旧代次。

中继默认监听 `127.0.0.1:8091`。GOST 插件总超时为 500 ms，中继向控制面请求的总超时为 200 ms。凭据与额度分别从最近一次有效同步起最多使用 30 秒。积压时暂停额度续期和恢复追平由 #14 接续。

包装器为每次 GOST 启动独占一个 `<spool>/<boot_id>` 目录，预先创建 `sessions.jsonl`。采样与扫描默认每秒一次，批次上限 1,000 条和 1 MiB，HTTP 超时 3 秒。扫描和上报参数归 YAML 的 `biz.recorder`。重试退避从 1 秒增至 30 秒。当前代次每 5 秒报告心跳，退出时间持久化到 `exited_at`，控制面确认后记为 `exit.confirmed`。GOST 异常退出时包装器尝试报告退出并返回失败，外部进程管理器可据此重启网关。重启后继续采集旧代次。

控制面整批确认后才更新同目录的 `confirmed.offset`。非法 UTF-8 和已关闭代次的无换行残尾停在原偏移并留证，其他代次继续采集。磁盘使用率 80% 时告警，90% 时升级告警。网关保留 Recorder 文件供补报和追查。

真实悟空测试额外使用 `PROXY_GATEWAY_TEST_BINARY`，并读取控制面目录下的 `configs/file/wukong-test.local.yaml`，字段为 `address`、`username` 和 `password`。测试通过原版 GOST 访问 HTTPS 目标，再查询控制面真实上报的租户日用量。 [GOST 和 PostgreSQL 集成用例配置](../../../docs/standards/proxy-development-conventions.md#pr-检查与业务回归)
