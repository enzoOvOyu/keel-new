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
