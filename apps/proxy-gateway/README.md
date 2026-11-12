# proxy-gateway

网关服务（Go）。包装并管理原版 GOST v3.3.3 子进程，为租户提供 SOCKS5 接入并转发至上游供应商（一期为悟空）。负责：

- 本机认证中继：GOST 认证插件调用中继，中继向控制面校验代理凭据与额度
- 管理 GOST 子进程与配置渲染
- Recorder 文件采集：读取 GOST 写入的会话记录，批量上报控制面，整批确认后才推进本地偏移，支持重启后补报
- 上报网关生命周期（心跳、退出）

## 目录

| 路径 | 说明 |
| --- | --- |
| `cmd` | 入口与 Wire 装配 |
| `internal/biz`、`service`、`server`、`data`、`pkg` | 分层实现 |
| `internal/conf` | 配置 Protobuf 与生成代码 |
| `configs` | `init.yaml` 与 `file/server.yaml` 配置模板 |
| `gost` | GOST 配置模板（`auth.example.yaml`、`wukong.example.yaml`） |
| `integration` | GOST 集成用例 |

## 构建与启动

需要原版 GOST v3.3.3，并与网关部署在同一宿主机。在本目录执行：

```sh
cp configs/file/server.yaml configs/file/server.local.yaml   # 填写网关 ID、控制面地址与令牌、GOST 路径等
cp configs/init.yaml configs/file/init.local.yaml            # source.file 指向 ./configs/file/server.local.yaml
make build
./bin/proxy-gateway -conf ./configs/file/init.local.yaml
```

Windows 下可执行文件带 `.exe` 后缀。字段含义见 [GOST 网关启动和 Recorder 采集](gost/README.md)。

## 常用命令

| 命令 | 说明 |
| --- | --- |
| `make build` | 构建到 `bin/proxy-gateway` |
| `make config` | 生成配置 Protobuf 代码 |
| `make wire` | 重新生成依赖注入 |
| `make lint` | 运行 golangci-lint |

## 文档

- [GOST 网关启动和 Recorder 采集](gost/README.md)
- [悟空 SOCKS5 隧道接入](../../docs/integrations/wukong-socks5-tunnel.md)
- [GOST 文件 Recorder](../../docs/integrations/gost-file-recorder.md)
- [开发规范](../../docs/standards/proxy-development-conventions.md)
- [AGENTS.md](AGENTS.md)
