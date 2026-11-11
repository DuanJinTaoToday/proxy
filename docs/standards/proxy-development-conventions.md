# 大仓代码组织与开发规范

本文记录一期已确认的工程约定。产品范围与验收标准见[一期 Spec](../features/proxy-phase-one/proxy-phase-one-spec.md)，部署和计量链路见[一期技术方案](../features/proxy-phase-one/proxy-phase-one-architecture.md)，实施顺序见[开发任务](../features/proxy-phase-one/proxy-phase-one-development.md)。

## 已确认

### 仓库与目录

使用一个 Git 仓库和一个根目录 Go module。控制面、网关和控制台按部署单元放在 `apps/`，各自保有构建和交付入口。以下是目标布局，目录和包随对应任务建立。

```text
proxy/
  go.mod
  api/
    proxycontrol/
      v1/
        reporting.proto
        usage.proto
        # 同目录存放生成的 Go 绑定
  apps/
    proxy-control/
      cmd/
        main.go
      bin/                 # 构建产物
      configs/
      internal/
        conf/
          protos/
        data/
          model/
        server/
        proxyauth/
        usage/
    proxy-gateway/
      collector/
        main.go
        internal/
      gost/
      bin/                 # 按目标平台存放两个二进制
      dist/                # 按目标平台存放发布归档
    proxy-console/
  pkg/
  docs/
  prototypes/
```

| 位置 | 职责 |
| --- | --- |
| `apps/proxy-control/` | `cmd/main.go` 负责配置加载、依赖组装和启动 Kratos。`bin/` 存放构建产物，由构建创建并由 Git 忽略。 |
| `apps/proxy-control/internal/` | `server/` 负责服务注册和中间件。业务按能力分包，例如 `proxyauth/` 负责代理认证，`usage/` 负责用量接收、增量计算和查询。业务包随职责增长再拆分。 |
| `apps/proxy-gateway/` | 原版 GOST 配置归 `gost/`。同机采集器归 `collector/`，负责读取 Recorder 文件、批量上报和保存已确认偏移。 |
| `apps/proxy-console/` | 租户与运营控制台，作为前端应用保有自己的构建入口，通过控制面 API 查询。先打通接口和后端实现。 |
| `pkg/` | 放多个应用实际复用的 Go 包，按职责命名。应用私有业务、存储实现和配置类型留在所属应用的 `internal/`。 |

### 接口与依赖

- 工具链固定为 Go 1.27.4、Kratos v3.0.0、Protobuf Go v1.36.12 和 `protoc` 28.3，接口和配置使用 `proto3`。本地开发和 CI 使用相同的 Go 和 `protoc` 版本。
- 跨应用通过协议交互。平台自有的服务间接口以 Protobuf 为契约，按接口提供方和版本放入 `api/<服务名>/<版本>/`。
- `proxy-control` 提供的接口归 `api/proxycontrol/v1/`，PB 包名为 `proxycontrol.v1`。网关上报和控制台查询分别在 `reporting.proto` 和 `usage.proto` 定义，可分别声明服务并绑定对应的身份与权限规则。
- 一期只启用 HTTP 服务。采集器通过 HTTP 批量上报，平台自有接口用 `google.api.http` 声明路由并生成 HTTP/JSON 绑定。
- API 的 PB 源文件和生成的 Go 绑定归 `api/`。使用 `protoc` 和固定版本的生成插件，源文件与受影响的生成物同 PR 提交。
- 已发布的 `v1` 接口在滚动部署期间兼容仍在运行的旧调用方，包括 HTTP 路由、JSON 字段和批次重发的确认语义。保留已发布字段的编号，删除字段时使用 `reserved`，不兼容变更通过新版本交付。
- GOST 认证插件遵循 GOST 自有协议，控制面将其请求适配到 `proxyauth/`。

### 开发工具初始化

参考 `rock-stack` 的 `cgvmagent`，在仓库根目录运行 `make init`，使用 Go 1.27.4 自动安装项目所需的命令行工具。Go 工具使用 `go install <命令包>@<固定版本>` 安装，本地和 CI 使用相同版本。

| 工具 | 命令包 | 固定版本 |
| --- | --- | --- |
| Protobuf Go 生成插件 | `google.golang.org/protobuf/cmd/protoc-gen-go` | `v1.36.12` |
| Kratos HTTP 生成插件 | `github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v3` | `v3.0.0-20260526000039-30da04b769dc` |
| golangci-lint | `github.com/golangci/golangci-lint/v2/cmd/golangci-lint` | `v2.15.0` |
| Goose | `github.com/pressly/goose/v3/cmd/goose` | `v3.30.0` |

`make init` 下载并校验官方 `protoc` 28.3 发布包，将编译器和 Go 工具安装到被 Git 忽略的 `.tools/bin/`，将标准 Protobuf 定义安装到 `.tools/include/`。应用共用的 `build/tools.mk` 通过仓库内路径指定 `protoc`，控制面生成 PB 时直接使用该路径。支持 Windows amd64、Linux amd64/arm64 和 macOS amd64/arm64。golangci-lint 使用 v2 配置。 [Protobuf v28.3 发布页](https://github.com/protocolbuffers/protobuf/releases/tag/v28.3)

各应用在自己的目录运行 Make 命令，也可从仓库根目录用 `make -C <应用目录> <目标>` 调用。

| 应用 | 命令 |
| --- | --- |
| `apps/proxy-control/` | `make build` 构建控制面，`make pb` 运行 `make api` 和 `make config`，`make lint` 检查控制面和 API Go 代码。 |
| `apps/proxy-gateway/` | `make build` 构建 collector，`make lint` 检查网关 Go 代码。整包目标随打包流程实现。 |

控制面启动后可通过 `GET /healthz` 检查 HTTP 进程是否可响应。控制面的 `make dao` 和 `make wire` 随对应实现加入。

HTTP 注解依赖的 `annotations.proto` 和 `http.proto` 来自 [Google APIs 固定提交](https://github.com/googleapis/googleapis/tree/547f8fdb4b91dbac70d540b3757520f0608ae356/google/api)，随 [许可证](../../third_party/googleapis/LICENSE) 一起保存在 `third_party/googleapis/`。

### 数据库、迁移与 DAO

一期使用 PostgreSQL，由 `proxy-control` 管理。Goose v3.30.0 管理表结构迁移，运行时通过 GORM 执行查询、写入和事务。应用启动时不调用 `AutoMigrate`。

| Go 模块 | 固定版本 | 用途 |
| --- | --- | --- |
| `gorm.io/gorm` | `v1.31.2` | 查询、写入和事务 |
| `gorm.io/driver/postgres` | `v1.6.3` | GORM 的 PostgreSQL 驱动 |
| `gorm.io/gen` | `v0.3.29` | 反向生成持久化模型 |
| `gorm.io/plugin/dbresolver` | `v1.6.2` | 为兼容 GORM v1.31.2 显式固定的 GORM Gen 间接依赖 |

表结构变更按以下顺序执行。

1. 在控制面编写版本化 Goose SQL 迁移，并应用到 Dev PostgreSQL。
2. 运行 `make dao`，由 GORM Gen 通过只读连接读取 Dev PostgreSQL 中指定表的结构。只调用 `GenerateModel` 和 `Execute` 生成持久化 Go 模型到 `apps/proxy-control/internal/data/model/`，省略生成查询代码的 `ApplyBasic` 和 `ApplyInterface`。
3. 在同一个 PR 提交迁移和生成模型。发布时用 Goose 显式执行同一套迁移。

### 配置管理

控制面的配置 PB 源文件放在 `apps/proxy-control/internal/conf/protos/`。在控制面目录运行 `make config`，由 `protoc` 和固定版本的 Go 生成插件将配置类型生成到 `internal/conf/`。配置 PB 源文件和对应的 `.pb.go` 同 PR 提交。运行时配置归 `configs/`，实际凭据由部署环境注入。

### PR 检查与业务回归

- Go 改动通过格式检查、`golangci-lint run` 和 `go test ./...`。
- PB 改动通过 `make pb` 验证编译。PB 或生成配置变更时，CI 使用固定版本重新生成并检查生成物是否与提交一致。PB 命名规范在代码评审中检查。
- 首版正式发布后，涉及已发布契约的改动以 `proxy-control` 最近一次正式发布的 Git 标签为基线，例如 `proxy-control/v0.1.0`。评审核对 Protobuf 字段、HTTP 路由和 JSON 绑定的兼容性。
- 改动计量、回放或租户权限时，同一个 PR 提供能验证对应行为的自动化用例。用固定时间和 Recorder 样本验证累计快照取差值、跨午夜归属和失败连接不计量，按改动补充重发与租户隔离用例。
- 使用临时 PostgreSQL 的 HTTP 集成测试延后。真实 GOST、悟空和跨机器场景按[一期开发任务](../features/proxy-phase-one/proxy-phase-one-development.md)联调验收。

### 网关构建、打包与发布

`proxy-gateway` 使用一个网关版本，每个目标平台发布一个整包。原版 GOST 和自研 collector 随同一个包交付，部署后分别作为独立进程运行。

- `gost/` 存放 GOST 配置模板和上游产物锁定清单。一期沿用原版 GOST `v3.3.3`，按平台锁定上游资产及其 SHA-256，构建时下载、校验并解包。升级时在同一个 PR 更新锁定清单和对应的 Recorder 验收样本。
- `collector/` 存放采集器源码。从网关发布标签对应的仓库提交构建 `proxy-gateway-collector`，用网关版本和源码 commit 标识该构建。
- `bin/<os>-<arch>/` 存放 `gost` 和 `proxy-gateway-collector`，`dist/` 存放 `proxy-gateway-<version>-<os>-<arch>.tar.gz` 及同名 `.sha256` 校验文件。`bin/` 和 `dist/` 由构建和打包生成，并由 Git 忽略。
- 归档包含两个二进制、配置模板、适用的上游许可证和声明，以及 `manifest.json`。清单记录网关版本、源码 commit、GOST 上游版本、目标平台及两个二进制的 SHA-256。实际凭据、Recorder 文件和采集偏移由部署环境单独管理。
- 发布标签采用 `proxy-gateway/v0.1.0` 形式。GOST、collector 或随包交付的配置模板发生变化时，发布新的网关版本。CI 为每个目标平台构建一次，验收和部署使用同一份归档，并核对校验和。
- 首批只发布 `linux-amd64`。发布归档及其 `.sha256` 校验文件上传到 [DuanJinTaoToday/proxy 的 GitHub Releases](https://github.com/DuanJinTaoToday/proxy/releases)，关联对应的网关发布标签。保留历史发布产物供回滚使用。

### 网关升级与回滚

升级和回滚按网关整包版本操作，保留 Recorder 文件和采集偏移。发布前验证新 collector 能补报旧 GOST 写入的未确认记录，也验证上一版 collector 能读取新版 GOST 已写入的未确认记录。若上一版 collector 无法读取新版记录，发布前须明确如何补报这些记录并确认成功，再切回上一版整包。计量与回放规则见[一期技术方案](../features/proxy-phase-one/proxy-phase-one-architecture.md)。

## 待定项

| 事项 | 当前讨论 |
| --- | --- |
| 控制台技术栈 | Vue 是候选。框架、构建工具、部署方式和是否生成前端客户端，等接口和后端打通后再定。 |
