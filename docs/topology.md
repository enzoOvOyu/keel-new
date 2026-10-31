# Keel 数据中心可视化系统 · 产品与交互设计 V2

更新：2026-10-31。状态：**第一阶段设计评审；文档与可运行原型，未批准进入生产实施**。

本文件是唯一主设计。完整阅读原 V1 后保留：四种独立业务视图、CMDB 稳定身份、事实/观测分层、默认聚合、React SVG 机柜、G6/ELK、渐进加载。修正：重复导航、泛化查询接口、物理与逻辑路径混用、多个状态维度混成单一状态、把目标能力写成既成事实。本阶段不修改正式页面、数据库、业务 API 或根设计令牌。

## 1. 产品定位与范围

面向网络工程师、交付工程师和运维人员，回答“在哪、怎样组织、如何端接、凭什么确认”。CMDB 提供资产与持久关系事实；BOM 提供规划来源；配置、观测和验收证据保持领域所有权。可视化是授权查询投影，不是第二资产库。

| 工作台 | 核心问题 | 主体表达 | 不承担 |
|---|---|---|---|
| 全局拓扑 | 设备在哪个机房/Pod/机柜？ | 层级钻取、机柜组分布、范围明细 | 端口全图、无坐标的伪机房平面图 |
| 网络架构 | 实际有哪些 Fabric 和角色互联？ | 角色分层图、Spine–Leaf 连接矩阵 | 根据设备名猜 VLAN 或 IB |
| 物理布线 | 两端如何端接？ | Connection Map / Matrix / Port Panel / Inventory | LLDP/FDB 当线缆 |
| 链路追踪 | 从这个端口能证实到哪里？ | 有序路径段、断点、证据 | 设备 BFS 伪装转发路径 |
| 现有机柜详情 | 单柜装了什么、占哪些 U、如何连接？ | 精确 SVG 立面与设备/线序工作区 | 新建重复机柜管理系统 |

六阶段交付、项目授权、既有五/八 Tab 设备抽屉不改变。maintained 遵守当前 IPMI-only 限制；这些观察页面不提供设备执行或审批绕过。

## 2. 实际审计与差距

审计基于代码、现有 OpenAPI、迁移及本地 PostgreSQL **只读聚合**，不是根据历史聊天推断。状态含义：**已确认**=代码/模型存在；**不完整**=存在但不满足目标；**不支持**=当前路径无实现；**需要扩展**=后续最小兼容增量。数据库中的演示记录不是真机证据。

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

### 2.1 本次只读数据库快照

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

## 4. 四个模块的页面设计

### 4.1 全局拓扑

**布局**：Sidebar｜标题与单行搜索｜层级面包屑｜大画布（70%）+ 可收起范围详情（30%）；底部按需展开当前范围目录。禁止双份常驻机柜树挤占空间。

**主内容**：数据中心→region（实际语义为机房/区域，不擅自改名）→Pod→已有机柜组→Rack；缺级直接跳过，孤立资产进入“位置未确认”组。无 floorplan 坐标时显示层级关系，不显示米制位置。容器计数递归按稳定 ID 去重，标注统计范围/部分结果；GPU 数量来自部件或明确规格来源，不能把节点数当 GPU 数。

**操作**：单击选中并显示摘要，明确“展开”进入下级；双击等效展开，不在单击时同时跳页。机柜点“打开机柜”进入单柜；搜索服务器显示结果及父级路径，定位/打开明确分开。面包屑返回；适应画布、缩放、平移、保存/恢复视图。

**详情/表格**：当前容器的 Rack/Server/Switch 数、位置来源、异常对象；范围目录列名称/类型/父级/完整性，点击选择同一对象。健康为空显示未采集，不给整容器绿灯。

**异常**：无权限不泄露子级计数；未知父节点单独列；分页/截断明确“当前已加载”而非全量统计；失败保留已加载范围并支持重试。

### 4.2 网络架构

**布局**：紧凑 Fabric 选择 + Pod/角色筛选；内部“角色拓扑 / 连接矩阵 / 交换机目录”；右侧选中交换机或矩阵单元格。

**Fabric**：Ethernet 是技术族，RoCE 是其业务/协议属性，OOB/Storage 是用途，不能当成互斥同级枚举。使用 fabric_id、technology、purpose、roce_mode；只展示有基线/成员证据的网络。IB 依赖独立技术与端口证据；NVLink/NVSwitch 单独互联类型，不进入 Ethernet Fabric。当前原型只有 fixture 明确的 RoCE Fabric；其余不捏造可选数据。

**角色拓扑**：Spine/Leaf/ToR/Server 分层来自角色字段，不强塞四层；缺角色进入“未分类”。全局到交换机层，服务器按机柜分组；选中才展开局部服务器。角色布局不是物理方位。普通线表示 CMDB 端接记录，虚线仅期望设计，LLDP 以旁证标记出现。

**矩阵**：行 Spine、列 Leaf；计数只取完整且属于当前范围的 Cable，单元格展示记录数/速率及证据冲突。点单元格打开成员线缆、A/B 端口；“无记录”不自动等于断线。对比批准基线时才标“应有而缺失”。

**交换机详情**：身份/角色/型号/机柜、已登记端口数（不冒充型号总端口数）、端口清单和异常；设备完整详情复用 NetworkDeviceDrawer；LLDP/DDM 的 source/time/freshness 与配置版本分别显示。端口→物理布线，机柜→具体单柜。

**异常**：无 Fabric 成员关系显示待配置；旧观测只显示历史；没有批准冗余方案，冗余结论 unknown。

### 4.3 物理布线

**布局**：一条对象/介质/状态筛选工具栏 + 四模式 Tab + 主工作区 + 单对象证据面板。

| 模式 | 专属交互与信息 |
|---|---|
| Connection Map | 默认设备组聚合；连线束显示 count 与成员查询引用，点击打开该束明细；选中线缆突出 A/B 设备、端口，其他关系弱化；展开最多一层，超限转目录 |
| Connection Matrix | 默认 Rack×Rack，切换 Switch×Switch 或 Server Group×Switch Group；仅计授权完整记录，缺端记录另列不丢弃；选择单元格得到精确底层 Cable 集合 |
| Port Panel | 选择设备→真实登记端口列表/矢量示意，连接占用与 oper_state 分开；无型号几何标“编号示意”；点端口查线缆及同口冲突，不用 peer_port_id 覆盖 Cable |
| Cable Inventory | ID/介质/长度/A设备端口/B设备端口/来源/记录完整性/观测新鲜度；搜索、状态筛选、排序、分页、导出当前筛选结果；中间端接需要真实分段记录 |

右侧分组：物理记录、设计期望、采集观测、验证结论、原始来源；支持一键 Physical Trace、定位 A/B 机柜。缺 B 端显示断点，不生成虚拟设备；多条占用同端口展示全部冲突。无连接记录与确认空闲分别显示，不能自动把未登记端口显示为空闲。

### 4.4 链路追踪

**布局**：起点设备/端口、可选终点、Physical/Logical 模式、开始/清空；左侧有序路径段占主要宽度，右侧证据；底部“路径明细 / 冗余证据 / 关联范围”。不放全网图。

**Physical Trace**：端口→Cable→端口；若有明确 patch-through 关系才能继续进入下一段。到交换机端口即终止当前物理段，不能自动穿过交换芯片。设备级邻接只能作为“关联范围”，不得标为同一根端到端物理路径。遇分叉让用户选择记录，不静默取第一条。

**Logical Trace**：必须具备指定时间/VRF/地址族/方向的配置与转发表证据；缺少则展示 unsupported/证据缺口。可展示已知配置/邻居，但不能画成已验证转发路径。本阶段原型明确禁用开始逻辑追踪，显示所缺数据。

**路径结果拆维度**：integrity=complete/incomplete/conflict/unknown；verification=unverified/matched/conflict/unknown；freshness 单独显示。完整的记录可以尚未验证。每段可点出双端/来源/时间/版本/工单引用；缺段显示断点与去核查入口。

**冗余**：比较两条有证据的端口路径，共同 cable/设备/电源故障域分别计算；缺少第二路径或共享风险数据只给“未知”。不因两根线就宣称高可用。

**影响**：只列直接物理关联资产、已证实业务依赖；可达服务器≠业务受影响服务器。GPU 数只有明确服务器/GPU 关系完整且范围授权时展示，不以 BFS 泛洪统计全网损失。

## 5. 机柜详情增强

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
| 框架 | Sidebar 220–240px，顶栏52px；页面标题44px、工具栏40px；1440×900 内容首屏从约160px开始，禁止大段简介 |
| 字体 | 标题20px/600，分区13px/600，正文12–13px，来源/ID11px等宽；信息层级靠字重/位置而非重复卡片 |
| 间距 | 4/8/12/16/24px；表格行36–40px；控件32px；圆角6px；仅浮层轻阴影 |
| 面板 | 对象面板320px，可关闭；1280px降为280px；窄屏转单详情层，不挤成不可读三栏 |
| 状态 | 正常/警告/故障/未知/维护分别有文字与图标；维护蓝色暂停；规划与健康不同维度，禁止以蓝色规划暗示正常 |
| 节点 | 全局容器显示范围和计数；网络角色分层、设备图标；端口只在局部显示；不为每个 Fabric 用不同彩虹色 |
| 连线 | 物理记录实线、批准设计虚线、观测为证据徽标；冲突叠警示，不用颜色单独编码；没有事实的边不画 |
| 标签 | 默认只容器/设备短名；端口与Cable ID仅选中时显示；长名详情完整展示；低缩放隐藏次级标签 |
| 操作 | 搜索Enter定位、Esc关闭面板、键盘按钮/列表等效画布操作、焦点可见；表格行点击与明确“打开”一致 |

解决毛线团：先选范围，再选层级，再按需端口；正式实施默认200可见节点/500边预算，超出聚合/目录，不通过压缩文字硬塞。选择线缆只突出相关双端。组件复用≠四页面复用同一个数据模型与布局。

## 7. 高保真原型入口与运行

独立目录：[topology/prototype/](topology/prototype/)。只读引用 dev 的 shadcn、ELK Worker、RackElevation；原型 graph.tsx 基于现有 TopologyCanvas 隔离调整缩放和图例；独立 CSS、数据适配器和路由，不引入真实 IdentityProvider 或 cmdbFetch，不配置 `/api` 代理。

```bash
cd docs/topology/prototype
npm run dev     # 5181，复用 dev/node_modules；无需安装另一套依赖
npm run check   # 本原型类型检查
npm run build   # 独立 dist，不影响 dev/dist
```

| 原型 | 地址 | 主要验收点 |
|---|---|---|
| global-topology | http://localhost:5181/global-topology | 层级钻取、设备搜索、范围详情、机柜定位 |
| network-architecture | http://localhost:5181/network-architecture | 角色图/Spine-Leaf矩阵/交换机列表、端口与布线跳转 |
| physical-cabling | http://localhost:5181/physical-cabling | Map/Matrix/Panel/Inventory、筛选、Cable双端详情 |
| link-tracing | http://localhost:5181/link-tracing | 端口选择、有序段、缺端、证据、逻辑能力边界 |
| rack-detail | http://localhost:5181/rack-detail | 正反面、U位、选设备/端口、草稿、Mapping与布线 |

原型 URL 支持 rack/asset/cable/port、mode、scope、q；这些是原型内 view-state，正式实施由 Adapter 映射到 §3 既有参数。按钮和筛选本地生效，标记 **SYNTHETIC · 设计原型**；保存仅 `keel.topology.prototype.*` 本地存储；没有 CMDB 写入操作。

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

- G6 v5：已安装 5.1.1；复用 Graph、node/edge click、setElementState、drag-canvas、zoom-canvas。全局层级与网络角色分别生成 ViewModel。
- ELK 0.12.1：只布局不判关系；独立 Worker；规划中显式 port id/side/fixed-order 支持正交端口约束。当前 worker 仅 node/edge，端口约束**尚未实现**。
- SVG：复用 RackElevation 并按模型元数据增强；Port Panel 缺面板坐标时编号示意。
- React：端口级路径分段/证据列表；不引入第三套拓扑引擎。
- Sigma.js 仅候选；G6Lite 未安装也未验证，不选型。除非 G6 聚合方案的实际基准不达标，否则不增加渲染器。参考 [ELK 模型](https://eclipse.dev/elk/documentation/tooldevelopers/graphdatastructure.html)、[elkjs Worker](https://github.com/kieler/elkjs)、[Sigma](https://www.sigmajs.org/)、[Rackula](https://github.com/RackulaLives/Rackula)。Rackula 仅借鉴交互，未复制代码/素材。G6官网本次请求超时，API以已安装5.1.1类型和现有运行实现核对，未引用其他版本方法。

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
| GET /cmdb/visualization/subgraph | project_id?, root_id?, view=hierarchy/fabric/physical, fabric_id?, depth=1或2, cursor?, limit≤200, snapshot_id? | VisualSubgraph：snapshot_id、nodes、edges、groups、coverage、next_cursor；聚合边 member_query_ref，不内嵌无限ID |
| GET /cmdb/visualization/matrix | scope_ids、axes=rack/switch/group、fabric_id?、snapshot_id | ConnectionMatrix：行列稳定ID、cells(count/missing/evidence/member_query_ref)、coverage；count仅完整授权记录 |
| GET /cmdb/visualization/connections | asset_id/port_id/member_query_ref、status?、sort、cursor、limit | ConnectionPage：Cable、双端port/device、fact_layer、source_ref、evidence_refs、next_cursor；未知端用null与原因 |
| POST /cmdb/visualization/trace | project_id?、mode=physical/logical、from_port_id、to_port_id?、at?、vrf_id?、snapshot_id | TraceResult：segments、breakpoints、integrity、verification、coverage、reason；只读查询，受现有CSRF约束；无逻辑能力返回422 CAPABILITY_UNAVAILABLE |

Scope 必须基于会话获授权项目求交集；不可把客户端省略 project_id 解释为所有项目。cursor 绑定用户/授权版本/过滤/snapshot；拒绝跨范围复用。返回错误沿用 ErrorEnvelope(request_id,error)，400参数、403权限、404不可见、409快照过期、422不支持、504超时。未完成分页/截断一律 partial，不报告路径完整。端点与观测分别授权，不能用边泄露不可见设备。

加载：目录/详情/矩阵/追踪 on-demand；范围摘要60s；仅已有实时事件域未来使用 SSE 增量 invalidation，断线显示 stale，不伪造订阅实现。Query Key 含 scope+授权版本+view+filter+snapshot；取消旧范围请求。布局缓存 key 含上述范围+数据revision+算法version+展开集合，不存秘密。

## 10. 性能策略与后续预算

本阶段只验证原型交互，不做千卡性能达标声明。现有 `INFRASTRUCTURE-PERF.json` 是合成数据聚合与ELK结果，不是原始10k节点帧率。

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

1. **当前阶段**：完成本文审计/设计与五原型；浏览器核对指定交互、截图、原型类型/构建；完成即停。
2. **确认后才实施**：补专用查询契约及Scope/Snapshot；拆四个领域ViewModel，保留生产路由与设备抽屉。
3. 全局/网络聚合与矩阵→物理四视图与证据→端口Trace→机柜增量增强；沿用CMDB/ChangeSet，不建第二套资产模型。
4. 真实数据质量核查后再做规模基准，不用Demo流畅替代千卡证据。

已定：Sidebar唯一模块导航；机柜Mapping保留；继承现有token；原型独立5181；物理trace不穿越交换机；RoCE不是和Ethernet互斥的技术枚举；布局不改实物。

待决、不阻塞本阶段：① patch_panel双面端口与内部贯通的现场数据来源；② logical trace所需FIB/VRF采集与保留窗口；③ 型号前后面板几何的来源/授权；④ PDU插口与电源故障域采集；⑤ 规模性能预算的目标硬件。缺证据项保持unknown/unsupported，不自行补造。

## 13. 验收与实际结果

本阶段验收只证明**设计原型**，不证明生产 API、写入、真机和千卡规模能力。

| 场景 | 预期 |
|---|---|
| 模块差异 | 全局层级、角色架构、布线四视图、路径证据、机柜立面五种专属工作区 |
| 搜索/导航 | 搜索同一资产→具体柜→端口→Cable→追踪，URL刷新仍定位相同ID |
| 矩阵/端口 | 单元格对应底层Cable集合；端口详情双端相符，无记录不等于故障 |
| 证据反例 | incomplete/conflict/stale可选；logical unsupported明确原因；不自动生成边 |
| 机柜 | 正反同身份/U；选设备/端口联动；移动草稿冲突阻止保存、撤销恢复；不写CMDB |
| 隔离 | 原型网络无 `/api` 请求；正式 dev/backend/页面/迁移不变 |
| 视觉 | 1440×900 与1280宽检查；主要工作区首屏可见；没有重复导航/大段说明 |

实际运行结果与截图见 [原型验证记录](topology/prototype/validation.md)（仅记录真实执行的检查）。未实现或受数据限制的目标以§2、§9.3、§12为准，不以原型按钮替代后端交付。
