# Keel 分支整合交接

本轮仅整合代码和历史，不部署、不恢复已暂停的开发。以下是各分支交接快照；运行环境以实际进程为准，历史部署段落不代表当前运行版本。

## BOM / 项目管理分支交接快照

# Keel 当前开发交接

## 当前任务：通用智能开局工程 P0 审计（等待用户审核）

独立工作树 `/home/ubuntu/keel-engineering-p0`，分支 `audit/intelligent-deployment-p0`，代码基线2d66974。已本地读取指定MM_Layout_GB300样本（在 `/home/ubuntu/`），完成九项代码/资料审计；报告入口 `docs/intelligent-deployment/P0-REVIEW.txt`，完整报告及结构证据同目录JSON。15 Sheet、94,353公式；源文件摘要未变，不入Git。P0只新增报告和决策记录，业务代码、数据库、API、部署均未改动。未运行外部AI、冒烟/回归或构建。P1架构详细设计及任何实现必须等待用户审核本次方向；不得恢复旧Agent补丁任务。


## 用户决定：停止旧Agent补丁并重新开发开局配置

已按用户指令清理13个测试/验收项目，关联BOM导入、项目资产及CMDB专属数据已删除，13项对象清理均完成；保留审计、审批、共享型号、3个固定演示项目及工作目录原始XLSX。清单见 `docs/topology/test-project-cleanup-20261031.json`。不要继续对旧测试批次重试或恢复旧Agent修补任务；开局配置模块按用户后续新需求重新开发。本次仅实际清理及结果核对，未运行冒烟/回归。


## 当前前端热修复：删除项目不重载页面

前端代码 `0deb3a6`，当前5180源目录未变，通过Vite热更新生效；后端继续994bb1c。删除后在页内更新授权快照、项目列表与项目缓存，后台刷新Session，不再清空整个QueryClient或强制刷新页面。修正启动占位/真实删除Modal说明，拒绝旧授权资产请求回写，空授权清空资产。未运行冒烟、回归或实际删除，详情见项目删除记录。


## 当前部署：项目删除（2026-10-31，覆盖下方历史环境）

用户已授权部署，运行代码 `994bb1c`，集成项目删除及 Flows `95df50f`，保留此前BOM/拓扑修复。8081运行 `/tmp/keel-api-project-delete-1031`，5180前端源目录改为 `/home/ubuntu/keel-project-delete-deploy/dev`；API原配置逐字节保留。启动器及私有部署记录 `/tmp/keel-project-delete-deployment-1031`，不得打印环境文件。迁移025 Flows游标、026项目删除已应用。项目列表提供删除确认；没有实际删除任何项目，没有运行冒烟/回归。Go构建及受限内存前端打包通过，合并后的完整TypeScript构建被终止，不能声称类型检查通过。全库备份失败，完成结构备份；部署详情见 `docs/topology/project-deletion-deployment-20261031.json`。


## 项目删除功能开发（未部署）

独立工作树 `/home/ubuntu/keel-project-delete-dev`，分支 `feat/project-delete`。实现项目列表删除确认、Session/CSRF及project:delete校验、项目专属数据事务清理和上传对象Outbox重试；保留审计、审批和共享型号/资产。迁移026尚未执行。用户要求停止测试后不再运行检查；共享版本引用修正后未复测，实际限制及检查见 `docs/topology/project-deletion-20261031.md`。此节不改变下方当前运行版本。

## 当前运行环境 · BOM 新文件根因修复（2026-10-31）

此节覆盖下方旧运行版本。后端 `7a49bca`，分支 `fix/bom-source-closure`，工作树 `/home/ubuntu/keel-bom-closure-dev`；8081 运行 `/tmp/keel-api-bom-v6p-flows-1031`，启动器 `/tmp/keel-bom-v6p-run-api.py`。前端5180、Flows及原配置未替换。部署证据 `/tmp/keel-bom-v6p-20261031/deployment.json`，秘密环境文件不得打印或入Git。

新文件33/33提取完成，216/216显式类型核对通过；整批确认/提交曾422，完整闭环仍未通过。用户已批准并执行一个空能力的测试型号登记。实际分批入库：30台交换机、14个机柜、8个端口，共52个唯一资产；两次Outbox均已发布，8/8端口归属正确。项目资产58条是版本历史，不是58个唯一资产。此前口头40交换机/4机柜统计不准确，以此次实际API/数据库结果为准。

本次补修CMDB尺寸类型/空值导致投影失败、分批提交候选ID未映射正式资产ID导致依赖失败；部署后同批重试通过。层级/Fabric返回52资产，物理线缆仍0，四类连接矩阵均空，Trace为no_record。线缆速率证据、端点/占用冲突及其他候选审核尚未闭环，不能把HTTP200或资产入库当作完整拓扑通过。不猜测源文件缺失事实，不继续重复全量识别。

独立项目 `proj-d8a90883-3361-465f-8793-7b18346cf0b8`，导入 `fd0f48a6-32e0-440f-a958-9c9c1d545527`。全局测试型号revision `2417e33c-afb1-4414-9e53-2022af410b72`需单独记录清理，删除项目不会删除它。原文件未修改。实测/清理记录见 `docs/topology/bom-real-subset-acceptance-20261031.json`，修复清单见 `docs/topology/bom-source-closure-fixes-20261031.json`。私有原始证据 `/tmp/keel-tx-layout-v6-20261031` 不入Git。

## 当前运行环境 · BOM v5 与 Flows 合并发布（2026-10-31）

用户已授权发布本次 BOM 根因修复。当前后端源码 **fdffbcb**（在 Flows 部署基线 `6884439` 上集成 `67c7f1d`），独立分支 `deploy/bom-v5-flows-1031`、工作树 `/tmp/keel-bom-v5-deploy`。运行 `/tmp/keel-api-bom-v5-flows-1031`，API `127.0.0.1:8081`；5180前端仍从 `/home/ubuntu/keel-flows-dev/dev` 运行。没有替换前端、Flows依赖、数据库或配置。该节覆盖下方旧运行版本及“未部署”的历史记录。

- 已确认运行二进制的源码版本/摘要，进程环境与发布前逐字节一致，工作目录保持 `/home/ubuntu/keel/dev/backend`。
- 集成版 API/AI/BOM/CMDB/Flows 定向测试与构建通过，18 项启动/只读检查通过（含登录、BOM进度、拓扑矩阵/物理追踪及Flows）。无活动任务，正式提交/资产数量前后不变。
- 本次未运行新模型识别、真实文件入库或浏览器绘图验收。已有批次的旧候选不会因更换二进制自动重算；需从工作台按用户验收安排继续分析。
- 启动器 `/tmp/keel-bom-v5-run-api.py`；回退启动器 `/tmp/keel-bom-v5-rollback-api.py` 恢复原 `/tmp/keel-api-flows-combined-1031`。私有运行环境快照位于 `/tmp/keel-bom-v5-deployment-20261031/`，不得打印或入Git。
- 后续构建/发布必须包含 `67c7f1d` 或等价集成 `fdffbcb`；不要直接用未集成这项修复的旧工作树后端覆盖当前8081。

发布证据：[bom-v5-deployment-20261031.json](docs/topology/bom-v5-deployment-20261031.json)。


## 2026-10-31 · 五轮 BOM 验收缺陷根因修复（当前）

用户已重新授权彻底修复已发现问题，覆盖下方历史暂停开发记录。代码在独立分支 `fix/bom-acceptance-root-causes`，工作树 `/tmp/keel-bom-root-fix`，基于已部署的 Agent+Topology 集成代码；未修改其他开发者分支，未部署、未重启服务。

- 修复整批/局部识别状态混淆、表头修正后的行资格丢失、错误字段语义、重复光模块/候选 ID、合并后的引用悬空及第一次关联不稳定。诊断版本 `workbook-agent/v5`。
- 两份原始 XLSX 离线回放各得到 189 条交换机、7,154 条布线候选，其中 7,150 条关联两端物理端口；4 条来源依赖缺失仍阻断。162,715 个源单元格全部保留，NS 第 3 行保持一条线缆，图候选集合重放稳定。
- 现有 GPU/NIC/BMC/PSU 等物理部件可以拥有来源明确的端口；拓扑保留独立部件归属，有上级设备时优先聚合到设备，歧义和跨项目访问继续阻断。没有把部件改成服务器。
- P2P-18 旧方案不满足物理端口契约，被拒绝作为线缆；估算/数量信息完整保留，仍需正确参考/物料方案和人工处置。型号审批、单位、尺寸及原始缺失信息未被补造。
- Go API/AI/BOM/CMDB 定向检查、两文件本地回放、PostgreSQL 投影幂等/完整拓扑读取/部件归属隔离检查通过。新增合成测试位于独立项目的回滚事务中，无持久化测试资产。
- 本轮无新外部模型调用、无真实文件正式提交、无浏览器绘图验收。不能把离线恢复结果当作新模型准确率或上线验收。

逐问题记录：[bom-root-fixes-20261031.json](docs/topology/bom-root-fixes-20261031.json)。到此结束开发，后续部署/集中验收按用户安排。

更新：2026-10-31 UTC。先读本文件，再按下列索引定位；无需重扫全仓。历史交接可用 `git show 6601444:HANDOFF.md` 查阅。

## 当前已发布环境 · 2026-10-31（覆盖下方开发阶段记录）

- 用户现已授权发布；入口恢复统一为 **http://localhost:5180/flows**，API仍`127.0.0.1:8081`。临时5190/8096与合成本地Agent已关闭；不要重新启动它们。
- 运行源码 **9f16065**，分支`feat/flows-redesign`、目录`/home/ubuntu/keel-flows-dev`，未合并或推送main。4403c63/9f16065保留原8081的两项BOM Agent修复，相关BOM/AI文件与原部署分支比对无差异。
- 前端tmux`keel-dev-5180`从本工作区`dev/`启动；API tmux`keel-cmdb-api`运行`/tmp/keel-api-flows-combined-1031`，工作目录继续为`/home/ubuntu/keel/dev/backend`，保留原数据库、对象存储、凭据路径和账号。
- 原业务库新增迁移021…024，未装载任何隔离合成资产。备份位置见`/tmp/keel-flows-deploy-backup-current`，私有目录含部署前完整pg_dump与原进程环境，不得打印或入Git。
- API启动器`/tmp/keel-flows-run-deployed-api.py`读取私有原环境备份和`dev/deploy/flows/.env.deployed`覆盖；后者只配置引擎/Agent/SSH受控路径，不覆盖原DATABASE_URL或密码。Worker经Docker bridge调用现有5180代理，不新增用户入口。
- Kestra8095与Agent网关8097/TLS注册8098是本机后端依赖，不是新增前端入口；真实设备公网TLS/Bootstrap仍需部署条件。
- [发布检查](docs/flows-redesign/deployment-validation.json)：原admin登录、5个Flows只读API、3页面渲染共9项通过；容器到5180健康200、无票据worker回调403。没有设备执行、BOM AI调用或真实数据导入。
- 应用回退：停当前两个tmux服务，使用备份中原API环境恢复`/tmp/keel-api-agent-topology-1031`（同原cwd），前端从`/home/ubuntu/keel-visualization-dev/dev`恢复原5180命令。新增表不自动drop，数据库无需为了旧应用读取而破坏性回滚；完整dump只用于独立授权的数据恢复。

以下Flows独立环境描述为部署前记录，当前端口和运行版本以上述发布节为准。

## 2026-10-31 当前任务：Flows 全栈功能分支（先读）

- 工作区 **`/home/ubuntu/keel-flows-dev`**，分支 **`feat/flows-redesign`**；代码提交`63ca2c9`、`826b964`、开发补齐`afe8cb9`。没有合并/推送main，也没有更改原5180/8081运行服务。
- 任务包原文及八页draw.io在`docs/flows-redesign/`；优先读[实施记录](docs/flows-redesign/implementation.md)与[真实API契约](docs/api/flows.md)，无需重扫旧Flows或全仓。
- 独立前端 **http://127.0.0.1:5190/flows**，API `172.22.0.1:8096`，Kestra `127.0.0.1:8095`，gRPC `localhost:8097`，TLS注册`https://localhost:8098`。远程浏览需端口转发；没有发布到原5180。
- Docker项目`keel-flows`，Postgres中分开`kestra`与`keel_flows_dev`。后者仅原库schema+合成项目/资产，未复制原业务行；迁移021…024只应用这里。不要运行写死原容器的`backend/migrate.sh`。
- 私有配置/新隔离admin密码：`dev/deploy/flows/.env.local`，0600且忽略；原账号密码未改。PKI在同目录`pki/`。API只复用原`.env`的对象存储配置，DATABASE_URL由隔离配置覆盖，禁止输出秘密。
- API `/tmp/keel-flows-api`，PID `/tmp/keel-flows-api.pid`，日志同名`.log`；Agent `/tmp/keel-flows-agent`，PID同名`.pid`，state `/tmp/keel-flows-agent-state`。不要删除state或重复注册。重启前核验PID的exe，不要kill原8081。
- 实际通过：Go定向测试/构建、TypeScript+生产构建、真实Kestra+双合成SSH协议失败重试/幂等、实际非rootAgent/mTLS实时输出、原生Pause/Resume、文件读写、版本冲突、游标与跨项目拒绝、8项浏览器交互。
- 本轮已补齐：Kestra驱动的作用域批次调度、每目标多步骤与输出传递、二进制版本/分发、持久事件游标重放、既有审批与单次执行绑定、实际任务取消。新增API/迁移/测试证据见实施记录。
- 能力边界：每目标32个串行依赖步骤、步骤结果32KiB、二进制资源1MiB；Agent0.2.0支持job.pipeline。待验收仅生产Bootstrap/NOS/真实8目标混合/规模与长期运行，不得宣称生产通过。测试Cron已停用。
- 修改定位：`dev/src/features/flows/`、`dev/src/lib/api/flows.ts`、`dev/backend/internal/flows/`、`cmd/{api/flows.go,agent/}`、迁移021…024、OpenAPI FlowsReal。规则和运行证据链接均见实施记录。

以下为原BOM/拓扑环境的保留历史，勿把运行版本与当前Flows分支混用。

## 2026-10-31 后端 Agent 部署更新（优先于下方历史版本）

当前8081已切换为 `/tmp/keel-api-agent-topology-1031`，源码 `ad8918b`，来自独立集成分支 `deploy/bom-agent-topology-1031`（工作区 `/tmp/keel-agent-deploy`）。该版本保留拓扑基线 b184645，并集成 Astra 的 e3f6cd3、74ab238 Agent 修复。启动配置仍使用 `/home/ubuntu/keel/dev/backend/.env`，前端5180仍使用本工作区。本分支没有合入这些后端代码，后续重新部署时必须保留集成修复，不能直接从旧后端源码重建覆盖。

定向 Go 检查、构建和部署启动检查通过；没有新启动真实识别/入库验收。证据见 `docs/topology/agent-deployment-20261031.json`；旧二进制 `/tmp/keel-api-4541850` 保留用于回退。下方4541850部署描述是历史状态。

## 当前分支与运行环境

- 仓库目标：`enzoOvOyu/keel-new`。工作区 `/home/ubuntu/keel-visualization-dev`，分支 `feat/datacenter-visualization-approved`；未合并 main。
- 已部署业务代码 `4541850`；后续交接/契约文档提交不改变运行代码。
- 前端 http://localhost:5180 ，tmux `keel-dev-5180`，目录为本工作区 `dev/`。
- 后端 `127.0.0.1:8081`，tmux `keel-cmdb-api`，运行 `/tmp/keel-api-4541850`。配置和存储工作目录仍为 `/home/ubuntu/keel/dev/backend`，不要复制或输出 `.env`。
- 原目录 `/home/ubuntu/keel` 的源码仍在 `keel-new-main` / `978db7f`。5181 批准原型未调整，额外 5182/8082 已停止。
- 用户已授权本次部署；本轮没有数据库迁移、资产导入或主分支合并。后续文档维护不隐含再次部署授权。
- 启动与回退命令见 [DEPLOYMENT](docs/DEPLOYMENT.md) 末节；不要为了接续开发重复启动或重启。

## 已完成及入口

| 页面 | 当前入口 | 实现位置 |
|---|---|---|
| 全局拓扑 | `/topology?view=global` | `dev/src/features/infrastructure/approved/pages/global-topology.tsx` |
| 网络架构 | `/topology?view=network` | 同目录 `network-architecture.tsx` |
| 物理布线 | `/topology?view=cabling` | 同目录 `physical-cabling.tsx` |
| 链路追踪 | `/topology?view=trace` | 同目录 `link-tracing.tsx` |
| 机柜可视化 | `/racks?view=gallery` | 同目录 `rack-detail.tsx` 及 `approved/rack/` |

页面共享范围、URL、缓存由 `approved/ApprovedTopologyWorkspace.tsx` 维护，组件不能各自创造资产身份。服务器与交换机详情继续复用现有抽屉。

## API 与代码定位

首先查 [真实 API 契约总入口](docs/REAL-API-CONTRACTS.md) → [可视化使用契约](docs/api/visualization.md) → [机器注册表](docs/api/real-api-registry.json)。字段权威为 [OpenAPI](docs/openapi.yaml)，不在交接里重复定义 DTO。

- 真实接口：GET subgraph、matrix、connections，POST trace；兼容 GET visualization。完整路径、operationId、参数及错误见上述契约。
- Go 路由：`dev/backend/cmd/api/cmdb.go`；查询 `dev/backend/internal/cmdb/visual_queries.go`，兼容投影 `visualization.go`，位置校验 `visual_placement.go`。
- 前端 Adapter：`dev/src/lib/api/infrastructure.ts`；生成类型 `dev/src/lib/schemas/openapi.ts`。
- CMDB 是唯一资产事实源。布局草稿不修改安装事实；实际位置变更走现有 ChangeSet。物理/逻辑/运行观测分层，缺数据不能伪造连接或正常状态。

## 实际检查与限制

- 已有构建/类型生成、Go 编译及定向纯逻辑检查通过，见 [代码检查记录](docs/topology/code-validation.json)。纯文档收尾不重复运行。
- 已部署四个新接口启动 HTTP 200，五页面加载，无 pageerror 和失败可视化请求；兼容旧接口不在本次四接口探测范围。见 [部署证据](docs/topology/deployment-validation.json)。
- groups 查询瓶颈已修复：单次观察 6568ms → 447ms；未修改超时或数据库配置。这不是生产性能 SLO。
- 未完成真实导入验收、全部关键交互闭环或千卡/大型生产规模基准。真实数据用户后续统一联调，不能把页面加载当作完整验收。
- 逻辑/历史 Trace 缺能力时明确拒绝；物理路径存在不等于已验收通过。规划快照、精确面板、实时监控和 PDU 缺少事实时不制造展示数据。
- BOM 完整自主工具调用 Agent 的历史未完成状态保持，不因本次可视化交付宣称已解决；相关现状查 [BOM 实施状态](docs/BOM-REAL-IMPLEMENTATION-STATUS.md) 和 [工作台设计](docs/BOM-WORKBENCH-V2.md)。

## 后续修改顺序

1. 用户指出要调整的页面/行为后，先查模块契约与 [实施报告](docs/topology/implementation-report.md)，再定向读取涉及的代码。
2. 契约变化同步 OpenAPI、生成类型、Go/Adapter 和注册表；UI 调整保护批准原型及既有身份/权限模型。
3. 只运行受影响的必要检查，更新本文件与证据；不重复全仓 review，不提前导入或合并 main。
4. 文档/登记一致性：`node dev/scripts/check-real-api-docs.mjs`（只读，不调用服务）。

## 2026-10-31 用户要求暂停后的提交

- 仅调整机柜页现有视图按钮，区分“3D 机房 / 机柜阵列 / 单柜工作台”；3D 仍使用已有 `view=gallery&overlay=capacity`，没有新增独立路由、Sidebar 入口或渲染引擎。
- 用户随后纠正方向并要求停止开发、提交；未继续实现独立 3D 页面，后续需求需重新确认。变更位置 `dev/src/features/infrastructure/approved/pages/rack-detail.tsx`。
- TypeScript `tsc --noEmit` 与 `git diff --check` 通过；本次没有浏览器交互验证，没有重启服务或合并 main。5180 使用该 worktree 的 Vite，源文件修改可能已被热更新读取，不应将未重启等同于界面未变化。

## 当前 Flows 重构分支（2026-10-31）

最新任务包见 `docs/flows-redesign/`，实施索引 `docs/flows-redesign/implementation.md`。本任务已隔离到 `/home/ubuntu/keel-flows-dev` / `feat/flows-redesign`，基线 d8a91b4；不要在运行5180的可视化 worktree 继续写本任务。包内指定真实 Kestra/Go/Agent/SSH 落地，覆盖旧 Mock-only 的 Flows 阶段限制；其他业务不扩展。新底座 `keel-flows-*` 独立容器，Kestra 仅127.0.0.1:8095，未替换现有5180/8081，未修改原数据库。


## 作业平台分支交接快照（开发仍暂停）

# Keel 当前开发交接

## 当前任务接续入口：Kestra原生作业平台（已按用户要求暂停）

- **状态**：用户最新要求“记录handoff然后暂停”。已停止后续开发；当前整体完成度约20%（功能估算），不是完整交付。恢复时从本节继续，不重扫全部代码。
- **分支/目录**：`feat/jobs-kestra-ui` / `/home/ubuntu/keel-kestra-ui-dev`。原生列表/时间线`a9ae943`；原生代码/多面板壳层`3df73d7`；QuickRun/模板修复`564a1e5`。本次暂停提交还包含下述Outputs增量。已推送到`enzoOvOyu/keel-new`对应分支；未合并main、未部署。
- **Git推送认证已核实**：使用原项目私钥路径`/home/ubuntu/keel/deploy_key`（公钥`deploy_key.pub`），不是`~/.ssh/id_ed25519`或`keel_github_ed25519`。本仓库local `core.sshCommand`已固定该IdentityFile和IdentitiesOnly；不要打印/复制私钥到Git。当前功能分支及其余本地已提交分支均已成功推送origin，remote仍为`git@github.com:enzoOvOyu/keel-new.git`，main未合并/覆盖。开发仍按用户要求暂停。
- **唯一方案**：[KESTRA-UI-REPLACEMENT.md](docs/flows-redesign/KESTRA-UI-REPLACEMENT.md)，包括用户新增的LC-01…14生命周期范围。直接复用固定Kestra commit `49b62a2c820d37e98a1c7b63aa35fd83f960d4ec` 的Vue组件；Keel保留React外壳、项目/权限/CMDB/审批/Agent与SSH边界。
- **实际完成**：原生design-system与React/Vue桥、样式隔离；两列表基础接线、搜索/状态URL恢复；RunTimeline接线；KsEditor/Monaco与原生多面板壳层（YAML+只读结构）；Outputs原生SidebarList搜索、选择、预览、复制/下载接线。结构摘要不是No-code，Outputs侧栏不是完整变量/表达式调试器。
- **实际验证**：列表构建与三个浏览器检查通过；编辑器构建与三个浏览器检查通过（挂载、键盘输入同步、Visual/YAML切换保留）；Outputs构建与三个浏览器检查通过（挂载、搜索选择更新详情、JSON下载）。Outputs检查仅为浏览器注入的明确synthetic执行，未写业务库，不能当真实引擎产物验收。证据：[native-ui目录](docs/flows-redesign/native-ui/)。
- **已修故障/限制**：编辑器ASI初始化错误已修；一次完整构建137退出，裁减为本期YAML所需generic Worker后通过。JSON/TypeScript语义Worker未启用。编辑器结构面板间距样式在截图后定向修正，完整最终视觉及拖动/合并/最大化/布局恢复仍待验证。旧React详情/Visual仍在过渡中。
- **立即接续事项**：优先完善原生No-code节点/属性与Topology、Files/Docs，接入完整执行详情；同时补列表服务器分页/total/最后执行摘要，避免永远停留在本地分页。已检查上游FlowController/search、QueryFilterFormat源码位置，但尚未编写新目录查询接口。
- **其余未完成**：完整KSFilter、列管理、批量动作；原生Gantt/TaskRun/Attempt/日志/完整变量/指标/依赖；LC并发/排队/补跑/跨执行检索/源码搜索；QuickRun/Agents替换；证书轮换/持久批量安装、设备事件触发、ZTP消费者及真实API契约与联调。不要把软件缺口归类为只等真机。
- **代码入口**：`dev/src/features/jobs-native/`（新Vue模块）；`dev/src/vendor/kestra/`（来源与许可证）；`features/flows/FlowsWorkspace.tsx`（React消费者）；`features/flows/Builder.tsx`（YAML入口）。`dev/scripts/import-kestra-components.py`只从显式提供且commit匹配的上游checkout复制依赖。临时参考checkout `/tmp/keel-kestra-ui-source`不是交付运行依赖。
- **验证脚本/日志**：本机`/tmp/keel-native-ui-check.mjs`、`/tmp/keel-native-editor-check.mjs`、`/tmp/keel-native-outputs-check.mjs`用浏览器拦截提供分支dist，不开新端口；脚本登录读取本机私有环境引用，不得输出/提交秘密。最新构建日志`/tmp/keel-native-outputs-build.log`。恢复时无代码变动不重复已过检查。
- **环境保护**：5180仍由`/home/ubuntu/keel-flows-dev/dev`运行；本分支未切换服务。其他窗口持续更新8081，下面BOM v5段落只是本分支历史，不得当当前二进制。部署前核验负责窗口最新运行版本，不能用本分支旧后端基线覆盖BOM修复。迁移025未由本任务应用，恢复时先检查跨分支迁移编号是否占用。不要停止其他窗口服务、导入资产或恢复旧合成Agent。

以下为历史实施与发布记录，不作为本任务的最新完成状态。

## 作业平台 UI 整改增量 · 2026-10-31

- `feat/flows-redesign` / `/home/ubuntu/keel-flows-dev`：界面统一名「作业平台」，路由/API兼容。Builder、执行详情、筛选、调度参数、Agents等修改见[最新整改记录](docs/flows-redesign/implementation.md)。
- 5180前端由现有Vite加载；**没有替换下方BOM v5后端**。新增执行定义API、分页/时间筛选/同步游标及迁移025均待后端发布，不能直接用本旧基线覆盖8081。
- 契约入口：[真实API](docs/api/flows.md)、OpenAPI FlowsReal及生成类型。剩余证书轮换、受控事件触发和ZTP消费者是代码缺口，不能统称待真机测试。


## 当前运行环境 · BOM v5 与 Flows 合并发布（2026-10-31）

用户已授权发布本次 BOM 根因修复。当前后端源码 **fdffbcb**（在 Flows 部署基线 `6884439` 上集成 `67c7f1d`），独立分支 `deploy/bom-v5-flows-1031`、工作树 `/tmp/keel-bom-v5-deploy`。运行 `/tmp/keel-api-bom-v5-flows-1031`，API `127.0.0.1:8081`；5180前端仍从 `/home/ubuntu/keel-flows-dev/dev` 运行。没有替换前端、Flows依赖、数据库或配置。该节覆盖下方旧运行版本及“未部署”的历史记录。

- 已确认运行二进制的源码版本/摘要，进程环境与发布前逐字节一致，工作目录保持 `/home/ubuntu/keel/dev/backend`。
- 集成版 API/AI/BOM/CMDB/Flows 定向测试与构建通过，18 项启动/只读检查通过（含登录、BOM进度、拓扑矩阵/物理追踪及Flows）。无活动任务，正式提交/资产数量前后不变。
- 本次未运行新模型识别、真实文件入库或浏览器绘图验收。已有批次的旧候选不会因更换二进制自动重算；需从工作台按用户验收安排继续分析。
- 启动器 `/tmp/keel-bom-v5-run-api.py`；回退启动器 `/tmp/keel-bom-v5-rollback-api.py` 恢复原 `/tmp/keel-api-flows-combined-1031`。私有运行环境快照位于 `/tmp/keel-bom-v5-deployment-20261031/`，不得打印或入Git。
- 后续构建/发布必须包含 `67c7f1d` 或等价集成 `fdffbcb`；不要直接用未集成这项修复的旧工作树后端覆盖当前8081。

发布证据：[bom-v5-deployment-20261031.json](docs/topology/bom-v5-deployment-20261031.json)。

更新：2026-10-31 UTC。先读本文件，再按下列索引定位；无需重扫全仓。历史交接可用 `git show 6601444:HANDOFF.md` 查阅。

## 当前已发布环境 · 2026-10-31（覆盖下方开发阶段记录）

- 用户现已授权发布；入口恢复统一为 **http://localhost:5180/flows**，API仍`127.0.0.1:8081`。临时5190/8096与合成本地Agent已关闭；不要重新启动它们。
- 运行源码 **9f16065**，分支`feat/flows-redesign`、目录`/home/ubuntu/keel-flows-dev`，未合并或推送main。4403c63/9f16065保留原8081的两项BOM Agent修复，相关BOM/AI文件与原部署分支比对无差异。
- 前端tmux`keel-dev-5180`从本工作区`dev/`启动；API tmux`keel-cmdb-api`运行`/tmp/keel-api-flows-combined-1031`，工作目录继续为`/home/ubuntu/keel/dev/backend`，保留原数据库、对象存储、凭据路径和账号。
- 原业务库新增迁移021…024，未装载任何隔离合成资产。备份位置见`/tmp/keel-flows-deploy-backup-current`，私有目录含部署前完整pg_dump与原进程环境，不得打印或入Git。
- API启动器`/tmp/keel-flows-run-deployed-api.py`读取私有原环境备份和`dev/deploy/flows/.env.deployed`覆盖；后者只配置引擎/Agent/SSH受控路径，不覆盖原DATABASE_URL或密码。Worker经Docker bridge调用现有5180代理，不新增用户入口。
- Kestra8095与Agent网关8097/TLS注册8098是本机后端依赖，不是新增前端入口；真实设备公网TLS/Bootstrap仍需部署条件。
- [发布检查](docs/flows-redesign/deployment-validation.json)：原admin登录、5个Flows只读API、3页面渲染共9项通过；容器到5180健康200、无票据worker回调403。没有设备执行、BOM AI调用或真实数据导入。
- 应用回退：停当前两个tmux服务，使用备份中原API环境恢复`/tmp/keel-api-agent-topology-1031`（同原cwd），前端从`/home/ubuntu/keel-visualization-dev/dev`恢复原5180命令。新增表不自动drop，数据库无需为了旧应用读取而破坏性回滚；完整dump只用于独立授权的数据恢复。

以下Flows独立环境描述为部署前记录，当前端口和运行版本以上述发布节为准。

## 2026-10-31 当前任务：Flows 全栈功能分支（先读）

- 工作区 **`/home/ubuntu/keel-flows-dev`**，分支 **`feat/flows-redesign`**；代码提交`63ca2c9`、`826b964`、开发补齐`afe8cb9`。没有合并/推送main，也没有更改原5180/8081运行服务。
- 任务包原文及八页draw.io在`docs/flows-redesign/`；优先读[实施记录](docs/flows-redesign/implementation.md)与[真实API契约](docs/api/flows.md)，无需重扫旧Flows或全仓。
- 独立前端 **http://127.0.0.1:5190/flows**，API `172.22.0.1:8096`，Kestra `127.0.0.1:8095`，gRPC `localhost:8097`，TLS注册`https://localhost:8098`。远程浏览需端口转发；没有发布到原5180。
- Docker项目`keel-flows`，Postgres中分开`kestra`与`keel_flows_dev`。后者仅原库schema+合成项目/资产，未复制原业务行；迁移021…024只应用这里。不要运行写死原容器的`backend/migrate.sh`。
- 私有配置/新隔离admin密码：`dev/deploy/flows/.env.local`，0600且忽略；原账号密码未改。PKI在同目录`pki/`。API只复用原`.env`的对象存储配置，DATABASE_URL由隔离配置覆盖，禁止输出秘密。
- API `/tmp/keel-flows-api`，PID `/tmp/keel-flows-api.pid`，日志同名`.log`；Agent `/tmp/keel-flows-agent`，PID同名`.pid`，state `/tmp/keel-flows-agent-state`。不要删除state或重复注册。重启前核验PID的exe，不要kill原8081。
- 实际通过：Go定向测试/构建、TypeScript+生产构建、真实Kestra+双合成SSH协议失败重试/幂等、实际非rootAgent/mTLS实时输出、原生Pause/Resume、文件读写、版本冲突、游标与跨项目拒绝、8项浏览器交互。
- 本轮已补齐：Kestra驱动的作用域批次调度、每目标多步骤与输出传递、二进制版本/分发、持久事件游标重放、既有审批与单次执行绑定、实际任务取消。新增API/迁移/测试证据见实施记录。
- 能力边界：每目标32个串行依赖步骤、步骤结果32KiB、二进制资源1MiB；Agent0.2.0支持job.pipeline。待验收仅生产Bootstrap/NOS/真实8目标混合/规模与长期运行，不得宣称生产通过。测试Cron已停用。
- 修改定位：`dev/src/features/flows/`、`dev/src/lib/api/flows.ts`、`dev/backend/internal/flows/`、`cmd/{api/flows.go,agent/}`、迁移021…024、OpenAPI FlowsReal。规则和运行证据链接均见实施记录。

以下为原BOM/拓扑环境的保留历史，勿把运行版本与当前Flows分支混用。

## 2026-10-31 后端 Agent 部署更新（优先于下方历史版本）

当前8081已切换为 `/tmp/keel-api-agent-topology-1031`，源码 `ad8918b`，来自独立集成分支 `deploy/bom-agent-topology-1031`（工作区 `/tmp/keel-agent-deploy`）。该版本保留拓扑基线 b184645，并集成 Astra 的 e3f6cd3、74ab238 Agent 修复。启动配置仍使用 `/home/ubuntu/keel/dev/backend/.env`，前端5180仍使用本工作区。本分支没有合入这些后端代码，后续重新部署时必须保留集成修复，不能直接从旧后端源码重建覆盖。

定向 Go 检查、构建和部署启动检查通过；没有新启动真实识别/入库验收。证据见 `docs/topology/agent-deployment-20261031.json`；旧二进制 `/tmp/keel-api-4541850` 保留用于回退。下方4541850部署描述是历史状态。

## 当前分支与运行环境

- 仓库目标：`enzoOvOyu/keel-new`。工作区 `/home/ubuntu/keel-visualization-dev`，分支 `feat/datacenter-visualization-approved`；未合并 main。
- 已部署业务代码 `4541850`；后续交接/契约文档提交不改变运行代码。
- 前端 http://localhost:5180 ，tmux `keel-dev-5180`，目录为本工作区 `dev/`。
- 后端 `127.0.0.1:8081`，tmux `keel-cmdb-api`，运行 `/tmp/keel-api-4541850`。配置和存储工作目录仍为 `/home/ubuntu/keel/dev/backend`，不要复制或输出 `.env`。
- 原目录 `/home/ubuntu/keel` 的源码仍在 `keel-new-main` / `978db7f`。5181 批准原型未调整，额外 5182/8082 已停止。
- 用户已授权本次部署；本轮没有数据库迁移、资产导入或主分支合并。后续文档维护不隐含再次部署授权。
- 启动与回退命令见 [DEPLOYMENT](docs/DEPLOYMENT.md) 末节；不要为了接续开发重复启动或重启。

## 已完成及入口

| 页面 | 当前入口 | 实现位置 |
|---|---|---|
| 全局拓扑 | `/topology?view=global` | `dev/src/features/infrastructure/approved/pages/global-topology.tsx` |
| 网络架构 | `/topology?view=network` | 同目录 `network-architecture.tsx` |
| 物理布线 | `/topology?view=cabling` | 同目录 `physical-cabling.tsx` |
| 链路追踪 | `/topology?view=trace` | 同目录 `link-tracing.tsx` |
| 机柜可视化 | `/racks?view=gallery` | 同目录 `rack-detail.tsx` 及 `approved/rack/` |

页面共享范围、URL、缓存由 `approved/ApprovedTopologyWorkspace.tsx` 维护，组件不能各自创造资产身份。服务器与交换机详情继续复用现有抽屉。

## API 与代码定位

首先查 [真实 API 契约总入口](docs/REAL-API-CONTRACTS.md) → [可视化使用契约](docs/api/visualization.md) → [机器注册表](docs/api/real-api-registry.json)。字段权威为 [OpenAPI](docs/openapi.yaml)，不在交接里重复定义 DTO。

- 真实接口：GET subgraph、matrix、connections，POST trace；兼容 GET visualization。完整路径、operationId、参数及错误见上述契约。
- Go 路由：`dev/backend/cmd/api/cmdb.go`；查询 `dev/backend/internal/cmdb/visual_queries.go`，兼容投影 `visualization.go`，位置校验 `visual_placement.go`。
- 前端 Adapter：`dev/src/lib/api/infrastructure.ts`；生成类型 `dev/src/lib/schemas/openapi.ts`。
- CMDB 是唯一资产事实源。布局草稿不修改安装事实；实际位置变更走现有 ChangeSet。物理/逻辑/运行观测分层，缺数据不能伪造连接或正常状态。

## 实际检查与限制

- 已有构建/类型生成、Go 编译及定向纯逻辑检查通过，见 [代码检查记录](docs/topology/code-validation.json)。纯文档收尾不重复运行。
- 已部署四个新接口启动 HTTP 200，五页面加载，无 pageerror 和失败可视化请求；兼容旧接口不在本次四接口探测范围。见 [部署证据](docs/topology/deployment-validation.json)。
- groups 查询瓶颈已修复：单次观察 6568ms → 447ms；未修改超时或数据库配置。这不是生产性能 SLO。
- 未完成真实导入验收、全部关键交互闭环或千卡/大型生产规模基准。真实数据用户后续统一联调，不能把页面加载当作完整验收。
- 逻辑/历史 Trace 缺能力时明确拒绝；物理路径存在不等于已验收通过。规划快照、精确面板、实时监控和 PDU 缺少事实时不制造展示数据。
- BOM 完整自主工具调用 Agent 的历史未完成状态保持，不因本次可视化交付宣称已解决；相关现状查 [BOM 实施状态](docs/BOM-REAL-IMPLEMENTATION-STATUS.md) 和 [工作台设计](docs/BOM-WORKBENCH-V2.md)。

## 后续修改顺序

1. 用户指出要调整的页面/行为后，先查模块契约与 [实施报告](docs/topology/implementation-report.md)，再定向读取涉及的代码。
2. 契约变化同步 OpenAPI、生成类型、Go/Adapter 和注册表；UI 调整保护批准原型及既有身份/权限模型。
3. 只运行受影响的必要检查，更新本文件与证据；不重复全仓 review，不提前导入或合并 main。
4. 文档/登记一致性：`node dev/scripts/check-real-api-docs.mjs`（只读，不调用服务）。

## 2026-10-31 用户要求暂停后的提交

- 仅调整机柜页现有视图按钮，区分“3D 机房 / 机柜阵列 / 单柜工作台”；3D 仍使用已有 `view=gallery&overlay=capacity`，没有新增独立路由、Sidebar 入口或渲染引擎。
- 用户随后纠正方向并要求停止开发、提交；未继续实现独立 3D 页面，后续需求需重新确认。变更位置 `dev/src/features/infrastructure/approved/pages/rack-detail.tsx`。
- TypeScript `tsc --noEmit` 与 `git diff --check` 通过；本次没有浏览器交互验证，没有重启服务或合并 main。5180 使用该 worktree 的 Vite，源文件修改可能已被热更新读取，不应将未重启等同于界面未变化。

## 当前 Flows 重构分支（2026-10-31）

最新任务包见 `docs/flows-redesign/`，实施索引 `docs/flows-redesign/implementation.md`。本任务已隔离到 `/home/ubuntu/keel-flows-dev` / `feat/flows-redesign`，基线 d8a91b4；不要在运行5180的可视化 worktree 继续写本任务。包内指定真实 Kestra/Go/Agent/SSH 落地，覆盖旧 Mock-only 的 Flows 阶段限制；其他业务不扩展。新底座 `keel-flows-*` 独立容器，Kestra 仅127.0.0.1:8095，未替换现有5180/8081，未修改原数据库。

## 作业平台最新设计方向：Kestra原生UI替换方案
- 新分支 `feat/jobs-kestra-ui`、独立目录 `/home/ubuntu/keel-kestra-ui-dev`；现有5180/8081未切换到新工作区。
- 用户要求先提交替换方案，见[Kestra UI替换](docs/flows-redesign/KESTRA-UI-REPLACEMENT.md)。原生Vue组件复用为推荐路线，当前仅源码审查及设计，尚未实施界面替换。
- 分支携带Quick Run版本冻结、完整原生模板转换、Files脚本载入等未部署修复；之前的构建/定向测试通过不代表原生Vue模块已验收。
- 用户进一步要求纳入Kestra中有用的遗漏信息页；替换方案已增加LC-01…14：执行时间线/Attempt、恢复重跑、变量文件、指标、依赖、Flow日志/趋势/并发、调度补跑、异常等待视图、全局日志、源码Docs及历史导航。源码中的Enterprise锁定demo页已区分，不计为可直接复用的真实能力；均为方案增补，尚未实现。

## 原生UI实施起点（开发中）
- 用户已从方案阶段转为授权实施。新分支原生组件/两列表代码在`dev/src/features/jobs-native`，来源组件在`dev/src/vendor/kestra`，React消费者仍是`features/flows/FlowsWorkspace.tsx`。只在新工作区改动，原运行环境未切换。
- 构建和浏览器两列表挂载已通过；检查用浏览器拦截提供新dist，不启动新端口或写入资产/作业数据。尚未接原生编辑器和LC生命周期页面。具体当前限制见flows-redesign实施记录末节，不用旧“全部完成”描述。


## 历史分支记录：feat/job-platform-rebuild

# Keel 当前开发交接

更新：2026-10-31 UTC。先读本文件，再按下列索引定位；无需重扫全仓。历史交接可用 `git show 6601444:HANDOFF.md` 查阅。

## 2026-10-31 两份真实工作簿五轮 API 验收（最新测试状态）

用户重新授权测试，并要求以 API 为主、每份文件5次。本轮在已部署的 ad8918b 上沿用两套独立项目，完成首轮整批重分析及同七组来源坐标的四轮复测，共10个v4识别任务，现已停止追加识别。未修改业务代码，未新增截图或正式资产。

**结论：BOM→CMDB→拓扑闭环未通过。** bomv1首轮出现1296个重复光模块候选ID，后续清零；Cabling_NS第3行的线缆候选数量为1→2→2→1→1。两文件完整分析首轮均未识别189条交换机来源，后续仅选定行逐步识别。两项目均无有效依赖闭包，提交/CMDB资产/Outbox均为0，不能把空拓扑API的200当作导入后图表通过。

门禁负向检查正常；现有合成项目的705条线缆、矩阵成员/分页、直接物理追踪和跨项目引用隔离对照通过。该对照不能替代真实工作簿闭环。原IR均15 Sheet/162715单元格，五轮前后摘要不变。两个项目各保留1条来源核实后确认的草稿端口，未正式提交；测试项目和批次ID、问题清单及清理指针见 `docs/topology/bom-import-acceptance-20261031.json`。完整私有证据在 `/tmp/keel-bom-topology-test-20261031/fixed-agent/REPORT.json`。不继续修复或追加测试，等待用户下一项安排。

## 2026-10-31 后端 Agent 部署更新（优先于下方历史版本）

当前8081已切换为 `/tmp/keel-api-agent-topology-1031`，源码 `ad8918b`，来自独立集成分支 `deploy/bom-agent-topology-1031`（工作区 `/tmp/keel-agent-deploy`）。该版本保留拓扑基线 b184645，并集成 Astra 的 e3f6cd3、74ab238 Agent 修复。启动配置仍使用 `/home/ubuntu/keel/dev/backend/.env`，前端5180仍使用本工作区。本分支没有合入这些后端代码，后续重新部署时必须保留集成修复，不能直接从旧后端源码重建覆盖。

定向 Go 检查、构建和部署启动检查通过；没有新启动真实识别/入库验收。证据见 `docs/topology/agent-deployment-20261031.json`；旧二进制 `/tmp/keel-api-4541850` 保留用于回退。下方4541850部署描述是历史状态。

## 当前分支与运行环境

- 仓库目标：`enzoOvOyu/keel-new`。工作区 `/home/ubuntu/keel-visualization-dev`，分支 `feat/datacenter-visualization-approved`；未合并 main。
- 已部署业务代码 `4541850`；后续交接/契约文档提交不改变运行代码。
- 前端 http://localhost:5180 ，tmux `keel-dev-5180`，目录为本工作区 `dev/`。
- 后端 `127.0.0.1:8081`，tmux `keel-cmdb-api`，运行 `/tmp/keel-api-4541850`。配置和存储工作目录仍为 `/home/ubuntu/keel/dev/backend`，不要复制或输出 `.env`。
- 原目录 `/home/ubuntu/keel` 的源码仍在 `keel-new-main` / `978db7f`。5181 批准原型未调整，额外 5182/8082 已停止。
- 用户已授权本次部署；本轮没有数据库迁移、资产导入或主分支合并。后续文档维护不隐含再次部署授权。
- 启动与回退命令见 [DEPLOYMENT](docs/DEPLOYMENT.md) 末节；不要为了接续开发重复启动或重启。

## 已完成及入口

| 页面 | 当前入口 | 实现位置 |
|---|---|---|
| 全局拓扑 | `/topology?view=global` | `dev/src/features/infrastructure/approved/pages/global-topology.tsx` |
| 网络架构 | `/topology?view=network` | 同目录 `network-architecture.tsx` |
| 物理布线 | `/topology?view=cabling` | 同目录 `physical-cabling.tsx` |
| 链路追踪 | `/topology?view=trace` | 同目录 `link-tracing.tsx` |
| 机柜可视化 | `/racks?view=gallery` | 同目录 `rack-detail.tsx` 及 `approved/rack/` |

页面共享范围、URL、缓存由 `approved/ApprovedTopologyWorkspace.tsx` 维护，组件不能各自创造资产身份。服务器与交换机详情继续复用现有抽屉。

## API 与代码定位

首先查 [真实 API 契约总入口](docs/REAL-API-CONTRACTS.md) → [可视化使用契约](docs/api/visualization.md) → [机器注册表](docs/api/real-api-registry.json)。字段权威为 [OpenAPI](docs/openapi.yaml)，不在交接里重复定义 DTO。

- 真实接口：GET subgraph、matrix、connections，POST trace；兼容 GET visualization。完整路径、operationId、参数及错误见上述契约。
- Go 路由：`dev/backend/cmd/api/cmdb.go`；查询 `dev/backend/internal/cmdb/visual_queries.go`，兼容投影 `visualization.go`，位置校验 `visual_placement.go`。
- 前端 Adapter：`dev/src/lib/api/infrastructure.ts`；生成类型 `dev/src/lib/schemas/openapi.ts`。
- CMDB 是唯一资产事实源。布局草稿不修改安装事实；实际位置变更走现有 ChangeSet。物理/逻辑/运行观测分层，缺数据不能伪造连接或正常状态。

## 实际检查与限制

- 已有构建/类型生成、Go 编译及定向纯逻辑检查通过，见 [代码检查记录](docs/topology/code-validation.json)。纯文档收尾不重复运行。
- 已部署四个新接口启动 HTTP 200，五页面加载，无 pageerror 和失败可视化请求；兼容旧接口不在本次四接口探测范围。见 [部署证据](docs/topology/deployment-validation.json)。
- groups 查询瓶颈已修复：单次观察 6568ms → 447ms；未修改超时或数据库配置。这不是生产性能 SLO。
- 未完成真实导入验收、全部关键交互闭环或千卡/大型生产规模基准。真实数据用户后续统一联调，不能把页面加载当作完整验收。
- 逻辑/历史 Trace 缺能力时明确拒绝；物理路径存在不等于已验收通过。规划快照、精确面板、实时监控和 PDU 缺少事实时不制造展示数据。
- BOM 完整自主工具调用 Agent 的历史未完成状态保持，不因本次可视化交付宣称已解决；相关现状查 [BOM 实施状态](docs/BOM-REAL-IMPLEMENTATION-STATUS.md) 和 [工作台设计](docs/BOM-WORKBENCH-V2.md)。

## 后续修改顺序

1. 用户指出要调整的页面/行为后，先查模块契约与 [实施报告](docs/topology/implementation-report.md)，再定向读取涉及的代码。
2. 契约变化同步 OpenAPI、生成类型、Go/Adapter 和注册表；UI 调整保护批准原型及既有身份/权限模型。
3. 只运行受影响的必要检查，更新本文件与证据；不重复全仓 review，不提前导入或合并 main。
4. 文档/登记一致性：`node dev/scripts/check-real-api-docs.mjs`（只读，不调用服务）。

## 2026-10-31 用户要求暂停后的提交

- 仅调整机柜页现有视图按钮，区分“3D 机房 / 机柜阵列 / 单柜工作台”；3D 仍使用已有 `view=gallery&overlay=capacity`，没有新增独立路由、Sidebar 入口或渲染引擎。
- 用户随后纠正方向并要求停止开发、提交；未继续实现独立 3D 页面，后续需求需重新确认。变更位置 `dev/src/features/infrastructure/approved/pages/rack-detail.tsx`。
- TypeScript `tsc --noEmit` 与 `git diff --check` 通过；本次没有浏览器交互验证，没有重启服务或合并 main。5180 使用该 worktree 的 Vite，源文件修改可能已被热更新读取，不应将未重启等同于界面未变化。


## 历史分支记录：fix/bom-acceptance-root-causes

# Keel 当前开发交接


## 当前运行环境 · BOM v5 与 Flows 合并发布（2026-10-31）

用户已授权发布本次 BOM 根因修复。当前后端源码 **fdffbcb**（在 Flows 部署基线 `6884439` 上集成 `67c7f1d`），独立分支 `deploy/bom-v5-flows-1031`、工作树 `/tmp/keel-bom-v5-deploy`。运行 `/tmp/keel-api-bom-v5-flows-1031`，API `127.0.0.1:8081`；5180前端仍从 `/home/ubuntu/keel-flows-dev/dev` 运行。没有替换前端、Flows依赖、数据库或配置。该节覆盖下方旧运行版本及“未部署”的历史记录。

- 已确认运行二进制的源码版本/摘要，进程环境与发布前逐字节一致，工作目录保持 `/home/ubuntu/keel/dev/backend`。
- 集成版 API/AI/BOM/CMDB/Flows 定向测试与构建通过，18 项启动/只读检查通过（含登录、BOM进度、拓扑矩阵/物理追踪及Flows）。无活动任务，正式提交/资产数量前后不变。
- 本次未运行新模型识别、真实文件入库或浏览器绘图验收。已有批次的旧候选不会因更换二进制自动重算；需从工作台按用户验收安排继续分析。
- 启动器 `/tmp/keel-bom-v5-run-api.py`；回退启动器 `/tmp/keel-bom-v5-rollback-api.py` 恢复原 `/tmp/keel-api-flows-combined-1031`。私有运行环境快照位于 `/tmp/keel-bom-v5-deployment-20261031/`，不得打印或入Git。
- 后续构建/发布必须包含 `67c7f1d` 或等价集成 `fdffbcb`；不要直接用未集成这项修复的旧工作树后端覆盖当前8081。

发布证据：[bom-v5-deployment-20261031.json](docs/topology/bom-v5-deployment-20261031.json)。


## 2026-10-31 · 五轮 BOM 验收缺陷根因修复（当前）

用户已重新授权彻底修复已发现问题，覆盖下方历史暂停开发记录。代码在独立分支 `fix/bom-acceptance-root-causes`，工作树 `/tmp/keel-bom-root-fix`，基于已部署的 Agent+Topology 集成代码；未修改其他开发者分支，未部署、未重启服务。

- 修复整批/局部识别状态混淆、表头修正后的行资格丢失、错误字段语义、重复光模块/候选 ID、合并后的引用悬空及第一次关联不稳定。诊断版本 `workbook-agent/v5`。
- 两份原始 XLSX 离线回放各得到 189 条交换机、7,154 条布线候选，其中 7,150 条关联两端物理端口；4 条来源依赖缺失仍阻断。162,715 个源单元格全部保留，NS 第 3 行保持一条线缆，图候选集合重放稳定。
- 现有 GPU/NIC/BMC/PSU 等物理部件可以拥有来源明确的端口；拓扑保留独立部件归属，有上级设备时优先聚合到设备，歧义和跨项目访问继续阻断。没有把部件改成服务器。
- P2P-18 旧方案不满足物理端口契约，被拒绝作为线缆；估算/数量信息完整保留，仍需正确参考/物料方案和人工处置。型号审批、单位、尺寸及原始缺失信息未被补造。
- Go API/AI/BOM/CMDB 定向检查、两文件本地回放、PostgreSQL 投影幂等/完整拓扑读取/部件归属隔离检查通过。新增合成测试位于独立项目的回滚事务中，无持久化测试资产。
- 本轮无新外部模型调用、无真实文件正式提交、无浏览器绘图验收。不能把离线恢复结果当作新模型准确率或上线验收。

逐问题记录：[bom-root-fixes-20261031.json](docs/topology/bom-root-fixes-20261031.json)。到此结束开发，后续部署/集中验收按用户安排。

更新：2026-10-31 UTC。先读本文件，再按下列索引定位；无需重扫全仓。历史交接可用 `git show 6601444:HANDOFF.md` 查阅。

## 2026-10-31 Agent 修复已部署（最新运行状态）

用户明确授权将此前 Agent 修复部署到当前拓扑服务。后端现运行 `/tmp/keel-api-agent-topology-1031`，业务源码 `ad8918b`，独立集成分支 `deploy/bom-agent-topology-1031`、工作区 `/tmp/keel-agent-deploy`；在拓扑基线 `b184645` 上集成原修复 `e3f6cd3`、`74ab238`。8081、原配置/受控秘密工作目录和5180前端保持原入口。下方 `4541850` 二进制信息作为历史记录，已被本段覆盖。

定向 Go API/AI/BOM/CMDB 检查和构建通过；健康、登录、拓扑 subgraph/matrix/connections/兼容查询均通过启动检查，缺失端口 trace 返回404。切换前活动BOM任务为0。没有启动新识别、入库或完整业务验收；此前旧版本首轮测试不能充当修复版验收。证据见 `docs/topology/agent-deployment-20261031.json`。回退后端可恢复 `/tmp/keel-api-4541850`，无需数据库回退。

## 当前分支与运行环境

- 仓库目标：`enzoOvOyu/keel-new`。工作区 `/home/ubuntu/keel-visualization-dev`，分支 `feat/datacenter-visualization-approved`；未合并 main。
- 已部署业务代码 `4541850`；后续交接/契约文档提交不改变运行代码。
- 前端 http://localhost:5180 ，tmux `keel-dev-5180`，目录为本工作区 `dev/`。
- 后端 `127.0.0.1:8081`，tmux `keel-cmdb-api`，运行 `/tmp/keel-api-4541850`。配置和存储工作目录仍为 `/home/ubuntu/keel/dev/backend`，不要复制或输出 `.env`。
- 原目录 `/home/ubuntu/keel` 的源码仍在 `keel-new-main` / `978db7f`。5181 批准原型未调整，额外 5182/8082 已停止。
- 用户已授权本次部署；本轮没有数据库迁移、资产导入或主分支合并。后续文档维护不隐含再次部署授权。
- 启动与回退命令见 [DEPLOYMENT](docs/DEPLOYMENT.md) 末节；不要为了接续开发重复启动或重启。

## 已完成及入口

| 页面 | 当前入口 | 实现位置 |
|---|---|---|
| 全局拓扑 | `/topology?view=global` | `dev/src/features/infrastructure/approved/pages/global-topology.tsx` |
| 网络架构 | `/topology?view=network` | 同目录 `network-architecture.tsx` |
| 物理布线 | `/topology?view=cabling` | 同目录 `physical-cabling.tsx` |
| 链路追踪 | `/topology?view=trace` | 同目录 `link-tracing.tsx` |
| 机柜可视化 | `/racks?view=gallery` | 同目录 `rack-detail.tsx` 及 `approved/rack/` |

页面共享范围、URL、缓存由 `approved/ApprovedTopologyWorkspace.tsx` 维护，组件不能各自创造资产身份。服务器与交换机详情继续复用现有抽屉。

## API 与代码定位

首先查 [真实 API 契约总入口](docs/REAL-API-CONTRACTS.md) → [可视化使用契约](docs/api/visualization.md) → [机器注册表](docs/api/real-api-registry.json)。字段权威为 [OpenAPI](docs/openapi.yaml)，不在交接里重复定义 DTO。

- 真实接口：GET subgraph、matrix、connections，POST trace；兼容 GET visualization。完整路径、operationId、参数及错误见上述契约。
- Go 路由：`dev/backend/cmd/api/cmdb.go`；查询 `dev/backend/internal/cmdb/visual_queries.go`，兼容投影 `visualization.go`，位置校验 `visual_placement.go`。
- 前端 Adapter：`dev/src/lib/api/infrastructure.ts`；生成类型 `dev/src/lib/schemas/openapi.ts`。
- CMDB 是唯一资产事实源。布局草稿不修改安装事实；实际位置变更走现有 ChangeSet。物理/逻辑/运行观测分层，缺数据不能伪造连接或正常状态。

## 实际检查与限制

- 已有构建/类型生成、Go 编译及定向纯逻辑检查通过，见 [代码检查记录](docs/topology/code-validation.json)。纯文档收尾不重复运行。
- 已部署四个新接口启动 HTTP 200，五页面加载，无 pageerror 和失败可视化请求；兼容旧接口不在本次四接口探测范围。见 [部署证据](docs/topology/deployment-validation.json)。
- groups 查询瓶颈已修复：单次观察 6568ms → 447ms；未修改超时或数据库配置。这不是生产性能 SLO。
- 未完成真实导入验收、全部关键交互闭环或千卡/大型生产规模基准。真实数据用户后续统一联调，不能把页面加载当作完整验收。
- 逻辑/历史 Trace 缺能力时明确拒绝；物理路径存在不等于已验收通过。规划快照、精确面板、实时监控和 PDU 缺少事实时不制造展示数据。
- BOM 完整自主工具调用 Agent 的历史未完成状态保持，不因本次可视化交付宣称已解决；相关现状查 [BOM 实施状态](docs/BOM-REAL-IMPLEMENTATION-STATUS.md) 和 [工作台设计](docs/BOM-WORKBENCH-V2.md)。

## 后续修改顺序

1. 用户指出要调整的页面/行为后，先查模块契约与 [实施报告](docs/topology/implementation-report.md)，再定向读取涉及的代码。
2. 契约变化同步 OpenAPI、生成类型、Go/Adapter 和注册表；UI 调整保护批准原型及既有身份/权限模型。
3. 只运行受影响的必要检查，更新本文件与证据；不重复全仓 review，不提前导入或合并 main。
4. 文档/登记一致性：`node dev/scripts/check-real-api-docs.mjs`（只读，不调用服务）。

## 2026-10-31 用户要求暂停后的提交

- 仅调整机柜页现有视图按钮，区分“3D 机房 / 机柜阵列 / 单柜工作台”；3D 仍使用已有 `view=gallery&overlay=capacity`，没有新增独立路由、Sidebar 入口或渲染引擎。
- 用户随后纠正方向并要求停止开发、提交；未继续实现独立 3D 页面，后续需求需重新确认。变更位置 `dev/src/features/infrastructure/approved/pages/rack-detail.tsx`。
- TypeScript `tsc --noEmit` 与 `git diff --check` 通过；本次没有浏览器交互验证，没有重启服务或合并 main。5180 使用该 worktree 的 Vite，源文件修改可能已被热更新读取，不应将未重启等同于界面未变化。
