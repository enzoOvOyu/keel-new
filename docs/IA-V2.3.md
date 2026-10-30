> 历史IA设计记录：导航原则可追溯；原数字门禁主链已失效，现行规则见DELIVERY-LIFECYCLE.md。

# Keel IA v2.3：导航、路由、页面与数据协议

状态：设计及前端实施计划；现有前端尚未完成本规范。权威业务约束仍以 PRD、SYSTEM-DESIGN、OpenAPI 和 maintained/IPMI-only 规则为准。

## 1. 导航树与路由登记

`/p/:projectId/deploy/*` 全部绑定 ProjectContext；`/ops/*` 跨当前用户获授权项目，默认项目过滤可清除；`/admin/*` 由会话 permissions[] 控制。项目目录是选择项目的入口，其余部署页面无有效授权 projectId 不加载业务数据。

| 区域 | 页面及子入口 | 路由 | 加载 | 缓存与权限 |
|---|---|---|---|---|
| 部署 | 项目列表/新建/模板选择 | `/p/:projectId/deploy/projects` | on-demand | project + permission revision；project:read/create |
| 部署 | 项目总览/时间线/G1–G6/阻断 | `/p/:projectId/deploy/overview` | poll 60s | project；delivery:read |
| 部署 | Provision：BOM/PXE/镜像/BIOS/固件 | `/p/:projectId/deploy/provision/:view` | BOM按需；批次poll 5s | project + batch/version；provision:read，操作另校验权限 |
| 部署 | Validation：门禁/预收集/PCIe/LLDP | `/p/:projectId/deploy/validation/:view` | on-demand | project + evidence revision；validation:read |
| 部署 | NetTest：RoCE/报告 | `/p/:projectId/deploy/nettest/:view` | 批次poll 5s，报告按需 | project + test ID；test:read |
| 部署 | Config：ConfigSet/网络角色/应用记录 | `/p/:projectId/deploy/config/:view` | 审核按需 | project + revision；config:read/approve |
| 部署 | Fabric：设备/ZTP/Golden/PushJob/无损/DDM/日志/规则 | `/p/:projectId/deploy/fabric/:view` | 健康摘要poll 60s；端口事件SSE；详情按需 | project + device + revision；fabric:read，PushJob沿用原审批 |
| 部署 | NCCL：批次/报告/基线/阈值 | `/p/:projectId/deploy/nccl/:view` | 批次poll 5s，报告按需 | project + report/version；test:read |
| 部署 | Cleanup：优雅退出 | `/p/:projectId/deploy/cleanup` | poll 5s | project + job；cleanup:read/execute |
| 运维 | 全局交付Dashboard | `/ops/dashboard` | poll 60s | authorized project set + filter；dashboard:read |
| 运维 | Fleet：服务器/设备/网口/台账/采集 | `/ops/fleet/:view` | poll 5s；端口事件SSE | authorized set + optional project filter；fleet:read |
| 运维 | Issues：统一池/统计 | `/ops/issues/:view` | poll 5s | authorized set + filter；issues:read/resolve |
| 运维 | Monitor：告警/采集策略/BMC | `/ops/monitor/:view` | 告警SSE，策略按需 | authorized set + filter；monitor:read |
| 运维 | Flows：Flow/Execution/蓝图/介质 | `/ops/flows/:view` | Execution状态/日志SSE；编辑按需 | project scope + execution；flows:read/run |
| 运维 | CMDB：设备360°/项目资料/生命周期 | `/ops/cmdb/:view` | 聚合poll 60s；详情按需 | authorized set + optional filter；cmdb:read |
| 运维 | AI：对话/Provider/知识库 | `/ops/ai/:view` | on-demand | authorized set + conversation；ai:read/use；真实服务未配置时明确不可用 |
| 系统 | 用户权限/通知/API Token/审计/项目模板/型号/参数/SSH信任 | `/admin/:view` | on-demand | session permission revision；各资源专用admin权限，服务端重验 |

参数 `:view` 的可选值应由路由注册表明确列举，未知值返回404；不得将未实现功能伪装成已完成页面。部署项目列表创建项目之前的无项目入口可由全局 ProjectSwitcher 提供，选定已有项目后进入带 projectId 的项目列表；新建动作的真实授权和项目归属由项目 API 决定。

## 2. 唯一 ProjectContext 与查询边界

Provider 是每浏览器标签页的内存状态，包含 current project ID、授权项目及 permission revision。顶部 ProjectSwitcher 和部署项目列表都调用 `switchProject(projectId)`；先检查目标在当前会话授权集合中。禁止 localStorage/BroadcastChannel 在不同标签页共享可变提交范围。

部署路由 URL 的 projectId 必须与上下文一致；刷新或深链按服务端授权初始化，未经授权返回403。切换项目先中止旧项目请求及 SSE、移除旧作用域内正在进行的查询并关闭旧抽屉，再导航新 projectId；缓存 key 至少包含 projectId、权限版本、资源ID、视图/过滤条件。失效和授权撤销取消请求、清理不可访问缓存。所有部署请求显式携带 project_id；设备操作在 provider/服务端再次验证资产归属。

运维列表的查询 key 包含完整 authorizedProjectIds、可清除的 projectFilter、permission revision 和筛选/分页。切换项目只调整默认过滤，不删除其他授权项目的告警、工单或缓存；用户显式清除过滤后读取授权集合内全部数据，不读未授权项目。批量确认冻结 project_id、稳定目标ID集合、资产/配置revision及确认时间，提交时重新授权和检查版本，变更返回409并要求重新预览。

## 3. DrawerDetail 与专属抽屉

基础设施：右侧唯一 DrawerDetail，列表行/状态列可打开；URL 使用 `drawer=server|network&device_id=...&tab=...`，含返回路径。关闭恢复列表筛选/滚动；上下设备及左右方向键跳转同一授权结果集，不跨项目绕过权限。Tab 内容首次激活才查询，缓存 key 绑定 project/asset/tab/permission revision；保留每个 Tab 滚动位置。关联设备、Issue、Execution 使用路由与返回路径，不叠加详情抽屉；危险确认可以使用非详情确认对话框。

ServerDetailDrawer 固定头部 hostname/SN/型号/项目/机柜/U、交付/维护/Agent、ServerLampPanel 和权限控制的电源/UID。五 Tab：资产台账（CPU/GPU/NIC/内存/磁盘/BIOS/BMC）、映射详情（GPU-NIC/PCIe/NUMA/LLDP）、网口面板（SVG端口/链路/MAC/对端/DOM）、PCIe拓扑（设备树/BDF/NVLink/NUMA/TopologyDiff）、监控详情（D1–D6及1h/24h/7d、来源/告警）。

NetworkDeviceDrawer 固定八 Tab：基础信息；配置信息（planned/approved baseline/intended/observed/diff）；配置脚本；配置事件；运行状态；无损网络（PFC/ECN/CNP/队列/缓冲/LL-01～07）；光模块（Rx/Tx/温度/电压/偏置/趋势）；日志。LLDP/FDB/ARP 等既有运维能力保留为相关 Tab 的子面板，不删减。

每个动态字段提供 source、observed_at、freshness、support；同时保留 quality 和 policy_allowed，null/0/unsupported/unknown/stale/degraded 不混淆。maintained 始终 IPMI-only，禁止 SSH/Agent/Redfish 及代理绕过；agentless、BMC失联与历史数据明确显示。电源/UID等危险操作须权限、确认、审批/租约、审计与回读；无可用通道时禁用并说明原因。

## 4. 数据加载和事件协议

SSE：Execution状态/日志、端口变化、告警；带 project 或授权项目集合、Last-Event-ID，断线指数退避重连、增量补齐并按 event_id 去重。服务器在连接、事件投递和重连时重新验证权限与项目；拒绝未授权的项目或事件。模拟环境用固定事件序列和可控断线，不建立真实设备连接。

Poll 5s：Fleet、工单池、PXE/测试批次、清理任务。Poll 60s：Dashboard KPI、项目时间线、CMDB聚合、Fabric健康摘要。On-demand：详情各Tab、NCCL报告、PCIe拓扑、规则编辑、BOM导入、配置审核。页面离开或项目改变时清理对应定时器、订阅和请求；新页面先登记作用域、权限、协议、缓存、取消与状态，再实现。

## 5. 页面职责与双向链路

观测：Dashboard/Fleet/Monitor/CMDB；处置：Issues/Flows/授权操作；验证：Validation/NetTest/NCCL/Golden。端口异常 → 事件 → 工单 → 处置 → 定向重验证 → 关闭，反向返回保留 project/asset/event/issue/execution/evidence IDs、筛选、滚动与权限范围。观测不绕过审批，验证不得把缺失证据判 PASS。

## 6. 差距与实施顺序

当前 Mock 只有 `proj-b300-demo`，无多授权项目、统一 ProjectContext、双平面路由、项目作用域 API/Query/SSE、两个规定抽屉和完整运维 Tab 数据。不得通过复制资产或硬编码计数冒充跨项目验收。现有平铺路由、同步 `list()` 与全局 store、单抽屉 AssetDetail 和通用 TabSet 均须按本规范重构并做兼容迁移；现有功能不能被仅重定向取代。

1. 冻结 project/session permissions、授权项目与跨项目列表、抽屉 Tab 和事件流专用 OpenAPI 契约；补合成多项目/权限/事件 Mock fixture 和契约测试。
2. 引入 TanStack Query、ProjectContext、双平面路由及清理策略；权限服务层和批量快照先行，旧链接做可追溯兼容映射。
3. 实现共享 DrawerDetail 与服务器五 Tab、网络八 Tab；按需查询、URL/键盘/滚动/权限测试。
4. 逐域迁移导航与功能，补异常→工单→处置→定向验证→关闭双向链；实施 SSE/poll/on-demand 注册和状态组件、light/dark/响应式。
5. 使用两个以上授权项目、未授权项目、并发标签、延迟响应/重连和权限撤销做浏览器与契约验收；用户确认前不接 Go。

本次不声明上述代码、项目创建、真实多项目数据、真实 SSE 或设备后端已实现。
