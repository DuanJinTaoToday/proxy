# 本地 Docker 测试环境

## 启动与更新

启动 Docker Desktop 的 Linux 容器引擎，确认 `make`、`docker compose` 可用，并且本机的 `8443` 和 `1080` 端口空闲或已由该环境占用。在仓库根目录运行：

```sh
make test-env
```

同一命令用于首次搭建和日常更新。它从当前工作区构建镜像，启动 PostgreSQL 并等待就绪，运行 Goose 迁移和幂等初始化，再更新服务并等待健康检查。构建会使用当前文件内容，包括未提交的代码改动。

## 镜像和容器命名

Compose 项目名为 `proxy-test`。项目服务的镜像使用 `proxy-<组件>:<版本>`，当前版本为 `0.0.1`。容器使用 `<组件>-1`。容器间通过 **Compose 服务名** 访问，修改容器显示名称时可保留内部地址。

| Compose 服务 | 镜像 | 容器 | 职责 |
| --- | --- | --- | --- |
| `control` | `proxy-control:0.0.1` | `control-1` | 控制面 API |
| `console` | `proxy-console:0.0.1` | `console-1` | Caddy 提供 HTTPS、托管前端并转发 API |
| `gateway` | `proxy-gateway:0.0.1` | `gateway-1` | SOCKS5 接入、凭据校验、Recorder 上报 |
| `proxy-wukong-mock` | `proxy-wukong-mock:0.0.1` | `wukong-mock-1` | 使用原版 GOST v3.3.3 模拟悟空 SOCKS5 转发 |
| `db` | `postgres:17` | `proxy-test-db-1` | 测试数据库 |
| `migrate` | `proxy-test-migrate:latest` | Compose 自动命名 | 执行 Goose 和初始化 SQL，结束后移除容器 |

## 访问和联调

- 控制台访问 `https://localhost:8443`。测试账号为 `alice`（Alpha）、`bob`（Beta）和 `operator`（运营），初始密码均为 `proxy-local-test`。
- SOCKS5 访问 `127.0.0.1:1080`。由运营 API 为测试租户充值，再用该租户账号发放 SOCKS5 凭据。通过代理访问 `http://console:8080/fixture` 可验证容器内转发，响应体为 `proxy-test`。
- HTTPS 使用 Caddy 测试 CA。浏览器需要信任时，从 Docker Desktop 的 `console-1` 容器导出 `/data/caddy/pki/authorities/local/root.crt`，导入当前用户的受信任根证书。网关通过挂载的 CA 校验 `https://console`。

## 搭建与维护约定

- 在 [Compose 配置](../../deploy/test/compose.yaml) 定义服务、依赖、健康检查和数据卷，在 [Dockerfile](../../deploy/test/Dockerfile) 定义各服务的构建阶段。Caddy、控制面、网关和 GOST 模板都在 `deploy/test/`，修改后通过 `make test-env` 构建并应用。
- 控制台和 SOCKS5 入口只绑定宿主机回环地址。控制面、模拟悟空和数据库使用 Docker 网络通信。测试配置使用本地专用账号、令牌和 CA。
- 数据库结构通过 Goose 迁移维护。初始化 SQL 负责创建测试网关和令牌摘要，重复执行可保留已有记录。PostgreSQL 数据、网关 Recorder 文件和 Caddy CA 分别存放在 `db`、`recorder` 和 `caddy` 命名卷，日常更新和停止服务均保留这些卷。

在 Docker Desktop 的 **Containers** 中查看 `proxy-test`。也可以从仓库根目录运行：

```sh
docker compose -f deploy/test/compose.yaml ps
docker compose -f deploy/test/compose.yaml logs --tail 100
docker compose -f deploy/test/compose.yaml stop
```
