# 大仓代码组织与开发规范

本文记录一期已确认的工程约定。产品范围与验收标准见[一期 Spec](../features/proxy-phase-one/proxy-phase-one-spec.md)，部署和计量链路见[一期技术方案](../features/proxy-phase-one/proxy-phase-one-architecture.md)，实施顺序见[开发任务](../features/proxy-phase-one/proxy-phase-one-development.md)。

## 已确认

### 仓库与目录

仓库使用根目录 `go.mod`，在 `apps/` 下按部署单元组织源码、配置和构建入口。`api/` 存放公开契约及生成物，`build/` 和 `third_party/` 存放共享构建工具和第三方协议定义，`docs/` 和 `prototypes/` 分别存放项目文档和独立验证原型。

应用间通过 API 契约交互。Go 应用间共享公开契约和 `pkg/` 中实际被多个应用复用的包。应用私有实现放在各自的 `internal/`，应用内多个包确有复用时可放入 `internal/pkg/`。本地覆盖配置和调试产物可放在应用的 `local/` 并配置 Git 忽略。

- 有明确指定且可用的实现时，优先直接复用，改动限于版本兼容和本项目接入所需的部分。

### 应用分层与测试归属

- 请求按 `server → service → biz` 处理。`server` 装配 HTTP、路由和中间件，`service` 转换协议参数和响应，`biz` 按业务能力分包并使用自己的类型。`data` 实现业务层定义的接口并转换持久化模型，由 Wire 注入。
- 控制面的跨层测试和共享 fixture 放在 `integration/`，HTTP 层测试留在 `internal/server/`。
- 控制面的用量维护逻辑和批次条数配置归 `internal/biz/usage/`。`internal/server/server.go` 通过 Provider 获取模块启用状态，职责限于路由和服务生命周期。计量写入、确认偏移和归档锁由同一个数据库事务保护。
- 数据库等中间件连接及文件存储归数据层的 `repo`，访问其他服务的 HTTP 客户端归 `acl`。网关访问控制面的 HTTP 客户端和传输配置归 `internal/data/acl/`，Recorder 文件、确认偏移、退出标记和磁盘监控归 `internal/data/repo/`。调整归属时同步更新配置和 Wire。网关对应配置为 `data.acl.control` 和 `data.repo.recorder`。
- 网关的 `internal/biz/authrelay/` 负责认证决策、缓存和状态刷新，`internal/biz/recorder/` 负责采集调度、批次确认与重放及 GOST 代次状态上报。`internal/data/gost/` 负责配置渲染、子进程和退出状态。`internal/pkg/authmetrics/` 存放认证中继各层共享的指标。涉及文件系统或持久化时兼顾 Windows 和 Unix 实现。
- 控制台的项目清单、构建配置和独立构建入口放在应用根目录，页面、组件和应用逻辑归 `src/`，控制面 API 调用和响应映射归 `src/api/`。租户和运营操作由控制面执行权限检查，界面根据 API 响应渲染可用状态。

### 控制台技术栈

- 一期控制台采用 Vue 3、TypeScript、Vite 和 Vue Router。租户和运营功能共用 `proxy-console`。
- `proxy-console` 独立构建和发布 `dist` 静态产物。HTTPS 入口在同一域名下托管静态文件、支持直接访问前端路由，并按路由转发控制台所需的 API 给 `proxy-control`。正式环境复用哪个 HTTPS 入口，在部署时确认。
- 一期在 `src/api/` 手写集中封装的 TypeScript `fetch` 客户端，统一处理 Bearer token、错误响应和响应映射。64 位字节字段按接口的 JSON 字符串读取，保留精度后再比较或换算显示单位。

### 启动与 Wire

- 两个 Go 应用的 `cmd/` 只包含 `main.go`、`wire.go`、`wire_gen.go`。
- `main` 依次加载配置、初始化日志、调用 `wireApp`、运行 `app.Run`。模块启用条件、后台任务和子进程生命周期由对应层处理，依赖装配交给各层 Provider。
- `cmd/wire.go` 组合各层的 Wire ProviderSet。接口归使用方，接口绑定归提供实现的层。修改注入器、ProviderSet 或相关构造函数后，在对应应用目录运行 `make wire`，将生成的 `cmd/wire_gen.go` 一起提交。

### 接口与依赖

- 工具链固定为 Go 1.27.4、Kratos v3.0.0、Protobuf Go v1.36.12 和 `protoc` 28.3，接口和配置使用 `proto3`。本地开发和 CI 使用相同的 Go 和 `protoc` 版本。
- 跨应用通过协议交互。平台自有的服务间接口以 Protobuf 为契约，源文件按接口提供方和版本放入 `api/protos/<服务名>/<版本>/`。
- `proxy-control` 的接口源文件位于 `api/protos/proxycontrol/v1/`，PB 包名为 `proxycontrol.v1`。网关上报和控制台查询分别在 `reporting.proto` 和 `usage.proto` 定义，可分别声明服务并绑定对应的身份与权限规则。
- 一期只启用 HTTP 服务。采集器通过 HTTP 批量上报，平台自有接口用 `google.api.http` 声明路由并生成 HTTP/JSON 绑定。
- 在 `apps/proxy-control/` 运行 `make api`，用固定版本的 `protoc` 和插件生成 `api/proxycontrol/v1/` 下的 Go 绑定，以及 `api/proxycontrol/openapi.yaml`。源文件与受影响的生成物同 PR 提交。
- 已发布的 `v1` 接口在滚动部署期间兼容仍在运行的旧调用方，包括 HTTP 路由、JSON 字段和批次重发的确认语义。保留已发布字段的编号，删除字段时使用 `reserved`，不兼容变更通过新版本交付。
- GOST 认证插件遵循 GOST 自有协议。网关本机认证中继接收该协议，控制面提供凭据管理和网关状态同步。

### 开发工具初始化

在仓库根目录运行 `make init`，使用 Go 1.27.4 自动安装项目所需的命令行工具。Go 工具使用 `go install <命令包>@<固定版本>` 安装，本地和 CI 使用相同版本。

| 工具 | 命令包 | 固定版本 |
| --- | --- | --- |
| Protobuf Go 生成插件 | `google.golang.org/protobuf/cmd/protoc-gen-go` | `v1.36.12` |
| Kratos HTTP 生成插件 | `github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v3` | `v3.0.0-20260526000039-30da04b769dc` |
| OpenAPI 生成插件 | `github.com/google/gnostic/cmd/protoc-gen-openapi` | `v0.7.1` |
| golangci-lint | `github.com/golangci/golangci-lint/v2/cmd/golangci-lint` | `v2.15.0` |
| Goose | `github.com/pressly/goose/v3/cmd/goose` | `v3.30.0` |
| Wire | `github.com/google/wire/cmd/wire` | `v0.7.0` |

`make init` 下载并校验官方 `protoc` 28.3 发布包，将编译器和 Go 工具安装到被 Git 忽略的 `.tools/bin/`，将标准 Protobuf 定义安装到 `.tools/include/`。应用共用的 `build/tools.mk` 通过仓库内路径指定 `protoc`，控制面生成 PB 时直接使用该路径。支持 Windows amd64、Linux amd64/arm64 和 macOS amd64/arm64。golangci-lint 使用 v2 配置。 [Protobuf v28.3 发布页](https://github.com/protocolbuffers/protobuf/releases/tag/v28.3)

各应用在自己的目录运行 Make 命令，也可从仓库根目录用 `make -C <应用目录> <目标>` 调用。

| 应用 | 命令 |
| --- | --- |
| `apps/proxy-control/` | `make build` 构建控制面，`make pb` 运行 `make api` 和 `make config`，`make lint` 检查控制面和 API Go 代码。 |
| `apps/proxy-gateway/` | `make build` 构建 `proxy-gateway`，`make lint` 检查网关 Go 代码。整包目标随打包流程实现。 |
| `apps/proxy-console/` | `make init` 安装依赖，`make run` 启动 Vite，`make build` 检查类型并打包到 `dist/`。 |

控制面启动后可通过 `GET /healthz` 检查 HTTP 进程是否可响应。控制面的 `make dao` 用于生成持久化模型。

HTTP 注解依赖的 `annotations.proto` 和 `http.proto` 来自 [Google APIs 固定提交](https://github.com/googleapis/googleapis/tree/547f8fdb4b91dbac70d540b3757520f0608ae356/google/api)，随 [许可证](../../third_party/googleapis/LICENSE) 一起保存在 `third_party/googleapis/`。

### 数据库、迁移与 DAO

一期使用 PostgreSQL，由 `proxy-control` 管理。Goose v3.30.0 管理表结构迁移，运行时通过 GORM 执行查询、写入和事务。应用启动时不调用 `AutoMigrate`。

控制面的 `NewData` 直接调用 `pkg/client/db` 的 `NewPgsqlClient` 初始化认证库和用量库。认证库禁用 GORM 查询日志，用量库初始化时检查所需表结构。初始化失败时关闭本次已经打开的连接。

Goose SQL 迁移放在控制面的 `migrations/`。`make dao` 的入口和开发库连接配置直接写在 `apps/proxy-control/internal/data/gen/gen.go`，提交时保持 `databaseDSN` 为空。`server.yaml` 只承载应用运行配置。

| Go 模块 | 固定版本 | 用途 |
| --- | --- | --- |
| `gorm.io/gorm` | `v1.31.2` | 查询、写入和事务 |
| `gorm.io/driver/postgres` | `v1.6.3` | GORM 的 PostgreSQL 驱动 |
| `gorm.io/gen` | `v0.3.29` | 反向生成持久化模型 |
| `gorm.io/plugin/dbresolver` | `v1.6.2` | 为兼容 GORM v1.31.2 显式固定的 GORM Gen 间接依赖 |

表结构变更按以下顺序执行。

1. 在控制面编写版本化 Goose SQL 迁移，并应用到 Dev PostgreSQL。
2. 运行 `make dao`，由 GORM Gen 通过只读连接读取 Dev PostgreSQL 中指定表的结构。只调用 `GenerateModel` 和 `Execute` 生成持久化 Go 模型到 `apps/proxy-control/internal/data/repo/model/`，省略生成查询代码的 `ApplyBasic` 和 `ApplyInterface`。
3. 在同一个 PR 提交迁移和生成模型。发布时用 Goose 显式执行同一套迁移。

### 配置管理

两个 Go 服务在各自的 `internal/conf/protos/` 维护 `init.proto`、`server.proto`、`data.proto`、`biz.proto`。修改后在应用目录运行 `make config`，将 proto 和生成在 `internal/conf/` 的对应 `.pb.go` 一起提交。

从应用目录启动，默认 `-conf ./configs`，也可传入单个入口文件。`main.go` 直接调用 Kratos 的 `config.New`、`Load`、`Scan` 读取定义配置源和 `zap` 的 `configs/init.yaml`。各应用在自己的 `internal/conf/config.go` 中实现 `AppConfig`、`GetBootstrap` 和 `Register`，由 Kratos `Watch` 分发配置变更。

应用运行配置集中在 `configs/file/server.yaml`，按 `server`、`data`、`biz` 分组，对需要说明的字段就地加注释。文件源通过 `source.file` 指定，相对路径以进程工作目录为基准。启用 Apollo 时通过 `source.apollo` 选择单个 YAML 或 JSON namespace。配置中包含凭据时使用 Git 忽略的 `configs/**/*.local.yaml`，并限制读取权限。

`zap` 支持控制台、日志级别及滚动文件。构建时可通过 `make build GO_LDFLAGS='-X main.GitTag=v0.1.0 -X main.GitCommit=<commit> -X main.BuildTime=<RFC3339>'` 注入版本信息。

#### 本地启动

从对应应用目录操作。将 `configs/file/server.yaml` 复制为 `configs/file/server.local.yaml`，填写本地配置。将 `configs/init.yaml` 复制为 `configs/file/init.local.yaml`，把 `source.file` 指向 `./configs/file/server.local.yaml`。运行 `make build` 后，用 `./bin/<应用名> -conf ./configs/file/init.local.yaml` 启动。Windows 的可执行文件带 `.exe` 后缀。

### Cron

两个 Go 应用统一使用 `pkg/cron` 调度。`internal/service/cron.go` 定义 `CronService` 及任务方法，`internal/server/cron.go` 负责注册任务和管理调度生命周期。任务名称对应 `CronService` 的方法，表达式放在 `server.yaml` 的 `server.cron_tasks` 下。控制面通过 `MaintainUsage` 调度用量维护，网关通过 `PrintHello` 定时打印日志。

Cron Server 通过 `AppConfig.Register("server", ...)` 注册监听，收到 `Notify` 后按更新后的 `cron_tasks` 重新注册任务，无需重启。将列表设为 `[]` 可停止后续调度。其他应用配置通过重启生效。

### PR 检查与业务回归

- Go 改动通过格式检查、`golangci-lint run` 和 `go test ./...`。
- PB 改动通过 `make pb` 验证编译。PB 或生成配置变更时，CI 使用固定版本重新生成并检查生成物是否与提交一致。PB 命名规范在代码评审中检查。
- 首版正式发布后，涉及已发布契约的改动以 `proxy-control` 最近一次正式发布的 Git 标签为基线，例如 `proxy-control/v0.1.0`。评审核对 Protobuf 字段、HTTP 路由和 JSON 绑定的兼容性。
- 改动计量、回放或租户权限时，同一个 PR 提供能验证对应行为的自动化用例。用固定时间和 Recorder 样本验证直接取 Recorder 增量字段并用累计值校验、跨午夜归属和失败连接不计量，按改动补充重发与租户隔离用例。
- 控制面的 PostgreSQL 集成用例使用 `PROXY_CONTROL_TEST_DSN` 指定有 schema 创建权限的本地数据库。网关的真实 GOST 认证、采集和子进程退出用例使用 `PROXY_GOST_TEST_BINARY` 指定 GOST v3.3.3 可执行文件的绝对路径。悟空和跨机器场景见[一期开发任务](../features/proxy-phase-one/proxy-phase-one-development.md)。

### 网关构建、打包与发布

`proxy-gateway` 使用一个网关版本，每个目标平台发布一个整包。原版 `gost` 和自研 `proxy-gateway` 随同一个包交付。部署时启动 `proxy-gateway`，由它启动和管理 `gost` 子进程。

- `gost/` 存放 GOST 配置模板和上游产物锁定清单。一期沿用原版 GOST `v3.3.3`，按平台锁定上游资产及其 SHA-256，构建时下载、校验并解包。升级时在同一个 PR 更新锁定清单和对应的 Recorder 验收样本。
- `apps/proxy-gateway/cmd/` 是 Go 服务构建入口。`make build` 生成 `bin/proxy-gateway`，Windows 产物带 `.exe` 后缀。从网关发布标签对应的仓库提交构建 `proxy-gateway`，用网关版本和源码 commit 标识该构建。
- `bin/<os>-<arch>/` 存放 `gost` 和 `proxy-gateway`，`dist/` 存放 `proxy-gateway-<version>-<os>-<arch>.tar.gz` 及同名 `.sha256` 校验文件。`bin/` 和 `dist/` 由构建和打包生成，并由 Git 忽略。
- 归档包含两个二进制、配置模板、适用的上游许可证和声明，以及 `manifest.json`。清单记录网关版本、源码 commit、GOST 上游版本、目标平台及两个二进制的 SHA-256。实际凭据、Recorder 文件和采集偏移由部署环境单独管理。
- 发布标签采用 `proxy-gateway/v0.1.0` 形式。GOST、`proxy-gateway` 或随包交付的配置模板发生变化时，发布新的网关版本。CI 为每个目标平台构建一次，验收和部署使用同一份归档，并核对校验和。
- 首批只发布 `linux-amd64`。发布归档及其 `.sha256` 校验文件上传到 [DuanJinTaoToday/proxy 的 GitHub Releases](https://github.com/DuanJinTaoToday/proxy/releases)，关联对应的网关发布标签。保留历史发布产物供回滚使用。

### 网关升级与回滚

升级和回滚按网关整包版本操作，保留 Recorder 文件和采集偏移。发布前验证新版本的 `proxy-gateway` 能补报旧 GOST 写入的未确认记录，也验证上一版 `proxy-gateway` 能读取新版 GOST 已写入的未确认记录。若上一版 `proxy-gateway` 无法读取新版记录，发布前须明确如何补报这些记录并确认成功，再切回上一版整包。计量与回放规则见[一期技术方案](../features/proxy-phase-one/proxy-phase-one-architecture.md)。

### Spec 更新

替换 Spec 时，把仍有效的约定并入现行文档，更新引用，并从工作树移除旧稿。需要追溯时查阅 Git 历史。
