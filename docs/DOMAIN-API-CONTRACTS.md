# DOMAIN-API-CONTRACTS.md — 前端全量 API 资源契约

> 当前实现与接续入口：[HANDOFF](../HANDOFF.md)、[真实 API 契约台账](REAL-API-CONTRACTS.md)。设计目录不代表接口已实现或已部署；以台账中的范围和验证证据为准。


> 本文定义 mock-first 阶段前端所需的核心资源模型和操作语义。`openapi.yaml` 是 HTTP wire contract；本文是资源字段/行为的补充说明。若字段冲突，以经审核后的 OpenAPI schema 为准，并同步修订本文。

## 1. 通用响应

```json
{"data": {}, "request_id": "req_01..."}
```

列表响应：

```json
{"data": [], "page": {"limit": 50, "next_cursor": null, "has_more": false}, "request_id": "req_01..."}
```

错误响应见 `API-CONVENTIONS.md`。时间使用 ISO-8601 UTC；页面按项目时区显示。列表使用 cursor pagination，通用默认 limit=50、最大 200；资源专用 OpenAPI 参数优先（如 CMDB dossier 最大100）。持久化领域资源使用稳定身份、项目与版本；聚合投影、证据及只读响应不强制补造 `created_at/updated_at/revision`，实际字段与 envelope 以各资源 OpenAPI schema 为准。

## 2. 项目与总览

`Project`: `id, code, name, status, template_id, region/site, owner, created_at, updated_at`。

`ProjectOverview`: `project, bom_summary, device_summary, network_stage_summary, gate_summary, active_executions, open_issues, recent_events, cmdb_projection_status`。

接口：`GET/POST /projects`、`GET/PATCH /projects/{project_id}`、`GET /projects/{project_id}/overview`。

## 3. BOM 导入

现有 BOM API 保持；核心资源：
- `BOMImport`: `id, project_id, source_file_ref, content_sha256, template_revision_id, status, sheet_count, entity_counts, issue_counts, mapping_revision, validation_revision, cmdb_projection_status, created_by, created_at`。
- `SheetRegion`: `sheet_name, region_id, range, detected_kind, header_rows, row_count, column_count, confidence, sensitive_columns_detected`。
- `FieldMappingCandidate`: `id, source{sheet,cell,row,column,header,value_redacted}, target_entity, target_field, normalized_value, rule_source(template|deterministic|ai|manual), confidence, status, rationale, revision`。
- `ValidationIssue`: `id, severity(BLOCK|WARN|INFO), code, message, source_location, entity_ref, field, expected, actual_redacted, resolution_status, rule_version`。
- `BOMPreview`: `batch_id, content_hash, creates, updates, unchanged, conflicts, entity_counts, issue_counts, cmdb_effects, base_revision`。

必须支持上传、读取解析状态、Sheet/region 列表、映射候选、修正映射、校验、issues、preview、build standard package、commit。commit 使用 Idempotency-Key；BLOCK 未清零返回 422；同批次异摘要返回 409；敏感值永不返回。

## 4. CMDB

`CMDBEntity`: `id, entity_type, stable_ref, project_id, display_name, current_status, source_event_id, source_refs[], relationships_summary, freshness, projection_status, created_at, updated_at, revision`。
`CMDBRelationship`: `id, from_entity_id, relation_type, to_entity_id, source_event_id, valid_from, valid_to`。
`LifecycleEvent`: `id, entity_id, event_type, occurred_at, actor, source_domain, source_ref, revision_ref, summary_redacted, status`。

接口：实体搜索/详情/relationships/timeline/event 详情。CMDB 从 BOM 导入任务即开始投影。CMDB 不提供重复“手动创建同一资产”的写入口；需要修正资产时调用权威业务域 API，并由事件投影更新 CMDB。投影状态 `pending|complete|failed` 可见。

## 5. Flow

`Flow`: `id, project_id, name, description, status(draft|active|disabled|archived), revision, target_types(server|switch|both), script_ref, script_language, parameter_schema, timeout_seconds, retry_policy, concurrency_limit, failure_policy, schedule_summary, last_execution, success_rate, created_by, updated_at`。

`TargetSelector`: `target_type, explicit_device_ids[], filters{site,rack,role,tags,status}, resolved_count, target_snapshot_id`。运行前必须预览解析目标并冻结快照，执行后不因资产筛选变化而改变目标集合。

`Schedule`: `id, flow_id, enabled, expression, expression_type(cron|interval), timezone, next_run_at, last_run_at, misfire_policy, revision`。

操作：list/detail/create/update/revisions/validate/run/schedules/enable/disable。脚本执行只允许针对已纳管、项目授权且符合目标状态的设备；敏感参数使用 secret reference，不返回 secret value。生产网络写操作需要单独审批/PushJob，不允许普通 Flow 绕过网络变更主链路。

## 6. Execution

`Execution`: `id, flow_id, flow_revision, project_id, status, trigger_source, started_by, target_snapshot_id, target_count, counts{queued,running,success,warning,failed,killed,result_unknown}, started_at, finished_at, duration_ms, schedule_id, parameter_snapshot_redacted, cmdb_projection_status, request_id`。
`TaskRun`: `id, execution_id, task_key, target_id, status, attempt_count, started_at, finished_at, duration_ms, exit_code, error_summary, retryable`。
`ExecutionLog`: `id, execution_id, task_run_id, target_id, attempt, timestamp, level, message_redacted, cursor`。
`ExecutionOutput`: `id, execution_id, target_id, name, media_type, size_bytes, sha256, artifact_ref, created_at`。

控制接口：pause/resume/kill/retry-failed。返回值说明是已接受命令还是已完成动作；kill 不得在尚无终止回执时返回最终 KILLED。日志支持 `cursor`, `target_id`, `level`, `since`, `until`。实时更新使用 SSE；断线后从 cursor 恢复。

## 7. Device / Fabric

`Device`: `id, project_id, category(server|switch), hostname, serial_redacted, model_ref, role, site, rack, rack_u, management_address_redacted, lifecycle_status, managed_status, health_status, last_seen_at, freshness, source_bom_item_id`。
`PhysicalPort`: `id, device_id, physical_name, port_index, breakout_parent_id, role, speed, mac_redacted, admin_state, oper_state, peer_port_id, source_bom_item_id`。
`NetworkBaseline`: `id, project_id, version, status(draft|tested|released|deprecated), source_ref, approved_by[], content_hash, created_at`。
`PushJob`: `id, project_id, status, target_snapshot_id, baseline_revision, diff_summary, precheck, approvals, progress, started_at, finished_at, rollback_state`。

网络配置下发操作必须先 diff/precheck/approval；无数据或不支持的能力返回 `unknown`，不能用空列表表示正常。

## 8. Gates / Delivery

`GateEvidence`: `id, subject_type, subject_id, gate_id, verdict(PASS|FAIL|BLOCKED|WAIVED), evidence_refs[], rule_versions[], collected_at, valid_until, waiver, revision`。
`DeliveryRun`: `id, project_id, target_snapshot_id, current_stage, status, gate_summary, started_at, finished_at, blocked_reasons[], cmdb_projection_status`。

所有证据必须带来源、采集时间、有效期和版本依赖。WAIVED 必须含审批人、理由、范围、有效期和整改计划。

## 9. Issues / Monitoring

`Issue`: `id, project_id, issue_type(T1..T7), source_domain, source_ref, severity, status, title, affected_entities[], evidence_refs[], assignee_id, resolution, created_at, updated_at, closed_at`。
`Alert`: `id, project_id, rule_id, severity, status, source, subject_ref, first_seen_at, last_seen_at, count, summary, evidence_refs[]`。

处置后可发起定向重验；关闭工单必须填写 resolution 和证据引用。工单与告警变化自动投影到 CMDB timeline。

## 10. Templates / Media / Parameters / Audit

- `ProjectTemplate`: 型号/网络/服务器基线、BOM 字段映射 revision、参数目录默认值、Flow blueprints、门禁策略、状态与审批。
- `MediaArtifact`: 名称、类型、版本、架构/型号兼容、SHA256、approval state、retirement state；真实二进制使用 artifact ref，不在 JSON 内传输。
- `Parameter`: key、type、allowed values、default、scope、template refs、compatibility constraints、revision。
- `AuditEvent`: actor、action、resource、project scope、request_id、before/after redacted diff、timestamp、approval refs。

所有写操作按权限/项目范围检查并审计；秘密不得进入 `before/after`、日志或 API 响应。

## 11. Mock/真实 API 兼容验收

1. 每个前端页面至少绑定一个 API contract 和固定 fixture。
2. mock 响应必须通过 OpenAPI schema 验证。
3. mock 与真实 API 在同一场景下具有相同字段、状态和错误码。
4. API 字段改动先更新 OpenAPI，再更新 mock、Go handler 和页面类型；禁止只改其中一处。
5. 所有长任务和执行控制接口测试 accepted/running/terminal/error 状态；所有 CMDB projection 测试 pending/complete/failed/replay。

## Rack Diagram API 域

- Rack/Diagram 查询只读；生成创建新 draft revision，不修改当前 published revision。
- Placement 编辑只作用于指定 draft revision，必须传 `If-Match` 或 `revision_id`；冲突返回 409。
- Publish 请求指定 `revision_id`，服务端重新校验 BOM/model/policy 版本与阻断问题；不允许仅凭前端校验状态发布。
- Export 必须指定 revision 和 format（svg/png/pdf），并返回文件任务/下载引用；敏感字段不进入图纸标签或文件。
- Schema 详情见 `RACK-DIAGRAM.md` 与 `openapi.yaml`。

机柜图的生成、revision、placement、校验、发布与导出由 `RACK-DIAGRAM.md` 定义；布局发布和位置变更必须产生 CMDB 生命周期事件，不允许只更新前端 SVG。

## OPS-2026-10-29 · 交付后运维设计增补

### 复用域接口与页面绑定
| 页面 / 交互 | API | 专用返回类型与行为 |
|---|---|---|
| 运维服务器/交换机列表 | GET /projects/{project_id}/operations/devices | OpsDeviceListEnvelope；category/机柜/型号/交付/维护/健康/告警及fabric/pod/role/nos/config过滤；复用Device稳定ID |
| 服务器/交换机详情 | GET /devices/{device_id}/operations/detail | OpsDeviceDetailEnvelope；公共身份/维护/通道/观测context，server或switch专属详情；缺能力字段null+原因 |
| 基线/意图/实测 | GET /devices/{device_id}/operations/configurations；GET .../config-diffs | OpsConfigurationListEnvelope / OpsConfigDiffListEnvelope；明确fact_kind与不可变revision；不提供直接写actual入口 |
| 端口/LLDP/FDB/ARP/DDM | GET /devices/{device_id}/operations/ports；.../lldp；.../fdb；.../arp；.../ddm | 对应OpsPort/LLDP/FDB/ARP/DDMListEnvelope；分页、时间范围、来源与阈值；breakout指向既有port父子关系 |
| 监控/硬件/健康/无损 | GET .../metrics；.../telemetry；.../logs | OpsMetric/Telemetry/LogListEnvelope；指标单位、nullable值、counter reset；历史快照不可变；日志按cursor检索 |
| BMC传感器/SEL/电源 | GET .../bmc；GET .../bmc/sel；POST .../power-jobs | OpsBmcEnvelope / OpsLogListEnvelope / OpsJobEnvelope；maintained下Redfish policy_denied，IPMI允许性仍独立校验 |
| 固件库存/升级 | GET .../firmware；POST .../firmware-upgrade-jobs；GET /firmware-upgrade-jobs/{job_id}；POST .../approve；POST .../execute | OpsFirmwareListEnvelope / OpsFirmwareUpgradeJobEnvelope；兼容、批准介质、策略、租约、审批/回读；不自动刷写 |
| 采集策略/任务 | GET/POST /projects/{project_id}/collection-policies；PATCH /collection-policies/{policy_id}；POST /devices/{device_id}/collection-jobs；GET /collection-jobs/{job_id}；POST .../retry | OpsCollectionPolicyList/Envelope、OpsJobEnvelope；周期/手动、timeout、safe retry、last_success、状态保留 |
| 维护/维修/复验/退役 | GET/POST /devices/{device_id}/maintenance-events；POST /devices/{device_id}/retirement-jobs | OpsMaintenanceEventList/Envelope、OpsJobEnvelope；绑定Issue/审批/验证，不能改变ipmi_only或恢复通道 |
| 告警/工单/作业/审计关联 | GET /devices/{device_id}/operations/links | OpsAssetLinksEnvelope；关联现有Alert/Issue/Execution/PushJob/Approval/Audit IDs与时间，不复制平行工单或告警系统 |

### 操作语义
ObservationContext是所有新观测的必需字段。ConfigBaseline映射既有NetworkBaselineRevision或ConfigSetRevision；ConfigRevision为统一引用。FirmwareInventory按component_asset_id关联BOM子件；CollectionJob/FirmwareUpgradeJob复用Job/Execution/Lease，所有命令写审计和Outbox。

维护状态变更事件附reason、window、issue/change/approval/verification refs和expected_revision；状态非法或版本冲突拒绝。retirement_jobs必须确认legal_hold/保留策略、采集停止、凭据撤销及关联历史保留；结果未知必须对账。

全局device路径仍按资源所属project授权；list/detail/history/export/job/subscribe同策略。维护态限制与能力状态独立：supported并不代表policy_allowed。ops Mock与HTTP实现使用同一OpenAPI类型；新增设计schema不代表Handler已经实现。

运维退役终态写入只允许已批准 RetirementJob 完成其停止采集、凭据撤销与保留检查后产生；MaintenanceRequest 不允许直接选择 retired，禁止通过普通维护事件绕过退役审批。

## IA v2.3 · 双平面与统一抽屉

IA v2.3 专用契约：GET /ia/session 返回授权项目、permissions[]、permission_revision；GET /ia/cross-project/{resource} 使用授权范围与可选项目过滤；GET /ia/projects/{project_id}/devices/{device_id}/tabs/{tab} 仅懒加载指定领域Tab；GET /ia/events/stream 使用 Last-Event-ID 与事件权限复核。数据模型见 OpenAPI IA* schema。历史 Ops API、网络审批和门禁契约继续生效。

## IA v2.4 实施增补

保留IA、Ops和现有领域资源，新增前先收紧专用schema；不以通用数据对象冒充冻结契约。Fabric/Fleet/Flows的子页面由各自领域API提供。


## IA v2.4 Mock 前端实施记录

导航树、完整页面清单、路由/权限/作用域/加载策略、五/八Tab抽屉与API对照统一见 [IA24-IMPLEMENTATION-MATRIX.md](IA24-IMPLEMENTATION-MATRIX.md)。原FR-0…24、门禁、BOM与维护态规则保留。当前实现为前端Mock，不表示已实现Go/真实设备/真实AI。权威种子为 `fixtures/demo-full-64-b300/demo-seed.json`；原包83集合，补齐与规范化后集合数量以当前manifest为准，主项目硬件数量不变。结构检查、数据修复、契约与交互结果以IA24-*报告为准，无法采集/缺少证据保持unknown或阻断。


## IA24-05 · 用户确认的GPU-NIC默认映射
用户于本轮明确：没有单独说明时，GPU1–8分别对应NIC1–8。对当前B300切片按此规则确认512条GPU-NIC关系，NIC9/10保留存储角色；不再把已有LLDP链路显示成全部unknown。矩阵行GPU、列NIC，点击显示物理端口、线缆、交换机、来源与观测；配对关系与线序/新鲜度判据分开，异常不能被配对规则覆盖。


## 当前六阶段主链路修正

本轮用户明确废弃数字门禁作为主链。当前唯一业务顺序、状态机、逐目标放行、安全交接、独立BOM菜单与API约束以[DELIVERY-LIFECYCLE.md](DELIVERY-LIFECYCLE.md)为准。历史编号只按[DELIVERY-MIGRATION.md](DELIVERY-MIGRATION.md)追溯，不得用于解锁。验机失败关联统一Issues并定向复验；NCCL不反写验机；清理/归档/客户签署齐全后才进入维护态。NP仍是网络内部断言。当前仅前端Mock与契约实现，没有真实设备后端。


## 单柜详情工作区增补

固定单柜窄画布、左侧层级导航和右侧八Tab为本轮布局要求；交互、API、元数据、Mock来源及待执行验收见[RACK-WORKSPACE.md](RACK-WORKSPACE.md)。资产/位置和既有安全模型不变。右侧按需加载，不以缺失指标显示正常。当前构建/浏览器验证按用户安排等待统一执行。


## Fabric Topology 增补

FabricTopologyGraph返回nodes/edges/total_nodes/total_edges/next_cursor/warnings/snapshot_at/request_id。node/edge引用CMDB UID，物理边必须source_port_id/target_port_id/cable_id。detail返回双端、显式network关联、逐字段Diff、LLDP/FDB/ARP/DDM/BGP及统一Issue/Execution。物理/逻辑查询与节点/边详情均具专用schema，不能退化GenericObject。

完整导航、模型、交互、API表与实施边界见[FABRIC-TOPOLOGY.md](FABRIC-TOPOLOGY.md)。


## 无损网络分层证据 v2

Lossless* Schema定义scope、适配范围、四层证据、原值/单位/窗口delta、有效期、判据原因、Issue与Execution。list默认当前端口/规则最新观测，历史raw保留；证据冲突或缺口不可PASS。collect仅消费只读Mock帧，不重写采集时间；retest不自动修复、不自动调参。

规则/官方章节/命令/版本/判据/证据对照、历史迁移、真实缺口见[LOSSLESS-EVIDENCE.md](LOSSLESS-EVIDENCE.md)。


## CMDB 全链路资产档案增补

CmdbDossier*资源Schema明确目录状态维度、关系类别与规划/观测、字段来源、配置版本、生命周期和质量。raw evidence为允许领域资源的专用schema union，禁止任意集合访问。实际浏览器返回的十类response（含人工审计证据）另用JSON Schema逐项验证，结果见CMDB-DOSSIER-VALIDATION.json。

唯一详细设计与模型边界见[CMDB-DOSSIER.md](CMDB-DOSSIER.md)。

## BOM 真实开发增补

用户已授权在 `dev/` 真实实现 BOM 模块。数据与权限边界、版本/审核/Outbox、真实六步工作台及运行验收统一见 [BOM-REAL-DESIGN.md](BOM-REAL-DESIGN.md)。FR-2.1 和唯一 BOM/资产身份保持；其他模块的 Mock 状态不因本模块接入而视为后端已完成。实际通过范围以 BOM-REAL-ACCEPTANCE.json 为准。

## 2026-10-30 · Identity 真实契约

用户、角色/权限、项目成员、Session、登录审计、操作审计及审批的专用 Identity* Schema 与各操作以 openapi.yaml 为唯一 HTTP 定义；不得再以 GenericObject 实现这些接口。全局管理员资格与项目业务签署资格分离；审批检查只校验冻结范围，明确不执行设备。实现/安全边界见 IDENTITY-RBAC.md。

## CMDB 真实领域契约

OpenAPI中的CmdbSchema*、CmdbChange*、CmdbRevision*、CmdbRelationshipRevision与原CmdbDossier*共同构成真实契约。保持五/八Tab抽屉旧数据形状；资产创建/更新/退役/换件、关系新增/结束统一由ChangeSet表达；禁止客户端直接覆盖批准配置与采集字段。

接口对照、模型与实现边界统一见 [CMDB-REAL-DESIGN.md](CMDB-REAL-DESIGN.md)，具体HTTP Schema以openapi.yaml为准。

## 项目与 AI 配置真实落地

AIProviderConfiguration/Command/TestCommand/TestResult/Option是系统配置与BOM共享契约。连接失败由status=failed及error_code/http_status表达，不伪造成功；AI任务继续使用mapping/v1候选与人工审核契约。

唯一详细说明与API对照见 [PROJECT-AI-CONFIGURATION.md](PROJECT-AI-CONFIGURATION.md)。


## BOM Intelligence Agent 当前实施（2026-10-30）

本轮仅 AI 智能识别：上传→真实分析进度→分类审核→确认入库。交换机/端接优先，单主编排器+Workbook IR 工具+独立 Schema/来源/覆盖校验。当前范围替代此前 BOM 双模式/六步 UI 描述，历史数据不删除；详细字段、状态、API、安全和验证边界统一见 [BOM-WORKBENCH-V2.md](BOM-WORKBENCH-V2.md)。保留 FR-2.1、项目权限、唯一资产身份和原事务 Outbox。完整导入入库闭环由用户验收；不以旧报告宣称当前通过。


## 2026-10-31 数据中心可视化增补

新增运维 `/topology` 与真实 CMDB 机柜工作台；投影接口、布局/事实边界、asset.relocate 审核发布、验证记录和未完成项统一见 [DATACENTER-VISUALIZATION.md](DATACENTER-VISUALIZATION.md)，HTTP schema 见 openapi.yaml。沿用原资产身份和权限，不把计划连接或 synthetic 数据视为真机验证。

## VISUAL-REAL-2026-10-31 · 已批准可视化的真实只读投影

`GET /api/v1/cmdb/visualization` 保留原摘要查询，增加 `detail=full`；类型定义以 OpenAPI `InfrastructureSnapshot` 为唯一 HTTP 定义。正式五个可视化页面通过 `infrastructureApi.fullSnapshot` 读取 PostgreSQL 中既有资产和类型化关系，不导入原型 fixture。

- `project_id` 是会话授权范围的过滤条件；缺省为当前用户所有获授权项目，而不是所有项目。资源级授权时关系两端均须可见。不可见/不存在端点用空引用保留缺口，禁止回显隐藏 ID。
- full 默认最多 5000 个源资产，可用 `limit=1..5000` 降低；关系最多 20000 条，均限定于本次可见源资产集合。超限 `truncated=true`、`coverage.complete=false`；客户端须提示缩小项目/机柜/focus，不能宣称全量或完整路径。旧摘要默认源资产上限 20000 保持兼容。本轮未实现 cursor 分页、历史快照保留或无限范围下载。
- `assets` 包含已登记机柜/设备/部件/端口/网络实体；`relationships` 保留类型、方向端点、fact_layer、source/source_ref/revision，不把 logical_membership 变成 Cable。`cables` 来自现有 Cable 与双端 port 关系，保留缺端记录。`owner_device_id` 仅沿明确设备/组成关系受控解析；不按显示名配对。
- 同次查询使用 PostgreSQL repeatable-read 只读事务。`snapshot_id` 是范围和资产/关系修订指纹，不是可回查的历史版本；`read_at` 只是查询时间，不能替代观测采集时间。`source_ref` 保留实际来源；数据库中源为 synthetic/fixture 的记录不能因接入真实 API 被标成真实采集。
- `coverage.total_assets` 是授权源集合数量，`returned_assets` 为本次实际返回数量。机柜/focus 查询仍可返回触及该范围的对端，因此查询数量不是该机柜实物数量。设备、端口、线缆与机柜的正式主键保持不变。
- 物理贯通不能通过任意交换机端口自动拼接。当前无显式 patch-through 时只证明 Cable 双端；缺少 FIB/VRF/BGP 等逻辑转发证据应显示 unsupported，不能用设备级 BFS 冒充逻辑路径。

投影复核补充：缺失的 `gpu_count`/`nic_count` 仅在源查询未截断且有明确 `contains` 成员时，以唯一组件 ID 确定性计数，并附 `*_count_source` 和关系 ID；没有成员不补零，不覆盖已登记数量。位置附 `position_fact_layer`、`position_source_ref`、`position_revision` 与完整 `located_in_sources`。规划/观测引用不同时保留分层候选并报数据质量提示，不将最后一条随机排序关系当成实际安装；同层多目标不自动选中。Cable 双端事实层不一致使用 `mixed`，两端独立来源仍可查。

`findings[].severity/category` 表达数据质量提示，不是 PASS/FAIL。端口占用、速率不一致、未采集面板等均不能直接宣称设备故障或验机失败；必须通过原业务规则和适用证据判断。可视化不得把所有 warning 统一转换成硬件 FAIL。

## VISUAL-PAGED-2026-10-31 · 正式可视化受控查询

批准设计 §9.3 的四个端点已接入同一 CMDB Service（没有新资产表、布局写入或导入）：

| 端点 | 受控查询与结果 |
|---|---|
| `GET /cmdb/visualization/subgraph` | `view=hierarchy/fabric/physical`，`root_id/fabric_id/q/depth`。SQL 先确定最多 200 个种子 ID，再加载其结构上下文；层级 depth1 只设备/位置摘要，depth2 才加载组成和端口。返回兼容 InfrastructureSnapshot 的 `assets/cables/relationships/findings`，另有真实授权 `groups.counts` 与 `page`。 |
| `GET /cmdb/visualization/matrix` | `axes=server_switch/leaf_spine/rack_leaf/port_pair`，按真实 Cable 双端在 SQL 聚合、按 cell 分页；Axis 仍是原 InfrastructureAsset。`rack_leaf` 仅柜内服务器→Leaf，不混交换机上联。cell 保留分类 `groups(fact_layer/media_type/speed)` 和可回查 `member_query_ref`。 |
| `GET /cmdb/visualization/connections` | `root_id/asset_id/port_id/member_query_ref/q/status` 先过滤再按 Cable ID 分页；member 引用冻结原范围与分类，不返回整库后客户端筛选。可见孤立资产或端口即使无线缆也返回本体。`status` 当前仅 `all/incomplete`，其余状态必须明确是当前窗口筛选。 |
| `POST /cmdb/visualization/trace` | body `project_id/mode/from_port_id/to_port_id?/candidate_cable_id?/snapshot_id?`；body/query 项目冲突拒绝，按有效项目重新授权。from/to 必须为可见 PhysicalPort；不存在/不可见/错误类型统一404。多 Cable 为 ambiguous，明确选择的 candidate 仍须属于起始端口。不通过交换机推断内部转发；logical 缺 FIB/VRF 证据返回422。 |

所有分页 `limit=1..200`，默认100。cursor 用进程内 HMAC 验签，绑定用户、`permission_version`、授权 Projects/Pairs、过滤参数及当前资产/关系修订指纹；每次仍重新进行后端授权。进程重启后旧 cursor 明确无效，不作为权限凭据。不同 view/root 可复用同一授权快照指纹；发生变更或复用过期 cursor/snapshot 返回409，不自动续到新快照。没有保留历史快照，`at/evidence_at` 请求422，不能静默返回当前数据。

`groups.counts` 是授权范围现存关系的去重登记计数，非实物验收结论；最多200个group，超过则 `truncated=true` / `coverage.complete=false`。结构上下文最多5000资产、20000关系，任何损失都标partial。subgraph/connections 的coverage计量资产；matrix使用专用 `returned_cells/total_cells`，Cable总数另在included/excluded中，不能混用单位。rack `registered_device_count/placement_coverage/visualization_summary` 区分完整本柜几何与对端摘要，不能把邻接的少量设备当成完整空柜/满柜。正反面容量继续由同一几何区间并集计算，不在API猜测未知U位或深度。

Fabric 筛选仅使用显式 device/port→logical_membership→network_segment/Fabric 归属及已有fabric字段，不根据线缆生成逻辑网络；`__unassigned__` 是查询筛选值，不是新资产。`unplaced=true` 是位置未分配视图范围，不创建虚拟机柜。搜索兼容顶层属性与旧 `payload.fields`，匹配SN、Hostname、管理/BMC/IP、MAC、physical_name和稳定ID。端口多设备归属保持未知，不使用最小ID或第一条关系代替；矩阵排除缺口，连接页仍保留异常记录。分类空值是未登记，不换算速率、不补零、不自动判健康。超时返回504。

本轮只执行新查询的纯逻辑/契约检查及Go编译，未连接数据库运行这些新端点，未重启服务、未导入/迁移数据；此前full投影性能记录不能当成新分页SQL性能证明。
