# 大仓代码组织与开发规范

本文记录一期已确认的工程约定。产品范围与验收标准见[首版商业交付 Spec](../features/proxy-first-release/proxy-first-release-spec.md)，部署和计量链路见[用量技术方案](../features/proxy-phase-one/proxy-phase-one-architecture.md)。

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

控制台的页面结构、目录、组件、主题与双语规则见 [Proxy Console 前端规范](proxy-console-frontend.md)。实施顺序与验收见 [前端设计与实施计划](../features/proxy-first-release/proxy-console-frontend-design.md)。

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
- 租户日用量 CSV 按首版模块契约使用 Go HTTP 下载路由，在 `usage.proto` 的 `openapi.v3.document` 声明 `text/csv`。下载与 JSON 查询共用筛选和聚合。
- 在 `apps/proxy-control/` 运行 `make api`，用固定版本的 `protoc` 和插件生成 `api/proxycontrol/v1/` 下的 Go 绑定，以及 `api/proxycontrol/openapi.yaml`。源文件与受影响的生成物同 PR 提交。
- 每个 RPC 用 `openapi.v3.operation` 声明摘要，字段注释说明用途、单位、默认值和空值语义。无参数的 RPC 使用 `BaseRequest`，无业务数据的成功响应使用 `BaseResp`。
- 请求字段的静态约束用 `validate.rules` 声明，并生成 Go 校验方法。控制面在业务层执行校验，以保留鉴权顺序和现有 HTTP 错误。跨字段及依赖服务端状态的约束在业务层维护。
- 已发布的 `v1` 接口在滚动部署期间兼容仍在运行的旧调用方，包括 HTTP 路由、JSON 字段和批次重发的确认语义。保留已发布字段的编号，删除字段时使用 `reserved`，不兼容变更通过新版本交付。
- GOST 认证插件遵循 GOST 自有协议。网关本机认证中继接收该协议，控制面提供凭据管理和网关状态同步。

### 开发工具初始化

在仓库根目录运行 `make init`，使用 Go 1.27.4 自动安装项目所需的命令行工具。Go 工具使用 `go install <命令包>@<固定版本>` 安装，本地和 CI 使用相同版本。

| 工具 | 命令包 | 固定版本 |
| --- | --- | --- |
| Protobuf Go 生成插件 | `google.golang.org/protobuf/cmd/protoc-gen-go` | `v1.36.12` |
| Kratos HTTP 生成插件 | `github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v3` | `v3.0.0-20260526000039-30da04b769dc` |
| OpenAPI 生成插件 | `github.com/google/gnostic/cmd/protoc-gen-openapi` | `v0.7.1` |
| Protobuf 校验插件 | `github.com/envoyproxy/protoc-gen-validate` | `v1.3.3` |
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

OpenAPI 和校验注解的 proto 定义分别来自 Gnostic v0.7.1 和 protoc-gen-validate v1.3.3，随许可证保存在 `third_party/openapiv3/` 和 `third_party/validate/`。

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

线上运行配置模板位于 `configs/file/server.yaml`，Docker 测试环境的应用运行配置位于 `configs/file/test-server.yaml`。两者均按 `server`、`data`、`biz` 分组，对需要说明的字段就地加注释。文件源通过 `source.file` 指定，相对路径以进程工作目录为基准。启用 Apollo 时通过 `source.apollo` 选择单个 YAML 或 JSON namespace。配置中包含凭据时使用 Git 忽略的 `configs/**/*.local.yaml`，并限制读取权限。

`zap` 支持控制台、日志级别及滚动文件。构建时可通过 `make build GO_LDFLAGS='-X main.GitTag=v0.1.0 -X main.GitCommit=<commit> -X main.BuildTime=<RFC3339>'` 注入版本信息。

#### 本地启动

从对应应用目录操作。将 `configs/file/server.yaml` 复制为 `configs/file/server.local.yaml`，填写本地配置。将 `configs/init.yaml` 复制为 `configs/file/init.local.yaml`，把 `source.file` 指向 `./configs/file/server.local.yaml`。运行 `make build` 后，用 `./bin/<应用名> -conf ./configs/file/init.local.yaml` 启动。Windows 的可执行文件带 `.exe` 后缀。

### Cron

两个 Go 应用统一使用 `pkg/cron` 调度。`internal/service/cron.go` 定义 `CronService` 及任务方法，`internal/server/cron.go` 负责注册任务和管理调度生命周期。任务名称对应 `CronService` 的方法，表达式放在运行配置的 `server.cron_tasks` 下。控制面通过 `MaintainUsage` 调度用量维护，网关通过 `ReportUsage` 扫描 Recorder 并上报用量。

Cron Server 通过 `AppConfig.Register("server", ...)` 注册监听，收到 `Notify` 后按更新后的 `cron_tasks` 重新注册任务，无需重启。将列表设为 `[]` 可停止后续调度。其他应用配置通过重启生效。

### `to-spec` 与 `to-tickets`

需求对齐、编写 Spec 和拆票由 Matt Pocock 工作流负责，按「确认共享设计 → `to-spec` → 场景走查 → `to-tickets`」推进。涉及多张票的共享决定由用户确认后作为拆票依据。

需要比较共享设计方案时，可调用 `pstack:architect`，并明确在给出候选方案和取舍后结束本轮。确认共享设计后，由 `to-tickets` 将约定的行为、验收条件和依赖交给实现票。

#### 确认共享设计

先选一条代表性业务流程，从客户操作走到 API 响应、订单状态和资金变化。提前对齐影响多张票的接口、状态和验收规则。沿用现有设计时，引用对应章节或页面。

| 设计 | 拆票前需确认 | 记录位置 |
| --- | --- | --- |
| 数据模型 | 核心实体及关系、状态由谁推进、哪些写入必须一起成功、扣款和交付如何保证幂等。 | [首版数据模型](../features/proxy-first-release/proxy-first-release-data-model.md) |
| API 契约 | 跨模块和前后端共享的操作、关键输入输出、状态与错误语义，以及调用方收到结果后的动作。 | [首版模块契约](../features/proxy-first-release/proxy-first-release-module-contracts.md) |
| 前端设计 | 共用页面结构、组件规范、关键流程的加载、处理中、失败和重试状态。涉及新风格或新交互时，先确认一页代表性页面。 | 已确认的页面、原型或控制台设计说明。 |

字段命名、索引、内部接口和单页微调等只影响一张票的细节，由该票决定。

#### 用 `to-spec` 固化业务承诺

Spec 写清交付范围、客户可以依赖的行为、关键业务规则和整链路验收条件，并链接对应的共享设计。供应商能力等尚待验证时，记录现有证据支持的承诺、待补的证据，以及影响开工、关票还是开售。产品承诺发生变化时，先确认并更新 Spec，再同步受影响的 Issue。

#### 拆票前走查场景

选一条主路径，以及会改变资金、交付状态或客户下一步操作的异常路径。逐场景确认以下三项。

1. 状态和资金怎么变。明确谁推进状态，以及操作前后订单、钱包和交付记录的变化。
2. API 和页面怎么衔接。明确接口返回、页面呈现和客户可以采取的下一步操作。
3. 验收证据从哪里来。明确操作入口、可观察结果和负责验证的切片。

例如，静态 IP 购买提交旧报价后，订单和钱包无变化，API 返回新价，页面展示新价并要求客户重新确认。这条场景可以直接成为实现票的验收项。

未知项仍会改变共享状态、接口或验收预期时，先安排验证票，明确它阻塞哪些实现票。其余切片可以继续拆。

#### 用 `to-tickets` 拆成可独立验收的业务切片

每张票写清本票交付的行为、可复跑的验收条件和开工依赖，并链接相关 Spec 和设计章节。切片可以同时涉及数据、API 和页面，以能独立演示或验证为准。

- 开工依赖使用 GitHub 原生 `Blocked by`，适用于上游结论出来前无法按确定口径实现的情况。
- 关票或开售依赖写进验收条件，适用于本票可以先实现、验收需要下游联调证据的情况。按[跨 Issue 验收模板](../agents/issue-tracker.md#跨-issue-验收)指定执行票、操作与预期和证据回填位置。

`to-tickets` 先给出拆票清单供评审。用户确认切片和依赖关系并授权发布后，再创建 GitHub Issue。

### Issue 实现与交付

收到“实现 Issue”或 `implement` 请求时，由 `implement` 统一负责接票、实现、提交、评审、项目级 `verify` 和交付。需要设计比较、缺陷回归测试或注释审查时，`implement` 可按需调用 `pstack:architect`、`pstack:tdd` 或 `pstack:no-comments`。子步骤结束后回到本节流程，由 `implement` 决定下一步并汇总验收证据。

#### 接票、worktree 与分支

1. 读取 Issue、父 Spec 和阻塞关系，开工前报告工单标题、基线和工作位置。把每条验收条件对应到操作入口、预期结果和负责执行的票，包括本票承接的跨 Issue 验收项。前置 Issue 尚未合并时，先判断本票能否独立实现。确实需要基于未合并改动时，明确采用哪个基线。
2. 新 Issue 默认从最新 `origin/main` 创建独立 worktree，使用 `codex/issue-<编号>-<简短描述>` 分支。继续同一 Issue 或修复其评审发现时，复用已有 worktree 和任务分支。用户指定工作区、分支或基线时，按指定位置执行。
3. 无 Issue 的小改动可以留在当前工作区。由 `implement` 负责提交时，默认使用 `codex/<简短描述>` 分支。直接提交到本地 `main` 需用户明确指定。

#### 实现中变更 Issue

| 发现 | 处理方式 |
| --- | --- |
| 事实错误或引用过期 | 用仓库或实测证据核对，修正对应段落。验收行为一致时，实现可以继续。 |
| 验收条件缺失、冲突或需要变更 | 在当前会话给出原文、建议改文、证据和影响，待用户确认。等待期间继续做口径明确的部分。确认后更新 Issue 正文及受影响的测试，按新口径实现。 |
| 发现额外需求 | 影响原交付验收时，确认后纳入本票并更新验收条件。可以独立交付时，另开关联 Issue。 |
| 子票与父 Spec 冲突 | 子票有误时按 Spec 修正。产品决策变更时，先确认并更新父 Spec，再同步受影响的子票。 |

Issue 正文维护当前交付范围、验收条件和依赖。影响范围、验收或依赖时，在评论里记录改动、依据和受影响的工单。编辑或新建 GitHub 工单需要用户授权，可在开工时一并授权修正事实及链接。授权之前，在当前会话提供改文草案。供应商实测交接和跨 Issue 验收按 [Issue 拆票模板](../agents/issue-tracker.md#拆票模板) 填写。

#### 本地提交、评审与验收

1. 按 Issue 及 Spec 实现，在已约定的公开接口或行为边界上逐个切片验证，完成 [PR 检查与业务回归](#pr-检查与业务回归) 中适用的检查。
2. 检查通过后，先把本次改动提交到任务分支，再用 `code-review` 从开工时记录的基线评审到当前 `HEAD`，核对开发规范和 Issue／Spec 验收要求。
3. 评审发现需要修复时，在同一分支修改，重跑受影响的检查，提交并复审。
4. 对评审通过的 `HEAD` 执行项目级 [verify](../../.agents/skills/verify/SKILL.md)。实现 Issue 的请求已包含在本机隔离环境部署并验收的授权。Agent 自行运行本票约定的用户流程，定位并修复失败。修复涉及代码或验收脚本时，回到提交和评审步骤，然后从修复后的版本重新部署、复跑。
5. 交付时报告分支、最终 commit、检查和评审结果，逐条列出验收条件、实测结果及本轮证据链接。全部验收项有通过证据后才报告 Issue 完成。有阻断或约定由下游执行的条目时，报告已通过项和待验收项，注明阻断原因、执行票（如有）以及还缺的证据。

#### 推送、PR 与关票

用户可在开工时一并授权远端操作。默认完成本地提交、评审和验收后交付。已有授权按下列条件继续执行。已打开的 PR 需要持续跟进 CI 或评审时，可按需调用 `pstack:babysit`。

| 操作 | 执行条件 |
| --- | --- |
| Push | 用户已授权推送，且评审完成。默认推任务分支。直推 `origin/main` 需用户明确指定。 |
| 开 PR | 用户已授权开 PR，且本地评审完成。开 PR 的授权包含推送任务分支。PR 默认以 `main` 为目标，附测试结果和待验收项。 |
| 合并 PR | 用户已授权合并，且对应评审和检查已通过。 |
| 关闭 Issue | 用户已授权关票，改动已进入目标分支，且工单约定的验收证据齐全，包括依赖下游 Issue 的联调证据。还有待验收项或尚无关票授权时，PR 使用 `Refs #<编号>` 关联工单。 |

### PR 检查与业务回归

- Go 改动通过格式检查、`golangci-lint run` 和 `go test ./...`。
- PB 改动通过 `make pb` 验证编译。PB 或生成配置变更时，CI 使用固定版本重新生成并检查生成物是否与提交一致。PB 命名规范在代码评审中检查。
- 首版正式发布后，涉及已发布契约的改动以 `proxy-control` 最近一次正式发布的 Git 标签为基线，例如 `proxy-control/v0.1.0`。评审核对 Protobuf 字段、HTTP 路由和 JSON 绑定的兼容性。
- 改动计量、回放或租户权限时，同一个 PR 提供能验证对应行为的自动化用例。用固定时间和 Recorder 样本验证直接取 Recorder 增量字段并用累计值校验、跨午夜归属和失败连接不计量，按改动补充重发与租户隔离用例。
- 控制面的 PostgreSQL 集成用例使用 `PROXY_CONTROL_TEST_DSN` 指定有 schema 创建权限的本地数据库。网关的真实 GOST 认证、采集和子进程退出用例使用 `PROXY_GOST_TEST_BINARY` 指定 GOST v3.3.3 可执行文件的绝对路径。悟空和跨机器场景见[历史验收索引](../features/proxy-phase-one/proxy-phase-one-development.md)。

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
