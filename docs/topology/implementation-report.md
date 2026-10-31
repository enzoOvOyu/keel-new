# 数据中心可视化 · 正式实施记录

更新：2026-10-31。状态：开发中，未完成交付。工作分支 `feat/datacenter-visualization-approved`，独立 worktree `/home/ubuntu/keel-visualization-dev`；未经用户要求不合并 main、不发布到当前测试环境。批准依据：`docs/topology.md` V3、`docs/rack-visualization/room-validation.md` 及其机柜原型引用。前端为 `dev/`，服务 5180；5181 独立设计原型保留。本报告不将原型历史验证算作正式实现验证。

## 本轮范围与入口

| 正式页面 | 设计表达与接入 |
|---|---|
| `/topology?view=global` | Cluster Map / Hybrid、Pod 下钻、局部连接矩阵、对象搜索与稳定 ID 定位 |
| `/topology?view=network` | Fabric Flow、按事实层/介质/速率拆分语义束、成员展开、Bipartite、交换机目录；Fabric 成员仅依据明确关系 |
| `/topology?view=cabling` | Connection Matrix、BioFabric、真实登记 Port Panel、Cable Inventory；缺端保留、不推测空闲端口 |
| `/topology?view=trace` | 端口—Cable—端口 Metro Route、关联支路 Tree、逐项证据 Table；无转发表证据的 Logical Trace 不支持 |
| `/racks`、`/racks/:rackId` | Gallery、Three.js 机房空间、Spatial/Dual/Front/Rear/Section、Mapping、展示草稿与既有 CMDB 位置变更申请 |

Sidebar 沿用现有唯一入口，内容区没有重复模块切换导航。服务器五 Tab 和交换机八 Tab 使用已有 DrawerDetail。筛选、选中、端口、线缆、机柜、相机写入 URL；跨页面保留站内返回路径。

## 代码与复用

- `dev/src/features/infrastructure/approved/`：按批准原型移植的五个独立工作区、范围 Context、共享对象详情；没有运行时 fixture 导入。
- `approved/pages/`：四拓扑工作台及机柜入口；`approved/rack/`：共享几何、SVG 双面、空间视图、端口、Mapping 与按需 Three.js 场景。
- `dev/src/app/main.tsx`：原 `/topology`、`/racks` 路由挂接；原业务导航和 ProjectContext 保留。
- `dev/src/lib/api/infrastructure.ts`：统一真实查询接口。
- `dev/backend/internal/cmdb/visualization.go`、`cmd/api/cmdb.go`：复用 CMDB 查询投影、授权与类型化关系，没有迁移或平行资产表。
- `docs/openapi.yaml`、生成 TypeScript、API-CONVENTIONS、DOMAIN-API-CONTRACTS：同步查询参数、响应字段与完整度。
- `dev/package.json`：加入原型已经使用的 Three.js 0.187.0（MIT），只按需加载；G6/ELK 已有版本不变。

复用 `cmdb_entity(item_uid)`、`cmdb_asset_scope`、`cmdb_relationship` 的 parent/contains/located_in/cable_endpoint_a/b/logical_membership、Schema、source_ref、资产版本与 ChangeSet。机柜、设备、端口和 Cable 同一稳定 ID；没有名称模糊匹配。

## API 与事实边界

没有另建平行 `/topology` 服务。扩展现有 `GET /api/v1/cmdb/visualization`：

- `detail=full` 返回授权资产（含组件、端口、Fabric）、Cable、类型化关系、质量问题。
- `project_id`、`rack_id`、`focus`、`limit`，源资产上限 5000，关系上限 20000；截断明确 partial。
- `snapshot_id` 为当前授权投影的版本指纹，`read_at` 是查询时间，**不是采集时间或可重放历史快照**。
- `coverage` 与 `truncated`、实体/关系来源、fact_layer 用于防止不完整结果冒充完整。
- 保留原摘要模式，保持统一错误结构与服务端项目/资源授权。
- 规划、观测关系保留不同来源；同层冲突不按任意返回顺序覆盖。计数可从完整、明确 contains 关系确定性派生，保留派生依据。

修正旧 `physicalPath` 的设备邻接 BFS：没有已确认 patch-through 或转发证据时，只能证明 Cable 双端，不能跨交换机拼一条假物理路径。

## 布局与真实资产分离

机房 3D 代理宽深及按 Pod 的排列是示意坐标，不是测绘图；U 高与位置使用实际查询字段。未知安装面、深度、面板、功耗、温度保持未知。没有已确认规划/实装配对快照，完整 Planned vs As-built 继续隐藏。

拖动、撤销、保存只改展示草稿；草稿按用户/项目/机柜隔离并校验资产版本。实际安装位置通过既有 CMDB ChangeSet 创建草稿，仍需原审核/发布流程。设备维护态约束及审批不变。

## 验证与性能

本轮仅必要契约、构建和只读关键交互检查，不进行真实数据导入、不提交测试资产、不连接设备。TypeScript 与生产构建已通过；四拓扑及双面/Mapping/PNG 的只读关键交互已执行。连续切换至 3D 模式出现等待超时，但直接打开 3D 入口可渲染，仍需在独立环境修复并完成验证。未执行的能力不得视为通过。

- [API 查询记录](api-validation.json)：当前本地库 4703 资产、705 Cable、6356 类型化关系，约 4.294 MB；三次 Domain Service 查询约 655–829ms（来源元数据扩展前记录，最终响应大小以后续记录为准）。它不是 HTTP 或浏览器 FPS。
- 数据来源包含 4697 条 synthetic_fixture，另有 BOM/人工数据；“真实 API”不代表真机采集。页面不以 fixture 填补缺测。
- 本轮不运行千卡/万节点生产规模压力测试，不宣称达到该规模性能。

## 实际限制与未完成能力

1. 完整游标子图、矩阵聚合后端、版本历史快照仍未实施。当前有限 full 投影明确标记不完整；大范围场景需后续服务端聚合，不能据此承诺千卡首屏。
2. 无已确认配线架内部贯通数据，物理追踪在直接物理段终止；无 FIB/VRF/同窗口转发证据，不提供实际 Logical Trace 或业务故障影响结论。
3. 测绘 XY、宽深、精确设备面板、实时遥测、PDU 端接未补造。完整规划/实装对比依赖真实快照，仍属于待接入能力。
4. 真实数据导入、端到端业务联调与生产规模性能由用户后续统一安排。
