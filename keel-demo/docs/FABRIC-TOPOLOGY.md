# Fabric Topology 物理布线与逻辑网络中心

## 范围与导航

独立 Fabric 子入口 `Topology`，路由 `/p/:projectId/deploy/fabric/topology`；内部 Physical Cabling / Logical Topology 两视图，用 `view=physical|logical` 持久化。严格项目作用域，权限fabric:read；查询key绑定project_id、身份、筛选、视图、分页与快照。当前只Mock，不接真实设备/Go。

## 数据所有权与模型

Physical节点引用现有Device稳定ID，边引用Cable稳定ID和A/B PhysicalPort；资产名称、位置、速率和介质来自BOM/CMDB。两端端口缺失的线缆返回数据质量问题，不能补造设备。

Logical节点独立表达Fabric/Pod、逻辑网络分区、设备角色；membership/BGP/containment是逻辑关系，不是Cable。`topology_networks`为明确的合成已批准演示规划，`topology_memberships`引用Device、PhysicalPort、批准基线、Rail/VLAN/VRF/VNI，`topology_observed_networks`为独立合成配置快照，`topology_bgp_sessions`为独立合成路由观测；均附来源/时间/版本。它们不创建Machine/Switch身份，不从物理线缆推导VLAN/VRF/VNI/BGP。

原始包未提供独立VNI规划和可绑定CMDB对端的BGP样本，新记录明确标记 `explicit_synthetic_topology_v1`。原始BGP/FDB/ARP/LLDP仍保留可查，不把规划当实测；无对应证据保持unknown。计划值、observed实际配置、接口/DDM观测分列。

## API 对照

| API | 数据与行为 |
|---|---|
| GET /projects/{project_id}/fabric/topology/physical | scope global/group/local，site/pod/rack/fabric/rail/role/health/q/device_id/network_id，cursor/limit，at历史 |
| GET /projects/{project_id}/fabric/topology/logical | 独立逻辑节点/边，planned/observed/compare模式，沿用筛选与上下文 |
| GET /projects/{project_id}/fabric/topology/details/{kind}/{id} | device/port/cable/network/edge，双端、diff、关联网络/线缆、LLDP/FDB/ARP、DDM、PFC/ECN/BGP、告警/Issues/Execution/事件 |
| GET /projects/{project_id}/fabric/topology/filters | 合法筛选项、演示快照时间、能力状态 |

On-demand可取消查询；默认分组/局部图，最多120节点，cursor渐进扩展。默认不轮询全量图；手动刷新当前视图。后续端口/告警SSE仅失效相关详情；Last-Event-ID沿用统一事件协议。无历史快照时返回明确unknown/空，不回退当前数据假装历史。

## 健康

- healthy：必须BOM两端完整、双侧LLDP一致且新鲜、接口up、速率与规划一致、DDM支持且无阈值异常。
- warning：明确速率降级、DDM告警或性能降级。
- critical：明确接口down、LLDP冲突或配置冲突。
- unknown：缺失/陈旧/不支持或无法关联；不能因其他证据正常盖过缺证据。
- planned：仅有规划，尚无观测。

逻辑健康使用批准基线、独立实际配置、BGP观测，不复用物理健康结论。绘图实线为观测关系，虚线为规划，Diff可查看字段级差异。

## 交互

缩放/平移/搜索/自动布局、角色/机房/Pod/机柜/Fabric/Rail/网络用途/异常筛选。全局分组下钻、局部邻居展开和渐进载入。线缆点击高亮两端设备/端口和向Spine关联路径，无关对象降亮。右侧显示双端、来源、实际状态、DDM、历史和工单；点击端口内联详情，点击设备仅打开统一设备抽屉。

物理→逻辑使用显式membership关联；逻辑→物理使用membership引用端口/Cable；切换保留筛选和选中设备，无关联则unknown。逻辑网络详情可定位异常设备和物理端口。全平台使用同一Issues/Execution ID；观察页面不发起未经审批的生产变更。

## 验收

拓扑两种模型独立、稳定ID、双端可追溯、线缆高亮、逻辑层级/规划实际差异、双向定位、设备抽屉、渐进加载、历史缺失、权限/unknown/stale/planned异常分支。按任务启动要求已实际执行构建与最小浏览器检查，结果见下节；完整业务/历史/权限集中验收仍NOT RUN，代码/契约生成不等于完整通过验收。


### 实际最小可用性检查

2026-10-30：生产构建首次通过；Chromium实际打开机柜与Topology，验证单柜/详情宽度、设备选择、环境懒加载/7d范围、精确线缆双端高亮、物理→逻辑与配置Diff，page_errors为空。详见TOPOLOGY-RACK-SMOKE.json。完整业务/权限/历史/导出回归仍NOT RUN，等待用户安排；不把本次最小检查等同全量验收。
