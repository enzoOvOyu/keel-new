# keel Implementation Plan

## 1. 执行规则

- 以 `PRD.md` 为业务验收依据、`SYSTEM-DESIGN.md` 为领域流程依据、`ARCHITECTURE.md` 为技术边界、`DESIGN-SYSTEM.md` 为前端规范、`API-CONVENTIONS.md` 为接口唯一契约。
- 先检查仓库现状、复用可用代码，不先重写无关模块；不要反复全仓 review 或把工作过度拆碎。
- 每个阶段只做与阶段目标相关的测试；完成后输出改动文件、关键实现、验证命令与未解决阻塞。
- 不伪造真实硬件验证。无设备时使用 `sim/device-sim` 和 golden contract；不可模拟的能力标 `unknown`。
- 任何未定义的权限、安全、删除、状态、性能和数据一致性规则，先登记决策，不以 TODO 绕过门禁。

## 2. P0 — 全量前端骨架、API 契约与 Mock Runtime（先做）

**目标**：用户可在浏览器中完整点选所有主要模块；每个页面消费统一 API 接口，数据由 mock provider 提供。

1. 建 React + TypeScript + Vite + shadcn/ui 应用壳、响应式 Sidebar/Header、项目上下文、路由、面包屑、权限占位。
2. 建 `api` interfaces + OpenAPI TypeScript types + `ApiProvider`；提供 `mock` / `http` 两种 provider，通过环境变量切换。
3. 先补全全平台 API 契约（BOM、CMDB、资产、Flow、Executions、日志、任务、网络底座、PXE、六阶段交付主链路、工单、监控、模板、介质、参数、审计、项目/权限）。
4. 建统一 DataTable、StatusBadge、IssuePanel、DiffViewer、SourceCellViewer、EvidenceList、ExecutionTimeline、LogViewer、TargetMatrix、ConfirmDialog。
5. 根据 DESIGN-SYSTEM 和 UI-REFERENCE 完成全部主菜单页面的 mock 版；模拟成功、空、错误、权限拒绝、部分失败、未知/过期数据和长任务状态。
6. 不接真实设备，不写生产执行后端；mock 任务按固定状态时间线可手动暂停、继续、重试、终止（明确标记 mock）。

**完成标准**：全导航无死链；所有主操作可交互；组件无散落硬编码假数据；所有页面通过统一 API provider 取数据；OpenAPI lint、TS 类型检查和前端 build 通过；桌面端关键页面视觉检查通过。

## 3. P1 — 真实 BOM 导入闭环（前端先行后接 Go）

1. 依据 `EXCEL-IMPORT-PROFILE.md` 实现工作簿解析：多 Sheet、非统一表头、横向 P2P 表、设备资产表、OOB 地址表、机柜布局网格和汇总表分型。
2. 原始文件安全保存、hash、单元格溯源；识别敏感列并禁止将密码/令牌传给 LLM 或写入输出。
3. 映射优先级：已批准项目模板规则 → 确定性解析器 → AI 候选；AI 不直接提交 BOM。
4. 建标准实体候选、三段校验、人工审核、修正记录、标准 BOM 包预览和确认提交。
5. 提交统一 `bom_item` 并在同一事务写 Outbox；CMDB 记录/关系由消费者幂等创建。失败可重放，不能出现 BOM 成功但 CMDB 永久缺失。
6. 用真实工作簿制作脱敏后的测试样本与人工标准答案；原文件不进入仓库，不复制明文凭据。

**完成标准**：所有识别项可回溯至文件/Sheet/单元格；未知列不丢弃；关键歧义阻断；密码不进入日志、AI 请求、mock fixture、CMDB 或导出；server/fabric 同批次提交；CMDB 自动关联导入源和标准物料。

## 4. P2 — Go 后端基础与 CMDB 全链投影

1. Go API、PostgreSQL migrations、对象存储、身份/RBAC、审计、统一错误、request_id。
2. 定义 CMDB 权威实体、关系、事件与来源追溯；各业务域写自身权威表，事务 Outbox 产生领域事件，CMDB projector 幂等更新视图。
3. 统一查询 API 支持资产详情、关系图、来源链、生命周期时间线和数据新鲜度。
4. 接入前端 `http` provider，保留同一接口类型和页面行为。

**完成标准**：迁移可重复部署；事件重放不产生重复记录；权限隔离生效；BOM、采集、执行、门禁、工单等记录均可追溯至 CMDB。

## 5. P3 — 纳管资产与批量脚本工作流（Kestra 风格）

1. 实现已纳管目标查询与分组：服务器/交换机、项目、机柜、角色、标签、状态；执行前冻结目标快照。
2. Flow 以“脚本/命令 + 目标选择 + 参数 + 频率/触发 + 超时/重试 + 并发/失败策略”为核心；支持一次执行、定时执行、启停定时器、版本历史。
3. Execution 页面参照 Kestra：Overview/Gantt、Topology、Targets 矩阵、Logs、Outputs、关联对象；显示状态、频次、生命周期、开始/结束、耗时、重试 attempt、退出码和失败原因。
4. 实现 Pause/Resume/Kill/Retry Failed Targets；执行状态以 DB 为准，断连/超时副作用结果未知时先对账，不盲目重放。
5. 首期采用 Go worker + 持久化队列/任务表；只有实际性能或隔离需求出现时才拆分 worker 服务。

**完成标准**：对 mock 设备可完整演示；对真实纳管设备仅在授权和审批范围内执行；单目标与批量状态、日志、频次、生命周期一致；操作审计和 CMDB 关联自动生成。

## 6. P4 — 自动机柜图（与资产/CMDB 数据接通）

1. 前端 Mock 阶段先完成机柜图工作区：多机柜选择、正反视图、SVG、U 刻度、设备详情、问题面板、生成预览、diff、人工调整、撤销/重做、发布和导出状态。
2. 基于 `EXCEL-IMPORT-PROFILE.md` 的 `Device Information`、`Layout`、`Layout 110-120`、功耗/工作量汇总及 P2P/端口 Sheet，生成标准化候选。不要把端口连线表误当机柜 U 位布局表。
3. Go 端实现 `RackGenerator`、`RackValidator`、`RackDiagramRevision` 和异步导出；先使用明确 rack/U/设备高度生成，缺失值输出 issue。
4. 关联 CMDB 中 Rack、BOMItem、PhysicalPort、Cable 和生命周期事件；布局发布后的位置修改走审批/版本事件。
5. 再生成必须展示 diff、默认保留人工覆盖；不得覆盖当前 published revision。

**完成标准**：详见 `RACK-DIAGRAM.md` §7；与 mock/真实 API 使用相同 OpenAPI schema。

## 7. P5 — 网络底座与服务器交付

按 PRD 实现网络基线/模板、ZTP、配置 Doublecheck、审批下发闭环、LLDP 三方校验、六阶段交付主链路、Mapping、工单、监控。每个域先完成 mock UI/API 契约，再逐模块接 Go 后端和 simulator。不得因 mock 缺少真实设备而将 unknown 伪装 PASS。

## 8. P6 — 端到端交付、退场与交付后维护

实现 Flow 编排、装机基础配置/RoCE子任务/NCCL测试、报告、优雅退出、凭据撤销、CMDB maintained 状态。所有执行、报告、门禁、审批和资产生命周期事件自动沉淀并互相关联。

## 9. 每阶段最小验证集

- Go：`go test ./...`、`go vet ./...`、关键领域规则/状态机测试。
- API：OpenAPI lint、成功/权限拒绝/参数错误/状态冲突/幂等/并发冲突契约测试。
- DB：空库迁移、约束、事务回滚、并发提交与唯一键测试。
- 前端：`tsc`、lint、build；关键表单和状态视图组件测试。
- E2E：只覆盖关键用户路径，不对每个静态组件重复做无价值测试。
- 网络：simulator + golden-file；真机仅在授权 PoC 环境执行。

## 10. Codex 每次任务输出格式

```text
完成：<具体能力>
修改：<关键文件列表>
验证：<执行的命令与结果>
未解决：<仅列真实阻塞；无则写无>
下一步：<最小必要动作>
```

## OPS-2026-10-29 · 交付后运维设计增补

本次仅更新设计与Mock契约，新增运维能力尚未开发。交付前端现有工作不因本设计被删改。

1. OPS-D0：冻结FR-18…22、实体复用/所有权、IPMI-only、观察上下文和专用OpenAPI；完成跨文档检查。
2. OPS-M1：在既有前端新增独立服务器/交换机运维目录与统一设备详情；先完整Mock API和上述集合，再做配置三方diff、硬件/端口/DDM趋势、采集Job、维护/升级/退役确认流程。
3. OPS-M2：固定异常场景与浏览器闭环：历史只读→策略拒绝→允许采集→漂移工单→允许通道变更→精确范围复验；Viewer/跨项目/禁用通道拒绝在provider层验证。
4. 用户确认页面后，OPS-G1逐模块Go接入：只读身份/配置/快照查询→采集策略/Job→监控日志存储适配→审批/固件/BMC→维修复验/退役。禁止为了并行后端改变前端先行。
5. maintained任务恢复、重试/fallback、外部来源接入和凭据吊销先做安全验收，再允许真实适配器。未定阈值/存储产品/保留数值记录DECISIONS，不用假成功绕过。

阶段完成条件以ACCEPTANCE的OPS项为准；文档通过不等于Mock/真机通过。

## IA v2.3 · 双平面与统一抽屉

IA v2.3 插入前端 Mock 阶段：先冻结 IA-V2.3.md 的页面注册、作用域/权限/加载协议及OpenAPI→补跨项目fixture与契约测试→ProjectContext/Query和路由→共享DrawerDetail及两类领域Tab→迁移业务页面和双向处置链→状态/可访问性/浏览器验收。用户确认后才逐域对接Go，不提前实现真实设备操作。

## IA v2.4 实施增补

按IA v2.4分五阶段：上下文/导航→Dashboard/BOM/CMDB/RackView→Fleet/Fabric/Monitor/Issues→Flows→Validation/NetTest/NCCL/Config/Provision/Cleanup/Admin；先契约和Mock，再按授权接Go。


## IA v2.4 Mock 前端实施记录

导航树、完整页面清单、路由/权限/作用域/加载策略、五/八Tab抽屉与API对照统一见 [IA24-IMPLEMENTATION-MATRIX.md](IA24-IMPLEMENTATION-MATRIX.md)。原FR-0…24、门禁、BOM与维护态规则保留。当前实现为前端Mock，不表示已实现Go/真实设备/真实AI。权威种子为 `fixtures/demo-full-64-b300/demo-seed.json`；原包83集合，补齐与规范化后集合数量以当前manifest为准，主项目硬件数量不变。结构检查、数据修复、契约与交互结果以IA24-*报告为准，无法采集/缺少证据保持unknown或阻断。


## 当前六阶段主链路修正

本轮用户明确废弃数字门禁作为主链。当前唯一业务顺序、状态机、逐目标放行、安全交接、独立BOM菜单与API约束以[DELIVERY-LIFECYCLE.md](DELIVERY-LIFECYCLE.md)为准。历史编号只按[DELIVERY-MIGRATION.md](DELIVERY-MIGRATION.md)追溯，不得用于解锁。验机失败关联统一Issues并定向复验；NCCL不反写验机；清理/归档/客户签署齐全后才进入维护态。NP仍是网络内部断言。当前仅前端Mock与契约实现，没有真实设备后端。


## 单柜详情工作区增补

固定单柜窄画布、左侧层级导航和右侧八Tab为本轮布局要求；交互、API、元数据、Mock来源及待执行验收见[RACK-WORKSPACE.md](RACK-WORKSPACE.md)。资产/位置和既有安全模型不变。右侧按需加载，不以缺失指标显示正常。当前构建/浏览器验证按用户安排等待统一执行。


## Fabric Topology 增补

执行顺序：六阶段+BOM独立入口→Rack三区布局与八Tab→Fabric Topology物理/逻辑交互及显式关联→契约/Mock/文档同步。前三部分只做前端Mock；集中构建/浏览器由用户安排，未验证保持NOT RUN，之后逐模块考虑Go。

完整导航、模型、交互、API表与实施边界见[FABRIC-TOPOLOGY.md](FABRIC-TOPOLOGY.md)。


## 无损网络分层证据 v2

先官方章节核对/迁移表→规则/证据专用契约→集中Mock反例与只读适配计划→共享组件及六阶段/Issues联动→相关规则/契约检查→提交keel-new。后续需取得真实脱敏NVUE输出、单位/能力/采样误差和主机响应证据后认证collector，不提前实现Go或自动调参。

规则/官方章节/命令/版本/判据/证据对照、历史迁移、真实缺口见[LOSSLESS-EVIDENCE.md](LOSSLESS-EVIDENCE.md)。


## CMDB 全链路资产档案增补

无损证据改造提交1631460后实施CMDB档案：现有域/契约读取→五视图与只读关系投影→统一Provider/来源/历史/质量→稳定ID演示补齐→相关构建/契约/交互检查→独立提交keel-new。未实现新CMDB后端、图数据库、通用低代码或真实采集。

唯一详细设计与模型边界见[CMDB-DOSSIER.md](CMDB-DOSSIER.md)。

## 真实开发接入基线

参见 [DESIGN-ALIGNMENT-AUDIT.md](DESIGN-ALIGNMENT-AUDIT.md) 与 [API-OPERATION-INDEX.md](API-OPERATION-INDEX.md)。保持现有页面、Provider 请求/响应和稳定资产身份；先接身份/权限与持久化基础，再按已批准模块顺序替换 HTTP Adapter。Mock 通过不等于真实授权、事务 Outbox、SSE 恢复或设备适配验证通过。资源专用分页、响应 envelope 与幂等字段以 OpenAPI 为准，不统一改写已实现接口。

## BOM 真实开发增补

用户已授权在 `dev/` 真实实现 BOM 模块。数据与权限边界、版本/审核/Outbox、真实六步工作台及运行验收统一见 [BOM-REAL-DESIGN.md](BOM-REAL-DESIGN.md)。FR-2.1 和唯一 BOM/资产身份保持；其他模块的 Mock 状态不因本模块接入而视为后端已完成。实际通过范围以 BOM-REAL-ACCEPTANCE.json 为准。

## 2026-10-30 · 身份切片接入

身份模块按版本迁移→权限目录/主体→Cookie/CSRF登录→管理与审批API→全局IdentityProvider→原登录页迁入→BOM移除独立登录→定向验收完成。BOM已真实接入，其他设备模块继续Mock；后续设备执行API必须消费同一权限与审批校验，不为本任务提前落地设备后端。见 IDENTITY-RBAC.md。

## CMDB 真实切片顺序

先固定UI截图/令牌及契约；随后迁移既有表与Schema Registry；实现资产/关系/来源/版本；接入BOM Outbox；以Adapter替换CMDB Mock；增补现有生命周期内ChangeSet；最后执行定向数据/授权/性能/视觉验证。禁止重写原工作台和设备抽屉。

接口对照、模型与实现边界统一见 [CMDB-REAL-DESIGN.md](CMDB-REAL-DESIGN.md)，具体HTTP Schema以openapi.yaml为准。

## 项目与 AI 配置真实落地

项目创建和系统级AI配置作为开局/CMDB优先真实切片交付；先验证空白项目与规则导入，再由用户配置并验证外部API，不提前实现其他真实设备执行。

唯一详细说明与API对照见 [PROJECT-AI-CONFIGURATION.md](PROJECT-AI-CONFIGURATION.md)。


## BOM Intelligence Agent 当前实施（2026-10-30）

本轮仅 AI 智能识别：上传→真实分析进度→分类审核→确认入库。交换机/端接优先，单主编排器+Workbook IR 工具+独立 Schema/来源/覆盖校验。当前范围替代此前 BOM 双模式/六步 UI 描述，历史数据不删除；详细字段、状态、API、安全和验证边界统一见 [BOM-WORKBENCH-V2.md](BOM-WORKBENCH-V2.md)。保留 FR-2.1、项目权限、唯一资产身份和原事务 Outbox。完整导入入库闭环由用户验收；不以旧报告宣称当前通过。


## 2026-10-31 数据中心可视化增补

新增运维 `/topology` 与真实 CMDB 机柜工作台；投影接口、布局/事实边界、asset.relocate 审核发布、验证记录和未完成项统一见 [DATACENTER-VISUALIZATION.md](DATACENTER-VISUALIZATION.md)，HTTP schema 见 openapi.yaml。沿用原资产身份和权限，不把计划连接或 synthetic 数据视为真机验证。

## 2026-10-31 · 可视化 V3 阶段边界

当前先完成 [topology.md](topology.md) 四页研究型设计与 `docs/topology/prototype/` 交互原型，机柜冻结。顺序：Cluster/Hybrid → Fabric Flow/Bipartite → Matrix/BioFabric → Metro/Tree/Table → 定向浏览器检查与文档一致性。完成后等待用户设计评审；未经确认不改正式页面、Go API或数据库，不将G6/ELK引擎替换作为本轮前置任务。

## 批准可视化本轮实施顺序 · 2026-10-31

当前五工作区前端已按批准原型接入；继续完成：

1. 复用 CMDB 查询投影补齐分范围子图、连接分页、矩阵及追踪契约，保留既有 API 兼容。
2. 四工作区与机柜改为按范围/选择加载，补游标版本、取消与未知状态。
3. 补齐原型交互细节及跨页面稳定 ID 联动，不增加新的资产事实库。
4. 一次集中执行受影响的类型、构建和纯逻辑检查，更新实施报告和 HANDOFF；真实导入及运行环境联调待用户安排。

仅在 `feat/datacenter-visualization-approved` 开发，未经指令不合并/发布，不自行调整运行环境。详见 [实施报告](topology/implementation-report.md)。


## Flows 包重构实施增补 · 2026-10-31

Flows包授权真实全栈实施，在feat/flows-redesign独立工作区交付。当前代码、下一步缺口与运行环境见实施记录；不隐含其他模块真实化或main合并。

详见 [Flows 实施记录](flows-redesign/implementation.md)、[真实 API 契约](api/flows.md)。字段/Schema仅定义于OpenAPI。
