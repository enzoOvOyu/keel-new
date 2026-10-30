# keel v3.4 Mock-first 数据演示包

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
- `MOCK-DATA-CONTRACT.md`：前端 mock 数据和交互行为契约；`fixtures/demo-64-b300/` 提供 64 台 B300 全域演示数据与 PostgreSQL 装载脚本；仅用于隔离演示数据库。
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

`fixtures/demo-64-b300/demo-seed.json` 是固定演示数据源，`seed-demo.sql` 写入隔离的 `keel_demo` schema。仅对一次性演示库执行：

```bash
PGOPTIONS="-c keel.allow_demo_seed=true" psql "$KEEL_DEMO_DATABASE_URL" -v ON_ERROR_STOP=1 -f fixtures/demo-64-b300/seed-demo.sql
```

SQL 默认拒绝执行，除非显式设置 `keel.allow_demo_seed=true`。不要对生产库运行，不要给演示服务配置真实设备执行权限。

## OPS-2026-10-29 · 交付后运维设计增补

新增交付后运维设计入口：[OPERATIONS-IMPACT.md](OPERATIONS-IMPACT.md)（已有/缺失/冲突与处理）、[OPERATIONS-DESIGN.md](OPERATIONS-DESIGN.md)（完整生命周期、独立页面、事实所有权与安全边界）。PRD新增FR-18…22，未重编号/删除原交付能力。ACCEPTANCE新增OPS-001…016，openapi.yaml增加专用Ops*契约。

本次设计变更不等于新增运维功能已经实现；尤其维护态IPMI-only禁止恢复SSH/Agent/Redfish，能力不足只显示历史/unknown/unsupported。完整文档变更与检查结果见OPERATIONS-CHANGELOG.md。
