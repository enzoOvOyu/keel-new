# 当前 API 操作索引

从 `openapi.yaml` 提取；base URL `/api/v1`。路径是 HTTP 路由，不是前端页面路由。参数、权限、安全、幂等、状态码和字段约束以原操作与专用 Schema 为准，本表不创建另一套契约。GenericObject 定义仍作历史兼容保留，但无引用。

| 方法 | 路径 | operationId | 请求 Schema | 成功响应 Schema | 生命周期 |
|---|---|---|---|---|---|
| POST | `/projects/{project_id}/bom-imports` | `createBOMImport` | inline / 无 body | BOMImportEnvelope | 当前契约 |
| GET | `/bom-imports/{import_id}` | `getBOMImport` | inline / 无 body | BOMImportEnvelope | 当前契约 |
| GET | `/bom-imports/{import_id}/sheets` | `listImportSheets` | inline / 无 body | SheetListEnvelope | 当前契约 |
| POST | `/bom-imports/{import_id}/map` | `mapBOMImport` | inline / 无 body | JobEnvelope | 当前契约 |
| GET | `/bom-imports/{import_id}/mappings` | `listBOMMappings` | inline / 无 body | MappingListEnvelope | 当前契约 |
| PATCH | `/bom-imports/{import_id}/mappings/{mapping_id}` | `updateBOMMapping` | inline / 无 body | MappingEnvelope | 当前契约 |
| POST | `/bom-imports/{import_id}/validate` | `validateBOMImport` | inline / 无 body | JobEnvelope | 当前契约 |
| GET | `/bom-imports/{import_id}/issues` | `listBOMImportIssues` | inline / 无 body | IssueListEnvelope | 当前契约 |
| GET | `/bom-imports/{import_id}/preview` | `previewBOMImport` | inline / 无 body | PreviewEnvelope | 当前契约 |
| POST | `/bom-imports/{import_id}/build-package` | `buildBOMPackage` | inline / 无 body | JobEnvelope | 当前契约 |
| POST | `/bom-imports/{import_id}/commit` | `commitBOMImport` | inline / 无 body | BOMBatchEnvelope | 当前契约 |
| GET | `/bom-batches/{batch_id}/items` | `listBOMItems` | inline / 无 body | BOMItemListEnvelope | 当前契约 |
| GET | `/jobs/{job_id}` | `getJob` | inline / 无 body | JobEnvelope | 当前契约 |
| GET | `/health/live` | `healthLive` | inline / 无 body | inline / 无 body | 当前契约 |
| GET | `/health/ready` | `healthReady` | inline / 无 body | inline / 无 body | 当前契约 |
| GET | `/projects` | `listProjects` | inline / 无 body | ListEnvelope | 当前契约 |
| POST | `/projects` | `createProject` | ProjectWriteRequest | ResourceEnvelope | 当前契约 |
| GET | `/projects/{project_id}` | `getProject` | inline / 无 body | ResourceEnvelope | 当前契约 |
| PATCH | `/projects/{project_id}` | `updateProject` | ProjectPatchRequest | ResourceEnvelope | 当前契约 |
| GET | `/cmdb/entities` | `listCMDBEntitys` | inline / 无 body | ListEnvelope | 当前契约 |
| POST | `/cmdb/entities` | `createCMDBEntity` | CMDBProjectionRequest | ResourceEnvelope | 当前契约 |
| GET | `/cmdb/entities/{entity_id}` | `getCMDBEntity` | inline / 无 body | ResourceEnvelope | 当前契约 |
| GET | `/projects/{project_id}/devices` | `listDevices` | inline / 无 body | ListEnvelope | 当前契约 |
| POST | `/projects/{project_id}/devices` | `createDevice` | DeviceWriteRequest | ResourceEnvelope | 当前契约 |
| GET | `/projects/{project_id}/devices/{device_id}` | `getDevice` | inline / 无 body | ResourceEnvelope | 当前契约 |
| PATCH | `/projects/{project_id}/devices/{device_id}` | `updateDevice` | DevicePatchRequest | ResourceEnvelope | 当前契约 |
| GET | `/projects/{project_id}/flows` | `listFlows` | inline / 无 body | ListEnvelope | 当前契约 |
| POST | `/projects/{project_id}/flows` | `createFlow` | FlowWriteRequest | ResourceEnvelope | 当前契约 |
| GET | `/projects/{project_id}/flows/{flow_id}` | `getFlow` | inline / 无 body | ResourceEnvelope | 当前契约 |
| PATCH | `/projects/{project_id}/flows/{flow_id}` | `updateFlow` | FlowWriteRequest | ResourceEnvelope | 当前契约 |
| GET | `/executions` | `listExecutions` | inline / 无 body | ListEnvelope | 当前契约 |
| POST | `/executions` | `createExecution` | RunFlowRequest | ResourceEnvelope | 当前契约 |
| GET | `/executions/{execution_id}` | `getExecution` | inline / 无 body | ResourceEnvelope | 当前契约 |
| PATCH | `/executions/{execution_id}` | `updateExecution` | ExecutionControlRequest | ResourceEnvelope | 当前契约 |
| GET | `/flows/{flow_id}/schedules` | `listSchedules` | inline / 无 body | ListEnvelope | 当前契约 |
| POST | `/flows/{flow_id}/schedules` | `createSchedule` | ScheduleWriteRequest | ResourceEnvelope | 当前契约 |
| GET | `/flows/{flow_id}/schedules/{schedule_id}` | `getSchedule` | inline / 无 body | ResourceEnvelope | 当前契约 |
| PATCH | `/flows/{flow_id}/schedules/{schedule_id}` | `updateSchedule` | SchedulePatchRequest | ResourceEnvelope | 当前契约 |
| GET | `/projects/{project_id}/fabric/devices` | `listFabricDevices` | inline / 无 body | ListEnvelope | 当前契约 |
| POST | `/projects/{project_id}/fabric/devices` | `createFabricDevice` | DeviceWriteRequest | ResourceEnvelope | 当前契约 |
| GET | `/projects/{project_id}/fabric/devices/{device_id}` | `getFabricDevice` | inline / 无 body | ResourceEnvelope | 当前契约 |
| PATCH | `/projects/{project_id}/fabric/devices/{device_id}` | `updateFabricDevice` | DevicePatchRequest | ResourceEnvelope | 当前契约 |
| GET | `/projects/{project_id}/network-baselines` | `listNetworkBaselines` | inline / 无 body | ListEnvelope | 当前契约 |
| POST | `/projects/{project_id}/network-baselines` | `createNetworkBaseline` | NetworkBaselineWriteRequest | ResourceEnvelope | 当前契约 |
| GET | `/projects/{project_id}/network-baselines/{baseline_id}` | `getNetworkBaseline` | inline / 无 body | ResourceEnvelope | 当前契约 |
| PATCH | `/projects/{project_id}/network-baselines/{baseline_id}` | `updateNetworkBaseline` | NetworkBaselinePatchRequest | ResourceEnvelope | 当前契约 |
| GET | `/push-jobs` | `listPushJobs` | inline / 无 body | ListEnvelope | 当前契约 |
| POST | `/push-jobs` | `createPushJob` | PushJobWriteRequest | ResourceEnvelope | 当前契约 |
| GET | `/push-jobs/{push_job_id}` | `getPushJob` | inline / 无 body | ResourceEnvelope | 当前契约 |
| PATCH | `/push-jobs/{push_job_id}` | `updatePushJob` | PushJobPatchRequest | ResourceEnvelope | 当前契约 |
| GET | `/projects/{project_id}/gates` | `listGateEvidences` | inline / 无 body | ListEnvelope | deprecated / 仅历史兼容 |
| POST | `/projects/{project_id}/gates` | `createGateEvidence` | inline / 无 body | inline / 无 body | deprecated / 仅历史兼容 |
| GET | `/projects/{project_id}/gates/{gate_id}` | `getGateEvidence` | inline / 无 body | ResourceEnvelope | deprecated / 仅历史兼容 |
| PATCH | `/projects/{project_id}/gates/{gate_id}` | `updateGateEvidence` | inline / 无 body | inline / 无 body | deprecated / 仅历史兼容 |
| GET | `/projects/{project_id}/issues` | `listIssues` | inline / 无 body | ListEnvelope | 当前契约 |
| POST | `/projects/{project_id}/issues` | `createIssue` | IssueWriteRequest | ResourceEnvelope | 当前契约 |
| GET | `/projects/{project_id}/issues/{issue_id}` | `getIssue` | inline / 无 body | ResourceEnvelope | 当前契约 |
| PATCH | `/projects/{project_id}/issues/{issue_id}` | `updateIssue` | IssuePatchRequest | ResourceEnvelope | 当前契约 |
| GET | `/projects/{project_id}/alerts` | `listAlerts` | inline / 无 body | ListEnvelope | 当前契约 |
| POST | `/projects/{project_id}/alerts` | `createAlert` | AlertWriteRequest | ResourceEnvelope | 当前契约 |
| GET | `/projects/{project_id}/alerts/{alert_id}` | `getAlert` | inline / 无 body | ResourceEnvelope | 当前契约 |
| PATCH | `/projects/{project_id}/alerts/{alert_id}` | `updateAlert` | AlertPatchRequest | ResourceEnvelope | 当前契约 |
| GET | `/project-templates` | `listProjectTemplates` | inline / 无 body | ListEnvelope | 当前契约 |
| POST | `/project-templates` | `createProjectTemplate` | TemplateWriteRequest | ResourceEnvelope | 当前契约 |
| GET | `/project-templates/{template_id}` | `getProjectTemplate` | inline / 无 body | ResourceEnvelope | 当前契约 |
| PATCH | `/project-templates/{template_id}` | `updateProjectTemplate` | TemplatePatchRequest | ResourceEnvelope | 当前契约 |
| GET | `/media` | `listMediaArtifacts` | inline / 无 body | ListEnvelope | 当前契约 |
| POST | `/media` | `createMediaArtifact` | MediaWriteRequest | ResourceEnvelope | 当前契约 |
| GET | `/media/{media_id}` | `getMediaArtifact` | inline / 无 body | ResourceEnvelope | 当前契约 |
| PATCH | `/media/{media_id}` | `updateMediaArtifact` | MediaPatchRequest | ResourceEnvelope | 当前契约 |
| GET | `/parameter-catalog` | `listParameters` | inline / 无 body | ListEnvelope | 当前契约 |
| POST | `/parameter-catalog` | `createParameter` | ParameterWriteRequest | ResourceEnvelope | 当前契约 |
| GET | `/parameter-catalog/{parameter_key}` | `getParameter` | inline / 无 body | ResourceEnvelope | 当前契约 |
| PATCH | `/parameter-catalog/{parameter_key}` | `updateParameter` | ParameterPatchRequest | ResourceEnvelope | 当前契约 |
| GET | `/users` | `listUsers` | inline / 无 body | ListEnvelope | 当前契约 |
| POST | `/users` | `createUser` | UserWriteRequest | ResourceEnvelope | 当前契约 |
| GET | `/users/{user_id}` | `getUser` | inline / 无 body | ResourceEnvelope | 当前契约 |
| PATCH | `/users/{user_id}` | `updateUser` | UserPatchRequest | ResourceEnvelope | 当前契约 |
| GET | `/audit-events` | `listAuditEvents` | inline / 无 body | ListEnvelope | 当前契约 |
| POST | `/audit-events` | `createAuditEvent` | AuditProjectionRequest | ResourceEnvelope | 当前契约 |
| GET | `/audit-events/{event_id}` | `getAuditEvent` | inline / 无 body | ResourceEnvelope | 当前契约 |
| GET | `/cmdb/entities/{entity_id}/relationships` | `listCMDBRelationships` | inline / 无 body | ListEnvelope | 当前契约 |
| GET | `/cmdb/entities/{entity_id}/timeline` | `getCMDBTimeline` | inline / 无 body | ListEnvelope | 当前契约 |
| GET | `/cmdb/events/{event_id}` | `getCMDBEvent` | inline / 无 body | ResourceEnvelope | 当前契约 |
| GET | `/projects/{project_id}/overview` | `getProjectOverview` | inline / 无 body | ResourceEnvelope | 当前契约 |
| GET | `/devices/{device_id}/ports` | `listDevicePorts` | inline / 无 body | ListEnvelope | 当前契约 |
| GET | `/devices/{device_id}/snapshots` | `listDeviceSnapshots` | inline / 无 body | ListEnvelope | 当前契约 |
| GET | `/flows/{flow_id}/revisions` | `listFlowRevisions` | inline / 无 body | ListEnvelope | 当前契约 |
| POST | `/flows/{flow_id}/validate` | `validateFlow` | FlowValidationRequest | ResourceEnvelope | 当前契约 |
| POST | `/flows/{flow_id}/run` | `runFlow` | RunFlowRequest | JobEnvelope | 当前契约 |
| POST | `/schedules/{schedule_id}/enable` | `enableSchedule` | EmptyRequest | ResourceEnvelope | 当前契约 |
| POST | `/schedules/{schedule_id}/disable` | `disableSchedule` | EmptyRequest | ResourceEnvelope | 当前契约 |
| GET | `/executions/{execution_id}/targets` | `listExecutionTargets` | inline / 无 body | ListEnvelope | 当前契约 |
| GET | `/executions/{execution_id}/tasks` | `listExecutionTasks` | inline / 无 body | ListEnvelope | 当前契约 |
| GET | `/executions/{execution_id}/logs` | `listExecutionLogs` | inline / 无 body | ListEnvelope | 当前契约 |
| GET | `/executions/{execution_id}/outputs` | `listExecutionOutputs` | inline / 无 body | ListEnvelope | 当前契约 |
| POST | `/executions/{execution_id}/pause` | `pauseExecution` | ExecutionControlRequest | ResourceEnvelope | 当前契约 |
| POST | `/executions/{execution_id}/resume` | `resumeExecution` | ExecutionControlRequest | ResourceEnvelope | 当前契约 |
| POST | `/executions/{execution_id}/kill` | `killExecution` | ExecutionControlRequest | ResourceEnvelope | 当前契约 |
| POST | `/executions/{execution_id}/retry-failed` | `retryFailedTargets` | ExecutionControlRequest | JobEnvelope | 当前契约 |
| GET | `/projects/{project_id}/fabric/devices/{device_id}/ports` | `listFabricPorts` | inline / 无 body | ListEnvelope | 当前契约 |
| POST | `/push-jobs/{push_job_id}/approve` | `approvePushJob` | ApprovalRequest | ResourceEnvelope | 当前契约 |
| POST | `/push-jobs/{push_job_id}/execute` | `executePushJob` | ExecutionControlRequest | JobEnvelope | 当前契约 |
| POST | `/push-jobs/{push_job_id}/rollback` | `rollbackPushJob` | ExecutionControlRequest | JobEnvelope | 当前契约 |
| GET | `/devices/{device_id}/gate-evidence` | `listDeviceGateEvidence` | inline / 无 body | ListEnvelope | 当前契约 |
| POST | `/delivery-runs` | `createDeliveryRun` | RunFlowRequest | JobEnvelope | 当前契约 |
| GET | `/delivery-runs/{run_id}` | `getDeliveryRun` | inline / 无 body | ResourceEnvelope | 当前契约 |
| POST | `/issues/{issue_id}/revalidate` | `revalidateIssue` | ExecutionControlRequest | JobEnvelope | 当前契约 |
| POST | `/issues/{issue_id}/close` | `closeIssue` | CloseIssueRequest | ResourceEnvelope | 当前契约 |
| POST | `/alerts/{alert_id}/acknowledge` | `acknowledgeAlert` | AcknowledgeRequest | ResourceEnvelope | 当前契约 |
| GET | `/auth/session` | `getSession` | inline / 无 body | ResourceEnvelope | 当前契约 |
| POST | `/auth/logout` | `logout` | EmptyRequest | ResourceEnvelope | 当前契约 |
| GET | `/project-permissions` | `listProjectPermissions` | inline / 无 body | ListEnvelope | 当前契约 |
| POST | `/project-templates/{template_id}/approve` | `approveProjectTemplate` | ApprovalRequest | ResourceEnvelope | 当前契约 |
| POST | `/media/{media_id}/approve` | `approveMediaArtifact` | ApprovalRequest | ResourceEnvelope | 当前契约 |
| POST | `/media/{media_id}/retire` | `retireMediaArtifact` | EmptyRequest | ResourceEnvelope | 当前契约 |
| POST | `/issues/{issue_id}/assign` | `assignIssue` | AssignIssueRequest | ResourceEnvelope | 当前契约 |
| GET | `/projects/{project_id}/racks` | `listRacks` | inline / 无 body | Rack | 当前契约 |
| GET | `/projects/{project_id}/rack-diagrams` | `listRackDiagrams` | inline / 无 body | RackDiagram | 当前契约 |
| POST | `/projects/{project_id}/rack-diagrams` | `createRackDiagram` | CreateRackDiagramRequest | RackDiagramEnvelope | 当前契约 |
| POST | `/projects/{project_id}/rack-diagrams/generate` | `generateRackDiagram` | GenerateRackDiagramRequest | RackDiagramGenerationEnvelope | 当前契约 |
| GET | `/projects/{project_id}/rack-diagrams/generations/{job_id}` | `getRackDiagramGeneration` | inline / 无 body | RackDiagramGenerationEnvelope | 当前契约 |
| GET | `/projects/{project_id}/rack-diagrams/{diagram_id}` | `getRackDiagram` | inline / 无 body | RackDiagramEnvelope | 当前契约 |
| GET | `/projects/{project_id}/rack-diagrams/{diagram_id}/revisions/{revision_id}` | `getRackDiagramRevision` | inline / 无 body | RackDiagramRevisionEnvelope | 当前契约 |
| POST | `/projects/{project_id}/rack-diagrams/{diagram_id}/revisions/{revision_id}/validate` | `validateRackDiagramRevision` | inline / 无 body | RackDiagramRevisionEnvelope | 当前契约 |
| POST | `/projects/{project_id}/rack-diagrams/{diagram_id}/revisions/{revision_id}/placements` | `upsertRackPlacement` | UpsertRackPlacementRequest | RackDiagramRevisionEnvelope | 当前契约 |
| POST | `/projects/{project_id}/rack-diagrams/{diagram_id}/publish` | `publishRackDiagram` | inline / 无 body | RackDiagramEnvelope | 当前契约 |
| GET | `/projects/{project_id}/rack-diagrams/{diagram_id}/export` | `exportRackDiagram` | inline / 无 body | RackDiagramGenerationEnvelope | 当前契约 |
| GET | `/projects/{project_id}/operations/devices` | `opsListDevices` | inline / 无 body | OpsDeviceListEnvelope | 当前契约 |
| GET | `/devices/{device_id}/operations/detail` | `opsGetDeviceDetail` | inline / 无 body | OpsDeviceDetailEnvelope | 当前契约 |
| GET | `/devices/{device_id}/operations/configurations` | `opsListConfigurations` | inline / 无 body | OpsConfigurationListEnvelope | 当前契约 |
| GET | `/devices/{device_id}/operations/config-diffs` | `opsListConfigDiffs` | inline / 无 body | OpsConfigDiffListEnvelope | 当前契约 |
| GET | `/devices/{device_id}/operations/ports` | `opsListPorts` | inline / 无 body | OpsPortListEnvelope | 当前契约 |
| GET | `/devices/{device_id}/operations/telemetry` | `opsListTelemetry` | inline / 无 body | OpsTelemetryListEnvelope | 当前契约 |
| GET | `/devices/{device_id}/operations/lldp` | `opsListLLDP` | inline / 无 body | OpsLLDPListEnvelope | 当前契约 |
| GET | `/devices/{device_id}/operations/fdb` | `opsListFDB` | inline / 无 body | OpsFDBListEnvelope | 当前契约 |
| GET | `/devices/{device_id}/operations/arp` | `opsListARP` | inline / 无 body | OpsARPListEnvelope | 当前契约 |
| GET | `/devices/{device_id}/operations/ddm` | `opsListDDM` | inline / 无 body | OpsDDMListEnvelope | 当前契约 |
| GET | `/devices/{device_id}/operations/metrics` | `opsListMetrics` | inline / 无 body | OpsMetricListEnvelope | 当前契约 |
| GET | `/devices/{device_id}/operations/logs` | `opsListLogs` | inline / 无 body | OpsLogListEnvelope | 当前契约 |
| GET | `/devices/{device_id}/operations/firmware` | `opsListFirmware` | inline / 无 body | OpsFirmwareListEnvelope | 当前契约 |
| GET | `/devices/{device_id}/operations/bmc` | `opsGetBmc` | inline / 无 body | OpsBmcEnvelope | 当前契约 |
| GET | `/devices/{device_id}/operations/bmc/sel` | `opsListBmcSEL` | inline / 无 body | OpsLogListEnvelope | 当前契约 |
| GET | `/devices/{device_id}/operations/links` | `opsGetAssetLinks` | inline / 无 body | OpsAssetLinksEnvelope | 当前契约 |
| POST | `/devices/{device_id}/operations/power-jobs` | `opsCreatePowerJob` | OpsPowerRequest | OpsJobEnvelope | 当前契约 |
| POST | `/devices/{device_id}/operations/firmware-upgrade-jobs` | `opsCreateFirmwareUpgrade` | OpsFirmwareUpgradeRequest | OpsFirmwareUpgradeJobEnvelope | 当前契约 |
| GET | `/firmware-upgrade-jobs/{job_id}` | `opsGetFirmwareUpgrade` | inline / 无 body | OpsFirmwareUpgradeJobEnvelope | 当前契约 |
| POST | `/firmware-upgrade-jobs/{job_id}/approve` | `opsApproveFirmwareUpgrade` | OpsApprovalRequest | OpsFirmwareUpgradeJobEnvelope | 当前契约 |
| POST | `/firmware-upgrade-jobs/{job_id}/execute` | `opsExecuteFirmwareUpgrade` | OpsJobControlRequest | OpsFirmwareUpgradeJobEnvelope | 当前契约 |
| GET | `/projects/{project_id}/collection-policies` | `opsListCollectionPolicies` | inline / 无 body | OpsCollectionPolicyListEnvelope | 当前契约 |
| POST | `/projects/{project_id}/collection-policies` | `opsCreateCollectionPolicy` | OpsCollectionPolicyRequest | OpsCollectionPolicyEnvelope | 当前契约 |
| PATCH | `/collection-policies/{policy_id}` | `opsUpdateCollectionPolicy` | OpsCollectionPolicyRequest | OpsCollectionPolicyEnvelope | 当前契约 |
| POST | `/devices/{device_id}/collection-jobs` | `opsCreateCollectionJob` | OpsCollectionRequest | OpsJobEnvelope | 当前契约 |
| GET | `/collection-jobs/{job_id}` | `opsGetCollectionJob` | inline / 无 body | OpsJobEnvelope | 当前契约 |
| POST | `/collection-jobs/{job_id}/retry` | `opsRetryCollectionJob` | OpsJobControlRequest | OpsJobEnvelope | 当前契约 |
| GET | `/devices/{device_id}/maintenance-events` | `opsListMaintenanceEvents` | inline / 无 body | OpsMaintenanceEventListEnvelope | 当前契约 |
| POST | `/devices/{device_id}/maintenance-events` | `opsCreateMaintenanceEvent` | OpsMaintenanceRequest | OpsMaintenanceEventEnvelope | 当前契约 |
| POST | `/devices/{device_id}/retirement-jobs` | `opsCreateRetirementJob` | OpsRetirementRequest | OpsJobEnvelope | 当前契约 |
| GET | `/operations/jobs/{job_id}` | `opsGetJob` | inline / 无 body | OpsJobEnvelope | 当前契约 |
| GET | `/operations/retention-policies/{policy_id}` | `opsGetRetentionPolicy` | inline / 无 body | OpsRetentionPolicyEnvelope | 当前契约 |
| GET | `/ia/session` | `getIASession` | inline / 无 body | IASessionEnvelope | 当前契约 |
| GET | `/ia/cross-project/{resource}` | `listIACrossProject` | inline / 无 body | IACrossProjectEnvelope | 当前契约 |
| GET | `/ia/projects/{project_id}/devices/{device_id}/tabs/{tab}` | `getIADeviceTab` | inline / 无 body | IATabEnvelope | 当前契约 |
| GET | `/ia/events/stream` | `streamIAEvents` | inline / 无 body | IAEvent | 当前契约 |
| GET | `/ia/ai-providers` | `listIAAIProvider` | inline / 无 body | IAAIProviderEnvelope | 当前契约 |
| GET | `/ia/ai-jobs` | `listIAAIJob` | inline / 无 body | IAAIJobEnvelope | 当前契约 |
| GET | `/ia/knowledge-sources` | `listIAKnowledgeSource` | inline / 无 body | IAKnowledgeSourceEnvelope | 当前契约 |
| GET | `/ia/ai-feedback` | `listIAAIFeedback` | inline / 无 body | IAAIFeedbackEnvelope | 当前契约 |
| GET | `/ia/notification-settings` | `listIANotificationSetting` | inline / 无 body | IANotificationSettingEnvelope | 当前契约 |
| GET | `/ia/api-tokens` | `listIAAPIToken` | inline / 无 body | IAAPITokenEnvelope | 当前契约 |
| GET | `/ia/ssh-trust` | `listIASshTrust` | inline / 无 body | IASshTrustEnvelope | 当前契约 |
| GET | `/ia/collection-policies` | `listIACollectionPolicy` | inline / 无 body | IACollectionPolicyEnvelope | 当前契约 |
| GET | `/projects/{projectId}/timeline` | `fullprojecttimelinesprojectsprojectIdtimeline` | inline / 无 body | FullProjectTimelinesListEnvelope | 当前契约 |
| GET | `/dashboard/summary` | `fulldashboardsnapshotsdashboardsummary` | inline / 无 body | FullDashboardSnapshotsListEnvelope | 当前契约 |
| GET | `/fleet/servers` | `fullserversfleetservers` | inline / 无 body | FullServersListEnvelope | 当前契约 |
| GET | `/fleet/servers/{serverId}` | `fullserversfleetserversserverId` | inline / 无 body | FullServersListEnvelope | 当前契约 |
| GET | `/fleet/servers/{serverId}/runtime` | `fullserverruntimefleetserversserverIdruntime` | inline / 无 body | FullServerRuntimeListEnvelope | 当前契约 |
| GET | `/fleet/servers/{serverId}/bmc` | `fullbmcinventoryfleetserversserverIdbmc` | inline / 无 body | FullBmcInventoryListEnvelope | 当前契约 |
| GET | `/fleet/servers/{serverId}/bmc/sensors` | `fullbmcsensorsfleetserversserverIdbmcsensors` | inline / 无 body | FullBmcSensorsListEnvelope | 当前契约 |
| GET | `/fleet/servers/{serverId}/firmware` | `fullfirmwareinventoryfleetserversserverIdfirmware` | inline / 无 body | FullFirmwareInventoryListEnvelope | 当前契约 |
| GET | `/fleet/servers/{serverId}/config/diff` | `fullserverobservedconfigsfleetserversserverIdconfigdiff` | inline / 无 body | FullServerObservedConfigsListEnvelope | 当前契约 |
| GET | `/fabric/switches` | `fullswitchesfabricswitches` | inline / 无 body | FullSwitchesListEnvelope | 当前契约 |
| GET | `/fabric/switches/{switchId}/runtime` | `fullswitchruntimefabricswitchesswitchIdruntime` | inline / 无 body | FullSwitchRuntimeListEnvelope | 当前契约 |
| GET | `/fabric/switches/{switchId}/config/baseline` | `fullswitchconfigbaselinesfabricswitchesswitchIdconfigbaseline` | inline / 无 body | FullSwitchConfigBaselinesListEnvelope | 当前契约 |
| GET | `/fabric/switches/{switchId}/config/running` | `fullswitchrunningconfigsfabricswitchesswitchIdconfigrunning` | inline / 无 body | FullSwitchRunningConfigsListEnvelope | 当前契约 |
| GET | `/fabric/switches/{switchId}/config/diff` | `fullswitchconfigdiffsfabricswitchesswitchIdconfigdiff` | inline / 无 body | FullSwitchConfigDiffsListEnvelope | 当前契约 |
| GET | `/fabric/switches/{switchId}/interfaces` | `fullswitchinterfacesfabricswitchesswitchIdinterfaces` | inline / 无 body | FullSwitchInterfacesListEnvelope | 当前契约 |
| GET | `/fabric/switches/{switchId}/lldp` | `fulllldpneighborsfabricswitchesswitchIdlldp` | inline / 无 body | FullLldpNeighborsListEnvelope | 当前契约 |
| GET | `/fabric/switches/{switchId}/fdb` | `fullfdbentriesfabricswitchesswitchIdfdb` | inline / 无 body | FullFdbEntriesListEnvelope | 当前契约 |
| GET | `/fabric/switches/{switchId}/arp` | `fullarpentriesfabricswitchesswitchIdarp` | inline / 无 body | FullArpEntriesListEnvelope | 当前契约 |
| GET | `/fabric/switches/{switchId}/ddm` | `fullddmsamplesfabricswitchesswitchIdddm` | inline / 无 body | FullDdmSamplesListEnvelope | 当前契约 |
| GET | `/fabric/switches/{switchId}/bgp` | `fullswitchbgpneighborsfabricswitchesswitchIdbgp` | inline / 无 body | FullSwitchBgpNeighborsListEnvelope | 当前契约 |
| GET | `/fabric/switches/{switchId}/logs` | `fullswitchlogsfabricswitchesswitchIdlogs` | inline / 无 body | FullSwitchLogsListEnvelope | 当前契约 |
| GET | `/bom/imports` | `fullbomimportsbomimports` | inline / 无 body | FullBomImportsListEnvelope | 当前契约 |
| GET | `/bom/imports/{importId}/sheets` | `fullbomsheetregionsbomimportsimportIdsheets` | inline / 无 body | FullBomSheetRegionsListEnvelope | 当前契约 |
| GET | `/bom/imports/{importId}/mappings` | `fullbommappingcandidatesbomimportsimportIdmappings` | inline / 无 body | FullBomMappingCandidatesListEnvelope | 当前契约 |
| GET | `/bom/imports/{importId}/issues` | `fullbomissuesbomimportsimportIdissues` | inline / 无 body | FullBomIssuesListEnvelope | 当前契约 |
| GET | `/cmdb/entities/{entityId}/relationships` | `fullcmdbrelationshipscmdbentitiesentityIdrelationships` | inline / 无 body | FullCmdbRelationshipsListEnvelope | 当前契约 |
| GET | `/cmdb/entities/{entityId}/timeline` | `fulllifecycleeventscmdbentitiesentityIdtimeline` | inline / 无 body | FullLifecycleEventsListEnvelope | 当前契约 |
| GET | `/racks/global-map` | `fullrackglobalmapsracksglobalmap` | inline / 无 body | FullRackGlobalMapsListEnvelope | 当前契约 |
| GET | `/racks` | `fullracksracks` | inline / 无 body | FullRacksListEnvelope | 当前契约 |
| GET | `/racks/{rackId}/diagram` | `fullrackdiagramsracksrackIddiagram` | inline / 无 body | FullRackDiagramsListEnvelope | 当前契约 |
| GET | `/racks/{rackId}/placements` | `fullrackplacementsracksrackIdplacements` | inline / 无 body | FullRackPlacementsListEnvelope | 当前契约 |
| GET | `/rack-diagrams/revisions` | `fullrackdiagramrevisionsrackdiagramsrevisions` | inline / 无 body | FullRackDiagramRevisionsListEnvelope | 当前契约 |
| GET | `/flows` | `fullflowsflows` | inline / 无 body | FullFlowsListEnvelope | 当前契约 |
| GET | `/flows/{flowId}/revisions` | `fullflowrevisionsflowsflowIdrevisions` | inline / 无 body | FullFlowRevisionsListEnvelope | 当前契约 |
| GET | `/flow-blueprints` | `fullflowblueprintsflowblueprints` | inline / 无 body | FullFlowBlueprintsListEnvelope | 当前契约 |
| GET | `/flow-target-groups` | `fullflowtargetgroupsflowtargetgroups` | inline / 无 body | FullFlowTargetGroupsListEnvelope | 当前契约 |
| GET | `/executions/{executionId}/targets` | `fullexecutiontargetsexecutionsexecutionIdtargets` | inline / 无 body | FullExecutionTargetsListEnvelope | 当前契约 |
| GET | `/executions/{executionId}/tasks` | `fulltaskrunsexecutionsexecutionIdtasks` | inline / 无 body | FullTaskRunsListEnvelope | 当前契约 |
| GET | `/executions/{executionId}/logs` | `fullexecutionlogsexecutionsexecutionIdlogs` | inline / 无 body | FullExecutionLogsListEnvelope | 当前契约 |
| GET | `/executions/{executionId}/outputs` | `fullexecutionoutputsexecutionsexecutionIdoutputs` | inline / 无 body | FullExecutionOutputsListEnvelope | 当前契约 |
| GET | `/schedules` | `fullschedulesschedules` | inline / 无 body | FullSchedulesListEnvelope | 当前契约 |
| GET | `/issues` | `fullissuesissues` | inline / 无 body | FullIssuesListEnvelope | 当前契约 |
| GET | `/alerts` | `fullalertsalerts` | inline / 无 body | FullAlertsListEnvelope | 当前契约 |
| GET | `/monitor/metrics` | `fullmetricseriesmonitormetrics` | inline / 无 body | FullMetricSeriesListEnvelope | 当前契约 |
| GET | `/monitor/collections` | `fullcollectionjobsmonitorcollections` | inline / 无 body | FullCollectionJobsListEnvelope | 当前契约 |
| GET | `/monitor/sources` | `fullmonitoringsourcesmonitorsources` | inline / 无 body | FullMonitoringSourcesListEnvelope | 当前契约 |
| GET | `/delivery/gates` | `fullgateevidencedeliverygates` | inline / 无 body | FullGateEvidenceListEnvelope | deprecated / 仅历史兼容 |
| GET | `/delivery/mapping-revisions` | `fullmappingrevisionsdeliverymappingrevisions` | inline / 无 body | FullMappingRevisionsListEnvelope | 当前契约 |
| GET | `/fabric/push-jobs` | `fullpushjobsfabricpushjobs` | inline / 无 body | FullPushJobsListEnvelope | 当前契约 |
| GET | `/fabric/network-baselines` | `fullnetworkbaselinesfabricnetworkbaselines` | inline / 无 body | FullNetworkBaselinesListEnvelope | 当前契约 |
| GET | `/admin/users` | `fullusersadminusers` | inline / 无 body | FullUsersListEnvelope | 当前契约 |
| GET | `/admin/templates` | `fulltemplatesadmintemplates` | inline / 无 body | FullTemplatesListEnvelope | 当前契约 |
| GET | `/admin/device-models` | `fulldevicemodelsadmindevicemodels` | inline / 无 body | FullDeviceModelsListEnvelope | 当前契约 |
| GET | `/admin/parameters` | `fullparametersadminparameters` | inline / 无 body | FullParametersListEnvelope | 当前契约 |
| GET | `/admin/media` | `fullmediaadminmedia` | inline / 无 body | FullMediaListEnvelope | 当前契约 |
| GET | `/admin/audit-events` | `fullauditeventsadminauditevents` | inline / 无 body | FullAuditEventsListEnvelope | 当前契约 |
| GET | `/ai/providers` | `fullaiprovidersaiproviders` | inline / 无 body | FullAiProvidersListEnvelope | 当前契约 |
| GET | `/ai/tasks` | `fullaitasksaitasks` | inline / 无 body | FullAiTasksListEnvelope | 当前契约 |
| GET | `/ai/evaluations` | `fullaievaluationsaievaluations` | inline / 无 body | FullAiEvaluationsListEnvelope | 当前契约 |
| GET | `/ia/projects/{project_id}/servers/{server_id}/mapping` | `getServerMappingMatrix` | inline / 无 body | IAMappingEnvelope | 当前契约 |
| GET | `/projects/{project_id}/delivery/stages` | `listDeliveryStages` | inline / 无 body | DeliveryStageListEnvelope | 当前契约 |
| GET | `/projects/{project_id}/delivery/summary` | `getDeliveryStageSummary` | inline / 无 body | DeliveryStageSummaryEnvelope | 当前契约 |
| POST | `/projects/{project_id}/delivery/stages/{stage_id}/start` | `startDeliveryStage` | DeliveryStageCommand | DeliveryStageStartEnvelope | 当前契约 |
| POST | `/projects/{project_id}/delivery/stages/{stage_id}/complete` | `completeDeliveryStage` | DeliveryStageCommand | DeliveryStageListEnvelope | 当前契约 |
| POST | `/projects/{project_id}/delivery/stages/{stage_id}/waivers` | `waiveDeliveryStage` | DeliveryWaiverRequest | DeliveryStageListEnvelope | 当前契约 |
| GET | `/projects/{project_id}/validation/checks` | `listValidationChecks` | inline / 无 body | ValidationCheckListEnvelope | 当前契约 |
| POST | `/issues/{issue_id}/validation-remediations` | `applyMockValidationRemediation` | ValidationRemediationRequest | ValidationCheckListEnvelope | 当前契约 |
| POST | `/issues/{issue_id}/validation-retests` | `startValidationRetest` | ValidationRetestRequest | ValidationCheckListEnvelope | 当前契约 |
| GET | `/projects/{project_id}/handover-records` | `listHandoverRecords` | inline / 无 body | HandoverListEnvelope | 当前契约 |
| POST | `/projects/{project_id}/handover-records` | `prepareHandover` | HandoverPrepareRequest | HandoverListEnvelope | 当前契约 |
| POST | `/handover-records/{handover_id}/advance` | `advanceHandoverStep` | HandoverAdvanceRequest | HandoverEnvelope | 当前契约 |
| GET | `/projects/{project_id}/racks/{rack_id}/workspace/{tab}` | `getRackWorkspaceTab` | inline / 无 body | RackWorkspaceTabResponse | 当前契约 |
| GET | `/projects/{project_id}/fabric/topology/physical` | `getFabricPhysicalTopology` | inline / 无 body | FabricTopologyGraph | 当前契约 |
| GET | `/projects/{project_id}/fabric/topology/logical` | `getFabricLogicalTopology` | inline / 无 body | FabricTopologyGraph | 当前契约 |
| GET | `/projects/{project_id}/fabric/topology/details/{kind}/{id}` | `getFabricTopologyDetail` | inline / 无 body | FabricTopologyDetail | 当前契约 |
| GET | `/projects/{project_id}/fabric/topology/filters` | `getFabricTopologyFilters` | inline / 无 body | FabricTopologyFilters | 当前契约 |
| GET | `/projects/{project_id}/fabric/devices/{device_id}/lossless/evidence` | `listLosslessEvidence` | inline / 无 body | LosslessResultsEnvelope | 当前契约 |
| GET | `/projects/{project_id}/fabric/devices/{device_id}/lossless/conclusion` | `getLosslessConclusion` | inline / 无 body | LosslessConclusionEnvelope | 当前契约 |
| POST | `/projects/{project_id}/fabric/devices/{device_id}/lossless/collect` | `collectMockLosslessSnapshot` | inline / 无 body | LosslessResultsEnvelope | 当前契约 |
| POST | `/projects/{project_id}/fabric/devices/{device_id}/lossless/retests` | `retestLosslessEvidence` | LosslessRetestRequest | LosslessRetestEnvelope | 当前契约 |
| GET | `/projects/{project_id}/fabric/devices/{device_id}/lossless/raw/{evidence_id}` | `getLosslessRawFrame` | inline / 无 body | LosslessRawEnvelope | 当前契约 |
| GET | `/cmdb/dossier/catalog` | `queryCmdbDossierCatalog` | inline / 无 body | CmdbDossierCatalogEnvelope | 当前契约 |
| GET | `/cmdb/dossier/tree` | `getCmdbDossierTree` | inline / 无 body | CmdbDossierTreeEnvelope | 当前契约 |
| GET | `/cmdb/dossier/relations` | `getCmdbDossierRelations` | inline / 无 body | CmdbDossierRelationsEnvelope | 当前契约 |
| GET | `/cmdb/dossier/assets/{asset_id}/provenance` | `getCmdbDossierprovenance` | inline / 无 body | CmdbDossierSourcesEnvelope | 当前契约 |
| GET | `/cmdb/dossier/assets/{asset_id}/configuration` | `getCmdbDossierconfiguration` | inline / 无 body | CmdbDossierConfigEnvelope | 当前契约 |
| GET | `/cmdb/dossier/assets/{asset_id}/lifecycle` | `getCmdbDossierlifecycle` | inline / 无 body | CmdbDossierLifecycleEnvelope | 当前契约 |
| GET | `/cmdb/dossier/quality` | `getCmdbDossierQuality` | inline / 无 body | CmdbDossierQualityEnvelope | 当前契约 |
| PATCH | `/cmdb/dossier/manual-fields` | `patchCmdbManualFields` | CmdbDossierManualRequest | CmdbDossierManualEnvelope | 当前契约 |
| GET | `/cmdb/dossier/saved-views` | `listCmdbSavedViews` | inline / 无 body | CmdbDossierViewsEnvelope | 当前契约 |
| POST | `/cmdb/dossier/saved-views` | `createCmdbSavedView` | CmdbDossierSaveViewRequest | CmdbDossierSavedViewEnvelope | 当前契约 |
| GET | `/cmdb/dossier/evidence/{resource}/{record_id}` | `getCmdbDossierEvidence` | inline / 无 body | CmdbDossierEvidenceEnvelope | 当前契约 |
