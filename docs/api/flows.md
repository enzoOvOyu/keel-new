# Flows 真实 API 使用契约

2026-10-31 · `feat/flows-redesign` · 范围仅 Flows/Executions/Agents。字段唯一权威为 [OpenAPI](../openapi.yaml)，前端类型由它生成，不能在本文件复制第二套 DTO。

## 入口与所有权

- HTTP 前缀 `/api/v1`；页面只调用 `dev/src/lib/api/flows.ts`。Go 路由 `cmd/api/flows.go`，领域服务 `internal/flows/`，迁移021…024。完整实施与限制见 [实施记录](../flows-redesign/implementation.md)。
- Kestra **2.0.7** 是 Flow/YAML/Revision/Trigger/Execution/TaskRun 的权威；PostgreSQL 保存项目授权、绑定、冻结目标、通道结果、Agent、SSH受控引用、审计与 Outbox，不复制 CMDB 资产。
- CMDB稳定ID与`cmdb_asset_scope`是目标/项目归属来源。执行快照不随CMDB后续改名变化。Outbox向既有CMDB生命周期追加引用，不改变六阶段结论。
- 当前仅部署于隔离开发地址5190/API8096/Kestra8095；原5180/8081不含本次实现，不能拿旧部署版本推定此模块可用。

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
