# 数据中心拓扑与机柜 · 当前实现定版

当前采用批准 V3 原型的五工作区，已接真实 CMDB 并上线。主环境 `686e9ea` 包含查询和端口上下文修复；独立5181原型仅供设计对照。

## 页面与实际交互

| 工作区 | 路由 | 已实现内容 |
|---|---|---|
| 全局拓扑 | `/topology?view=global` | Cluster Map/Hybrid、分组下钻、机柜×Leaf和服务器×交换机矩阵、授权范围搜索/统计/分页 |
| 网络架构 | `/topology?view=network` | Fabric Flow/分层束/Bipartite、交换机目录，按明确成员过滤，点束加载真实成员 |
| 物理布线 | `/topology?view=cabling` | Connection Matrix/BioFabric/Port Panel/Cable Inventory，端口及双端上下文、缺端/歧义、分页 |
| 链路追踪 | `/topology?view=trace` | 端口—Cable—端口 Metro/Tree/Table，候选分支选择及证据；无记录/不完整/歧义分别显示 |
| 机柜 | `/racks`、`/racks/:rackId` | Gallery、3D空间、Spatial/Dual/Front/Rear/Section，设备/端口/线缆Mapping，草稿/撤销/恢复、SVG/PNG/打印PDF |

全局 Sidebar 为唯一模块导航；设备点击复用统一 Drawer。URL 保留稳定ID、范围、视图和相机；机柜 `rack_tab` 与设备 `tab` 分离。未授权项目保持拒绝，不切回默认项目。

## 事实边界

- 物理连接只来自登记 Cable/端口；逻辑 Fabric/Plane/成员须有明确 NetworkPlan 或成员事实。禁止从画布位置、名称或线缆推断 VLAN/VRF/VNI/BGP。
- 计划、实际、LLDP/运行观测分层。路径完整仅表示登记物理段可追溯，不是连通性/健康 PASS；无 FIB/VRF 证据不能跨交换机拼逻辑路径。
- 复用 CMDB 稳定身份、关系、ChangeSet；展示布局/3D代理外形不是施工位置，不写资产事实。实际落位变更冻结项目/目标/版本并走既有审批。
- 半U/0U、安装面、未知高度/容量保留；缺几何不能显示完整空闲U、确切碰撞或完整空柜。GPU登记实体与服务器声明GPU数量口径分开。
- 热力、功率、DDM、精确面板、计划/实装比较等依赖数据缺失时不可用，不能用原型常数补齐。

## 查询与实现

[可视化 API](../api/visualization.md)为当前详细契约；五接口共享 CMDB 授权和 snapshot/cursor/member 引用。默认先取局部范围和矩阵，选中后加载成员，不默认取 full5000。

Query Key 包含用户/权限修订/项目/过滤/快照；切换取消旧请求。409清旧分页/成员引用，显式重载；达到上限保留 partial。physical depth2 的明确设备 root 保留本设备完整端口，不向远端兄弟端口扩散。

代码：[工作区](../../dev/src/features/infrastructure/approved)、[API Adapter](../../dev/src/lib/api/infrastructure.ts)、[查询](../../dev/backend/internal/cmdb/visual_queries.go)、[几何质量](../../dev/backend/internal/cmdb/visual_placement.go)。

批准设计的完整视觉细节按需查：[拓扑V3](../archive/2026-11-03/topology.md)、[机柜设计](../archive/2026-11-03/rack-visualization.md)。其中当时的“未部署”不代表当前状态。原型和截图不迁移、不作为线上能力依据。

## 验证与缺口

[原上线检查](../topology/deployment-validation.json)覆盖当时四接口启动/五页面加载；[最新发布记录](../kickoff/DEPLOYMENT.json)覆盖当前三个GET及授权拓扑页面；[性能结果](../kickoff/PERFORMANCE-RESULT.json)含完整上下文与单次样本。

未完成现场全量实装/观测数据验收、全部交互和生产并发/FPS基准。最新connections单次2.453s，不能宣称全部查询2s目标达成。3D、未知几何或没加载到的数据不构成完整物理验收。
