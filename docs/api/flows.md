# 作业平台（Flows）真实 API 使用契约

2026-10-31 · `feat/flows-redesign` · 范围仅 Flows/Executions/Agents。字段唯一权威为 [OpenAPI](../openapi.yaml)，前端类型由它生成，不能在本文件复制第二套 DTO。

## 入口与所有权

- HTTP 前缀 `/api/v1`；页面只调用 `dev/src/lib/api/flows.ts`。Go 路由 `cmd/api/flows.go`，领域服务 `internal/flows/`，迁移021…024。完整实施与限制见 [实施记录](../flows-redesign/implementation.md)。
- Kestra **2.0.7** 是 Flow/YAML/Revision/Trigger/Execution/TaskRun 的权威；PostgreSQL 保存项目授权、绑定、冻结目标、通道结果、Agent、SSH受控引用、审计与 Outbox，不复制 CMDB 资产。
- CMDB稳定ID与`cmdb_asset_scope`是目标/项目归属来源。执行快照不随CMDB后续改名变化。Outbox向既有CMDB生命周期追加引用，不改变六阶段结论。
- 当前沿用5180/API8081，后端为BOM v5集成版本，准确源码和回退以HANDOFF顶部为准。本轮前端整改由5180的开发服务加载，以下新增后端能力尚未发布；禁止用旧分支二进制覆盖BOM修复。

## 授权、请求与一致性

复用HttpOnly Session、现有CSRF和`permissions[]`。读为`flows:read`、创建`flows:create`、编辑/Agent管理`flows:edit`、预检/执行/生命周期`flows:execute`；SSH信任写入另需`admin:edit`。每次请求由后端验证用户授权项目，不依赖前端角色名。`project_id`过滤可清除，读取仍限定授权范围；写操作必须明确单一项目。注册接口只在TLS入口交换单次令牌，不使用普通Session。

Flow namespace由`capabilities.namespace`提供（项目ID摘要），禁止前端自行去掉符号拼接。更新传`expected_revision`；旧Revision返回409，不覆盖。Run冻结project/asset IDs/revisions/transport；确认后资产版本变化或maintained会阻断。Run携带`Idempotency-Key`，同用户/项目/Flow/来源/请求摘要重放复用Execution，异摘要409；生命周期retry/restart创建新执行并保存parent。暂停/继续/终止交引擎确认。

Agent Preferred只在**未接受任务前**决定SSH fallback；接受后失联为RECONCILING，保留任务日志等待对账，禁止重复下发。Agent按task ID+摘要去重。maintained严格IPMI-only。交换机只读Command允许`nv show`/`show`；写配置必须走批准的Fabric流程。Agent本地非root；高危审批与提权并未由一般执行权限替代。

## 操作索引

| 方法与路径（省略/api/v1） | operationId | 请求Schema | 成功响应Schema/媒体 |
|---|---|---|---|
| GET `/flows` | listRealFlows | — | listRealFlowsEnvelope |
| POST `/flows` | createRealFlow | FlowsSave | createRealFlowEnvelope |
| GET `/flows/{flowId}/revisions` | getFlowRevisions | — | getFlowRevisionsEnvelope |
| GET `/flows/capabilities` | getFlowsCapabilities | — | getFlowsCapabilitiesEnvelope |
| GET `/flows/targets` | listFlowsTargets | — | listFlowsTargetsEnvelope |
| GET `/flows/executions` | listRealExecutions | — | listRealExecutionsEnvelope |
| GET `/flows/agents` | listRealAgents | — | listRealAgentsEnvelope |
| GET `/flows/ssh-trust` | listFlowsSSHTrust | — | listFlowsSSHTrustEnvelope |
| POST `/flows/ssh-trust` | saveFlowsSSHTrust | FlowsSSHTrustInput | saveFlowsSSHTrustEnvelope |
| POST `/flows/preflight` | preflightFlowsTargets | FlowsRun | preflightFlowsTargetsEnvelope |
| POST `/flows/quick-run` | createQuickRun | FlowsRun | createQuickRunEnvelope |
| GET `/flows/{flowId}` | getRealFlow | — | getRealFlowEnvelope |
| PUT `/flows/{flowId}` | updateRealFlow | FlowsSave | updateRealFlowEnvelope |
| POST `/flows/{flowId}/validate` | validateRealFlow | FlowsValidationRequest | validateRealFlowEnvelope |
| POST `/flows/{flowId}/execute` | executeRealFlow | FlowsRun | executeRealFlowEnvelope |
| GET `/flows/{flowId}/dependencies` | getFlowDependencies | — | getFlowDependenciesEnvelope |
| GET `/flows/{flowId}/files` | getFlowFiles | — | getFlowFilesEnvelope |
| GET `/flows/executions/{executionId}` | getRealExecution | — | getRealExecutionEnvelope |
| GET `/flows/executions/{executionId}/targets` | getExecutionTargets | — | getExecutionTargetsEnvelope |
| GET `/flows/executions/{executionId}/logs` | getExecutionLogs | — | getExecutionLogsEnvelope |
| GET `/flows/executions/{executionId}/outputs` | getExecutionOutputs | — | getExecutionOutputsEnvelope |
| GET `/flows/executions/{executionId}/audit` | getExecutionAudit | — | getExecutionAuditEnvelope |
| POST `/flows/executions/{executionId}/pause` | actionExecutionPause | — | actionExecutionPauseEnvelope |
| POST `/flows/executions/{executionId}/resume` | actionExecutionResume | — | actionExecutionResumeEnvelope |
| POST `/flows/executions/{executionId}/kill` | actionExecutionKill | — | actionExecutionKillEnvelope |
| POST `/flows/executions/{executionId}/retry-failed-targets` | actionExecutionRetryfailedtargets | — | actionExecutionRetryfailedtargetsEnvelope |
| GET `/flows/agents/{agentId}` | getRealAgent | — | getRealAgentEnvelope |
| POST `/flows/agents/enrollment-tokens` | issueAgentEnrollment | FlowsTokenRequest | issueAgentEnrollmentEnvelope |
| POST `/flows/agents/{agentId}/approve` | agentApprove | FlowsReason | agentApproveEnvelope |
| POST `/flows/agents/{agentId}/quarantine` | agentQuarantine | FlowsReason | agentQuarantineEnvelope |
| POST `/flows/agents/{agentId}/revoke` | agentRevoke | FlowsReason | agentRevokeEnvelope |
| GET `/flows/agents/{agentId}/tasks` | agentTasks | — | agentTasksEnvelope |
| GET `/flows/agents/{agentId}/audit` | agentAudit | — | agentAuditEnvelope |
| GET `/flows/executions/{executionId}/events` | followRealExecution | — | FlowsEventPageEnvelope |
| GET `/flows/templates` | getFlowsTemplate | — | getFlowsTemplateEnvelope |
| GET `/flows/{flowId}/namespace-file` | getNamespaceFile | — | getNamespaceFileEnvelope |
| PUT `/flows/{flowId}/namespace-file` | saveNamespaceFile | FlowsNamespaceFile | saveNamespaceFileEnvelope |
| POST `/flows/agents/bootstrap` | bootstrapFlowsAgent | FlowsBootstrap | bootstrapFlowsAgentEnvelope |
| POST `/flows/executions/{executionId}/restart` | actionExecutionRestart | — | actionExecutionRestartEnvelope |
| GET `/flows/executions/{executionId}/artifacts` | downloadExecutionArtifact | — | application/octet-stream |
| POST `/flows/agents/register` | registerFlowsAgent | FlowsEnrollmentInput | registerFlowsAgentEnvelope |
| GET `/flows/{flowId}/schedule` | getTargetSchedule | — | getTargetScheduleEnvelope |
| PUT `/flows/{flowId}/schedule` | saveTargetSchedule | FlowsSchedule | saveTargetScheduleEnvelope |
| GET `/flows/{flowId}/namespace-artifacts` | listNamespaceFileVersions | — | listNamespaceFileVersionsEnvelope |
| POST `/flows/{flowId}/namespace-artifacts` | uploadNamespaceBinary | FlowsFileUpload | uploadNamespaceBinaryEnvelope |
| POST `/flows/{flowId}/namespace-artifacts/{revisionId}/distribute` | distributeNamespaceRevision | FlowsRun | distributeNamespaceRevisionEnvelope |
| GET `/flows/{flowId}/namespace-artifacts/{revisionId}` | downloadNamespaceRevision | — | application/octet-stream |
| POST `/flows/approval-request` | requestFlowsApproval | FlowsApprovalRequest | requestFlowsApprovalEnvelope |
| GET `/flows/executions/{executionId}/steps` | getDeviceSteps | — | getDeviceStepsEnvelope |

## 加载、分页与状态

- Execution状态/日志走SSE；目标结果活动期5s补采，详情及文件按需加载。Events使用Keel PostgreSQL持久事件日志，递增数值ID支持Last-Event-ID/after补齐；游标必须属于当前授权Execution。原生状态/日志由Capture消费并去重，连接中持续检查Session权限。history=true提供同序JSON查询，每页200项。
- Execution目录为稳定`(created_at,id)`游标，默认50/最大100；cursor绑定用户、获授权项目与筛选。`state_filter_source`说明筛选来自Kestra投影，行值实时刷新。历史页按需加载，最新页5s刷新。
- 其他目录当前有界：Flow最多200、Target/Agent最多500、单次Run最多200目标；前端表格分页。不是无限量服务端目录分页承诺。
- Kestra状态保留原值。Target单独记录PENDING/RUNNING/SUCCESS/FAILED/RECONCILING与actual_transport/exit_code/stdout/stderr，不能将选定通道冒充实际完成。
- 原生调度执行5s发现/投影最近100项；不是全历史导入。未配置引擎/证书/信任时明确不可用，不返回模拟成功。

## 关键错误与安全

统一`error.code`与request_id，常见401会话、403权限/范围、409版本/幂等/执行冲突、422参数/通道/能力/维护态、503引擎未配置、502引擎故障。细分错误以Go服务和OpenAPI说明为准。上游错误正文不透传秘密。

SSH仅受控secret_ref/known_hosts_ref文件，不接受浏览器私钥；Strict Host Key。Agent令牌15min、仅哈希存储、TLS CSR、mTLS证书序列与注册表匹配；未知机器身份待审核。Namespace文本文件最多128KiB，不能上传私钥。Agent文件任务受限本地目录/摘要；下载产物只允许当前执行outputs中真实URI。

## 当前能力边界

可执行原生安全DAG，以及CMDB批次的**独立作用域调度**：`GET/PUT /flows/{flowId}/schedule`管理Kestra原生Cron标记Flow，Go只消费引擎触发记录；每次重检当前owner授权/Flow revision/目标通道，冻结新批次并以引擎execution ID幂等。原始带票据Flow不能直接添加不受控触发器；应使用该调度接口。保存后修改Flow需重存计划。高危操作需要本次具体目标版本的独立审批，不可作为长期无人审批授权。

`steps[]`支持最多32个Command/Bash/Python步骤、显式依赖和`inputs`环境变量到前置stdout的绑定；循环/缺失依赖拒绝。每目标按确定性拓扑顺序执行、fail-fast阻断后续，**不承诺步骤内部并行**。Agent需`job.pipeline`能力（0.2.0）；Agent/SSH通道由预检统一确定。步骤结果单独持久化，Task Runs可查看。单步骤输出最多32KiB，绑定消费该有界输出，超限明确标注。

Namespace支持文本编辑及**≤1MiB二进制资源版本**；上传携带expected_revision，旧版本不可变，可下载/分发指定SHA版本到Agent受控uploads目录。资源版本位于独立文件表与引擎存储，不进入CMDB资产。不是OS镜像仓库或无限大小介质上传。

审批复用现有`approval_record/approval_signature`，不建平行审批。`approval-request`冻结作业摘要、目标版本和领域动作，独立DL签署；Start/Dispatch验证审批状态、当前签署权限、有效期、目标/脚本摘要及领域执行权限。一个批准记录只可绑定一个新执行，同幂等请求复用原执行；失败重试需要新的审批。网络写配置仍只能走Fabric批准闭环，Critical NH未决规则不变。

终止同时请求Kestra停止调度和本地已接受任务取消；在线Agent收到精确Task ID的取消，SSH中断无法证明远端副作用已停止时保持RECONCILING，不伪装成功。

Bootstrap需运维预置root可信helper、包摘要及HTTPS分发，未对生产主机运行。生产8设备混合批次、NOS/发行版兼容性和规模验收仍未执行。最近100项原生触发发现上限与其他目录限制不变。

## 已运行证据

[浏览器记录](../flows-redesign/ui-validation.json)与[实施记录](../flows-redesign/implementation.md)分别区分页面、SSH协议仿真端点、实际本地Agent、生产未验证范围。机器注册表登记实现状态，不把模块检查冒充每一端点HTTP验收。


开发补齐证据：[新增运行检查](../flows-redesign/completion-runtime-validation.json)、[新增页面检查](../flows-redesign/completion-ui-validation.json)。这与原生产环境部署、真实设备验收是不同状态。


## 2026-10-31 UI 整改关联契约增量（后端待部署）

- 界面统一名「作业平台」，技术路由和权限前缀仍是 `flows`。
- `GET /flows`、`/flows/targets`、`/flows/agents` 增加 `limit=1…200` 与 `offset>=0`，仍返回原数组 envelope。前端 Provider 汇集分页；页间重复身份会显式提示重载，不能无声截断。内部目标身份解析不再套用 HTTP 列表上限。
- `GET /flows/executions` 增加 RFC3339 `since/until`、`mode`；原 source/transport/started_by 及游标继续生效。非法时间或反向时间范围返回 `EXECUTION_TIME_FILTER_INVALID`。
- `GET /flows/executions/{executionId}/definition`（`getExecutionDefinition` / `FlowsExecutionDefinition`）：经项目授权，从已冻结的 namespace/engine_flow_id/revision 读取源定义，涵盖 Quick Run。未冻结返回409；历史源定义不可用返回404。不能回退当前最新定义。旧后端下已有 Flow 使用原 revisions API 的准确版本，Quick Run 明确提示缺失能力。
- `capabilities` 新增 `execution_definition`、`agent_versions`、`bootstrap_ready`、`agent_public_gateway`、`enrollment_url`。只返回能力和公开地址，不返回凭据。
- 迁移025增加 `flow_sync_cursor`：固定时间窗内按页同步 Kestra 执行发现，避免始终只处理最新100条；另轮询旧活动执行状态。当前部署形态为单同步循环，不声明多实例协调/生产吞吐已验收。
- UI 调度表单保留保存的脚本类型、超时、目标并发和步骤；从 Flow 更新步骤是显式动作，不被刷新覆盖。

新增接口尚未进行8081联调或迁移执行。已运行检查与剩余缺口见实施记录，不能以生成类型通过冒充接口运行验收。


### 2026-11-02 正式脚本工作台接入（分支已实现、未部署）

- 新增/导入文本先为内存草稿，保存调用既有 `POST /flows/{flowId}/namespace-artifacts`（FlowsFileUpload），使用UTF-8的base64和读取版本的expected_revision；409保持草稿，不自动覆盖，用户可确认重新加载。项目ID、CSRF与权限沿用现有Adapter/后端。
- 内容读取优先使用 `GET /flows/{flowId}/namespace-artifacts/{revisionId}` 的原始字节响应；统一Adapter严格解码UTF-8，二进制只提供下载。未纳入Keel版本管理的历史引擎文本保留namespace-file只读兼容入口，其既有128KiB限制未改变。
- 保存后刷新文件版本、上下文和分发历史。内存草稿按identity/permission_revision/project/flow隔离；不持久化浏览器秘密。权限判断由后端复核。
- 脚本内联只修改待保存Flow，实际保存/发布仍走Flow Revision校验。引擎脚本与Keel设备通道不同；设备脚本继续使用受控steps。Namespace引用及Execution冻结文件版本的完整集成不在本批完成范围。
- 未新增HTTP操作或修改Schema；OpenAPI/生成类型和真实接口注册表保持原契约。此处的浏览器验证使用synthetic拦截响应，不表示Go/数据库/设备闭环已验证。历史文件版本列表仍有既有500条上限，需要后续分页契约，不能当成无限完整文件目录。


### 2026-11-02 原生工作台查询契约（代码实现，未发布）

OpenAPI为唯一字段定义。本节对应6个新增已登记查询：

| 方法/路径 | 响应资源 | 作用域及语义 |
|---|---|---|
| GET `/flows/catalog` | FlowsCatalogPage | 授权binding交集；q/namespace/target_type/state/sort/direction/limit/offset。total为服务端匹配总数，recent_execution按时间+ID选取最后执行；enabled/draft来自Kestra，不另建状态权威表。source_search=true时q至少2字符，使用引擎源码索引，精确namespace与已授权binding再次相交，不返回其他namespace或未登记Flow。 |
| GET `/flows/execution-summary` | FlowsExecutionSummary | 按flow_id/source/since/until聚合现有投影；引擎状态和设备结果分别计数，标注state_source。 |
| GET `/flows/log-search` | FlowsLogSearchPage | 已脱敏flow_event日志，按Flow/Execution/Task/等级/文本/时间筛选，游标绑定用户+授权项目+查询；时间范围是采集事件时间。coverage明确不含尚未捕获的历史日志。 |
| GET `/flows/executions/{executionId}/insights` | FlowsExecutionInsights | 稳定时间/ID相邻执行，真实parent_id谱系；按需查询引擎指标（metrics=false跳过），不将单位缺失解释为默认单位。引擎指标不可用返回metrics_error。 |
| GET `/flows/executions/{executionId}/variables` | FlowsExecutionVariables | 输入按执行原Revision的SECRET定义脱敏，定义不可得时遮蔽所有输入；结构化敏感键和文本再次脱敏。产物仍经现有授权下载接口。 |
| GET `/flows/task-documentation` | FlowsTaskDocumentation | 既有策略允许的Kestra类型从固定插件API读取；keel.device.bash/python/command返回JobStep契约说明，仅为展示类型，不是引擎插件，也不扩展执行权限。 |

所有操作逐请求使用flows:read与后端项目范围，400拒绝非法分页/时间/游标，404隐藏未授权执行，未连接引擎不伪造结果。所有响应包含request_id。相邻执行与谱系逐项授权；来源筛选不代表业务消费者已经接通。

现有`ValidatePolicy`禁止直接执行Worker Shell/Python插件，因此正式脚本工作台将已保存内容加入受控设备steps，进入Flow版本快照；不放宽策略来适配原型。原型Namespace/Worker脚本引用不计为已获准的正式执行能力。未保存草稿不会载入设备任务。此规则覆盖上节“普通引擎Flow新增Script”的初次实现描述。

加载：目录按URL分页/筛选按需读取，Execution及日志仍沿用SSE；详情Tab、插件Docs、变量、指标按需读取；聚合缓存60s。Catalog当前对授权namespace元数据服务端筛选后分页，尚未建立独立索引投影；大目录成本需在实际规模验收，不能宣称无限规模或没有N+1开销。

前端验证见`docs/flows-redesign/native-validation/result.json`；8组正式路由使用拦截synthetic API，无后端写入。后端`TestWorkspaceQueryIntegration`使用隔离keel_flows_dev数据库与本地synthetic Kestra，验证实际SQL/授权/脱敏，未连接设备。普通Go领域测试通过；cmd/api全包存在无关BOM测试失败，未删除或放宽用例。

### 2026-11-02 持久业务批次、历史补跑及证书续期（分支实现，未部署）

新增机器契约均在OpenAPI专用Flows* Schema及真实API台账；不复制字段定义。

| API | 权限 / 行为 |
|---|---|
| GET/POST `/flows/agents/bootstrap-jobs` | read/edit；按项目及安装版本冻结最多200台现有服务器，Idempotency-Key绑定owner/project/请求。服务端串行派发已有SSH Bootstrap，浏览器关闭不丢进度。 |
| GET `/flows/operation-jobs/{jobId}` | read且项目授权；返回持久逐项结果。最近批次列表最多50项，不表示无限历史。 |
| POST `/flows/operation-jobs/{jobId}/actions` | bootstrap需edit，backfill需execute；expected_revision并发检查，pause/resume/cancel/retry-blocked/reconcile/authorize-item。cancel只停止未派发项，UNKNOWN不能盲重试。 |
| POST `/flows/{flowId}/backfill/preview`、POST/GET `/flows/{flowId}/backfill` | 写需execute，读需read；历史区间/时区/已保存Schedule+Flow版本，返回实际Cron时刻、预检结果和preview_hash。每批最多200项，超限必须缩小窗口，不静默截断。 |
| GET `/flows/operation-jobs/{jobId}/items/{ordinal}/request` | execute；返回待审批的冻结设备/脚本请求。每个历史时刻有独立approval_context；申请与执行仍走现有ApprovalRecord/签署，不复用旧调度批准。 |
| GET `/flows/executions/{executionId}/replay-request`、POST `/flows/executions/{executionId}/replay` | execute；只针对终态原执行，全部或失败目标新运行。服务端核对原范围、重新预检与审批，生成parent谱系和独立幂等键，不直接调用引擎restart绕过设备控制。 |
| POST `/flows/agents/certificates/renew`、POST `/flows/agents/certificates/activate` | 专用TLS监听上的Agent mTLS身份，不接受浏览器Session替代。旧证书申请CSR，新公钥证书完成激活证明后，事务切换active serial。 |

批次完成表示Bootstrap请求已完成或新Execution已创建，不表示Agent身份批准或设备作业PASS。过期worker租约转UNKNOWN；bootstrap仅根据同资产/版本、任务后注册心跳对账；backfill仅关联已保存的相同approval_context执行，不盲目补发。派发前重载owner当前权限及维护态；maintained保持IPMI-only。项目存在批次历史时沿用PROJECT_FLOWS_REQUIRE_RETIREMENT，不能级联抹除执行记录。

历史预览使用robfig/cron标准5字段UNIX表达式和已保存时区；不是第二套未来调度器，实际调度仍由Kestra负责。Quartz/秒字段及Sunday=7等未支持变体明确422，不擅自转译；需另行兼容后才开放。每个历史项独立scope，不会把一次批准当长期授权。客户端一般execute接口拒绝自带approval_context，只有经服务端验证的补跑/重跑入口可提交。

Agent新参数`--renew-before`默认0（未启用），启用时要求HTTPS；`--renew-check-interval`默认1h是客户端检查间隔，不是监控保留/SLO。Agent空闲且服务端无活动任务才续期；完整新凭据暂存，激活成功后原子切换credentials-current，失败保留当前凭据和相同待续期公钥。旧serial在服务端不再获准派发。现有Agent须升级并配置参数；界面根据certificate.renew能力显示启用情况，不宣称当前已部署设备自动续期。

部署前须依序应用迁移044并部署同版本API/Agent；主API启动OperationWorker。该迁移仅在隔离keel_flows_dev执行，不在本轮切换5180/8081或其他共享服务。数据库保存业务操作进度和公开证书，不保存Agent私钥。证书和Bootstrap仍需要原有受控CA、可信helper、签名/摘要包及公开TLS网关配置；本轮未连接生产服务器。

实际证据：operation_jobs_test验证幂等、暂停/恢复、版本冲突、UNKNOWN对账；certificate_rotation_test及cmd/agent/certificates_test验证新旧密钥证明、续期失败保留、原子切换、维护态拒绝；backfill_integration_test验证旧审批被清除、两个历史项独立摘要、无批准时引擎零写入。Replay请求的范围和秘密脱敏由workspace_integration_test覆盖；完整审批签署后生产执行/设备兼容性及规模仍由后续环境联调验收。

`GET /flows/{flowId}/schedule/status`（flows:read）补齐实际TriggerRuntime和最近50条flow_schedule_fire消费历史；从已授权binding对应的已保存辅助Schedule读取Kestra `/triggers/{namespace}/{flowId}`，只取cron。nextEvaluationDate是引擎下一次检查时间，不冒充承诺执行时间；runtime_error与null表示未知，不按本地Cron猜算补值。历史execution_id指向已有Keel Execution；原引擎标记执行不会被当成设备作业成功。实现依据固定v2.0.7 ApiTriggerState，隔离集成检查实际时间/disabled状态和跨项目拒绝。

### 2026-11-02 列表处置与统计补齐

- Execution目录的来源、实际通道、创建时间、列选择和清除筛选已接回原生列表。批量暂停/继续/终止复用已有逐执行动作API；确认面板冻结项目/执行集合，逐项重读状态并在后端重新授权，状态改变或不适用项明确跳过。独立幂等键防止同一确认重复提交；“受理”不冒充设备已停止。失败目标重试仍走单执行的新预检/审批向导，不批量复制批准。
- `GET /flows/execution-summary`及`GET /flows/executions`新增revision精确筛选（1…1000000）；游标绑定包含版本的完整查询。图表下钻保留时间窗、Flow和版本，不跳回不相干的全历史。
- summary增加duration与daily：daily按执行创建日期UTC聚合；duration仅使用各执行最新已采集终态的startDate/endDate，明确sample_count/missing_count。零样本平均及百分位为null；只有一个样本时不显示P50/P95。多样本使用经验nearest-rank，不推导SLO或统计可靠性。计数与样本在同一只读Repeatable Read快照读取；未采集/非法/尚未结束时间不补零。趋势显示最近90个有记录UTC日，缺日不插值。
- 定向证据见native-validation/batch-result.json（synthetic拦截API）；隔离Go查询验证版本排除、无采集证据不补时长、真实样本统计及项目隔离。不是生产设备或全量性能认证。

### 2026-11-02 原型呈现所需契约补齐（未部署）

Catalog新增state_counts（授权搜索/Namespace/目标筛选后、选择状态前的服务端计数），recent_executions（同项目最近最多10次，最新在前，保留recent_execution兼容），triggers（现有作用域Schedule与Flow定义，来源独立），updated_at/updated_by（flow.save审计，不用创建时间冒充修改时间；缺失为null）。不新增资产或迁移。执行详情新增可空flow_name，取同项目绑定名称，不改变冻结engine_flow_id。独立Go查询集成已核对历史顺序、状态页签计数、审计来源、停用计划与项目隔离；前端使用同一批准原型synthetic数据对照，非生产HTTP验收。

保存视图仅为当前用户/权限版本/项目隔离的内存Query偏好，保存搜索和筛选，不保存执行目标或秘密。版本差异使用实际Revision source与当前草稿的Monaco Diff；源定义缺失时不生成假Diff。


### 2026-11-02 脚本工作台与 Quick Run 衔接

- 新建 Flow 的 Files 允许新增、导入及在线编辑，文件仅为用户/权限版本/项目隔离的 Query 内存草稿，不对 `new` 伪 ID 发文件 API。创建真实 Flow binding 后，草稿及当前文件选择转移至实际 Flow ID；再调用已有 namespace-artifacts 接口保存带 expected_revision 的文件版本。刷新未保存内容会丢失并提示，不声称已持久化。
- 文件保存冲突保留编辑内容；已保存的 Shell/Python 文件经确认后复制为受控设备步骤的内容快照，或替换选中的设备步骤。单 command 定义首次扩展为多步骤时保留原命令。Namespace 动态执行引用继续禁用，不放宽 Worker Shell/Python 策略；当前 JobStep 没有独立文件来源绑定字段，不宣称支持文件版本自动跟随。
- Visual、YAML、Topology、Docs 共用当前 Flow 定义；`keel.device.bash/python/command` 仅为 Docs/节点展示类型，提交给引擎的仍是原受控适配定义。Docs 按需使用既有 task-documentation 端点，不创建假插件。
- Quick Run 的草稿字段先按 JobStep 结构限制给予即时反馈，后端仍独立校验授权、执行能力、维护态、目标版本与审批。目标/脚本/通道变更清除预检与审批；异步预检仅回填匹配当前请求的结果。创建执行固定所选目标 ID 与预检返回的目标版本。审批记录不是前端可自证的授权，启动时仍由后端核验。


### 2026-11-02 原型复核修复：版本、上下文与目录

本节仅对应feat/jobs-kestra-ui开发代码，**未部署**；不改变已有部署证据。

- `FlowsRun.name`：可选、最多160字符、禁止换行；作为既有不可变request JSON的一部分持久化，不新增表/迁移。`FlowsExecution.name`读取该请求，历史缺失返回null；前端按执行名→业务Flow名→引擎ID显示。名称参与既有幂等/审批摘要，改名后重新确认，不能继续沿用旧批准。执行列表q同时支持执行名称与同项目业务Flow名称。
- `GET /flows/executions?attention=true`：按捕获的FAILED/WARNING/PAUSED/QUEUED/KILLED引擎状态，或FAILED/BLOCKED/RECONCILING目标结果筛选。非法布尔值400；状态与设备结果不合并改写。游标仍绑定用户、授权项目及全部查询条件。“全部/运行中/需要关注”是同一目录的视图。紧凑汇总使用既有execution-summary；不支持同口径的关键词/通道/模式/发起人组合时不显示汇总，不以当前加载页冒充完整总数。
- Execution源版本使用既有`GET /flows/executions/{executionId}/definition`，在当前页只读显示冻结定义及版本。缺失/错误不回落最新Flow。dirty Flow禁止直接运行，元数据修改也计入dirty。
- 定义节点与TaskRun实例分开：Topology点击后仅在明确唯一时直接选择；多迭代要求选择实例。设备关联仅接受稳定asset_id、TaskRun.value中的明确ID/asset_id，不按Hostname猜测。缺少关联时只展示已有设备stdout/stderr及缺口说明，不混入其他TaskRun日志。
- task_run、attempt、target、task_id、日志事件时间窗及指标/目标分页写入同一URL。连续Vue事件使用当前地址更新，避免后一次切Tab覆盖先一次任务筛选。目录return_to仅接受本地/flows路径，筛选/页码/列配置写URL；滚动仅存用户/权限版本/项目隔离的Query内存。
- Files显式选择新增或替换兼容设备脚本，默认跟随当前选中的脚本任务；不能转换Log为设备脚本。引用仍是已保存版本的内容快照。辅助面板优先复用右区；不删除原生分栏/最大化能力。当前JobStep仍无独立文件来源绑定、每步description/timeout字段，不把确认弹窗里的路径/版本冒充已持久绑定。
- Quick Run复用既有`GET /projects/{project_id}/approvals`（IdentityApproval，approval:read），显示同项目、申请人、动作、资源、目标范围匹配且未过期的approved候选。提交审批申请只获得待审记录，不自动当作批准；启动仍由Job Service复核摘要、版本、职责分离及有效签名。列表无权或记录未列出时保留显式ID入口，由后端核验。
- 变量输出按当前对象逐层浏览，不一次递归渲染所有输出；通过已授权artifact接口预览<=1MiB UTF-8文本，HTML等不直接嵌入。产出任务只按实际Task ID定位。Execution Metrics支持服务端task_id筛选；时间控件明确只筛选**当前页已有时间戳样本**，并非全量时序查询。状态历史点击按实际事件时间窗打开日志，不生成虚假历史。

证据：`docs/flows-redesign/parity-correction/result.json`（7组）、`enhancements.json`（3组）与截图；所有UI请求拦截为synthetic，没有后端/设备写入。隔离PostgreSQL的TestWorkspaceQueryIntegration覆盖attention/名称持久读取与搜索，TestExecutionTitleValidation覆盖名称约束；最终构建见HANDOFF。ZTP匹配按用户安排在Fabric联调，不计本轮缺口。
