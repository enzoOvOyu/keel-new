# Keel 数据中心可视化系统 · 产品与交互设计 V3

更新：2026-10-31。状态：**第一阶段设计评审；文档与可运行原型，未批准进入生产实施**。

本文件是四个拓扑模块的主设计。机柜后续独立设计见 [rack-visualization.md](rack-visualization.md)，其 Gallery/Spatial/Dual 原型已替换 5181/rack-detail，正式机柜页面保持不变。V3 依据用户最新纠偏，**只重做全局拓扑、网络架构、物理布线和链路追踪的展示方案与隔离原型**；当时机柜现状冻结。后续用户已单独授权机柜设计，边界见上方链接。保留既有 CMDB 身份、事实/观测分层及接口边界。研究方法与访问限制见 §14；本文明确区分论文结论、Keel 设计推断和已运行原型，不把科研算法名称当成实现证明。本阶段不修改正式页面、数据库、业务 API 或根设计令牌。
## 1. 产品定位与范围

面向网络工程师、交付工程师和运维人员，回答“在哪、怎样组织、如何端接、凭什么确认”。CMDB 提供资产与持久关系事实；BOM 提供规划来源；配置、观测和验收证据保持领域所有权。可视化是授权查询投影，不是第二资产库。

| 工作台 | 核心问题 | 主体表达 | 不承担 |
|---|---|---|---|
| 全局拓扑 | 设备在哪个机房/Pod/机柜？ | Cluster Map / Hybrid Graph、组内矩阵 | 端口全图、无坐标的伪机房平面图 |
| 网络架构 | 实际有哪些 Fabric 和角色互联？ | Fabric Flow / 语义束线、Bipartite | 根据设备名猜 VLAN 或 IB |
| 物理布线 | 两端如何端接？ | Connection Matrix 优先，BioFabric / Panel / Inventory | LLDP/FDB 当线缆 |
| 链路追踪 | 从这个端口能证实到哪里？ | Metro Route + Tree/Table | 设备 BFS 伪装转发路径 |
| 机柜可视化 | 后续独立设计 [rack-visualization.md](rack-visualization.md) | Gallery / Spatial / Dual，与拓扑按稳定 ID 联动 | 不修改正式机柜与 CMDB |

六阶段交付、项目授权、既有五/八 Tab 设备抽屉不改变。maintained 遵守当前 IPMI-only 限制；这些观察页面不提供设备执行或审批绕过。

## 2. 实际审计与差距

以下保留 V2（2026-10-31）的实际审计快照，描述的是正式系统，不是 V3 原型能力；本轮未重复数据库审计。审计基于代码、现有 OpenAPI、迁移及本地 PostgreSQL **只读聚合**，不是根据历史聊天推断。状态含义：**已确认**=代码/模型存在；**不完整**=存在但不满足目标；**不支持**=当前路径无实现；**需要扩展**=后续最小兼容增量。数据库中的演示记录不是真机证据。

| 对象 | 状态 | 实际证据与差距 |
|---|---|---|
| 四模块入口 | 已确认 | `dev/src/app/SidebarNavigation.tsx`：`/topology?view=global/network/cabling/trace`；机柜为同组 `/racks`；不再重复一级入口 |
| 全局拓扑 | 不完整 | `InfrastructureWorkspace.tsx` 的 graph useMemo 区分 global，但只按 data_center/region/pod 生成层级；机柜另下钻，缺独立范围统计/定位工作台 |
| 网络架构 | 不完整 | 复用同一 graph 的 devices/cables；无 Fabric 专用视图模型、连接矩阵、逻辑证据层 |
| 物理布线 | 不完整 | cabling 分支为 Cable 表；缺独立 Map/Matrix/Port Panel 组合 |
| 链路追踪 | 不完整 | `lib/api/infrastructure.ts:physicalPath` 折算为设备邻接 BFS；约束首尾端口但不验证交换机内穿越/配线架贯通；不能复用为真正物理追踪算法 |
| 机柜详情 | 已确认/不完整 | `RackElevation.tsx` 有 SVG、前后、U 位、拖动；workspace 有七个对象 Tab、草稿/ChangeSet、导出、设备抽屉和 URL。设备面板坐标仍是示意；精确面板未知时必须标注 |
| G6/ELK | 已确认 | 已安装 G6 **5.1.1**、ELK **0.12.1**；TopologyCanvas + layout.worker；现有 worker 无显式端口约束，图数据变化会重新布局 |
| 原型设施 | 不支持 | 未发现 Storybook、stories 或专用原型构建入口；已有 Vite 足够，新增 docs 下独立 root/config，使用 dev 的依赖，无生产路由注入 |
| UI 复用 | 已确认 | shadcn Button/Input/Dialog/Tabs/Table/Sheet；DrawerDetail、服务器五 Tab/交换机八 Tab；已有 SVG 可只读导入到原型 |
| 权威对象 | 已确认 | `cmdb_entity(item_uid)`、`cmdb_asset_scope`、版本 Schema、field_source、revision、changeset、external/config reference 已实现；不新建 Machine/Switch 身份 |
| 类型化关系 | 已确认 | `cmdb_relationship` + definition_id/fact_layer/source_ref/valid_to；contains、parent、located_in、cable_endpoint_a/b、logical_membership 等 |
| 可视化查询 | 不完整 | `GET /api/v1/cmdb/visualization` 仅五种关系白名单；服务器端最多扫描 20,000 对象，前端截节点；不是游标子图服务 |
| Fabric | 需要扩展 | DB 有 fabric/network_segment 和 logical_membership；visualization 资产白名单不含两者，字段未输出 fabric_id。不可声称已有完整跨平面查询 |
| 配线架/PDU | 不完整 | Schema extension 有 patch_panel/pdu；当前 DB 统计为 0；无已证实内部贯通/供电关系；不能生成示例后当作正式能力 |
| Logical Trace | 不支持 | 没有带 VRF/方向/时间一致性的 FIB/转发表追踪服务；BGP 邻居不等于转发路径 |
| 专业端口面板 | 需要扩展 | 端口身份/名称可用，精确型号面板几何通常缺失；先编号示意，不能按名称推断端口数量 |

### 2.1 V2 只读数据库快照（历史审计，非本轮重新验证）

实际表内：16 Rack、66 Server、26 Switch、512 GPU、640 NIC、1,668 Port、706 Cable；包含历史测试资产，不应删除来凑 64 台。data_center=1、region=4、pod=4、fabric=1、network_segment=40。

有效关系：cable_endpoint_a=706、cable_endpoint_b=706、contains=2,822、located_in=88、logical_membership=992、optic_on_port=1,024、parent=24。**两端关系行数齐全不等于端点语义与实物核验通过**。本阶段未跑全库质量/生产性能测试。

原型使用仓库 `fixtures/demo-full-64-b300/demo-seed.json` 的独立投影（64 Server、24 Switch、16 Rack、1,664 Port、704 Cable），不读取上述数据库。仅三条线缆证据叠加为陈旧/冲突/缺端反例；提取脚本记录在原型目录。原型显示 synthetic，不能充当当前数据库验收。

## 3. 信息架构、导航与上下文

```text
运维中心 / 数据中心拓扑
  全局拓扑      /topology?view=global
  网络架构      /topology?view=network
  机柜管理      /racks → /racks/:rackId
  物理布线      /topology?view=cabling
  链路追踪      /topology?view=trace
  数据质量      沿用已有入口，问题可就地展开
```

模块切换仅在 Sidebar；内容区不再复制这组 Tabs。内容 Tab 仅用于模块内部视图（如物理布线四种表达）。通用“机柜管理”仅 Sidebar 存在；设备/线缆详情中的按钮直接定位具体机柜/U/端口，无位置不显示假跳转。

- ProjectSwitcher 沿用唯一 ProjectContext；运维项目过滤可清空；授权过滤由后端复验。清空不清除其他项目数据。
- 正式 URL 沿用 project_id、selected、rack_tab、port、cable、face、focus；新增局部 mode、fabric_id、group、from、to、evidence_at、q。未识别 ID 明确提示，不通过名称近似匹配。
- 跳转携带稳定 ID 与 return_to（仅站内）；刷新可恢复视图/筛选/选中，返回恢复原位置。
- 右侧对象面板只有一个；打开统一设备抽屉时不再嵌套第二详情抽屉。Escape 关闭，焦点回到触发对象。
- 不在顶端展示 REAL API、on-demand、跨项目运维等实现说明。只保留一行标题/范围/搜索/主要动作；数据来源进入对象详情。

## 4. 四个模块的页面设计（V3 权威方向）

共同框架是“范围 → 结构 → 精确对象 → 证据”，不是四个同构 Dashboard。页首只有标题和一行工作工具，正文从约 170–210px 开始。默认用满主体宽度；选对象才打开 290px 证据栏。摘要放图内或行末，不建立一排重复 KPI 卡片。单个对象可在位置层、网络层重复出现，但始终引用同一资产 ID，跨层数量不能相加。

| 用户任务 | 首选表达 | 不适合直接采用的表达 | 补充表达 |
|---|---|---|---|
| 看集群边界、位置组织、局部密度 | Cluster Map + Hybrid Graph | 全量端口节点图、无坐标的伪平面地图 | Pod 内小矩阵、对象列表 |
| 看 Spine/Leaf 与计算组互联 | Fabric Flow + 语义束线 | 把带宽未知的线画成流量 Sankey | 选中束的 Bipartite 双端对照 |
| 核查谁接谁、端口是否冲突 | Connection Matrix | 依赖交叉曲线逐根找端点 | BioFabric、Port Panel、Inventory |
| 跟踪已证实路径、定位断点 | Metro Route | 把任意设备邻接 BFS 当业务路径 | 分支树 + 证据表 |

### 4.1 全局拓扑：Cluster Map / Hybrid Graph

**目标**：快速理解数据中心的组成，定位设备所在范围，发现局部连接结构与证据缺口。主画布按 CMDB Pod 分组，展示登记服务器小标记与 Rack×Leaf 局部矩阵；组外只画有成员引用的上联束。位置分组与网络关联同时可见，但位置并不由网络社区算法决定。

**页面结构**：位置面包屑 / 资产搜索 → Pod 聚合主画布 → 按需成员清单。没有选中对象时不占用右侧面板。Pod 内显示设备数量、机柜索引、服务器标记、机柜到 Leaf 的完整线缆条数；中央 Spine 骨干表示网络角色集合，不代表新的实物设备，也不是物理汇线节点。

**交互**：
- 搜索 Hostname/SN/稳定 ID，选结果进入所在 Pod 并打开对象证据。范围变化不改变资产身份。
- 点 Pod 展开为局部服务器×Leaf 矩阵和设备目录；点矩阵单元格得到确切 Cable 集合；小图中点机柜行对应单元格得到该柜服务器与 Leaf 的集合。
- 点组外连接束列出其全部原始线缆，分页显示；点设备打开对象栏，再进入其端口或既有机柜入口。
- 返回全局保持确定性的 Pod 顺序与容器位置；URL 保存 scope、asset、row、col、bundle、q。高亮/筛选不重新任意打乱布局。

**异常**：只统计完整端接；异常成员不能被聚合平均成健康。缺端记录在物理布线的独立待核查入口保留。位置未知的生产数据必须进入“未分组”，不能猜 Pod。原型只有一份历史快照，不提供虚假时间播放或波动率。

**研究转化**：借鉴 Graphon 的跨层抽象问题意识与 DynTrix 的组外连线/组内矩阵组合。当前使用**确定性 CMDB 分组**；没有拟合 graphon、概率补边或实现 DynTrix 时间算法。对非专家保留设备目录作为矩阵的文字入口。

### 4.2 网络架构：Fabric Flow / Edge Bundling

**目标**：看清 Fabric 内角色之间的连接组织与冗余设计证据，而非对着全量端口曲线寻找关系。

**页面结构**：Fabric / Pod / 高亮搜索 → Fabric Flow、Bipartite、交换机目录 → 选中束成员。主图三列：Spine、Leaf、计算设备组。服务器按机柜位置聚合，机柜图形只是包含这些服务器的视图容器。角色顺序由已登记数据确定，不按名称推断。

**束线设计**：先形成角色/设备组骨架，再为已知端接路由；同一设备对或 Leaf 到同一计算组的线缆才可聚合。当前原型为确定性语义聚合和 Bézier 通道，不是论文 Filter–Draw–Bundle 算法复现。每束保留成员 ID；线宽采用有上限的记录数编码，旁边有准确数量，不表示带宽、吞吐量或端口速率。不同事实层、Fabric、介质或不兼容证据在正式投影中不得混合。

**交互**：
- 点束线高亮相关端点，其他束降低亮度；“展开成员”显示最多 24 根可选择线，其余仍在分页成员表。
- “Bipartite”将该束转换为左右两列原始设备/端口配对，每页最多 12 条；点一条打开双端证据。它用于端点核查，不把 A/B 数据库顺序当业务流向。
- 点 Spine/Leaf 打开交换机信息与登记端口，完整设备页仍复用生产的唯一 NetworkDeviceDrawer。选计算组进入相应布线范围。
- 搜索只高亮相关关系，不因非匹配节点隐藏而断开实际记录。切 Pod 清理旧选中束。束选择与 Pod 写入 URL。

**Fabric 语义**：Ethernet 是技术族，RoCE 是其协议/业务属性，OOB/Storage 是用途。只显示有成员/基线证据的 Fabric；IB/NVLink 不根据设备名称自动出现。当前合成切片只演示一个明确的 RoCE Fabric；Spine/Leaf 图是规划物理互联，不声称已显示 VLAN/VRF/VNI/FIB。

**异常**：缺端记录不画成完整束，页脚显示可点击缺口；交叉和束的汇合处不形成图节点。没有批准冗余方案与故障域证据，不能从多根线判断高可用。

### 4.3 物理布线：Connection Matrix 优先

**目标**：让工程师能够精确核查连接，而不是欣赏一张网络画。默认矩阵，按需换成端口索引或原始记录，所有模式使用同一筛选后的 Cable 集合。

**页面结构**：Pod / 搜索 / 证据过滤 / 导出 → 表达切换 → 主视图 → 选中集合的端点表。矩阵范围明确显示“进入矩阵条数 / 范围总条数”，不能用省略其他连接制造完整假象。

| 表达 | 编码与操作 | 使用边界 |
|---|---|---|
| Connection Matrix（默认） | 行 Server、列 Switch；可切 Leaf×Spine。单元格为完整端接数，异常叠 `!`；点单元格下钻成员。行列固定表头，内部滚动，不缩小字体容纳全部端口 | `·` 表示没有记录，不代表已核实空闲/断线；不把矩阵当路径图 |
| BioFabric | 行是唯一 PhysicalPort，列是唯一 Cable；竖线两端方点才是端接。按设备/选中矩阵单元格加载，每组≤24列，可横向滚动、分页、点击列/端点 | 竖线穿过其他横行不表示连接；缺端仅画一个端点和问号。它是索引，不是路线 |
| Port Panel | 真实登记端口的紧凑编号面板，显示速率、记录数、证据状态；点端口显示关联 Cable | 未有几何数据时明确“编号示意”；没有线缆记录不等于端口空闲 |
| Cable Inventory | 搜索/排序/分页/导出；ID、A/B设备端口、介质、长度、证据 | 保存缺端、冲突及被矩阵排除的其他类型，不静默丢弃 |

**密度**：默认一个 Pod；生产端口层先限制设备/角色再加载。矩阵不得渲染全数据中心的 N×N 端口网格；稀疏物理连接采用二部矩阵（两类轴），不是强制对称邻接矩阵。BioFabric 的横线名固定可辨认，列编号与清单联动；完整 ID 在详情/清单，不挤在每个交叉点。

**例外与联动**：缺端托盘常驻，直接打开原记录；其他类型/缺端数量可进入 Inventory。选中矩阵 → BioFabric → 线缆证据 → Physical Trace，URL 的 row/col/asset/cable/port 保留稳定引用。右栏清晰区分规划物理记录、当前观测、验收结论和来源。

### 4.4 链路追踪：Metro Route + Tree/Table

**目标**：回答“从这个端口，证据确实允许追到哪里”，并快速识别中断原因。

**页面结构**：起始设备/端口、Physical/Logical、开始/清空 → 横向 Metro Route（≤240px 高）→ 下方左侧关联支路树、右侧证据表。选择站点才开右侧证据栏；不堆多张步骤卡。路线上的间距是阅读顺序，不表示距离/时延；箭头是阅读方向，不表示单向光纤或实际流量方向。

**路线语法**：端口站点 — Cable 站点 — 端口站点。相邻段只有在 CMDB 明确记录 patch-through 时才能连续；到交换机端口即停止。缺端用虚线终点/问号，保留 Cable ID，不制造未知设备资产。候选多条时显示分支选择，禁止静默取第一条。记录完整、证据验证、freshness 分开。

**Tree**：列出起始设备的其他独立端接，点击某个端口重新追踪该物理段；该树不宣称这些分支是当前路径的下一跳、备用链路或已证实故障影响。可折叠，避免无关分支挤压证据。

**Table**：逐项列出端点引用、介质/长度、登记来源与时间、当前观测缺口、验收结果、配置/转发及冗余限制。点击对象打开相同证据栏；“定位布线记录”回到同一 Cable，不按名称搜索另找一个对象。

**Logical Trace**：具备同一窗口/VRF/地址族/方向的 FIB/转发表证据后，才可用 Metro 多线路表达实际转发/冗余；缺证据时禁用查询并显示缺口。BGP、LLDP、FDB、ARP、DDM 都不能独自充当永久线缆或真实转发路径。本轮不伪造完整多跳或故障影响数量。

## 5. 机柜既有基线与后续设计入口

以下为 V2 正式系统基线，保留既有业务约束。后续用户授权的机柜产品设计、实际 CMDB 能力审计与 Gallery / Spatial / Dual 原型以 [rack-visualization.md](rack-visualization.md) 为唯一入口；下方旧线框不是新原型的布局要求。正式代码与四页拓扑不变，稳定 ID 跳转继续兼容。

复用 `/racks/:rackId` 与现有 SVG/领域校验，不新建机柜系统。头部一行显示位置面包屑、总/占用/空闲 U、设备数；操作为前后视图、编辑草稿、导出、定位相关拓扑（带当前柜 ID）。

```text
单柜标题、位置与容量                          刷新 / 导出
[机柜视图][设备][服务器][交换机端口][线序][电源空间][Mapping]
左侧 350–420px 精确立面 | 右侧主要信息区：当前设备、端口与证据
```

- U1 在底，位置 `y = top + (rackU - startU - height + 1) × unit`；占用按区间并集计数，不累加重叠；缺高度/位置设备放未分配栏，不猜测。
- 前/后共用同一身份与位置；设备类型图形不同，槽位/端口数仅画已登记数据。精确照片/型号几何未知时注明矢量示意。
- 设备 Tab 的适用字段：名称、类型、型号、SN/ID、U位/U高、面、管理/BMC地址、状态；Server 增加 CPU/GPU/内存/NIC/固件（缺失标未采集）；Switch 显示角色/登记端口/端接。
- 线序显示 Cable ID、双端设备/端口、介质/长度/速率/中间段及来源；点击即立面高亮两端，跨柜只在本柜画对应端并提供对端定位。
- Mapping **保留**：当前设备的身份、部件、端口、线缆、对端与 BOM 来源；不是 GPU-NIC 矩阵的替代物。
- PDU 只有存在实体/供电记录时显示容量/插口/冗余；当前无数据时显示未采集与缺口，不放假仪表。
- 拖动仅本地展示草稿，U吸附/冲突/越界即时显示，撤销恢复；保存展示草稿不得写位置事实。实际安装变更需现有 ChangeSet、权限、版本、审批与发布；本阶段原型不开放真实提交。
- SVG/PNG、打印 PDF 为表现导出，不生成新资产。新数据版本使旧草稿提示冲突，不静默覆盖。

## 6. 视觉规范与交互细则

继承 `DESIGN-SYSTEM.md` 现有 Inter/Segoe UI/Microsoft YaHei、13px 基础字、primary #6473df、foreground #28354a、border #e2e6ef、accent #eff2fd。设计优化仅在原型隔离样式中实现，不改全局 token。

| 元素 | 冻结设计 |
|---|---|
| 框架 | Sidebar 220–240px，顶栏52px；标题40px、工具栏40px；1440×900 主工作区从约170–210px开始，禁止大段简介 |
| 字体 | 标题20px/600，分区13px/600，正文12–13px，来源/ID11px等宽；信息层级靠字重/位置而非重复卡片 |
| 间距 | 4/8/12/16/24px；表格行36–40px；控件32px；圆角6px；仅浮层轻阴影 |
| 面板 | 默认无空详情栏，选中后290px，可关闭；1280px降为260px；窄屏转单详情层，不挤成不可读三栏 |
| 状态 | 正常/警告/故障/未知/维护分别有文字与图标；维护蓝色暂停；规划与健康不同维度，禁止以蓝色规划暗示正常 |
| 节点 | 全局容器内小矩阵，组外聚合；网络三列角色骨架；端口仅在Bipartite/BioFabric/Panel中展开；颜色不承担身份编码 |
| 连线 | 当前实线=已登记的规划物理端接，不代表实物验收；当前缺端虚线必须有断点标记。未来叠加approved设计用独立层图例，不能与断点混用；观测为旁证，冲突附文字/符号 |
| 标签 | 默认只容器/设备短名；端口与Cable ID仅选中时显示；长名详情完整展示；低缩放隐藏次级标签 |
| 操作 | 搜索Enter定位、Esc关闭面板、键盘按钮/列表等效画布操作、焦点可见；表格行点击与明确“打开”一致 |

解决毛线团：先选范围，再选层级，再按需端口；正式实施默认200可见节点/500边预算，超出聚合/目录，不通过压缩文字硬塞。选择线缆只突出相关双端。组件复用≠四页面复用同一个数据模型与布局。

## 7. 高保真原型入口与运行

独立目录：[topology/prototype/](topology/prototype/)。四页采用 React + SVG / HTML 矩阵，复用 dev 的 shadcn 依赖，不再让四页共用 `TopologyCanvas`。旧 graph.tsx 保留未引用，不作为新视图。机柜原型后续改用独立的共享几何、空间投影与双面 SVG，详见 rack-visualization.md；未改正式 RackElevation。本轮没有新增图形引擎或依赖。独立 CSS 以 `.viz-v3`/`v-*` 隔离，数据集中读取 synthetic fixture，不引入真实 IdentityProvider/cmdbFetch，也没有 `/api` 代理。
```bash
cd docs/topology/prototype
npm run dev     # 5181，复用 dev/node_modules；无需安装另一套依赖
npm run check   # 本原型类型检查
npm run build   # 独立 dist，不影响 dev/dist
```

| 原型 | 地址 | 主要验收点 |
|---|---|---|
| global-topology | http://localhost:5181/global-topology | Pod聚合/组内矩阵、搜索、局部下钻、束成员 |
| network-architecture | http://localhost:5181/network-architecture | Fabric Flow、束展开、Bipartite、交换机证据 |
| physical-cabling | http://localhost:5181/physical-cabling | 默认Connection Matrix、BioFabric、Panel/Inventory、缺端托盘 |
| link-tracing | http://localhost:5181/link-tracing | Metro Route、关联树、证据表、断点与逻辑能力边界 |
| rack-detail（独立后续设计） | http://localhost:5181/rack-detail | Gallery / Spatial / Dual / 能力驱动叠加 / Mapping；检查见 rack-visualization.md |

原型 URL 支持 rack/asset/cable/port、mode、scope/pod、row/col、bundle、order、state、q；这些是原型内 view-state，正式实施由 Adapter 映射到 §3 既有参数。按钮和筛选本地生效，标记 **SYNTHETIC · 设计原型**；保存仅 `keel.topology.prototype.*` 本地存储；没有 CMDB 写入操作。

## 8. CMDB 数据映射与最小扩展

| 设计对象/信息 | 现有权威实体/关系 | 边界与增量 |
|---|---|---|
| 数据中心/区域/Pod/机柜 | data_center/region/pod/rack + parent | room/rack-group 不重复建模；无数据跳层，展示分组不是实物 |
| 服务器/交换机/组件/端口 | server/switch/nic/gpu/port + contains | 稳定 item_uid；端口可能经 NIC 归属设备，需受控遍历 |
| 设备 U 位 | located_in + u_start/rack_u/u_height | 兼容旧字段；冲突按既有领域校验；展示坐标不覆盖它 |
| 物理线缆 | cable + cable_endpoint_a/b | 两端必须 port；recorded/planning 不等于验收通过 |
| 施工线序/中间段 | 现有 Cable 来源与实体引用 | 不能用普通 reference 自动推导贯通；未来新增受控 patch-through 定义前先确认实际数据 |
| Fabric/逻辑网络 | fabric/network_segment + logical_membership、配置引用 | 增量投影与基线版本；不可由线缆生成 VLAN/VRF/VNI/ASN |
| 光模块 | optic + optic_on_port | DDM 在观测域，仅存引用，不改物理端接 |
| 证据 | field_source/config_reference/external_reference | 规划/approved/intended/actual/operational/验收分层；原文仍在专用存储 |
| PDU/供电 | 扩展 Schema 的 pdu/psu | 当前无实物与供电边；不从电源数量推导冗余关系 |
| 变更 | revision/changeset/relationship_revision/outbox | 只使用已有 Domain Service；草稿展示另存 |

数据记录 `source`、`source_ref`、revision 与 evidence 的 observed_at/freshness/support 分开。结果完整度、证据核验、设备健康各自独立。不用累计历史计数证明当前窗口故障。

## 9. 技术架构与 API 对照

### 9.1 渲染与查询

- **V3 原型实际**：React + SVG 实现固定语义骨架/束线/Metro/BioFabric，HTML 表格实现矩阵和证据目录。没有换用第二套 UI 库；没有新增第三方渲染器。`visual.tsx` 统一 CMDB 索引、聚合与成员查询，四页保留各自 ViewModel。
- **正式系统既有能力**：G6 5.1.1、ELK 0.12.1 已安装。G6 适合后续任意范围展开的交互图；ELK Worker 适合复杂角色层级和端口排序。当前既有 Worker 没有显式端口约束，不声称完成。二维规则矩阵和路线不强制经过力导向布局。
- **布局不能决定关系**：骨架/曲线/矩阵排序只改变视觉；member IDs、两端关系和事实层从 Adapter 输入。批准/期望/观测/验收独立，不把 graphon 采样边写回 CMDB。
- **大规模备选**：如果服务端聚合 + G6 不能满足实际基准，再评估 Canvas矩阵/Sigma；本轮未安装 G6Lite/Sigma。SVG 每个端口一个 DOM 节点不是无限扩展方案，局部窗口必须限制。参考 [ELK数据模型](https://eclipse.dev/elk/documentation/tooldevelopers/graphdatastructure.html)、[Worker](https://github.com/kieler/elkjs)。

### 9.2 已实现接口（不修改）

以下路径均加 `/api/v1`，真实契约以 `openapi.yaml` 为准。

| API | 当前输入/输出 | 页面用途/限制 |
|---|---|---|
| GET /cmdb/visualization | project_id?, rack_id?, focus? → {data:InfrastructureSnapshot,request_id} | assets/cables/findings/total_assets/truncated；无 snapshot_id/cursor；focus最多设备两跳，不是路径证明 |
| GET /cmdb/dossier/catalog | 既有分页/过滤参数 | 搜索/目录，复用项目授权 |
| GET /cmdb/dossier/tree | 项目范围 | 位置导航，不让前端造身份 |
| GET /cmdb/dossier/relations | 既有资产关系参数 | 一跳关系及来源，不能把所有关系当物理边 |
| GET /cmdb/dossier/assets/{asset_id}/drawer/{tab} | 五/八Tab | 唯一设备详情，按需加载 |
| GET /cmdb/dossier/assets/{asset_id}/provenance、configuration、lifecycle | 资产ID | 证据/配置/生命周期来源 |
| GET /cmdb/dossier/quality | 范围/过滤 | 数据质量，不自动视为硬件故障 |
| /cmdb/changesets | 现有审核/发布契约 | 正式位置变更；原型不调用 |

### 9.3 下一阶段契约草案（不是已实现 API）

不另建 `/topology/*` 平行服务，扩展 CMDB 查询投影。用户确认设计后先在 OpenAPI 收紧专用 Schema，再实现；本阶段不改现有 OpenAPI。

| 建议端点 | 必须的输入 | 专用输出与行为 |
|---|---|---|
| GET /cmdb/visualization/subgraph | project_id?, root_id?, view=hierarchy/fabric/physical, fabric_id?, depth=1或2, cursor?, limit≤200, snapshot_id? | VisualSubgraph：snapshot_id、fact_layer、nodes、edges、groups、coverage、next_cursor；聚合边 member_query_ref，不内嵌无限ID |
| GET /cmdb/visualization/matrix | scope_ids、axes=server_switch/leaf_spine/rack_leaf/port_pair、fabric_id?、snapshot_id | ConnectionMatrix：行列稳定ID与ordering、cells(count/evidence/member_query_ref)、included_count/excluded_count/coverage；缺端另列；count仅完整授权记录 |
| GET /cmdb/visualization/connections | asset_id/port_id/member_query_ref、status?、sort、cursor、limit | ConnectionPage：Cable、双端port/device、fact_layer、source_ref、evidence_refs、next_cursor；未知端用null与原因 |
| POST /cmdb/visualization/trace | project_id?、mode=physical/logical、from_port_id、to_port_id?、at?、vrf_id?、snapshot_id | TraceResult：segments、breakpoints、integrity、verification、coverage、reason；只读查询，受现有CSRF约束；无逻辑能力返回422 CAPABILITY_UNAVAILABLE |

Scope 必须基于会话获授权项目求交集；不可把客户端省略 project_id 解释为所有项目。cursor 绑定用户/授权版本/过滤/snapshot；拒绝跨范围复用。返回错误沿用 ErrorEnvelope(request_id,error)，400参数、403权限、404不可见、409快照过期、422不支持、504超时。未完成分页/截断一律 partial，不报告路径完整。端点与观测分别授权，不能用边泄露不可见设备。

加载：目录/详情/矩阵/追踪 on-demand；范围摘要60s；仅已有实时事件域未来使用 SSE 增量 invalidation，断线显示 stale，不伪造订阅实现。Query Key 含 scope+授权版本+view+filter+snapshot；取消旧范围请求。布局缓存 key 含上述范围+数据revision+算法version+展开集合，不存秘密。

## 10. 性能策略与后续预算

本阶段只验证原型交互与可视化语义，不做千卡性能达标声明。现有 `INFRASTRUCTURE-PERF.json` 是合成数据聚合与ELK结果，不是原始10k节点帧率。

目标预算（待用户验收，**未测生产SLO**）：1440×900、明确记录CPU/GPU/浏览器；默认≤200节点/500边；局部响应p95≤1s、可交互首屏≤2s、Worker布局≤1.5s、平移中位帧率≥45fps。超时保留旧画布并提示缩小范围。10k/50k原始关系依赖服务端聚合，禁止靠前端先全量下载。

分规模记录100/300、500/2000、1000/5000、5000/20000、10000/50000的查询、传输字节、布局、渲染、峰值内存、交互、可见/总数量。布局任务有request序号，旧结果不覆盖新范围；增量展开固定未变节点，更新前后统计分开。此轮不运行这些生产规模测试。

## 11. 异常处理与典型业务流程

- 搜索B300服务器→位置面包屑→单柜背面→NIC端口→Cable双端→Physical Trace→每段来源。
- 从Fabric矩阵选Spine/Leaf→连接成员→观测冲突→对应端口/工单；观察页不自动下发配置。
- 缺端记录→突出缺失字段→查看原BOM来源/CMDB质量；不制造中间段补齐图。
- 有物理记录无观测→unverified；过期→stale；缺模型面板→编号示意；没有PDU→未采集；Logical Trace缺FIB→unsupported。
- 断网/error保留上下文并retry；empty提供清除本地筛选；permission禁止展开并不回显不可见资产；没有任何数据不出现空白画布。
- 布局草稿修改→即时冲突→撤销/保存独立草稿；仅正式实施审批工作流才能改安装事实。

## 12. 实施顺序与设计决策

1. **当前阶段 V3**：研究六篇资料，整改四页原型与本文；只验证相关交互、截图、类型/构建。四个拓扑页面完成即停，等待用户评审；后续授权的机柜设计独立记录。
2. **确认后才实施**：补专用查询契约及Scope/Snapshot；拆四个领域ViewModel，保留生产路由与设备抽屉。
3. 经批准后依次实现全局 Hybrid → 网络语义束线 → 布线 Matrix/BioFabric → Metro/Tree/Table；本轮不安排机柜设计。沿用 CMDB/ChangeSet。
4. 真实数据质量核查后再做规模基准，不用Demo流畅替代千卡证据。

已定：Sidebar唯一模块导航；机柜Mapping保留；继承现有token；原型独立5181；物理trace不穿越交换机；RoCE不是和Ethernet互斥的技术枚举；布局不改实物。

待决、不阻塞本阶段：① patch-through 的现场数据来源；② logical trace所需FIB/VRF采集与保留窗口；③ 规模性能预算的目标硬件；④ 多版本拓扑快照的可用性；⑤ 用户对矩阵排序与任务完成时间的验收。缺证据项保持unknown/unsupported，不自行补造。

## 13. 验收与实际结果

本阶段验收只证明**设计原型**，不证明生产 API、写入、真机和千卡规模能力。

| 场景 | 预期 |
|---|---|
| 模块差异 | Cluster/Hybrid、Fabric Flow、Connection Matrix、Metro/Tree/Table 四种专属工作区 |
| 搜索/导航 | 搜索同一资产→具体柜→端口→Cable→追踪，URL刷新仍定位相同ID |
| 矩阵/端口 | 单元格对应底层Cable集合；端口详情双端相符，无记录不等于故障 |
| 证据反例 | incomplete/conflict/stale可选；logical unsupported明确原因；不自动生成边 |
| 机柜保护 | 本轮不修改机柜源码/fixture/生产UI，既有截图与验证保持历史记录 |
| 隔离 | 原型网络无 `/api` 请求；正式 dev/backend/页面/迁移不变 |
| 视觉 | 1440×900 与1280宽检查；主要工作区首屏可见；没有重复导航/大段说明 |

实际运行结果与截图见 [V3原型验证记录](topology/prototype/validation-v3.md)；V2记录保留在 [历史记录](topology/prototype/validation.md)（仅记录真实执行的检查）。未实现或受数据限制的目标以§2、§9.3、§12为准，不以原型按钮替代后端交付。

## 14. 研究依据、方法取舍与视觉质量评价

以下是本轮实际查阅范围。论文用于设计推理，不是 Keel 正确性或生产性能的证明。没有复制论文项目代码；原型独立实现。

| 资料与可访问范围 | 相关方法 | Keel 采用 / 不采用 |
|---|---|---|
| [Graphon-Based Visual Abstraction for Large Multi-Layer Networks](https://doi.org/10.1109/TVCG.2025.3581034)，2025；核对 [作者摘要的 PubMed 收录](https://pubmed.ncbi.nlm.nih.gov/40536842/)，未取得可阅读全文 | 层内摘要、层间对齐、上下文采样，以概率模型抽象大图 | 借鉴多层概览与局部展开；本轮只做确定性聚合。采样或概率关系不能充当 CMDB 物理连接。未宣称阅读完整算法或复现 graphon |
| [Bundling-Aware Graph Drawing Revisited](https://repositum.tuwien.at/handle/20.500.12708/221774)，2025；作者机构摘要可访问；参考其 [2024 前作全文](https://drops.dagstuhl.de/storage/00lipics/lipics-vol320-gd2024/LIPIcs.GD.2024.15/LIPIcs.GD.2024.15.pdf)，两版不混称 | Filter → Draw → Bundle：先组织骨架，再路由剩余关系 | 先建立 Fabric 角色骨架，再按真实端点集合聚合。原型不是论文算法实现；不在任意混乱的全图上仅加曲线来掩盖结构 |
| [Quantitative Metrics for Edge Bundling of Network Visualizations](https://onlinelibrary.wiley.com/doi/full/10.1111/cgf.70443)，2026；全文，重点指标层次、讨论与局限 | 边/束/全图多层指标；指标相关性与冗余；需要结合人的任务评估 | 不用“少了几根线”或单一美观分数证明质量。本文定义可测工程指标，但不将其冒称论文全套指标/感知验证 |
| [Comparative Evaluation of Bipartite, Node-Link, and Matrix-Based Network Representations](https://visvar.github.io/assets/pdf/abdelaal2022comparative.pdf)，2023期刊卷；全文 | 150人研究；500节点概览与较小规模细节任务。Bipartite有利整体结构，矩阵在所研究任务中可靠；矩阵不擅长路径顺序 | 矩阵负责核线，Bipartite负责双端集合，Metro负责路径。实验并非数据中心现场验收，不能外推“矩阵适合所有任务/千卡生产已验证” |
| [DynTrix: A Hybrid Representation for Dynamic Graphs](https://doi.org/10.1111/cgf.15076)，2024；正文设计/实现/局限与 [作者实现](https://github.com/franklyn4000/DynTrix) | NodeTrix式局部矩阵与组间连线，稳定位置与联动选择；动态版本中的变化表达 | Pod内部矩阵+组外聚合保持位置记忆；默认小范围矩阵避免平方空间爆炸。没有连续快照，不模拟时间演化；不复制其实现或声称实现完整DynTrix |
| [Combing the hairball with BioFabric](https://doi.org/10.1186/1471-2105-13-275)，2012；全文中表示与交互 | 节点独占横行，边独占竖列，两端方点区分连接与交叉 | 用端口行/Cable列补充高密度核线；横向浏览和局部展开。不是网络流向，也不用于证明多跳路径；原论文规模不等于本原型规模 |

### 14.1 表达决策与事实边界

- **Hybrid Graph ≠ 同时叠所有层**：层内用适合该任务的编码；通过稳定 ID 联动层间。位置属于 CMDB parent/located_in，物理属于 Cable endpoint，逻辑属于批准配置/成员关系；不是一条没有类型的“关系线”。
- **Edge Bundling ≠ 删边**：聚合后的 member set 必须可完整回查。结构骨架是视图结构，不是新事实图；组间总量不能宣称每个组内节点互相连通。可选聚合与展开只能改变显示。
- **Adjacency Matrix ≠ 普通业务表格**：双轴对应明确实体，单元格编码关系；主任务为定位异常/缺口/重复连接。设备目录与关系矩阵分开。排序必须同时保持稳定 ID 与选择集合。
- **Bipartite 与 BioFabric 分工**：前者适合看两个端点集合的匹配；后者把每根线缆保留为独立列，适合审阅密集关系和多重边。A/B侧是记录约定，不推导上行/下行流量。
- **Metro Route 是 Keel 的交互设计推断**，不是以上论文中的性能结论。站点强调有序证据与断点；Tree承载相关对象；Table承载核验细节。不会为了画漂亮的长路线跨越无证据区间。

### 14.2 质量评价：先正确，再减少视觉负担

度量必须固定数据范围、布局版本、浏览器、viewport、缩放和筛选，区分原记录数量与可见图元数量。下表是 Keel 工程验收定义，不是论文指标的完整实现。

| 检查维度 | 可复核定义 | 本轮/后续 |
|---|---|---|
| 成员可追溯 | 聚合成员并集与当前可聚合完整记录集合一致；每个成员具有唯一Cable ID和真实端点 | 本轮在代表性束/矩阵交互核对；未跑生产全库审计 |
| 缺口保留 | 被矩阵/束排除的缺端记录数量及原ID仍可查询；不能“消失后全绿” | 本轮以缺B端反例验证 |
| 压缩比 | `1 - 可见语义束数 / 该视图完整Cable数`；只描述表示压缩，不称像素Ink Reduction或性能提升 | 原型Pod1为175条→20束（88.6%）；不代表所有704条都在该视图 |
| 选择正确性 | 点单元格/线缆列后，详情两端必须与原始Cable一致；交叉处不能产生新邻接 | 本轮关键交互与BioFabric端点核对 |
| 标签可读性 | 固定viewport下的相交标签对数、被裁切重要标签数量，选中态完整ID可见 | 本轮截图人工检查；未实现自动几何度量 |
| 路由失真 | 同端点曲线弧长/欧氏距离；零距离单独处理，比较同一底图 | 后续选布局算法时测，未实现论文指标套件 |
| 歧义/任务效率 | 工程师找到指定对端的正确率、时间；是否误把束交叉当连接；路径断点定位 | 待用户评审，未做受试者研究；不能以自动点击替代 |
| 密度与渐进加载 | 主图可见对象数、被聚合/排除数量、每次展开新增图元、响应时间 | 原型有12条Bipartite/24列BioFabric/24线展开窗口；生产限额需查询端实现 |

### 14.3 本轮设计决策与未解决项

已定：四页使用四种主要表达；矩阵作为物理布线默认；详情按需出现；不新增图形依赖；不修改机柜；原型仅 synthetic。绘图与证据使用同一稳定 ID；所有未观测数据保持未知。

待后续用户评审/数据准备：真实 Fabric 成员与链路用途是否足以区分多个网络；矩阵排序（位置、角色、端口号）哪种更适合现场核线；实际连续拓扑快照；patch-through/FIB/冗余故障域证据。Graphon完整论文尚未取得，因此不据其未核实算法作工程选型承诺。
