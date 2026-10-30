# IA v2.4 页面、路由、抽屉与API实施对照

本表由 `src/app/registry.ts` 生成；入口覆盖不等于所有业务验收通过。历史结果见 IA24 报告；六阶段迁移后的构建与交互检查按用户安排暂未执行，不沿用历史通过结论。

## 导航树

```text
顶栏: keel · dev · ⌘K · 通知 · ProjectSwitcher · 用户
部署平面
  部署项目
  项目总览
  BOM 开局配置
  Fabric 网络底座 → 网络就绪验收 / 网络设备 / Topology 拓扑与布线 / ZTP 开通 / Golden 配置 / Doublecheck / PushJob / 无损健康 / 光模块 DDM / 网络日志 / 规则治理
  Provision 装机 → PXE 批次 / OS 镜像 / BIOS 批量置位 / 固件升级
  Validation 验机 → 验机检查与结果 / 预收集记录 / PCIe 拓扑 / LLDP 四步核查 / GPU-NIC Mapping
  NCCL → 测试批次 / 测试报告 / 基线对比 / 阈值场景 / 网络溯源
  Cleanup → 清理计划 / 清理执行 / 归档与签署 / 维护态转换
  NetTest → RoCE 连通性 / 测试批次 / 报告与路径矩阵
  Config → ConfigSet / 网络角色方案 / 应用记录 / Agent 分发
运维平面
  Dashboard
  Fleet 服务器舰队 → 服务器列表 / 网口健康 / 资产台账 / 采集任务
  RackView 机柜机房
  Issues 工单 → 全局工单池 / 工单看板 / 处置统计 / 故障复测
  Monitor 监控中心 → 告警中心 / 采集策略 / 上报观测 / 维护期 BMC 监控
  Flows 工作流中心 → Flow 管理 / Execution 执行 / 蓝图库 / 介质库
  CMDB → 资产目录 / 资产关系 / 配置与来源 / 数据质量 / 项目资料 / 生命周期
  AI 助手 → AI 对话 / Provider 管理 / 知识库 / 识别任务 / 识别与反馈 / 评测报告
系统管理
  系统管理 → 用户与权限 / 通知设置 / API Token / 审计日志 / 项目模板 / 型号库 / 参数目录 / SSH 信任管理 / AI Provider 配置
```

## 页面与路由表

| 页面 | 路由 | 作用域 | 权限 | 加载 | 组件 | API/资源 |
|---|---|---|---|---|---|---|
| 部署项目 | `/projects` | project | project:read | on-demand | Projects | GET /projects |
| 项目总览 | `/p/:projectId/deploy/overview` | project | delivery:read | poll-60s | ProjectOverview | GET /projects/{project_id}/overview |
| BOM 开局配置 | `/p/:projectId/deploy/bom` | project | provision:read | on-demand | BOMWorkspace | /bom-imports; /projects/{project_id}/delivery/stages |
| Fabric 网络底座 | `/p/:projectId/deploy/fabric` | project | fabric:read | poll-60s | FabricWorkspace / NetworkWorkspace | /fabric/switches/*; /push-jobs/*; /network-baselines |
| Fabric 网络底座 / 网络就绪验收 | `/p/:projectId/deploy/fabric/readiness` | project | fabric:read | on-demand | FabricWorkspace / NetworkWorkspace | /fabric/switches/*; /push-jobs/*; /network-baselines |
| Fabric 网络底座 / 网络设备 | `/p/:projectId/deploy/fabric/devices` | project | fabric:read | SSE | FabricWorkspace / NetworkWorkspace | /fabric/switches/*; /push-jobs/*; /network-baselines |
| Fabric 网络底座 / Topology 拓扑与布线 | `/p/:projectId/deploy/fabric/topology` | project | fabric:read | on-demand | FabricWorkspace / NetworkWorkspace | /fabric/switches/*; /push-jobs/*; /network-baselines |
| Fabric 网络底座 / ZTP 开通 | `/p/:projectId/deploy/fabric/ztp` | project | fabric:read | SSE | FabricWorkspace / NetworkWorkspace | /fabric/switches/*; /push-jobs/*; /network-baselines |
| Fabric 网络底座 / Golden 配置 | `/p/:projectId/deploy/fabric/golden` | project | fabric:read | poll-60s | FabricWorkspace / NetworkWorkspace | /fabric/switches/*; /push-jobs/*; /network-baselines |
| Fabric 网络底座 / Doublecheck | `/p/:projectId/deploy/fabric/doublecheck` | project | fabric:read | poll-60s | FabricWorkspace / NetworkWorkspace | /fabric/switches/*; /push-jobs/*; /network-baselines |
| Fabric 网络底座 / PushJob | `/p/:projectId/deploy/fabric/pushjobs` | project | fabric:read | SSE | FabricWorkspace / NetworkWorkspace | /fabric/switches/*; /push-jobs/*; /network-baselines |
| Fabric 网络底座 / 无损健康 | `/p/:projectId/deploy/fabric/lossless` | project | fabric:read | poll-60s | FabricWorkspace / NetworkWorkspace | /fabric/switches/*; /push-jobs/*; /network-baselines |
| Fabric 网络底座 / 光模块 DDM | `/p/:projectId/deploy/fabric/ddm` | project | fabric:read | poll-60s | FabricWorkspace / NetworkWorkspace | /fabric/switches/*; /push-jobs/*; /network-baselines |
| Fabric 网络底座 / 网络日志 | `/p/:projectId/deploy/fabric/logs` | project | fabric:read | poll-60s | FabricWorkspace / NetworkWorkspace | /fabric/switches/*; /push-jobs/*; /network-baselines |
| Fabric 网络底座 / 规则治理 | `/p/:projectId/deploy/fabric/rules` | project | fabric:read | poll-60s | FabricWorkspace / NetworkWorkspace | /fabric/switches/*; /push-jobs/*; /network-baselines |
| Provision 装机 | `/p/:projectId/deploy/provision` | project | provision:read | on-demand | ProvisionWorkspace / MediaWorkspace | /projects/{project_id}/delivery/stages; /devices/{device_id}/operations/firmware |
| Provision 装机 / PXE 批次 | `/p/:projectId/deploy/provision/pxe` | project | provision:read | poll-5s | ProvisionWorkspace / MediaWorkspace | /projects/{project_id}/delivery/stages; /devices/{device_id}/operations/firmware |
| Provision 装机 / OS 镜像 | `/p/:projectId/deploy/provision/media` | project | provision:read | on-demand | ProvisionWorkspace / MediaWorkspace | /projects/{project_id}/delivery/stages; /devices/{device_id}/operations/firmware |
| Provision 装机 / BIOS 批量置位 | `/p/:projectId/deploy/provision/bios` | project | provision:read | on-demand | ProvisionWorkspace / MediaWorkspace | /projects/{project_id}/delivery/stages; /devices/{device_id}/operations/firmware |
| Provision 装机 / 固件升级 | `/p/:projectId/deploy/provision/firmware` | project | provision:read | on-demand | ProvisionWorkspace / MediaWorkspace | /projects/{project_id}/delivery/stages; /devices/{device_id}/operations/firmware |
| Validation 验机 | `/p/:projectId/deploy/validation` | project | validation:read | on-demand | ValidationWorkspace / DeliveryWorkspace | ValidationCheck; DeliveryStageRecord; /delivery/mapping-revisions |
| Validation 验机 / 验机检查与结果 | `/p/:projectId/deploy/validation/checks` | project | validation:read | on-demand | ValidationWorkspace / DeliveryWorkspace | ValidationCheck; DeliveryStageRecord; /delivery/mapping-revisions |
| Validation 验机 / 预收集记录 | `/p/:projectId/deploy/validation/collection` | project | validation:read | on-demand | ValidationWorkspace / DeliveryWorkspace | ValidationCheck; DeliveryStageRecord; /delivery/mapping-revisions |
| Validation 验机 / PCIe 拓扑 | `/p/:projectId/deploy/validation/pcie` | project | validation:read | on-demand | ValidationWorkspace / DeliveryWorkspace | ValidationCheck; DeliveryStageRecord; /delivery/mapping-revisions |
| Validation 验机 / LLDP 四步核查 | `/p/:projectId/deploy/validation/lldp` | project | validation:read | on-demand | ValidationWorkspace / DeliveryWorkspace | ValidationCheck; DeliveryStageRecord; /delivery/mapping-revisions |
| Validation 验机 / GPU-NIC Mapping | `/p/:projectId/deploy/validation/mapping` | project | validation:read | on-demand | ValidationWorkspace / DeliveryWorkspace | ValidationCheck; DeliveryStageRecord; /delivery/mapping-revisions |
| NCCL | `/p/:projectId/deploy/nccl` | project | test:read | poll-5s | TestWorkspace | StepReport; /executions/{execution_id}/* |
| NCCL / 测试批次 | `/p/:projectId/deploy/nccl/batches` | project | test:read | poll-5s | TestWorkspace | StepReport; /executions/{execution_id}/* |
| NCCL / 测试报告 | `/p/:projectId/deploy/nccl/reports` | project | test:read | on-demand | TestWorkspace | StepReport; /executions/{execution_id}/* |
| NCCL / 基线对比 | `/p/:projectId/deploy/nccl/baseline` | project | test:read | poll-5s | TestWorkspace | StepReport; /executions/{execution_id}/* |
| NCCL / 阈值场景 | `/p/:projectId/deploy/nccl/thresholds` | project | test:read | poll-5s | TestWorkspace | StepReport; /executions/{execution_id}/* |
| NCCL / 网络溯源 | `/p/:projectId/deploy/nccl/trace` | project | test:read | poll-5s | TestWorkspace | StepReport; /executions/{execution_id}/* |
| Cleanup | `/p/:projectId/deploy/cleanup` | project | cleanup:read | poll-5s | CleanupWorkspace / ProjectWorkspace | MaintenanceRecord; /devices/{device_id}/maintenance-events; IADeviceOperation |
| Cleanup / 清理计划 | `/p/:projectId/deploy/cleanup/plan` | project | cleanup:read | poll-5s | CleanupWorkspace / ProjectWorkspace | MaintenanceRecord; /devices/{device_id}/maintenance-events; IADeviceOperation |
| Cleanup / 清理执行 | `/p/:projectId/deploy/cleanup/executions` | project | cleanup:read | poll-5s | CleanupWorkspace / ProjectWorkspace | MaintenanceRecord; /devices/{device_id}/maintenance-events; IADeviceOperation |
| Cleanup / 归档与签署 | `/p/:projectId/deploy/cleanup/archive` | project | cleanup:read | poll-5s | CleanupWorkspace / ProjectWorkspace | MaintenanceRecord; /devices/{device_id}/maintenance-events; IADeviceOperation |
| Cleanup / 维护态转换 | `/p/:projectId/deploy/cleanup/maintained` | project | cleanup:read | poll-5s | CleanupWorkspace / ProjectWorkspace | MaintenanceRecord; /devices/{device_id}/maintenance-events; IADeviceOperation |
| NetTest | `/p/:projectId/deploy/nettest` | project | test:read | poll-5s | TestWorkspace | StepReport / IADeviceOperation |
| NetTest / RoCE 连通性 | `/p/:projectId/deploy/nettest/roce` | project | test:read | poll-5s | TestWorkspace | StepReport / IADeviceOperation |
| NetTest / 测试批次 | `/p/:projectId/deploy/nettest/batches` | project | test:read | poll-5s | TestWorkspace | StepReport / IADeviceOperation |
| NetTest / 报告与路径矩阵 | `/p/:projectId/deploy/nettest/reports` | project | test:read | on-demand | TestWorkspace | StepReport / IADeviceOperation |
| Config | `/p/:projectId/deploy/config` | project | config:read | on-demand | ConfigWorkspace / DeliveryWorkspace | ConfigSet; server_config_baselines; /parameter-catalog |
| Config / ConfigSet | `/p/:projectId/deploy/config/configset` | project | config:read | on-demand | ConfigWorkspace / DeliveryWorkspace | ConfigSet; server_config_baselines; /parameter-catalog |
| Config / 网络角色方案 | `/p/:projectId/deploy/config/roles` | project | config:read | on-demand | ConfigWorkspace / DeliveryWorkspace | ConfigSet; server_config_baselines; /parameter-catalog |
| Config / 应用记录 | `/p/:projectId/deploy/config/history` | project | config:read | on-demand | ConfigWorkspace / DeliveryWorkspace | ConfigSet; server_config_baselines; /parameter-catalog |
| Config / Agent 分发 | `/p/:projectId/deploy/config/agent` | project | config:read | on-demand | ConfigWorkspace / DeliveryWorkspace | ConfigSet; server_config_baselines; /parameter-catalog |
| Dashboard | `/dashboard` | cross | dashboard:read | poll-60s | GlobalOverview | /dashboard/summary; /ia/cross-project/{resource} |
| Fleet 服务器舰队 | `/fleet` | cross | fleet:read | poll-5s | DeviceDirectory / DrawerDetail | /fleet/servers/*; /monitor/collections |
| Fleet 服务器舰队 / 服务器列表 | `/fleet/servers` | cross | fleet:read | poll-5s | DeviceDirectory / DrawerDetail | /fleet/servers/*; /monitor/collections |
| Fleet 服务器舰队 / 网口健康 | `/fleet/ports` | cross | fleet:read | SSE | DeviceDirectory / DrawerDetail | /fleet/servers/*; /monitor/collections |
| Fleet 服务器舰队 / 资产台账 | `/fleet/assets` | cross | fleet:read | poll-5s | DeviceDirectory / DrawerDetail | /fleet/servers/*; /monitor/collections |
| Fleet 服务器舰队 / 采集任务 | `/fleet/collections` | cross | fleet:read | poll-5s | DeviceDirectory / DrawerDetail | /fleet/servers/*; /monitor/collections |
| RackView 机柜机房 | `/racks` | cross | cmdb:read | on-demand | RoomView / RackWorkspace | /racks/global-map; /racks/*; /rack-diagrams/revisions |
| Issues 工单 | `/issues` | cross | issues:read | poll-5s | IssueViews / IssuesWorkspace | /issues; /projects/{project_id}/issues; close / assign / retest |
| Issues 工单 / 全局工单池 | `/issues/pool` | cross | issues:read | poll-5s | IssueViews / IssuesWorkspace | /issues; /projects/{project_id}/issues; close / assign / retest |
| Issues 工单 / 工单看板 | `/issues/board` | cross | issues:read | poll-5s | IssueViews / IssuesWorkspace | /issues; /projects/{project_id}/issues; close / assign / retest |
| Issues 工单 / 处置统计 | `/issues/statistics` | cross | issues:read | poll-5s | IssueViews / IssuesWorkspace | /issues; /projects/{project_id}/issues; close / assign / retest |
| Issues 工单 / 故障复测 | `/issues/retest` | cross | issues:read | poll-5s | IssueViews / IssuesWorkspace | /issues; /projects/{project_id}/issues; close / assign / retest |
| Monitor 监控中心 | `/monitor` | cross | monitor:read | SSE | MonitorWorkspace / MonitoringWorkspace | /alerts; /monitor/metrics; /monitor/collections |
| Monitor 监控中心 / 告警中心 | `/monitor/alerts` | cross | monitor:read | SSE | MonitorWorkspace / MonitoringWorkspace | /alerts; /monitor/metrics; /monitor/collections |
| Monitor 监控中心 / 采集策略 | `/monitor/policies` | cross | monitor:read | SSE | MonitorWorkspace / MonitoringWorkspace | /alerts; /monitor/metrics; /monitor/collections |
| Monitor 监控中心 / 上报观测 | `/monitor/observations` | cross | monitor:read | SSE | MonitorWorkspace / MonitoringWorkspace | /alerts; /monitor/metrics; /monitor/collections |
| Monitor 监控中心 / 维护期 BMC 监控 | `/monitor/maintained` | cross | monitor:read | SSE | MonitorWorkspace / MonitoringWorkspace | /alerts; /monitor/metrics; /monitor/collections |
| Flows 工作流中心 | `/flows` | cross | flows:read | SSE | FlowWorkspace / BlueprintWorkspace / MediaWorkspace | /flows/*; /executions/*; /flow-blueprints; /media |
| Flows 工作流中心 / Flow 管理 | `/flows/flows` | cross | flows:read | SSE | FlowWorkspace / BlueprintWorkspace / MediaWorkspace | /flows/*; /executions/*; /flow-blueprints; /media |
| Flows 工作流中心 / Execution 执行 | `/flows/executions` | cross | flows:read | SSE | FlowWorkspace / BlueprintWorkspace / MediaWorkspace | /flows/*; /executions/*; /flow-blueprints; /media |
| Flows 工作流中心 / 蓝图库 | `/flows/blueprints` | cross | flows:read | SSE | FlowWorkspace / BlueprintWorkspace / MediaWorkspace | /flows/*; /executions/*; /flow-blueprints; /media |
| Flows 工作流中心 / 介质库 | `/flows/media` | cross | flows:read | SSE | FlowWorkspace / BlueprintWorkspace / MediaWorkspace | /flows/*; /executions/*; /flow-blueprints; /media |
| CMDB | `/cmdb` | cross | cmdb:read | on-demand | CMDBWorkspace | /cmdb/dossier/catalog; tree; relations; provenance; configuration; lifecycle; quality; manual-fields; saved-views |
| CMDB / 资产目录 | `/cmdb/assets` | cross | cmdb:read | on-demand | CMDBWorkspace | /cmdb/dossier/catalog; tree; relations; provenance; configuration; lifecycle; quality; manual-fields; saved-views |
| CMDB / 资产关系 | `/cmdb/relations` | cross | cmdb:read | on-demand | CMDBWorkspace | /cmdb/dossier/catalog; tree; relations; provenance; configuration; lifecycle; quality; manual-fields; saved-views |
| CMDB / 配置与来源 | `/cmdb/configuration` | cross | cmdb:read | on-demand | CMDBWorkspace | /cmdb/dossier/catalog; tree; relations; provenance; configuration; lifecycle; quality; manual-fields; saved-views |
| CMDB / 数据质量 | `/cmdb/quality` | cross | cmdb:read | on-demand | CMDBWorkspace | /cmdb/dossier/catalog; tree; relations; provenance; configuration; lifecycle; quality; manual-fields; saved-views |
| CMDB / 项目资料 | `/cmdb/projects` | cross | cmdb:read | on-demand | CMDBWorkspace | /cmdb/dossier/catalog; tree; relations; provenance; configuration; lifecycle; quality; manual-fields; saved-views |
| CMDB / 生命周期 | `/cmdb/history` | cross | cmdb:read | on-demand | CMDBWorkspace | /cmdb/dossier/catalog; tree; relations; provenance; configuration; lifecycle; quality; manual-fields; saved-views |
| AI 助手 | `/ai` | cross | ai:read | on-demand | AIManagement / AIWorkspace | /ai/providers; /ai/tasks; /ai/evaluations |
| AI 助手 / AI 对话 | `/ai/chat` | cross | ai:read | on-demand | AIManagement / AIWorkspace | /ai/providers; /ai/tasks; /ai/evaluations |
| AI 助手 / Provider 管理 | `/ai/providers` | cross | ai:read | on-demand | AIManagement / AIWorkspace | /ai/providers; /ai/tasks; /ai/evaluations |
| AI 助手 / 知识库 | `/ai/knowledge` | cross | ai:read | on-demand | AIManagement / AIWorkspace | /ai/providers; /ai/tasks; /ai/evaluations |
| AI 助手 / 识别任务 | `/ai/tasks` | cross | ai:read | on-demand | AIManagement / AIWorkspace | /ai/providers; /ai/tasks; /ai/evaluations |
| AI 助手 / 识别与反馈 | `/ai/feedback` | cross | ai:read | on-demand | AIManagement / AIWorkspace | /ai/providers; /ai/tasks; /ai/evaluations |
| AI 助手 / 评测报告 | `/ai/evaluation` | cross | ai:read | on-demand | AIManagement / AIWorkspace | /ai/providers; /ai/tasks; /ai/evaluations |
| 系统管理 | `/admin` | admin | admin:read | on-demand | AdminWorkspace / GovernanceWorkspace | /users; /project-templates; /media; /parameter-catalog; /audit-events; /ia/* |
| 系统管理 / 用户与权限 | `/admin/users` | admin | admin:read | on-demand | AdminWorkspace / GovernanceWorkspace | /users; /project-templates; /media; /parameter-catalog; /audit-events; /ia/* |
| 系统管理 / 通知设置 | `/admin/notifications` | admin | admin:read | on-demand | AdminWorkspace / GovernanceWorkspace | /users; /project-templates; /media; /parameter-catalog; /audit-events; /ia/* |
| 系统管理 / API Token | `/admin/tokens` | admin | admin:read | on-demand | AdminWorkspace / GovernanceWorkspace | /users; /project-templates; /media; /parameter-catalog; /audit-events; /ia/* |
| 系统管理 / 审计日志 | `/admin/audit` | admin | admin:read | on-demand | AdminWorkspace / GovernanceWorkspace | /users; /project-templates; /media; /parameter-catalog; /audit-events; /ia/* |
| 系统管理 / 项目模板 | `/admin/templates` | admin | admin:read | on-demand | AdminWorkspace / GovernanceWorkspace | /users; /project-templates; /media; /parameter-catalog; /audit-events; /ia/* |
| 系统管理 / 型号库 | `/admin/models` | admin | admin:read | on-demand | AdminWorkspace / GovernanceWorkspace | /users; /project-templates; /media; /parameter-catalog; /audit-events; /ia/* |
| 系统管理 / 参数目录 | `/admin/parameters` | admin | admin:read | on-demand | AdminWorkspace / GovernanceWorkspace | /users; /project-templates; /media; /parameter-catalog; /audit-events; /ia/* |
| 系统管理 / SSH 信任管理 | `/admin/ssh` | admin | admin:read | on-demand | AdminWorkspace / GovernanceWorkspace | /users; /project-templates; /media; /parameter-catalog; /audit-events; /ia/* |
| 系统管理 / AI Provider 配置 | `/admin/ai-provider` | admin | admin:read | on-demand | AdminWorkspace / GovernanceWorkspace | /users; /project-templates; /media; /parameter-catalog; /audit-events; /ia/* |

详情路由：`/p/:projectId/deploy/bom/:importId`、`/racks/:rackId`、`/flows/flows/:flowId`、`/flows/executions/:executionId`。

## 抽屉结构与复用

DrawerDetail → 统一头部、权限、URL、滚动记忆、上一台/下一台、方向键、AbortSignal / Query key。

| 领域 | 固定Tab | 资源与观测 |
|---|---|---|
| 服务器 | 资产台账 | servers / GPU / NIC / BMC / firmware / server baseline + observed |
| 服务器 | 映射详情 | mapping_pairs / 双侧LLDP / 人工确认入口 |
| 服务器 | 网口面板 | ports / cables / 速率 / MAC / peer |
| 服务器 | PCIe拓扑 | GPU/NIC BDF及所属设备；NUMA/NVLink边缺证据显示unknown |
| 服务器 | 监控详情 | server_runtime / metric_series / BMC sensors；维护态限制 |
| 交换机 | 基础信息 | switch身份 / NOS / ZTP / 位置 |
| 交换机 | 配置信息 | 批准基线与running结构化diff，独立Intended未提供时unknown |
| 交换机 | 配置脚本 | 基线/模板/参数引用；不编造未提供脚本原文 |
| 交换机 | 配置事件 | PushJob / 审批 / 审计 |
| 交换机 | 运行状态 | runtime / 物理接口 / breakout / LLDP / FDB / ARP / BGP |
| 交换机 | 无损网络 | NP与配置一致性；缺少PFC/ECN/CNP原始计数显示unknown |
| 交换机 | 光模块 | DDM五类单位 / 合成阈值 / 来源与时间；单样本不造趋势 |
| 交换机 | 日志 | switch_logs / 来源 / 时间 |

## 事实与实施边界

- 数据包83个原始集合；补齐统一BOM投影、待确认Mapping、六阶段记录与验机子任务、受控模拟采集和操作配置后集合数量以当前manifest为准，不再固定旧计数。硬件数量不变。所有补齐只写在统一fixture，不在组件制造业务数据。
- 真实Excel未提供。33个Sheet来自用户提供的结构Profile；本轮没有打开原始工作簿，不能宣称真实解析已验收。
- 维护态严格IPMI-only，原包允许Redfish的记录只保留历史，不赋予通道权限。
- 2条历史Execution的目标类型与Flow冲突，保留来源并隔离执行；缺少身份的user-ops不授权。
- 正式Go、设备执行、AI调用、Kestra引擎和生产调度未实现；定时/事件使用显式Mock触发。
- 批量操作需要绑定项目、目标、版本，配置变更仍经预检/备份/独立审批/复采；schema存在不代表真实Handler实现。
