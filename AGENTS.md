# AGENTS.md — 当前开发约束

更新：2026-11-03。本文件整合仍有效的约束；历史阶段指令归档，不再将旧全量 PRD 或 Mock 阶段要求套用所有模块。

## 开工与权威

- 先读 [HANDOFF](HANDOFF.md) → [docs/README](docs/README.md) → 本次模块页和 API 契约；按其索引检查受影响代码，禁止反复全仓扫描。
- 用户最新指令优先。模块化工作按 [MODULE-WORKFLOW](docs/MODULE-WORKFLOW.md) 实施；Kickoff、CMDB、拓扑、作业平台已有真实实现，Fabric 在独立分支。不得恢复“所有模块只能 Mock”或“一次完成全量 PRD”。
- 具体 HTTP 结构唯一来源为 `docs/openapi.yaml`。`docs/api/real-api-registry.json` 登记实现/部署/验证，不复制 Schema；未登记不等于未实现，有路由也不等于可执行。
- 契约变化同步生成类型、Go Handler/Service、前端 Adapter、模块/API 文档及台账；按需更新实施/发布证据。设计、实现、部署、验收四者分开记录。
- 遇到事实冲突，只定向核对并登记 `docs/DECISIONS.md`；不静默扩展范围。文档不得授权部署、数据导入或合并 main。

## 范围、代码与验证

- 只完成用户当轮业务目标；优先复用现有实体、组件、契约与服务，不进行无关重构、依赖升级、过度防御或新增架构。
- 前端 React/TypeScript/shadcn，页面通过 `dev/src/lib/api` 类型化 Provider；作业平台按用户授权复用 Apache-2.0 Kestra Vue 组件，保留许可/署名，其他模块不迁移框架。
- 修改 UI 前按需读 `docs/DESIGN-SYSTEM.md`、`docs/UI-REFERENCE.md`，保留当前 Shell、ProjectContext、统一 Drawer 和批准原型。不得用后端字段差异重写既有页面。
- Go 模块化后端、PostgreSQL，迁移显式版本化，不启动时删表。不得擅自改端口、启动新环境、迁移数据、导入资产或覆盖其他窗口服务。
- 冒烟、集中交互与回归时点由用户安排；不在每次页面变更后重复跑已通过检查。必要检查按影响范围选择，不弱化断言、不伪造结果。
- 纯文档只运行 `node dev/scripts/check-real-api-docs.mjs`、文档引用及 diff 检查，不跑应用构建、浏览器或数据库测试。该脚本不证明业务可用。
- 任务完成更新当前文档快照；简短报告实际完成、关键文件、实际检查、未完成/限制，不堆叠新历史段落。

## 事实与安全

- `bom_item`、工程发布与 CMDB 统一身份/关系继续复用；领域事实通过同事务 Outbox/幂等投影关联，不建第二资产库或重复作业/Telemetry 平台。
- 不把旧 BOM 前端退役解释为删除共享解析、历史事实、Outbox 或后端。当前入口为 Kickoff，旧前端地址应用404，不恢复旧五步门禁/菜单。
- 原始资料不可变可追溯。AI 仅候选，人工审核后发布；未知列/缺失字段保留。不得复制原始敏感 Excel、SN/MAC/IP、凭据进 fixture/Git/日志/提示词；秘密只保存受控引用。
- 物理 Cable、逻辑 NetworkPlan、LLDP/运行观测分层；不从名称/连线猜测 IP/VLAN/ASN/VNI/QoS/Breakout，不用零/默认值制造完整性。
- CMDB Schema 管版本化类型/扩展属性，不变成任意动态数据库；稳定设备/端口 ID、Breakout 父子关系、机柜几何与未知状态保留。布局草稿不写实际安装事实。
- 模拟能力明确 SIMULATION_ONLY，只读同一 `fixtures/demo-full-64-b300` 数据；不新增全局场景切换、假延迟、全数据重置或真实 Provider 的模拟 fallback。
- `maintained` 严格 IPMI-only；禁止 SSH/Agent/Redfish/代理绕过。执行未知先对账，接受任务后不得 fallback 重发。
- 统一 IdentityProvider、RBAC、项目及资源授权；角色权限以 `internal/identity/catalog.go` 和后端快照为准。重要写入保留审批职责分离、版本、幂等、审计和验证；Critical/NH 资格未决保持阻断。
- 全平台六阶段保持开局配置→网络就绪→装机→验机→NCCL→优雅退场。旧门禁只供历史；WAIVED 不等于 PASSED，不能跳过安全清理或客户签署。

## 工作区和提交

- 当前工作区、暂停事项、部署 revision 只在 HANDOFF 维护；独立分支不自动授权独立运行环境。
- 完成一项修改创建 Git 提交。推送前核对远程为 `enzoOvOyu/keel-new`，不得推旧仓库；未经指令不合并或推送 main。
- 不覆盖其他窗口的未提交修改。用户只要求文档时不得改代码/OpenAPI 定义/SDK/运行配置或执行部署。

历史约束全文：[归档](docs/archive/2026-11-03/previous/AGENTS.md)。只用于追溯，当前有效要求以上文及模块页为准。
