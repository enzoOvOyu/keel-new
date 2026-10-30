# IA v2.4 导航与实施契约

## 导航与路由

顶部：品牌、dev/staging/prod、⌘K、通知、ProjectSwitcher（名称/型号/生命周期）、用户菜单。项目选择仅保存在当前浏览器标签的内存上下文。

| 区域 | 一级入口及内部子页面 | 路由 | 作用域 / 加载 |
|---|---|---|---|
| 部署 | 部署项目：列表/新建/模板 | `/projects` | 权限范围项目，按需 |
| 部署 | 项目总览：生命周期/时间线/门禁/KPI/阻断/Flow | `/p/:projectId/deploy/overview` | 项目；poll 60s |
| 部署 | BOM 开局配置：上传/解析/映射/校验/审核提交 | `/p/:projectId/deploy/bom/:importId?` | 项目；按需 |
| 部署 | Provision：PXE/镜像/BIOS/固件 | `/p/:projectId/deploy/provision/:view?` | 项目；配置按需，批次poll 5s |
| 部署 | Validation：门禁/预收集/PCIe/LLDP | `/p/:projectId/deploy/validation/:view?` | 项目；按需 |
| 部署 | NetTest：RoCE/批次/报告/矩阵 | `/p/:projectId/deploy/nettest/:view?` | 项目；批次poll 5s、报告按需 |
| 部署 | Config：ConfigSet/角色/应用/Agent | `/p/:projectId/deploy/config/:view?` | 项目；按需 |
| 部署 | Fabric（唯一入口）：网络就绪/设备/Topology物理与逻辑/ZTP/Golden/Doublecheck/PushJob/无损/DDM/日志/规则 | `/p/:projectId/deploy/fabric/:view?` | 项目；端口/ZTP/PushJob SSE；健康及DDM摘要poll 60s，详情按需 |
| 部署 | NCCL：批次/报告/基线/阈值/溯源 | `/p/:projectId/deploy/nccl/:view?` | 项目；批次poll 5s、报告按需 |
| 部署 | Cleanup：计划/执行/归档签署/维护态 | `/p/:projectId/deploy/cleanup/:view?` | 项目；poll 5s |
| 运维 | Dashboard：交付与运维总览 | `/dashboard` | 已授权项目集合，默认项目过滤可清除；poll 60s |
| 运维 | Fleet（唯一入口）：服务器/网口健康/资产台账/采集任务 | `/fleet/:view?` | 已授权项目集合；poll 5s，端口SSE |
| 运维 | RackView：全局机房/单柜编辑器 | `/racks/:rackId?` | 已授权项目集合；按需 |
| 运维 | Issues：池/来源/阶段/看板/统计/复测 | `/issues/:view?` | 已授权项目集合；poll 5s |
| 运维 | Monitor：告警/采集策略/观测/BMC | `/monitor/:view?` | 已授权项目集合；告警SSE |
| 运维 | Flows（唯一入口）：Flow/Execution/蓝图/介质 | `/flows/:view?` | 已授权项目集合；执行与日志SSE |
| 运维 | CMDB：资产目录/资产关系/配置与来源/生命周期/数据质量；项目资料辅助入口 | `/cmdb/:view?` | 已授权项目集合；档案五视图按需，独立聚合摘要poll 60s |
| 运维 | AI：对话/Provider/知识库/识别/反馈/评测 | `/ai/:view?` | 已授权项目集合；按需，Mock无真实API |
| 系统 | 用户/通知/Token/审计/模板/型号/参数/SSH信任/AI Provider | `/admin/:view?` | permissions[]；按需 |

每个导航项绑定项目范围、权限、加载与取消策略；未知子页面404，不得以静态占位假装实现。详情使用同一 DrawerDetail：服务器五Tab、交换机八Tab，按Tab懒加载，URL与前后切换保持筛选、滚动及返回路径。业务闭环：设备异常→事件→告警→统一工单→分派→处置→定向复验→关闭。

## ProjectContext

项目切换仅当前标签页内有效；左侧项目列表与顶部ProjectSwitcher调用同一动作。部署URL、API请求、Query key、SSE、抽屉与批量快照都绑定 project_id。切换时取消旧请求/订阅及旧项目缓存，重新加载部署页面；运维只改变默认筛选，清除后展示当前身份全部授权项目。权限变动由provider与服务端双方检查，前端隐藏菜单不是授权。

## 安全和冲突

IA v2.4 §6.3 允许维护态 Redfish，与已批准 PRD FR-9 / OPS-D02 / AGENTS 的 IPMI-only 不一致，记录 IA24-01；在解决之前保持 IPMI-only，Redfish/SSH/Agent 拒绝。真实Excel不在工作区，记录 IA24-02，不声称已检查。原始真实凭据和行不写入 fixture、日志或 AI。

## 数据/契约

复用 BOMItem、Device、PhysicalPort、Cable、Rack、RackPlacement、CMDB、Flow、Execution、Issue、GateEvidence、PushJob 和已有Ops/IA OpenAPI；禁止平行资产真相。真实API与Mock应共享请求/响应模型。新增跨项目演示数据必须具有独立合成资产与真实引用，原64服务器/512GPU不变；数据必须表明来源、observed_at、freshness、support，unknown/unsupported/stale不能当正常或0。SSE需要Last-Event-ID及权限重验；按需页面不统一高频轮询。

## API 对照与组件复用

下表保留模块级资源概览，不是逐操作冻结清单。当前全部 HTTP 方法、精确路径、operationId 和请求/响应 Schema 见 [API-OPERATION-INDEX.md](API-OPERATION-INDEX.md)。新增领域以 DELIVERY-LIFECYCLE、RACK-WORKSPACE、FABRIC-TOPOLOGY、LOSSLESS-EVIDENCE、CMDB-DOSSIER 与 OpenAPI 对照，旧概览不能覆盖专用契约。

| 页面 | 现有契约 | 缺口 / 状态 |
|---|---|---|
| 项目上下文 | `/ia/session`, `/projects`, `/projects/{project_id}` | 当前ProjectContext与Mock已实现；真实服务端授权/取消与事件恢复仍需接入验收 |
| Dashboard / Fleet | `/ia/cross-project/{resource}`, `/projects/{project_id}/overview`, `/projects/{project_id}/operations/devices` | 跨项目汇总需明确授权集合，不能直接聚合所有Mock记录 |
| BOM | `/projects/{project_id}/bom-imports`, `/bom-imports/{import_id}/{sheets,mappings,validate,issues,preview,commit}` | AI任务/Provider专用契约与真实工作簿Profile仍待补全；无真实Excel时不得伪造解析 |
| RackView | `/projects/{project_id}/racks`, `/projects/{project_id}/rack-diagrams/*` | 跨项目机房汇总与授权筛选需专用契约；编辑复用既有revision、校验、发布和导出 |
| 设备抽屉 | `/ia/projects/{project_id}/devices/{device_id}/tabs/{tab}`, `/devices/{device_id}/operations/*` | 逐Tab领域字段、历史与状态应收紧；五/八Tab复用同一DrawerDetail，不嵌套 |
| Fabric | `/projects/{project_id}/fabric/devices`, `/projects/{project_id}/network-baselines`, `/push-jobs/*` | 现有专用schema；Topology物理/逻辑及Lossless分层证据接口见逐操作索引；真实采集未接入 |
| Flow/Execution | `/projects/{project_id}/flows`, `/flows/{flow_id}/{validate,run,revisions}`, `/executions/{execution_id}/{targets,tasks,logs,outputs,pause,resume,kill,retry-failed}` | YAML/No-Code、调度、目标快照和状态机共用领域模型；Kestra集成未决 |
| Issues/Monitor | `/projects/{project_id}/issues`, `/projects/{project_id}/alerts`, `/ia/events/stream` | 跨项目授权集合与事件恢复需验证；工单只有一套ID与状态机 |
| 系统管理 | `/users`, `/audit-events`, `/media`, `/parameter-catalog` 及对应既有模板/型号契约 | 权限和操作需provider/服务端复核；敏感Token不放入fixture |

共享组件：ProjectContext→导航/查询/批量快照；DrawerDetail→ServerDetailDrawer/NetworkDeviceDrawer；DataTable仅资源目录；Rack SVG、React Flow拓扑、日志流、证据和差异组件按领域组合。上述列表为设计覆盖，不代表前端或所有端点均已完成。



## 2026-10-30 全量数据包接入
最新权威演示数据为 `fixtures/demo-full-64-b300/demo-seed.json`（来源 `keel-ia-v2.4-full-demo-pack.zip`），覆盖83个集合和33个工作簿结构Sheet。停止在运行时合并旧扩展种子或额外A100资产；其他两个项目仅上下文，不虚构额外设备。导入检查见 `docs/IA24-DATA-IMPORT-REPORT.json`，覆盖见 `docs/IA-V2.4-MOCK-COVERAGE.md`。真实Excel仍未提供；workbook-profile只作为提供的结构元数据，不等于本轮实际检查原工作簿。新包的旧PRD不能删去FR-18…24或放宽既有IPMI-only；不可解析项目成员不得自动授权。


## 当前六阶段主链路修正

本轮用户明确废弃数字门禁作为主链。当前唯一业务顺序、状态机、逐目标放行、安全交接、独立BOM菜单与API约束以[DELIVERY-LIFECYCLE.md](DELIVERY-LIFECYCLE.md)为准。历史编号只按[DELIVERY-MIGRATION.md](DELIVERY-MIGRATION.md)追溯，不得用于解锁。验机失败关联统一Issues并定向复验；NCCL不反写验机；清理/归档/客户签署齐全后才进入维护态。NP仍是网络内部断言。当前仅前端Mock与契约实现，没有真实设备后端。
