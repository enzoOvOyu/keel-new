# 数据中心拓扑与机柜工作台（2026-10-31）

## 入口与事实边界

- 运维中心 `/topology`，内部为全局拓扑、网络架构、机柜组、物理布线、链路追踪和数据质量。
- 原 Fabric Topology 的 Mock/逻辑视图保留，并增加真实 CMDB 数据中心拓扑跳转，避免把旧演示当作真实接口。
- 保留 `/racks`、`/racks/:rackId`，默认接真实 CMDB 投影；旧 Mock 编辑器可用 `?legacy=1` 访问，未删除其代码和发布/版本能力。
- 全局首层仅显示现有 data_center/region/pod 的 parent 层级，不叠加全部线缆；点击进入机柜组再下钻 rack，避免一屏混合层级和全部布线；机柜节点进入详情，设备以稳定 ID 定位详情及其机柜。URL 保存 focus/selected/cable/port/face/rack_tab，不以名称匹配。
- 查询数据库不等于真机采集：当前主要是 CMDB 内的 synthetic 数据，界面显示标识。未知、陈旧和未采集不判 PASS。

## 数据与 API

`GET /api/v1/cmdb/visualization?project_id=&rack_id=&focus=`：沿用 cmdb:read、项目和资源级授权、5 秒查询上下文。复用 cmdb_entity/cmdb_relationship/cmdb_asset_scope，不建新资产或 Cable 表。只读 allowlist 字段，不输出原始 payload/秘密。

默认仅返回设备/位置摘要及线缆端点到设备的投影，机柜或局部请求才返回组件和端口。focus 最大两跳物理关联，不能当成真实流量路径或故障传播结论。单请求最多 20,000 个源对象，截断明确显示。前端按需查询、60 秒缓存和权限版本 Query Key；没有额外轮询。

物理关系仅取 cable_endpoint_a/b、contains、located_in；逻辑 membership 不转换为线缆。规划与记录事实用虚线，缺少 LLDP/接口证据不判 healthy。数据质量检查缺失位置、无效端点、端口重复占用，以及已知 U 位的缺失/越界/重叠。面板、电源、尺寸兼容、链路速率归一化等完整校验仍有缺口，不能宣称全部规则已完成。

## 两套引擎

- 拓扑：@antv/g6 5.x，按安装的 Graph 类型校验构造、render、destroy、setElementState API；ELK layered/ORTHOGONAL 在独立 Vite Web Worker 中布局。节点选择不重算布局；旧 Worker 随视图卸载取消。组件懒加载，避免 G6 进入其他页面首屏。
- 机柜：React 原生 SVG。U1 在底部，`y = top + (rackHeight-start-height+1)*unit`。无确认 U 位/高度不排列，正反面共用设备身份和位置。服务器、交换机颜色/轮廓区分，后面板仅显示已登记端口；高密度端口画布限制显示数量，右侧保留完整清单。
- 选中设备/端口才显示相关本柜线缆（上限24条）；跨柜关系在详情和拓扑追溯，不绘制虚构中间段。
- 键盘上下键、指针拖动与 U 吸附、落点预览、越界/重叠拦截、撤销/重做、缩放、SVG/PNG 导出；PDF 为浏览器打印到 PDF，未实现独立 PDF 排版器。

参考研究：[G6 Quick Start](https://g6.antv.antgroup.com/en/manual/getting-started/quick-start)、[ELK.js](https://github.com/kieler/elkjs)、[Rackula collision](https://github.com/RackulaLives/Rackula/blob/main/src/lib/utils/collision.ts)、[Rackula placement](https://github.com/RackulaLives/Rackula/blob/main/src/lib/stores/placement.svelte.ts)。Rackula MIT 许可已检查，仅借鉴 U 区间、安装面、键盘/指针交互，未复制 Svelte 代码。Keel 当前碰撞采用保守 U 区间，不声称实现 Rackula 的全部容器/半宽/全深度模型。

## 布局与实际位置

- 展示布局保存在独立浏览器 localStorage，附设备 revision 指纹；事实更新使旧草稿不再应用。
- “申请修改实际位置”创建既有 CMDB ChangeSet，新增专用 `asset.relocate` 操作，只接受 u_start，不能绕过普通只读字段限制。
- 发布仍遵守原申请/审核/发布权限与职责分离；发布时锁定机柜，校验版本、确认的机柜高度、整数 U 位、范围及同柜已知占位。更新 u_start 并同步已有 rack_u 兼容字段，走原 Revision/Audit/Outbox。没有普通拖动直接更新事实的接口。
- 尚未实际执行正式位置变更发布验收；多设备互换位置可能被逐条碰撞校验拒绝。缺少深度/安装面证据时不能宣称空间兼容已验证。

## 实际范围与验收记录

数据库只读盘点（含早前测试资产，不删除）：16 racks、66 servers、26 switches、512 GPU、640 NIC、1668 ports、706 cables。基础演示项目查询返回705条线缆，全授权查询706；这是现存数据，不通过前端伪造计数。

初次查询实测：全授权176ms/318259 bytes，基础演示项目185ms，单柜149ms（随后缩小了远端组件展开，未重复报更优值）。机器负载会改变延迟，不是SLA。

浏览器真实验证：全局机柜导航16条、G6 canvas/Worker布局；单柜正反面、设备选中、线序Tab和URL上下文，无JavaScript异常。截图在旧机器 `/tmp/infra-topology.png`、`/tmp/infra-rack.png`；不作为随Git迁移的唯一证据。Go编译、前端构建执行记录见交付；不因编译通过冒充业务验收。

`dev/scripts/infrastructure-perf.mjs` 生成独立 synthetic 100/300、500/2000、1000/5000、5000/20000、10000/50000 的输入规模，聚合到100节点/300边后运行同一ELK算法，结果在 INFRASTRUCTURE-PERF.json。只测聚合与布局，不测原始万节点全图渲染FPS，也不构成千卡生产能力证明。未写入正式CMDB。

## 仍需完成/验证

- 已提供设备/端口起止 ID 的物理设备邻接最短路径及双端分段表。真实多段配线架内部跳接、实际转发/故障传播仍缺证据，不能把设备邻接路径称为已证实流量路径；局部展开仍为两跳。
- 机房、Pod的复杂层级筛选、完整端口约束ELK布局、大规模查询分页仍需完善；画布默认200节点，可按200步长渐进到1000，边上限3000且显示截断提醒，不能冒充完整万节点可视化。
- 面板实际正反面归属、安装深度/宽度、PDU供电连接、面板照片无可靠数据，当前不推断；通用矢量示意不代表真实产品照片。
- 42U/45U/48U边界、重叠、端点路径和缺路径纯函数检查通过，45U/48U尚未完成对应浏览器验收；拖动保存、真实ChangeSet审核发布、跨页面全部七条联动仍需定向验证。
- 未对生产设备、生产千卡数据、GPU集群真实流量、全部LLDP/DDM观测做验收。

定向 Go 检查 TestVisualizationAndRelocationBoundaries 已通过（回滚事务）：无授权项目无资产输出、单柜线缆端点可见、原 U 位允许、零值/越界/已有占位拒绝；未创建或发布正式位置变更。

末次浏览器补充：键盘 U 位调整及展示草稿保存成功；从真实 API 读取 cable-srv-0001-01 的双端 ID 后，链路追踪分段表正确显示 srv-0001-nic-01-p1 到 sw-leaf-0001-port-01，仍标记 planned。未写入 CMDB。

质量规则补充：已按明确 Mbps/Gbps/Tbps 单位归一化比较双端登记速率；没有单位时不比较。缺少端口归属、物理关联和精确正反面板分别报告数据缺口，不自动创建硬件故障工单。宽度/深度双方有值时检查超规格，缺测不猜测。

跨页项目上下文：机柜导航和拓扑机柜节点的 URL 带 project_id，恢复到同一个 ProjectContext 过滤；默认跨项目过滤仍可清除。

实际视觉检查曾发现 G6 空 controlPoints 渲染中断及 Canvas 不解析 CSS 色值变量，已修复为显式空数组/可识别色值，并用无错误画布复查。全局页与布线页分开呈现，减少首屏连线拥挤。
