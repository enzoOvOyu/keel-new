# keel v3.4 Mock-first 数据演示包

> 当前实现与接续入口：[HANDOFF](../HANDOFF.md)、[真实 API 契约台账](REAL-API-CONTRACTS.md)。设计目录不代表接口已实现或已部署；以台账中的范围和验证证据为准。


## 必读入口
1. 根目录 `AGENTS.md`：执行约束和全局决策。
2. `docs/README.md`：文档权威顺序与当前开发策略。
3. 只按当前任务读取相关章节，不全量重复阅读。

## 文档目录
- `PRD.md`：业务范围与硬规则。
- `ARCHITECTURE.md`：Go 模块化单体、React 前端与运行边界。
- `SYSTEM-DESIGN.md`：领域模型、状态机、CMDB 全链沉淀、Flow 执行语义。
- `DESIGN-SYSTEM.md`：前端交互与视觉规范。
- `UI-REFERENCE.md`：Kestra/shadcn 参考项目、页面线框和视觉验收要求。
- `IMPLEMENTATION-PLAN.md`：前端 mock-first、后端逐模块接入顺序。
- `API-CONVENTIONS.md` + `DOMAIN-API-CONTRACTS.md` + `openapi.yaml`：唯一 API 契约；先设计全量 API，再实现 mock。
- `DATA-DICTIONARY.md`：实体、字段与关系。
- `ACCEPTANCE.md` / `TEST-STRATEGY.md`：验收和最小有效测试。
- `SECURITY.md` / `DEPLOYMENT.md`：安全和运行。
- `DECISIONS.md`：仅记录尚未决定且会影响实现的事项。
- `RACK-DIAGRAM.md`：基于 BOM/CMDB 自动生成机柜图的规则、交互、数据模型、API 与验收。
- `EXCEL-IMPORT-PROFILE.md`：基于 `TX B300 Layout-new.xlsx` 的真实开局资料解析规划。源 Excel 不打包。
- `MOCK-DATA-CONTRACT.md`：前端 mock 数据和交互行为契约；`fixtures/demo-full-64-b300/` 提供 64 台 B300 全域演示数据与 PostgreSQL 装载脚本；仅用于隔离演示数据库。
- `fixtures/bom-evaluation/`：脱敏评测模板，不包含原始工作簿或密码。

## 权威顺序
业务规则：PRD → 领域与状态：SYSTEM-DESIGN → 技术边界：ARCHITECTURE → 数据字段：DATA-DICTIONARY → API：API-CONVENTIONS + openapi.yaml → UI：DESIGN-SYSTEM + UI-REFERENCE → 验收：ACCEPTANCE。
发现冲突时只暂停受影响决策，登记 DECISIONS.md；不静默猜测，不因此阻塞无关页面。

## 当前开发策略
**前端先行、API 契约先行、全量 mock、后端逐模块接入。** 第一目标是完整可交互的控制台，而不是先实现所有后端。mock/真实 API 必须共用同一类型和状态语义。工作流以批量脚本执行与执行监控为核心，参考 Kestra UI 交互模式；不照搬其代码或把通用 DAG 引擎作为首期阻塞。

CMDB 从 BOM 导入开始自动沉淀全链记录；所有实体和事件通过权威业务表 + Outbox/幂等投影维护，不允许各模块各自手工维护一份 CMDB 数据。

## Codex 开工提示
先读 `AGENTS.md`、本文件、`IMPLEMENTATION-PLAN.md`、`DESIGN-SYSTEM.md`、`UI-REFERENCE.md`、`MOCK-DATA-CONTRACT.md`、`API-CONVENTIONS.md` 和 `openapi.yaml`。检查仓库后，从前端 P0 开始：全局壳层 + 路由 + 类型化 API client + mock provider；随后实现所有主要页面的可交互 mock。不要提前实现 Go 后端，不要在组件中硬编码 mock，不要无关重构或重复全仓审查。完成后按 AGENTS.md 的短格式报告实际改动和验证结果。

## Rack Diagram 功能

机柜图需求与生成边界见 `RACK-DIAGRAM.md`。前端阶段必须完成可交互 SVG Mock；后端接入后由 Go 确定性生成、校验和版本管理。Rackula/RackView 仅作交互/交付逻辑参考。

## Mock 数据库装载

`fixtures/demo-full-64-b300/demo-seed.json` 是固定演示数据源，`seed-demo.sql` 写入隔离的 `keel_demo` schema。仅对一次性演示库执行：

```bash
PGOPTIONS="-c keel.allow_demo_seed=true" psql "$KEEL_DEMO_DATABASE_URL" -v ON_ERROR_STOP=1 -f fixtures/demo-full-64-b300/seed-demo.sql
```

SQL 默认拒绝执行，除非显式设置 `keel.allow_demo_seed=true`。不要对生产库运行，不要给演示服务配置真实设备执行权限。

## OPS-2026-10-29 · 交付后运维设计增补

新增交付后运维设计入口：[OPERATIONS-IMPACT.md](OPERATIONS-IMPACT.md)（已有/缺失/冲突与处理）、[OPERATIONS-DESIGN.md](OPERATIONS-DESIGN.md)（完整生命周期、独立页面、事实所有权与安全边界）。PRD新增FR-18…22，未重编号/删除原交付能力。ACCEPTANCE新增OPS-001…016，openapi.yaml增加专用Ops*契约。

本次设计变更不等于新增运维功能已经实现；尤其维护态IPMI-only禁止恢复SSH/Agent/Redfish，能力不足只显示历史/unknown/unsupported。完整文档变更与检查结果见OPERATIONS-CHANGELOG.md。

## IA v2.3 · 双平面与统一抽屉

IA v2.3 设计及实施计划入口：[IA-V2.3.md](IA-V2.3.md)。该段为v2.3历史设计记录；本轮已进入v2.4前端Mock实施，当前覆盖与验收以IA24实施对照及实际报告为准。

## IA v2.4 实施增补

新增 [IA-V2.4.md](IA-V2.4.md)；与IA v2.3路由有差异时，以本轮用户明确的v2.4路由为准。


## 2026-10-30 全量数据包接入
最新权威演示数据为 `fixtures/demo-full-64-b300/demo-seed.json`（来源 `keel-ia-v2.4-full-demo-pack.zip`），覆盖83个集合和33个工作簿结构Sheet。停止在运行时合并旧扩展种子或额外A100资产；其他两个项目仅上下文，不虚构额外设备。导入检查见 `docs/IA24-DATA-IMPORT-REPORT.json`，覆盖见 `docs/IA-V2.4-MOCK-COVERAGE.md`。真实Excel仍未提供；workbook-profile只作为提供的结构元数据，不等于本轮实际检查原工作簿。新包的旧PRD不能删去FR-18…24或放宽既有IPMI-only；不可解析项目成员不得自动授权。


## IA v2.4 Mock 前端实施记录

导航树、完整页面清单、路由/权限/作用域/加载策略、五/八Tab抽屉与API对照统一见 [IA24-IMPLEMENTATION-MATRIX.md](IA24-IMPLEMENTATION-MATRIX.md)。原FR-0…24、门禁、BOM与维护态规则保留。当前实现为前端Mock，不表示已实现Go/真实设备/真实AI。权威种子为 `fixtures/demo-full-64-b300/demo-seed.json`；原包83集合，补齐与规范化后集合数量以当前manifest为准，主项目硬件数量不变。结构检查、数据修复、契约与交互结果以IA24-*报告为准，无法采集/缺少证据保持unknown或阻断。


## 当前六阶段主链路修正

本轮用户明确废弃数字门禁作为主链。当前唯一业务顺序、状态机、逐目标放行、安全交接、独立BOM菜单与API约束以[DELIVERY-LIFECYCLE.md](DELIVERY-LIFECYCLE.md)为准。历史编号只按[DELIVERY-MIGRATION.md](DELIVERY-MIGRATION.md)追溯，不得用于解锁。验机失败关联统一Issues并定向复验；NCCL不反写验机；清理/归档/客户签署齐全后才进入维护态。NP仍是网络内部断言。当前仅前端Mock与契约实现，没有真实设备后端。


## 分域实施与验证记录

[六阶段主链](DELIVERY-LIFECYCLE.md)、[单柜详情工作区](RACK-WORKSPACE.md)、[Fabric Topology](FABRIC-TOPOLOGY.md)。导航/路由/API见[实施对照](IA24-IMPLEMENTATION-MATRIX.md)，历史阶段记录见[状态记录](IMPLEMENTATION-STATUS.json)；后续专项结果分别见 LOSSLESS-VALIDATION.json、CMDB-DOSSIER-VALIDATION.json、RACK-MAPPING-RESTORE.json。报告仅证明各自范围，不能以旧 NOT RUN 或局部 PASS 代替当前全平台验收。

## 真实开发前设计核查

当前核查结果与边界见 [DESIGN-ALIGNMENT-AUDIT.md](DESIGN-ALIGNMENT-AUDIT.md)，逐操作契约见 [API-OPERATION-INDEX.md](API-OPERATION-INDEX.md)。本轮只校正文档，无功能或协议载荷变更。

## BOM 真实开发增补

用户已授权在 `dev/` 真实实现 BOM 模块。数据与权限边界、版本/审核/Outbox、真实六步工作台及运行验收统一见 [BOM-REAL-DESIGN.md](BOM-REAL-DESIGN.md)。FR-2.1 和唯一 BOM/资产身份保持；其他模块的 Mock 状态不因本模块接入而视为后端已完成。实际通过范围以 BOM-REAL-ACCEPTANCE.json 为准。

- [CMDB-REAL-DESIGN.md](CMDB-REAL-DESIGN.md)：真实CMDB模型、接口对照、Adapter保护与迁移边界。

- [PROJECT-AI-CONFIGURATION.md](PROJECT-AI-CONFIGURATION.md)：真实项目创建、系统管理AI配置和BOM接入。

## 机柜可视化设计评审入口

[rack-visualization.md](rack-visualization.md) 汇总本地 CMDB 能力审计、Rack Gallery、空间工程工作台、前后双面/剖面、Mapping、数据驱动叠加与实装比较前提。隔离原型位于 5181/rack-detail；不代表正式机柜页面已经改造。
