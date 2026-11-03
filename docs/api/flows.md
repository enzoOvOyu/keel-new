# 作业平台 · 当前 API 使用契约

当前主环境 `686e9ea`保留`9441a89`发布的原生作业平台。产品定版见[模块页](../modules/jobs.md)；唯一协议为[OpenAPI](../openapi.yaml)，登记见[注册表](real-api-registry.json)。旧“分支待部署”段落已归档。

## 集成和权限

前端[flows.ts](../../dev/src/lib/api/flows.ts) → [Go路由](../../dev/backend/cmd/api/flows.go) → [Job Service](../../dev/backend/internal/flows) → Kestra/受控设备通道。Kestra持有Flow/Revision/Trigger/Execution/TaskRun；Keel存授权、目标快照、审批、通道结果、Agent和审计/Outbox，不复制CMDB。

Session/CSRF和授权项目范围沿用统一身份。read/create/edit/execute分别受flows权限控制，SSH信任写入另需admin:edit；Agent注册/续期/激活使用专用TLS/mTLS约定，不接受浏览器Session代替设备身份。namespace从capabilities读取，不能由前端猜ID。

## 状态、并发和副作用

保存携带expected_revision，冲突409保留草稿。Run冻结项目/目标及修订/输入/通道并携带Idempotency-Key；同授权范围和摘要重放复用Execution，异摘要拒绝。审批摘要、签名、有效期与职责分离由服务端复核，不能复制到下一次执行。

Agent Preferred仅接受任务前可按策略fallback到SSH；接受后失联RECONCILING并对账，不再次下发。maintained IPMI-only；交换机写配置返回受批准Fabric适配约束。Pause/Resume/Kill依真实能力，Retry/Restart/Replay为经预检的新执行并保留谱系。

Files版本不可变，namespace-artifacts≤1MiB，expected_revision避免覆盖；UTF-8文本严格解码，二进制只下载。FlowsScriptSource的namespace是固定版本，inline为独立快照；steps.command冻结实际内容并校验项目/path/revision/SHA。description和timeout按FlowsJobStep约束，显式逐步时限需Agent能力支持，不静默忽略。

Cron/历史补跑复用既有引擎和Job Service。每次派发重新授权/预检/审批；操作批次租约过期保持UNKNOWN，证据不明确先对账。支持的5字段cron语义以现有预览实现为准，不擅自接受Quartz/秒字段。

## 加载和错误

- 当前原生目录`/flows/catalog`提供服务端total/分页；旧`/flows`、targets、agents的limit/offset兼容读取不能冒充无限完整列表。
- Execution目录使用服务端cursor及过滤；TaskRun/Attempt/target/日志时间窗共用URL。源定义读取冻结Revision，缺失不回退最新版本。
- 事件/日志SSE使用持久事件ID与Last-Event-ID/after续传；连接持续校验授权。设备输出与引擎日志分开，UNKNOWN不映射SUCCESS。
- 变量/产物脱敏后授权读取，UTF-8安全预览，HTML不直接嵌入。Metrics按真实task_id过滤；当前页时间筛选不是全量时序查询。
- 常见401/403走身份/授权错误；409版本/状态冲突需刷新或重新预检；422业务/脚本来源/通道约束需修正；引擎不可用或结果未知不模拟成功、不盲目重试。

## 操作索引

下表从本次注册表生成，只存方法/路径/operationId，Schema和参数不复制。每项实现/部署/验证状态仍看机器登记。未登记接口不能推断为不存在。

| 方法 | 路径（省略 `/api/v1`） | operationId |
|---|---|---|
| GET | `/flows` | `listRealFlows` |
| POST | `/flows` | `createRealFlow` |
| GET | `/flows/{flowId}/revisions` | `getFlowRevisions` |
| GET | `/flows/capabilities` | `getFlowsCapabilities` |
| GET | `/flows/targets` | `listFlowsTargets` |
| GET | `/flows/executions` | `listRealExecutions` |
| GET | `/flows/agents` | `listRealAgents` |
| GET | `/flows/ssh-trust` | `listFlowsSSHTrust` |
| POST | `/flows/ssh-trust` | `saveFlowsSSHTrust` |
| POST | `/flows/preflight` | `preflightFlowsTargets` |
| POST | `/flows/quick-run` | `createQuickRun` |
| GET | `/flows/{flowId}` | `getRealFlow` |
| PUT | `/flows/{flowId}` | `updateRealFlow` |
| POST | `/flows/{flowId}/validate` | `validateRealFlow` |
| POST | `/flows/{flowId}/execute` | `executeRealFlow` |
| GET | `/flows/{flowId}/dependencies` | `getFlowDependencies` |
| GET | `/flows/{flowId}/files` | `getFlowFiles` |
| GET | `/flows/executions/{executionId}` | `getRealExecution` |
| GET | `/flows/executions/{executionId}/targets` | `getExecutionTargets` |
| GET | `/flows/executions/{executionId}/logs` | `getExecutionLogs` |
| GET | `/flows/executions/{executionId}/outputs` | `getExecutionOutputs` |
| GET | `/flows/executions/{executionId}/audit` | `getExecutionAudit` |
| POST | `/flows/executions/{executionId}/pause` | `actionExecutionPause` |
| POST | `/flows/executions/{executionId}/resume` | `actionExecutionResume` |
| POST | `/flows/executions/{executionId}/kill` | `actionExecutionKill` |
| POST | `/flows/executions/{executionId}/retry-failed-targets` | `actionExecutionRetryfailedtargets` |
| GET | `/flows/agents/{agentId}` | `getRealAgent` |
| POST | `/flows/agents/enrollment-tokens` | `issueAgentEnrollment` |
| POST | `/flows/agents/{agentId}/approve` | `agentApprove` |
| POST | `/flows/agents/{agentId}/quarantine` | `agentQuarantine` |
| POST | `/flows/agents/{agentId}/revoke` | `agentRevoke` |
| GET | `/flows/agents/{agentId}/tasks` | `agentTasks` |
| GET | `/flows/agents/{agentId}/audit` | `agentAudit` |
| GET | `/flows/executions/{executionId}/events` | `followRealExecution` |
| GET | `/flows/templates` | `getFlowsTemplate` |
| GET | `/flows/{flowId}/namespace-file` | `getNamespaceFile` |
| PUT | `/flows/{flowId}/namespace-file` | `saveNamespaceFile` |
| POST | `/flows/agents/bootstrap` | `bootstrapFlowsAgent` |
| POST | `/flows/executions/{executionId}/restart` | `actionExecutionRestart` |
| GET | `/flows/executions/{executionId}/artifacts` | `downloadExecutionArtifact` |
| POST | `/flows/agents/register` | `registerFlowsAgent` |
| GET | `/flows/{flowId}/schedule` | `getTargetSchedule` |
| PUT | `/flows/{flowId}/schedule` | `saveTargetSchedule` |
| GET | `/flows/{flowId}/namespace-artifacts` | `listNamespaceFileVersions` |
| POST | `/flows/{flowId}/namespace-artifacts` | `uploadNamespaceBinary` |
| POST | `/flows/{flowId}/namespace-artifacts/{revisionId}/distribute` | `distributeNamespaceRevision` |
| GET | `/flows/{flowId}/namespace-artifacts/{revisionId}` | `downloadNamespaceRevision` |
| POST | `/flows/approval-request` | `requestFlowsApproval` |
| GET | `/flows/executions/{executionId}/steps` | `getDeviceSteps` |
| GET | `/flows/executions/{executionId}/definition` | `getExecutionDefinition` |
| GET | `/flows/catalog` | `getFlowsCatalog` |
| GET | `/flows/execution-summary` | `getFlowsExecutionSummary` |
| GET | `/flows/log-search` | `searchFlowsLogs` |
| GET | `/flows/executions/{executionId}/insights` | `getFlowExecutionInsights` |
| GET | `/flows/executions/{executionId}/variables` | `getFlowExecutionVariables` |
| GET | `/flows/task-documentation` | `getFlowsTaskDocumentation` |
| GET | `/flows/agents/bootstrap-jobs` | `listFlowsBootstrapJobs` |
| POST | `/flows/agents/bootstrap-jobs` | `createFlowsBootstrapJob` |
| GET | `/flows/operation-jobs/{jobId}` | `getFlowsOperationJob` |
| POST | `/flows/operation-jobs/{jobId}/actions` | `actFlowsOperationJob` |
| POST | `/flows/agents/certificates/renew` | `renewFlowsAgentCertificate` |
| POST | `/flows/agents/certificates/activate` | `activateFlowsAgentCertificate` |
| POST | `/flows/{flowId}/backfill/preview` | `previewFlowsBackfill` |
| POST | `/flows/{flowId}/backfill` | `createFlowsBackfill` |
| GET | `/flows/{flowId}/backfill` | `listFlowsBackfills` |
| GET | `/flows/operation-jobs/{jobId}/items/{ordinal}/request` | `getFlowsOperationItemRequest` |
| GET | `/flows/executions/{executionId}/replay-request` | `getFlowsReplayRequest` |
| POST | `/flows/executions/{executionId}/replay` | `createFlowsReplay` |
| GET | `/flows/{flowId}/schedule/status` | `getFlowsScheduleStatus` |
| GET | `/projects/{project_id}/approvals` | `listIdentityApprovals` |

## 已验证及未完成

[发布证据](../flows-redesign/deployment-20261102.json)、[交互整改](../flows-redesign/parity-correction/result.json)、[专项运行](../flows-redesign/completion-runtime-validation.json)区分只读上线检查、synthetic UI、真实本地Agent与SSH协议仿真。没有生产设备混合批次/完整NOS/规模验收。

ZTP事件消费者待Fabric联调，Namespace动态跟随与任意Worker脚本执行未开放，节点跳过/任意重放不能绕过设备预检。原API长篇历史和阶段限制见[归档](../archive/2026-11-03/previous/docs/api/flows.md)，不作为当前上线状态。
