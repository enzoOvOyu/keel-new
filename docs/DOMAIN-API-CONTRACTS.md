# DOMAIN-API-CONTRACTS.md — 前端全量 API 资源契约

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
