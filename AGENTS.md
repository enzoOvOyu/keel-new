# AGENTS.md — Codex 执行约束


整实现需求并保证正确性，以减少重复和无关工作节省 Token。

- 按需读取：先搜索定位，再读取相关代码、文档、调用方和测试；避免通篇读取仓库、反复扫描目录或重读未变化的内容。
- 复用上下文：沿用已确认的事实、文件位置和验证结果；长任务只保留简短进度记录，不重复推导和总结。
- 控制改动：完成解决问题所需的最小完整修改，遵循现有架构和规范；避免无关重构、批量格式化、过度抽象和新增依赖。
- 聚焦审查：默认审查本次 diff、直接依赖和受影响调用链。涉及共享接口、权限、安全、并发、事务或数据迁移时，按实际风险扩大审查范围。
- 按风险验证：优先复用现有测试，选择足以覆盖改动风险的检查；行为变化和缺陷修复补充必要的关键用例，纯文案或样式调整采用对应检查，避免重复测试和只复述实现的测试。
- 避免重复验证：相关代码、配置、依赖未变化且没有新风险时，不重复已经通过的检查；修复失败后先重跑相关检查，仅在影响范围扩大或仓库要求时执行更广泛回归。
- 保持质量底线：遵守仓库必需检查；不得通过删除测试、降低断言、忽略错误或伪造结果获得通过。无法验证时明确说明缺口及影响。
- 连续完成工作：按完整功能或问题闭环推进，避免过度拆分、重复规划和逐步询问；只有影响正确性的关键信息无法确认时才提问，同时继续不受阻的工作。
- 精简沟通：只报告关键发现、实际修改、验证结果和未解决事项；避免重复计划、长篇过程复述、整份文件回显及无关日志。

## 0. 当前全局产品决策（优先于旧描述）
- **开发策略：前端先行**。先完成全量页面、交互、路由、统一 API 契约和可切换 mock provider；mock 数据必须覆盖 loading/empty/error/permission/stale/partial/failure 等状态。用户确认页面与交互后，再按模块逐一接入 Go API。不得为了“前后端一起做”擅自改变该顺序。
- **工作流定位**：核心是对已纳管的服务器/交换机按目标组批量执行脚本/命令，并提供可交互的执行监控。UI/交互参照 Kestra 的 Flows、Executions、Gantt/Topology、Task Logs、Retries、Outputs、暂停/终止/重试生命周期；不复制 Kestra 代码，不把“自研完整通用编排引擎”作为首期前置条件。
- **CMDB 全链自动沉淀**：从原始 BOM 导入任务、源文件与映射证据开始，所有后续标准实体、资产变化、采集、执行、门禁、网络配置、测试、工单、审批、交付与维护态数据均自动建立 CMDB 记录或关联。CMDB 是统一资产视图与生命周期/关系投影，不是第二套独立写入事实源；通过同事务 Outbox/幂等消费者维护，失败可重放。
- **UI 质量优先**：不得仅用默认 shadcn 卡片拼页面。实现前必须阅读 `docs/DESIGN-SYSTEM.md` 与 `docs/UI-REFERENCE.md`，按页面线框图、信息层级、密度、状态和交互规范实现；每个主要页面都需有 mock 数据和可点选闭环。
- **真实开局 Excel**：`TX B300 Layout-new.xlsx` 是非标准开局资料样本。按 `docs/EXCEL-IMPORT-PROFILE.md` 设计解析器；不得将该原文件、真实凭据或敏感数据复制进 Git、fixture、提示词或外部 AI 请求。原始文件中的明文凭据必须被识别为敏感信息，只提取 Vault 引用/提示，绝不导入密码值。
- **契约先行**：前端全量模块先以 OpenAPI 定义 API，再以 mock adapter 实现。每个页面只能依赖 `web/src/lib/api` 暴露的类型化接口，不得直接写死散落在组件内的假数据。mock 与真实 API 必须使用相同请求/响应类型。

## 1. 开工顺序
1. 阅读本文件与 `docs/README.md`（若本文件位于仓库根目录，按实际路径定位）。
2. 只阅读当前任务相关的 PRD、SYSTEM-DESIGN、ARCHITECTURE、DATA-DICTIONARY、API-CONVENTIONS/openapi.yaml、DESIGN-SYSTEM、ACCEPTANCE 和 IMPLEMENTATION-PLAN 章节。
3. 先检查现有代码、依赖、迁移、路由和测试；能复用则复用。不要假设仓库为空，也不要无理由重写已有模块。

## 2. 权威顺序
- 业务范围、规则和门禁：`PRD.md`
- 领域状态机和流程：`SYSTEM-DESIGN.md`
- 技术边界：`ARCHITECTURE.md`
- 字段、关系、唯一键：`DATA-DICTIONARY.md`
- HTTP 契约：`API-CONVENTIONS.md` + `openapi.yaml`
- UI/交互：`DESIGN-SYSTEM.md`
- 验收：`ACCEPTANCE.md`
- 开发顺序：`IMPLEMENTATION-PLAN.md`
- 前端参考与页面线框：`UI-REFERENCE.md`
- 真实 Excel 导入规则：`EXCEL-IMPORT-PROFILE.md`
- 全量 mock 规则：`MOCK-DATA-CONTRACT.md`

发现冲突时，不要静默选择或扩展范围。仅停止受影响部分，将冲突、相关文件/章节、最小解决选项写入 `docs/DECISIONS.md`（不存在时创建），继续不依赖该决定的工作。

## 3. 不可违反的业务约束
- 统一 `bom_item` 是服务器与网络物料唯一事实源；不建平行 BOM。
- 原始开局文件不可变保存并可追溯到 Sheet/单元格；AI 只能提出候选映射，不可直接提交正式 BOM、审批或下发网络配置。
- 已审批模板映射 > 确定性规则 > AI 候选。未知列不能静默丢弃。
- BLOCK 未清零不得提交；关键身份/端口/线缆关系不确定不得自动放行。
- 标准 BOM 包服务器与网络物料同批次、同事务提交；同批次同摘要幂等，同批次异摘要拒绝覆盖。
- 网络方案由客户与网络工程师决定。平台不得擅自补造 IP/VLAN/ASN/VNI/QoS/breakout/链路方案。
- `PASS/FAIL/BLOCKED/WAIVED`、NP-1..NP-6、G1..G6、PushJob 状态按 PRD 实现。unknown、缺失或过期证据不等于 PASS。
- 生产网络配置必须校验、备份、审批、下发、复采验证；禁止绕过审批。
- 秘密不得写入日志、审计正文、测试 fixture、参数快照或 Git。

## 4. 技术约束
- 前端：React + TypeScript + shadcn/ui，复用仓库已有构建工具；不得引入第二套重叠 UI 组件体系。
- 后端：Go；按业务域模块化，未有明确扩展/隔离需求不得拆微服务。
- API 以 OpenAPI 契约为准。变更请求/响应/枚举时同步更新 OpenAPI、前端类型和契约测试。 对 OpenAPI 中仍为 `GenericObject` 的端点，必须先依据 `DOMAIN-API-CONTRACTS.md` 收紧成资源专用 schema，再实现对应 mock/后端；不得把宽泛对象当成已冻结契约。
- 数据库变更通过版本化 migration；禁止启动时自动删表/重建表。
- 业务状态以数据库为准；异步任务需持久化状态、幂等和失败恢复。

## 5. 效率与质量
- 一次完成一个有意义的端到端切片，不要把任务过度拆碎，也不要一次铺开所有模块。
- 不做无关重构、全仓库重复 review、无价值快照测试或重复运行同一测试。
- 只运行受本次变更影响的测试；失败时先定位原因，避免无差别重跑。
- 不为达到测试覆盖率而改弱业务判据或制造假实现。
- 对真实设备不可用的场景使用明确 simulator/mock；能力不可模拟时返回 unknown/unsupported，不伪造成功。
- 未经用户明确要求，不更改业务范围、依赖大版本、部署形态或数据库技术。

## 6. 前端先行实施硬规则
1. P0 先建立全局壳层、路由、设计 token、类型化 API client、mock provider、统一状态组件。
2. 先把所有主菜单页面做成可交互 mock 原型，优先覆盖项目总览、BOM 导入、CMDB、Flow 列表、执行详情、交换机/服务器资产、网络交付、G1–G6、工单、监控、模板/介质/参数。
3. 所有列表、筛选、排序、分页、抽屉、确认框、批量选择、任务日志、状态变更必须可操作；不允许“看起来像按钮但没行为”。
4. mock 必须从 API provider 返回，集中在 `web/src/mocks`；组件里禁止内嵌大段业务假数据。
5. 后端接入按模块逐个替换 provider；不得重写 UI 或更改 API 语义。OpenAPI 与 mock 类型、真实 Go handler 以同一契约生成/校验。
6. 视觉验收必须对照 `docs/UI-REFERENCE.md` 和 `docs/DESIGN-SYSTEM.md`，桌面端优先，支持窄屏基本适配；关键页面需提供截图/浏览器检查结果。

## 7. 每次任务交付格式
简短报告：
1. 完成内容；
2. 关键文件变更；
3. 实际运行的检查/测试及结果；
4. 未完成项和真实阻塞；
5. 需要产品决策的冲突（仅有时列出）。
不得声称运行过未实际执行的测试或已验证未验证的真机能力。

## Rack Diagram 约束

- 实现前读取 `docs/RACK-DIAGRAM.md`、`docs/EXCEL-IMPORT-PROFILE.md`、`docs/DATA-DICTIONARY.md` 与对应 API 契约。
- 机柜图先做 Mock UI；必须有真实 SVG 画布和可交互的前/后视图、U 位、设备详情、冲突、diff、人工调整和导出状态，不可只放静态截图。
- 不猜测缺失的机柜/U 位/设备高度；不覆盖已发布图；不把画布作为 BOM/CMDB 之外的第二事实源。
- 参考项目只借鉴行为，不复制代码或引入 Svelte/Flask 运行依赖。

## 固定演示数据
- 全平台统一使用 `fixtures/demo-64-b300/demo-seed.json` 与 `fixtures/demo-64-b300/seed-demo.sql`；不要在 React 页面或 API handler 中重复造另一套业务数据。
- 首先让所有页面通过类型化 API Provider 读取 fixture，完成可交互 Mock；数据库装载仅用于隔离演示环境，不是生产领域表设计。
- 64 台 B300 是合成演示切片；严禁复制原始 Excel 中的真实 SN、IP、MAC、账号、密码或完整原始行。
- Mock 的执行/网络下发只改变演示状态，必须显示 `SIMULATION_ONLY`，绝不能连接真实设备。

## OPS-2026-10-29 · 交付后运维设计增补

- 交付完成不是系统终点。新增运维域按PRD FR-18…FR-22、OPERATIONS-DESIGN.md、Ops* OpenAPI及OPS验收实现；先完整页面/API Mock契约，再逐模块Go。
- 本次运维变更先只改设计文档与Mock契约；不得把文档更新宣称为页面、采集器或真机能力实现。
- 复用BOM/Device/Component/PhysicalPort/Rack/Cable、基线、PushJob、Issue/Alert/Audit/Job，不另建Machine/Switch平行身份。CMDB是统一身份/关系/生命周期权威入口，业务域事实通过Outbox关联，不把所有原始指标/日志塞进CMDB表。
- approved baseline、intended、observed与planning分开；所有观测附来源/时间/保鲜/能力/质量/阈值；null/零值、unsupported/unknown/stale/degraded与门禁四值不可混用。
- maintained严格IPMI-only。不得因运维、维修、审批、定时采集、重试、fallback、恢复或第三方代理重新打开SSH/Agent/Redfish。不能采集就明确限制并保留历史；不伪造健康。
- 固件、电源、配置/回滚与退役动作必须有权限/确认/审批/租约/审计及验证；未知副作用先对账；秘密只保留受控引用。
- 采样间隔、阈值、保留天数、专用存储产品和RPO/RTO未确认时记录决策，不自行设为生产默认；无测试证据的OPS验收保持NOT RUN。
