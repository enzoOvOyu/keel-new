# keel API Conventions

**Status**: normative. HTTP handlers, generated frontend types, integration tests and docs must follow this contract. Detailed endpoint schemas live in `openapi.yaml`; if prose and OpenAPI differ, record and resolve the discrepancy before implementation.

## 1. Protocol and naming

- Base path: `/api/v1`; HTTPS in non-local environments.
- JSON request/response: UTF-8, `Content-Type: application/json`; upload uses `multipart/form-data`.
- Resource names are plural kebab-case (`/bom-imports`, `/push-jobs`); JSON properties are `snake_case` to align with domain and CSV schema.
- IDs are opaque strings. Do not expose database integer keys as public identity. Timestamps are RFC 3339 UTC (`2026-10-29T12:00:00Z`). Durations use seconds or explicitly named units.
- List endpoints return a consistent envelope with `data`, `page`, `request_id`. Default page size 50, max 200; cursor pagination for high-volume event/log streams.
- Mutations return the updated resource or `202 Accepted` with a job/resource reference. Long operations must not block the HTTP request until completion.
- No secrets in URLs, query strings, logs, audit payloads, or parameter snapshots.

## 2. Authentication, authorization and project scope

- Production uses the platform's configured identity provider/session mechanism. API supports short-lived bearer access tokens where applicable; refresh/session handling follows the deployed IdP.
- Every project resource is scoped by `project_id`. Server-side authorization is mandatory for every request; hiding buttons in React is not authorization.
- Roles: `DE`, `DL`, `NE`, `NDE`, `NH`, `Admin`, `SRE`, `Viewer`; actual permissions are module × action × project scope.
- Critical network change approval: NE + DE + NH; ordinary change: NE + DE. A user cannot satisfy multiple required approvals with one identity unless explicit policy allows it (default deny).
- Use `403` for authenticated but unauthorized; `404` may be used for resources outside the caller's scope to avoid disclosure, consistently.

## 3. Request headers

| Header | Rule |
|---|---|
| `Authorization` | `Bearer <token>` when token auth is configured |
| `X-Request-ID` | Optional client correlation ID; server validates length/characters or generates one |
| `Idempotency-Key` | Required for create/commit/publish/execute operations that may be retried; unique within actor + endpoint scope |
| `If-Match` | Required for edits to versioned mutable drafts when concurrent modification is possible; use ETag/revision |
| `Content-Type` | JSON except file upload |

Idempotency records store request hash and result. Same key + same request returns the original result; same key + different request returns `409 IDEMPOTENCY_KEY_REUSED`.

## 4. Response conventions

Success:

```json
{
  "data": { "id": "imp_01...", "status": "parsing" },
  "request_id": "req_01..."
}
```

List:

```json
{
  "data": [],
  "page": { "limit": 50, "next_cursor": null, "has_more": false },
  "request_id": "req_01..."
}
```

Error:

```json
{
  "error": {
    "code": "VALIDATION_FAILED",
    "message": "One or more fields failed validation.",
    "details": [
      { "path": "files[0].sheet_name", "code": "UNKNOWN_SHEET", "message": "Sheet could not be resolved." }
    ],
    "retryable": false
  },
  "request_id": "req_01..."
}
```

- `message` is safe for display; never include stack traces, secrets, raw credentials or full sensitive cell values.
- `details[].path` uses JSONPath-like request paths or domain locator paths.
- Error codes are stable machine contracts; UI branches on `code`, not English/Chinese message text.

## 5. HTTP status and error codes

| HTTP | Meaning | Typical codes |
|---|---|---|
| 400 | malformed request/schema | `INVALID_REQUEST`, `INVALID_JSON`, `UNSUPPORTED_FILE_TYPE` |
| 401 | no/invalid authentication | `UNAUTHENTICATED`, `TOKEN_EXPIRED` |
| 403 | permission denied | `FORBIDDEN` |
| 404 | absent/out-of-scope resource | `NOT_FOUND` |
| 409 | state/version/idempotency conflict | `STATE_CONFLICT`, `REVISION_CONFLICT`, `IDEMPOTENCY_KEY_REUSED`, `BATCH_DIGEST_CONFLICT` |
| 413 | payload too large | `FILE_TOO_LARGE` |
| 422 | syntactically valid but domain invalid | `VALIDATION_FAILED`, `BLOCKING_ISSUES_PRESENT`, `REFERENCE_AMBIGUOUS` |
| 429 | rate limited | `RATE_LIMITED` |
| 500 | unexpected server error | `INTERNAL_ERROR` |
| 502/503/504 | adapter/dependency unavailable/timeout | `DEPENDENCY_UNAVAILABLE`, `DEPENDENCY_TIMEOUT` |

`retryable=true` only if retrying the same request is safe. A timeout after a side-effect may have occurred must return a resource/job reference or `result_unknown`; it must not encourage blind replay.

## 6. Pagination, filtering and sorting

- Small/medium list: `limit`, `offset` only where stable; prefer cursor for events, logs and large datasets.
- Cursor response contains opaque `next_cursor`; clients must not construct it.
- Filter syntax: documented per endpoint, e.g. `status=BLOCKED&category=switch&search=leaf-01`.
- Sort syntax: `sort=created_at:desc`; only allowlisted fields. Invalid sort/filter returns `400 INVALID_REQUEST`.
- Server-side filter/sort/page occurs before data serialization. Large tables must not fetch all 10k+ rows to filter in browser.

## 7. Asynchronous jobs and event stream

- Upload parsing, AI mapping, validation, package build, PXE, collection, Flow, PushJob and exports are asynchronous.
- Create returns `202` with `job_id`/resource ID and current status. Poll `GET /jobs/{id}` or domain resource; do not expose internal queue IDs.
- Job common fields: `id`, `kind`, `status`, `progress` (`current`, `total`, `percent` nullable), `stage`, `created_at`, `started_at`, `finished_at`, `retryable`, `error_summary`, `request_id`.
- SSE endpoint: `GET /events/stream?resource_type=...&resource_id=...`; events include `event_id`, `sequence`, `occurred_at`, `resource_type`, `resource_id`, `event_type`, `data`. Support `Last-Event-ID`; client reconnects and deduplicates by `event_id`.
- DB is state source of truth; event stream is notification, not a substitute for fetching the resource snapshot.

## 8. BOM import API

### Endpoints

| Method | Path | Purpose |
|---|---|---|
| POST | `/projects/{project_id}/bom-imports` | Upload raw workbook or standard package; create import |
| GET | `/bom-imports/{import_id}` | Status and summary |
| GET | `/bom-imports/{import_id}/sheets` | Detected sheets/table regions |
| GET | `/bom-imports/{import_id}/mappings` | Candidate/approved field mappings |
| PATCH | `/bom-imports/{import_id}/mappings/{mapping_id}` | Correct a candidate mapping; draft revision only |
| POST | `/bom-imports/{import_id}/map` | Run/re-run deterministic + AI mapping stage |
| POST | `/bom-imports/{import_id}/validate` | Run validation against selected revision |
| GET | `/bom-imports/{import_id}/issues` | Issues filtered by severity/status/source locator |
| PATCH | `/bom-imports/{import_id}/issues/{issue_id}` | Resolve/confirm issue with reason; cannot override BLOCK rules without authorized policy |
| GET | `/bom-imports/{import_id}/preview` | Read-only normalized diff and entity counts |
| GET | `/bom-imports/{import_id}/issues/export` | Export CSV issue list |
| POST | `/bom-imports/{import_id}/build-package` | Build standard package and manifest after eligible validation |
| POST | `/bom-imports/{import_id}/commit` | Commit approved server+fabric BOM atomically |
| GET | `/projects/{project_id}/bom-batches` | List BOM versions/batches |
| GET | `/bom-batches/{batch_id}/diff` | Compare with previous committed version |
| GET | `/bom-batches/{batch_id}/items` | Query BOM items |

### Raw upload request

`multipart/form-data`: `file` (required), `template_revision_id` (required), optional `source_domain` (`network|server|combined`) and `declared_batch_id` if known. Project is taken from path and must match template scope. File extension is not trusted; inspect actual content. File size/row limits are deployment configuration and must be documented in runtime config.

Response: `202 Accepted` with `data.id`, `data.status=uploaded|parsing`, `data.source_document_id`, `data.content_sha256`.

### Mapping contract

Each mapping candidate contains:

```json
{
  "id": "map_...",
  "source": {"document_id":"doc_...","sheet":"Switches","cell":"B3","header":"Mgmt IP","raw_value":"10.20.1.11"},
  "target_field": "management_ip",
  "normalized_value": "10.20.1.11",
  "method": "template_rule|deterministic|llm_candidate|manual",
  "confidence": 0.98,
  "status": "candidate|confirmed|rejected|needs_review",
  "mapping_revision": 3,
  "reason": "Header alias and IPv4-shaped value match management IP field."
}
```

Confidence is advisory only. Never accept an AI mapping solely because `confidence` is high. Unknown source columns become issues; they cannot be silently dropped.

### Validation issue contract

Fields: `id`, `rule_id`, `rule_version`, `severity` (`BLOCK|WARN|INFO`), `code`, `message`, `source_locator` (`document_id/sheet/row/cell/column`), `entity_ref`, `field_path`, `expected`, `actual_summary`, `status` (`open|resolved|accepted_warning`), `resolution`, `resolved_by`, `resolved_at`.

Sensitive raw values may be masked in API responses according to field policy; authorized reviewers can inspect necessary source values.

### Commit preconditions

Commit must fail with `422 BLOCKING_ISSUES_PRESENT` if any unresolved BLOCK exists, and with `409 REVISION_CONFLICT` if source/mapping/validation revision changed after preview. Standard combined package commits server + fabric rows and relations in one DB transaction. Same batch + same digest is idempotent; same batch + different digest returns `409 BATCH_DIGEST_CONFLICT`. Committed data is immutable; correction uses a new revision/batch.

## 9. Common domain endpoints

| Domain | Endpoints (minimum contract) |
|---|---|
| Projects/templates | `GET/POST /projects`, `GET /projects/{id}`, `GET /projects/{id}/template-revisions`, `POST /templates/{id}/revisions`, `POST /template-revisions/{id}/release` |
| BOM/inventory | `GET /bom-batches/{id}/items`, `GET /bom-items/{id}`, `GET /bom-items/{id}/history`, `GET /projects/{id}/assets` |
| Device models | `GET/POST /device-models`, `POST /device-models/{id}/revisions`, `POST /device-model-revisions/{id}/release` |
| PXE/collection | `POST /projects/{id}/provisioning-jobs`, `GET /provisioning-jobs/{id}`, `POST /machines/{id}/collect`, `GET /machines/{id}/observations` |
| Gates/Mapping | `GET /projects/{id}/gate-evidence`, `GET /machines/{id}/gates`, `POST /gates/{id}/waivers`, `GET /mapping-revisions`, `POST /mapping-revisions` |
| Network | `GET /fabric/devices`, `GET /fabric/devices/{id}`, `GET /fabric/readiness`, `POST /push-jobs`, `GET /push-jobs/{id}`, `POST /push-jobs/{id}/precheck`, `POST /push-jobs/{id}/approve`, `POST /push-jobs/{id}/execute`, `POST /push-jobs/{id}/rollback` |
| Flow/execution | `GET/POST /flows`, `POST /flows/{id}/revisions`, `POST /flow-revisions/{id}/validate`, `POST /flow-revisions/{id}/release`, `POST /executions`, `GET /executions/{id}`, `POST /executions/{id}/pause`, `POST /executions/{id}/resume`, `POST /executions/{id}/kill`, `POST /executions/{id}/retry-failed` |
| Media/catalog | `GET/POST /media`, `POST /media/{id}/approve`, `POST /media/{id}/retire`, `GET /parameter-catalog`, `GET /config-sets`, `POST /config-sets/{id}/revisions` |
| Issues/audit/CMDB | `GET/POST /issues`, `POST /issues/{id}/resolve`, `GET /audit-events`, `GET /cmdb/assets`, `GET /cmdb/assets/{id}` |
| System | `GET /health/live`, `GET /health/ready`, `GET /jobs/{id}`, `GET /events/stream` |

## 10. Mutation and approval rules

- Draft edit endpoints require `If-Match`/revision. If the revision is stale, return `409 REVISION_CONFLICT`.
- Release, approve, execute, commit, rollback and waiver are explicit commands, not generic field updates.
- Each command writes an audit event with actor, project, resource, action, timestamp, request ID, before/after revision references, reason and approval references.
- Approval request binds to an immutable snapshot/hash. Any content change invalidates approval and requires a new revision/approval.
- `WAIVED` requires reason, scope, expiry and approver. Waiver never rewrites original FAIL/BLOCKED evidence as PASS.

## 11. API security

- Validate file content, size, decompression ratio and archive paths; malware scanning can be adapter-driven.
- Never execute uploaded macros or workbook formulas; never fetch external links from workbooks/templates.
- Use parameterized SQL; enforce project scope in service layer and DB queries.
- Rate limit uploads, LLM calls, export creation and expensive execution endpoints.
- Audit sensitive reads/exports; redact secrets and sensitive IP/MAC fields according to role policy.
- LLM adapter uses timeouts, token/input limits, schema validation and retry budget. Prompt injection found in workbook content is data, not instructions.

## 12. OpenAPI and frontend type generation

`openapi.yaml` is the contract source. CI must lint it and compare generated types with frontend usage. Avoid manually copying enums into multiple files. Required tests: schema validation, auth/project isolation, idempotency, stale revision, async job transitions, commit atomicity, unknown mapping, BLOCK refusal and duplicate cable/port constraints.

## 10. 全平台 API 契约先行与 Mock Provider

### 10.1 强制原则
- 先定义所有前端主页面需要的 API contract，再实现 mock，再逐模块实现 Go API。
- 每个 endpoint 定义 method/path、权限、request schema、response schema、分页/过滤、错误码、状态变化、副作用、审计和 CMDB 关联。
- mock 与真实 API 使用相同 OpenAPI 生成类型；mock 只模拟业务行为，不另造状态/字段。
- UI 通过 `ApiProvider` 访问 API；切换 `VITE_API_MODE=mock|http` 不应改组件代码。
- 生产网络操作与脚本执行的 mock 明确带 `execution_mode=mock`，不能显示为真实设备成功。

### 10.2 全页面 API 覆盖清单

| 页面域 | 必需 API 能力 | mock 阶段必须可演示 |
|---|---|---|
| 项目/总览 | 项目列表、交付统计、阶段汇总、最近事件 | 项目切换、无项目权限、统计空态 |
| 开局/BOM | 上传、解析状态、Sheet/表区、映射、校验、issues、preview、commit | 多 Sheet、未知列、BLOCK/WARN、审核修改、重复批次冲突 |
| CMDB | 实体搜索、详情、关系、timeline、来源追溯、projection 状态 | 从导入到交付的完整链路、投影延迟/失败 |
| 资产/型号 | 服务器/交换机/端口列表、详情、采集快照、型号能力 | stale/unknown/unsupported、身份冲突 |
| Flow | CRUD、版本、目标预览、参数 schema、schedule、run | 手动/定时运行、禁用、版本 diff |
| Execution | 列表、详情、targets、attempts、logs、outputs、pause/resume/kill/retry | running/partial/failed/result_unknown、日志持续追加 |
| 网络底座 | 基线、模板、ZTP、PushJob、diff、审批、DDM、LLDP、健康 | 审批阻断、unknown、漂移、回滚 |
| 服务器交付 | PXE、Redfish、六阶段交付主链路、Mapping、ConfigSet | 门禁未满足、证据过期、定向重验 |
| 工单 | 列表、详情、证据、指派、处置、重验、关闭 | T1-T7 与执行/端口关联 |
| 监控/告警 | 指标、日志、告警、趋势、事件时间线 | 数据缺失、降级、告警聚合 |
| 模板/介质/参数 | 版本、审批、兼容性、引用关系、回滚 | draft/released/deprecated、兼容失败 |
| 用户/审计 | 用户、角色、项目授权、审计事件 | 403、权限变更、脱敏审计 |

### 10.3 Mock 行为契约

- 以 `docs/MOCK-DATA-CONTRACT.md` 为准。mock 必须支持固定场景种子，便于复现；禁止 `Math.random()` 生成不可复现状态。
- 统一响应 envelope 与真实 API 一致。异步操作返回 `job_id` 或 `execution_id`，UI 轮询/SSE 获取状态。
- 关键操作会改变 mock store（创建 Flow、启动执行、暂停、重试、审核 BOM、提交 BOM、关闭工单），刷新后行为在当前 mock session 内保持一致。
- 错误通过真实错误码模拟，包括 400、401、403、404、409、422、429、500、503；页面展示用户可操作的错误提示和 request_id。

### 10.4 CMDB 写入语义

业务 API 不要求前端单独调用“同步 CMDB”。业务变更在后端事务内写 Outbox，CMDB projector 异步更新读模型。响应可返回 `cmdb_projection: pending|complete`，并提供关联查询；前端显示投影状态但不重复提交写入。事件契约与幂等规则见 `SYSTEM-DESIGN.md`。

## 11. 前端全量 API 端点清单

以下是完整页面需要的 endpoint inventory。实现某域前必须将该域请求/响应 schema 写入 `openapi.yaml`，不能仅凭此清单猜字段；mock 阶段也必须按同一 schema 实现。

| Domain | Endpoint group (suggested) |
|---|---|
| Projects | `GET/POST /projects`, `GET/PATCH /projects/{project_id}`, `GET /projects/{project_id}/overview` |
| BOM import | 现有 `/projects/{project_id}/bom-imports` 与 `/bom-imports/{id}/*` 全链路 |
| CMDB | `GET /cmdb/entities`, `GET /cmdb/entities/{entity_id}`, `GET /cmdb/entities/{entity_id}/relationships`, `GET /cmdb/entities/{entity_id}/timeline`, `GET /cmdb/events/{event_id}` |
| Inventory | `GET /projects/{project_id}/devices`, `GET /devices/{device_id}`, `GET /devices/{device_id}/ports`, `GET /devices/{device_id}/snapshots` |
| Flow definitions | `GET/POST /projects/{project_id}/flows`, `GET/PATCH /flows/{flow_id}`, `GET /flows/{flow_id}/revisions`, `POST /flows/{flow_id}/validate`, `POST /flows/{flow_id}/run` |
| Schedules | `GET/POST /flows/{flow_id}/schedules`, `PATCH /schedules/{schedule_id}`, `POST /schedules/{schedule_id}/enable`, `POST /schedules/{schedule_id}/disable` |
| Executions | `GET /executions`, `GET /executions/{execution_id}`, `GET /executions/{execution_id}/targets`, `GET /executions/{execution_id}/tasks`, `GET /executions/{execution_id}/logs`, `GET /executions/{execution_id}/outputs`, `POST /executions/{execution_id}/pause`, `/resume`, `/kill`, `/retry-failed` |
| Fabric | `GET /projects/{project_id}/fabric/devices`, `GET /fabric/devices/{device_id}`, `GET /fabric/devices/{device_id}/ports`, `GET/POST /projects/{project_id}/network-baselines`, `GET /push-jobs`, `GET /push-jobs/{push_job_id}`, `POST /push-jobs/{push_job_id}/approve`, `/execute`, `/rollback` |
| Delivery gates | `GET /projects/{project_id}/gates`, `GET /devices/{device_id}/gate-evidence`, `POST /delivery-runs`, `GET /delivery-runs/{run_id}` |
| Issues | `GET/POST /projects/{project_id}/issues`, `GET/PATCH /issues/{issue_id}`, `POST /issues/{issue_id}/revalidate`, `POST /issues/{issue_id}/close` |
| Monitoring | `GET /projects/{project_id}/metrics`, `GET /alerts`, `GET /alerts/{alert_id}`, `POST /alerts/{alert_id}/acknowledge` |
| Templates/media/catalog | `/project-templates`, `/media`, `/parameter-catalog`, each with list/detail/version/approve/retire actions defined in OpenAPI before implementation |
| Auth/audit | `/auth/session`, `/auth/logout`, `/users`, `/roles`, `/project-permissions`, `/audit-events` |

**注意**：此 inventory 是端点范围清单，不是所有 endpoint 已在 OpenAPI 中完整定义的声明。前端全量 mock 阶段必须先扩展 `openapi.yaml` 覆盖以上页面实际使用的 endpoint，再开始对接。

## 12. Rack Diagram API 约定

具体路径和 schema 以 `openapi.yaml` 与 `RACK-DIAGRAM.md` 为准。生成和导出使用异步任务（202 + job id）；读取指定 revision；编辑 placement 使用 `If-Match`；发布需重新校验并绑定精确 revision。不得提供无版本参数的覆盖式“保存当前图”接口。所有读写按 project RBAC，所有生成/编辑/发布/导出写审计与 CMDB 事件。

## OPS-2026-10-29 · 交付后运维设计增补

### 运维接口统一约束
沿用/api/v1、project RBAC、统一envelope、cursor分页和ErrorEnvelope。现有项目devices/fabric/devices、ports、snapshots、issues/alerts路径继续可用；新增 `/projects/{project_id}/operations/devices`、`/devices/{device_id}/operations/*`、`/projects/{project_id}/collection-policies`、`/collection-jobs/*`。asset_id映射既有稳定Device/BOM身份，不能仅凭全局device_id绕过项目鉴权。

- 列表：category、project（路径）、rack/model/delivery/health/alert/maintenance；交换机附fabric/pod/role/nos/configuration_state。分页默认50、最大200，过滤/排序字段白名单。设备详情为专用资源，不以GenericObject代替。
- 历史观测/指标/日志：since/until、snapshot_id或cursor，必须传明确资产范围；响应区分not_collected/available/expired，并带ObservationContext。unsupported、policy_denied、stale与无匹配数据不能折叠为[]+健康。
- 所有触发采集/电源/固件/维护/退役请求先RBAC+scope+AccessPolicy，参数冻结；异步202返回可查job_id/resource，UI不得把accepted画成执行成功。写操作使用Idempotency-Key；修改策略/维护状态需If-Match；冻结版本变更返回409并使旧审批失效。
- maintained/IPMI-only不因请求指定protocol或管理员权限而放宽。禁止SSH/Agent/Redfish及绕路代理；服务端返回403 ACCESS_POLICY_DENIED，能力不支持返回422 CAPABILITY_UNSUPPORTED，证据缺失/过期返回422 EVIDENCE_UNAVAILABLE；副作用未知返回任务result_unknown，不能直接重试执行。
- 请求/响应/日志/审计禁止秘密值。凭据由后端按用途解析受控绑定，普通设备接口只给credential_binding_id或受控脱敏引用；不接收原始password/token。
- 配置修改不新增旁路PATCH running-config；网络写操作仍走原PushJob状态机与NE+DE/Critical+NH。固件与电源任务必须验证独立租约、审批引用、动作/目标/版本、策略revision及执行前权限。
专用字段与路径以openapi.yaml Ops* schemas为准；行为见DOMAIN-API-CONTRACTS和OPERATIONS-DESIGN。

## IA v2.3 · 双平面与统一抽屉

IA v2.3：部署请求携带 project_id；跨项目查询服务端从会话授权集合求交，project_id 可选仅作为过滤，不接受客户端自称授权集合。分页与 request_id 沿用既有规范。SSE 支持 Last-Event-ID、断线补齐、连接及事件投递权限校验；项目变化取消旧请求。页面加载协议和查询 key 见 IA-V2.3.md。

## IA v2.4 实施增补

沿用/ia/session、/ia/cross-project、/ia/projects/.../tabs及/ia/events/stream；项目作用域、授权范围、Last-Event-ID、取消和错误语义见IA-V2.4，真实后端不得信任客户端权限列表。


## IA v2.4 Mock 前端实施记录

导航树、完整页面清单、路由/权限/作用域/加载策略、五/八Tab抽屉与API对照统一见 [IA24-IMPLEMENTATION-MATRIX.md](IA24-IMPLEMENTATION-MATRIX.md)。原FR-0…24、门禁、BOM与维护态规则保留。当前实现为前端Mock，不表示已实现Go/真实设备/真实AI。权威种子为 `fixtures/demo-full-64-b300/demo-seed.json`；原包83集合，补齐与规范化后集合数量以当前manifest为准，主项目硬件数量不变。结构检查、数据修复、契约与交互结果以IA24-*报告为准，无法采集/缺少证据保持unknown或阻断。


## 当前六阶段主链路修正

本轮用户明确废弃数字门禁作为主链。当前唯一业务顺序、状态机、逐目标放行、安全交接、独立BOM菜单与API约束以[DELIVERY-LIFECYCLE.md](DELIVERY-LIFECYCLE.md)为准。历史编号只按[DELIVERY-MIGRATION.md](DELIVERY-MIGRATION.md)追溯，不得用于解锁。验机失败关联统一Issues并定向复验；NCCL不反写验机；清理/归档/客户签署齐全后才进入维护态。NP仍是网络内部断言。当前仅前端Mock与契约实现，没有真实设备后端。


## 单柜详情工作区增补

固定单柜窄画布、左侧层级导航和右侧八Tab为本轮布局要求；交互、API、元数据、Mock来源及待执行验收见[RACK-WORKSPACE.md](RACK-WORKSPACE.md)。资产/位置和既有安全模型不变。右侧按需加载，不以缺失指标显示正常。当前构建/浏览器验证按用户安排等待统一执行。


## Fabric Topology 增补

新增/projects/{project_id}/fabric/topology/{physical|logical|filters}与/details/{kind}/{id}，资源专用FabricTopology* schemas。显式项目ID、范围/分组/过滤/cursor/limit(10..120)/at；local必须device_id。权限失败403、无对象/历史404、参数422。只读查询可重试，不产生审批或设备副作用；懒加载协议见FABRIC-TOPOLOGY。

完整导航、模型、交互、API表与实施边界见[FABRIC-TOPOLOGY.md](FABRIC-TOPOLOGY.md)。


### Rack Mapping：所选设备关联追溯（当前定义）

Mapping恢复重构前三列工作区中“设备与连接追溯”的内容，不再作为GPU-NIC矩阵页面。支持服务器和交换机：设备型号/SN/稳定ID/来源BOM、草稿与实际位置、“连接/端口”切换、线缆A/B两端设备/端口/介质/长度、MAC与状态、对端定位和统一设备详情抽屉。切换画布设备或定位对端保留Mapping上下文，清除旧设备的关联内容；连接和端口支持搜索、分批加载，缺失引用显示unknown。GPU-NIC矩阵继续由Topology/Validation承载。

workspace/mapping按需返回ports/cables及授权peer_ports/peer_devices；关联来自原有BOM/CMDB稳定ID，不创建资产副本。Cable双端与端口peer引用不同要明确提示冲突，不以端口状态或历史线序结果伪造当前验证通过。


## 无损网络分层证据 v2

新增项目显式lossless evidence/conclusion/collect/retests/raw资源专用Schema和LosslessProvider。查询on-demand+AbortSignal；复验冻结config/topology revision与幂等键；403/409/422独立；raw只返回授权设备合成帧，后续生产raw必须脱敏。

规则/官方章节/命令/版本/判据/证据对照、历史迁移、真实缺口见[LOSSLESS-EVIDENCE.md](LOSSLESS-EVIDENCE.md)。


## CMDB 全链路资产档案增补

新增/cmdb/dossier专用catalog/tree/relations/provenance/configuration/lifecycle/quality/manual-fields/saved-views/evidence接口。project_id可清除，仅授权项目；目录cursor+limit 1..100+排序；关系limit最多120、expanded最多10；写入冻结项目/资产/版本且拒绝白名单外字段，403/404/409/422及request_id统一。

唯一详细设计与模型边界见[CMDB-DOSSIER.md](CMDB-DOSSIER.md)。

## 真实开发接入基线

参见 [DESIGN-ALIGNMENT-AUDIT.md](DESIGN-ALIGNMENT-AUDIT.md) 与 [API-OPERATION-INDEX.md](API-OPERATION-INDEX.md)。保持现有页面、Provider 请求/响应和稳定资产身份；先接身份/权限与持久化基础，再按已批准模块顺序替换 HTTP Adapter。Mock 通过不等于真实授权、事务 Outbox、SSE 恢复或设备适配验证通过。资源专用分页、响应 envelope 与幂等字段以 OpenAPI 为准，不统一改写已实现接口。

## BOM 真实开发增补

用户已授权在 `dev/` 真实实现 BOM 模块。数据与权限边界、版本/审核/Outbox、真实六步工作台及运行验收统一见 [BOM-REAL-DESIGN.md](BOM-REAL-DESIGN.md)。FR-2.1 和唯一 BOM/资产身份保持；其他模块的 Mock 状态不因本模块接入而视为后端已完成。实际通过范围以 BOM-REAL-ACCEPTANCE.json 为准。

## 2026-10-30 · 真实身份 HTTP 约定

真实 API 使用 cookieSession + 写请求 X-CSRF-Token；旧开发 Bearer 不再作为绕过认证入口。授权从每次数据库主体与资源所属项目解析，不相信前端角色。401 表示会话不可用，403 表示权限/作用域/CSRF拒绝，409 表示身份或冻结快照冲突，422 表示输入/审批策略不满足，429 表示登录限流。真实路径与 Identity* Schema 见 openapi.yaml；业务/管理列表现有200条上限及前端分页边界见 IDENTITY-RBAC.md。

## CMDB 真实接口兼容

保留/cmdb/dossier响应和分页/作用域语义；新增registry、revision/diff、ChangeSet和关系历史专用Schema。资产写入通过ChangeSet操作，不以无审核CRUD绕过Domain Service。手工说明只允许授权注释。固定项目、资产集合、Schema与expected_revision；409表示版本/幂等冲突，422表示Schema或关系校验，403表示授权/职责分离拒绝。

接口对照、模型与实现边界统一见 [CMDB-REAL-DESIGN.md](CMDB-REAL-DESIGN.md)，具体HTTP Schema以openapi.yaml为准。
