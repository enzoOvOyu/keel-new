# Keel Spatial Rack Digital Twin · 工程空间视图设计

更新：2026-10-31。状态：**产品设计与独立可运行原型，待用户评审；不是生产功能交付**。

本文件是本轮机柜可视化的设计入口，接替 `topology.md` 中“机柜冻结”的阶段性决定。四个拓扑原型保持原设计；正式 `/racks`、CMDB、数据库及业务 API 均未修改。现有业务规则继续以 PRD、RACK-DIAGRAM、DATA-DICTIONARY 和 OpenAPI 为准。

## 本轮 V2 纠偏：空间工作台

用户要求升级为 Spatial Rack Digital Twin。本轮继续限于设计和 5181 独立原型；CMDB 的设备身份、安装事实、规划与观测所有权不变。空间孪生是数据与关系模型，不等于开启 3D 动画。当前几何和实时观测不足，产品呈现“部分空间模型”，不宣称已建立实时、毫米级数字孪生。

```text
现有导航 / 顶栏
[机柜阵列] RACK-01 / Pod 1       [资产 | 空间 | 计算] [比较] [数据依据]
┌ 柜内对象树 ┬ 空间主视图（可选择 / 聚焦 / 分解） ┬ 设备与工程信息工作区 ┐
│ 按U顺序    │ 平行斜投影 / 双面 / 剖面                │ 资产、Mapping、安装  │
│ 设备 / ID  │ 机柜参考坐标与真实U安装关系       │ 端口索引、端接证据   │
│ 相邻机柜   │ 几何已知→实体；未知→开放边界     │ 设备尺寸、约束与来源 │
│ 搜索       │ 选择设备后显示尺寸与关联引导线   │ 运行观测与能力缺口   │
└────────────┴ 同坐标的U占用导航 / 选中区间 ────┴────────────────────┘
```

- 信息按总览→设备→端口三个尺度展开，不把全部文本缩小塞进 Gallery。
- 视图状态、事实位置、观测状态分离。选择/相机/分解属于展示；拖动安装草稿仍需明确确认且不写 CMDB。
- 已知 U 高决定竖向几何；未知宽深用未标定参考轴。不得使用固定透视厚度冒充设备深度；不会生成无来源的风扇、盘位、PDU、功率或热力。
- 右侧留出足够宽度，以工程清单与结构图承载信息。源数据限制放进针对性字段/来源面板，不占据数行顶部说明。
- 审计作为数据依据保留。Gallery、空间工作台与双面精确投影各有职责，相互补充；V2 改变默认阅读顺序，不删除原有 Mapping、双面选择、草稿和追踪能力。

## 1. 定位与交付边界

用一套确定性安装模型，将 CMDB **已经登记且有来源**的身份、U 区间、几何与关系投影为 Gallery、空间工作台、前后双面和安装剖面。整体范围、安装关系与端口细节是三个互补尺度，共享选择上下文与证据，不维护第二套资产库。

第一期核心：Gallery 与 Pod 范围导航、空间平行投影、Dual Elevation、安装剖面、Asset / Capacity / Compute、设备 Mapping 与来源追溯。规划与实装比较保留明确入口，但当前数据不足时只展示缺口；Power / Health / Thermal 不因字段名称存在就自动启用。

本轮不做真实位置修改、实际设备访问、实时遥测接入、生产审批、新增 CMDB 实体或 3D 引擎。原型的展示草稿仅保存浏览器布局，不是安装事实、批准规划或 As-built。

## 2. 实际审计：数据能力与覆盖率

### 2.1 可复核依据

- [只读审计 SQL](rack-visualization/audit.sql)；[审计结果 JSON](rack-visualization/cmdb-capabilities.json)。时间为结果中的 `audited_at`。不包含凭据。
- 模型：`dev/backend/migrations/001_bom.sql`、`003_integrity.sql`、`014_cmdb_domain.sql`；Schema Registry：`dev/backend/internal/cmdb/schemas/core-v1.json` 及现有扩展。
- 真实查询：`dev/backend/internal/cmdb/visualization.go`；前端 Adapter：`dev/src/lib/api/infrastructure.ts`。
- 现有页面：`dev/src/features/infrastructure/InfrastructureWorkspace.tsx` 与 `RackElevation.tsx`。已有 U 立面、草稿、CMDB ChangeSet、设备抽屉、导出与 Mapping，保留正式实现。本原型不调用这些运行时 Provider。
- 独立原型数据：`docs/topology/prototype/fixture.json`，来自已有 `fixtures/demo-full-64-b300/demo-seed.json` 的脱敏合成投影，稳定 ID 与领域结构一致，不是现场采集。

**数据库与原型不可混算**：本地库有 66 台服务器、26 台交换机，包含测试资产；原型继续使用现有 64 / 24 切片。没有为了满足演示数量删除真实库中的另外 4 台设备。

### 2.2 字段可用性矩阵

覆盖率只说明字段存在，不说明现场真实性、可用性或实装验收通过。库中的示例主要为 synthetic；所有设备健康结论仍需合格观测证据。

| 数据 / 分类 | 实际覆盖与样本 | 可用于本期 | 判断 |
|---|---|---|---|
| Rack 身份与 U 高 · CMDB 直接 | 16/16，100%；`rack-0001` → 42U | 精确竖向刻度、Gallery | 已确认 |
| Rack → Pod · CMDB 直接 | 16/16，100% | 按 Pod 排列，不代表实物坐标 | 已确认 |
| Rack `position` · CMDB 直接 | 16/16；样本 `"01"` 为序号 | 仅可排序；不是测量 XY | 不支持平面图 |
| 服务器型号 · CMDB 直接 | 66/66；`srv-0001` → `HGX B300 (demo)` | 型号文本 | 已确认 |
| 交换机型号 · CMDB 直接 | 26/26；`sw-leaf-0001` → `SN5610 (demo)` | 型号与角色文本 | 已确认 |
| 服务器 rack_id / rack_u / u_height · CMDB 直接 | 各 64/66，97.0%；`srv-0001` → rack-0001 / U33 / 6U | 规划登记立面；缺失资产进入未定位清单 | 不完整 |
| 交换机 rack_id / rack_u / u_height · CMDB 直接 | 各 24/26，92.3%；`sw-leaf-0001` → U2 / 2U | 同上 | 不完整 |
| 位置关系事实层 · CMDB 直接 | 88 条 active located_in，均 planned / bom | 显示“规划登记”；不标 As-built | 已确认 |
| 安装面 / face · CMDB 直接 | 0/92，0% | 两面虚线占位参考，明确未知 | 需要扩展接入 |
| 设备深度 / full-depth · CMDB 直接 | 0/92，0%；机柜 depth_mm 0/16 | 不宣布正反面可共用同一 U 位 | 需要扩展接入 |
| front/rear 面板引用 · 静态素材 | 0/92；4 个 model revision 均无几何能力 | 标准矢量轮廓，不画型号端口或盘位 | 不支持精确面板 |
| GPU 数量 · CMDB 直接 | gpu_count 64/66；server→GPU contains 共 512 条 | 登记 GPU 数；不是利用率或健康卡数 | 已确认 / 部分覆盖 |
| CPU / memory_gb / gpu_model · 直接字段 | 审计服务器字段均 0/66 | 不从 B300 名称猜配件型号与配置 | 不支持当前投影 |
| 端口 / Cable · CMDB 直接 | 库中 1,668 / 706；原型 1,664 / 704 | 稳定 ID Mapping、端接、追踪入口 | 已确认；个别端点异常 |
| 物理端口面板坐标 · 静态素材 / 型号数据 | 当前查询没有可靠坐标 | 清单可用，面板热点不可定位 | 需要扩展 |
| 已确认实装位置 · CMDB 证据 | 0；rack_placement 表为空 | 显示实装数据前提，禁止生成假 Diff | 不支持 |
| 占用 U / 空闲区间 · 确定性派生 | 从有效 U 区间求并集 | RACK-01：29U 占位、13U 未占用 | 已确认 |
| 实时 Power · 外部监控 | rack-0001 存在 60 / 51.38 kW，但来源 planned / bom，freshness unknown | 不作为实时功耗；覆盖层不启用 | 不支持实时视图 |
| Health / Thermal · 外部监控 | 存在历史 synthetic 字段/传感器，未验证当前采集链与保鲜 | 不显示当前健康色或热力图 | 不支持当前结论 |
| PDU / 供电关系 · CMDB 直接 | PDU 0，缺插口端接 | 无纵向 PDU、冗余供电图 | 不支持 |

**遗留数据差异**：旧演示包有 88 条 `rack_placements.face=front`、4 个 diagram revision、2 个 device model、16 个环境样例和 32 个供电回路样例。它们没有成为当前 CMDB 中可靠的安装面、型号几何、实装或实时遥测证据。不能用这些样例静默补齐上表的 0%。`bom_revision` 的 23 条记录也不等于存在一对可比较的机柜规划/实装快照。

数据质量问题记录为缺口，本阶段不修改资产。生产接入应查询同一授权范围下的字段来源与关系事实层；不能把本次聚合计数当成永久常量。

## 3. 参考研究与取舍

### NetBox：数据驱动，不画不存在的现场

[Visual Explorer](https://netboxlabs.com/docs/visual-explorer/) 使用系统登记数据支持范围导航、机柜立面和对象详情；不是实时设备发现。Keel 借鉴“从组合总览下钻到单对象”和缺口可见的方式。Gallery 按 Pod 分组，缺物理坐标时不用机房平面布局；供电关系只有在 PDU、插口和端接已建模时才能展示。

[Rack](https://netbox.readthedocs.io/en/stable/models/dcim/rack/) 支持不同 U 高和位置体系；[Device](https://netbox.readthedocs.io/en/stable/models/dcim/device/) 区分最低占用 U、0U 和 full-depth；[Device Type](https://netbox.readthedocs.io/en/stable/models/dcim/devicetype/) 区分型号几何、组件模板与设备实例。Keel 借鉴这些语义，不迁移 NetBox 数据模型，也不把型号模板更新当成现有资产已经换型。

[Device Type Library](https://github.com/netbox-community/devicetype-library) 可作为后续经确认的型号资料来源。导入须匹配 manufacturer/model/part number、记录版本与许可；当前没有复制图库、面板或默认几何。不能仅因 demo 型号字符串相似就套用产品图片。

### Rackula：精确坐标与可撤销编辑

阅读了 [项目](https://github.com/RackulaLives/Rackula)、[Architecture](https://github.com/RackulaLives/Rackula/blob/main/docs/reference/ARCHITECTURE.md) 以及其类型与碰撞实现。其架构采用 Svelte + SVG、明确单柜边界及命令历史；并不是本次 Gallery 能力的现成实现。Keel 借鉴单坐标体系、安装面检查、拖动与撤销语义，以现有 React + SVG 自行实现。仓库 LICENSE 为 MIT；本次未复制代码或素材，没有嵌入 Svelte。

### ISO/TR 24464:2025：保真度分开表达

实际阅读的是用户提供的[预览](https://gso-sims-preview-doc-aws.s3-eu-west-1.amazonaws.com/iso-tr-24464-2025-en.html)，包含范围、术语及目录，**不是全文**。借鉴其对可视化保真度的关注，Keel 区分：空间分辨率（U 坐标）、几何细节（面板素材）、时间有效性（observed_at/freshness）与事实可信度。此为设计取舍，不宣称符合 ISO 认证或完整实施该报告。

### Sunbird：空间上下文与连接细节互补

实际阅读 [Data Center CMDB](https://www.sunbirddcim.com/data-center-cmdb)、[Data Center Visualization](https://www.sunbirddcim.com/product/data-center-visualization)，并用浏览器查看后者的 X-ray 柜体/侧面展示。其价值是把机柜内部安装、设备深度/对齐、连接和容量联系起来，而不是单纯增加透视。Keel 采用可隐藏的柜侧参考面、设备选择、安装剖面、关联线缆与容量面板。没有测量深度时，只呈现开放参考边界；没有机房坐标时，不生成 Sunbird 示例中的三维楼面。没有复制页面、模型或素材。

### 可视化理论到实际交互

| 原始研究 / 方法 | Keel 采用的设计 | 本原型可操作内容 / 边界 |
|---|---|---|
| [Heer & Shneiderman, Interactive Dynamics for Visual Analysis, 2012](https://idl.cs.washington.edu/files/2012-InteractiveDynamics-CACM.pdf) | 概览、语义缩放、选择过滤、关联多视图 | Gallery 删除微小文字；单柜保留范围缩略图；设备树、U 导航、画布和详情同步选择。不是全图仅放大 |
| [A Coordination Model for Exploratory Multi-View Visualization, 2003](https://www.cs.kent.ac.uk/pubs/2003/1654/) | 共享对象而非共享像素 | asset/port/cable ID 驱动相机、双面和详情；切换视角保留当前对象和端接，不通过名称猜关联 |
| [Li et al., Interactive 3D Exploded View Diagrams, SIGGRAPH 2008](https://grail.cs.washington.edu/projects/exview3D/) | 选择性分解，保留装配位置上下文 | 仅分离选中设备的展示平面，原 U 位留轮廓与引导线。没有零件几何/阻挡关系，不实现论文的自动拆卸规划，也不把分解方向当维护抽出方向 |
| 工程制图：正投影、平行斜投影、局部剖面、尺寸界线 | 不同问题采用不同投影 | 空间看前后安装关系，Dual 看两面，正视核对 U，剖面看已知/未知深度；宽深未标定时不宣称毫米等比或碰撞可行 |
| Focus + Context / 按需细节 | 局部高亮但保留范围 | 聚焦弱化不相关设备，选线缆保留两端；右侧端口索引展开为线缆证据与追踪，不把全柜端口塞在画布上 |

上述为基于研究的产品取舍，不是对论文算法或性能的实现宣称。双面立面仍有精确核对价值，空间视图不能取代它；也不能拿未来运行遥测功能充当本期设计效果。

## 4. 信息架构与交互

### 4.1 Rack Gallery

```text
现有 Sidebar / 顶栏（保持）
Spatial Rack
[空间工作台 | 机柜阵列 | 规划/实装]      数据能力
[设备/型号/SN 搜索] [Pod]        叠加层：资产 / 空间 / 计算
范围摘要：机柜、登记 U、服务器、登记 GPU、交换机
┌ Pod 1：4 个缩略立面 ┐ ┌ Pod 2：4 个缩略立面 ┐
└ 柜号 / U / 数量    ┘ └ 同一几何模型       ┘
┌ Pod 3              ┐ ┌ Pod 4              ┐
紧凑图例：设备类别、未知面；不是实际机房坐标
```

- Gallery 使用同一 `layoutRack` 与 `Elevation`，空间主图同样消费此布局输出。缩略尺度降低标签密度，设备占位不变；不按设备数平均排布。
- 每柜显示名称、U 高、占用并集、未占用 U、服务器/GPU/交换机登记数量与“实装/健康未核验”。缺记录显示未知。
- 搜索设备、型号、SN、资产 ID 或机柜；命中设备后打开其所在柜并选择该设备。Pod 筛选写 URL。
- 不绘制假行列坐标；响应式阵列只是视图排版。
- 零匹配显示空状态，保留筛选。返回阵列保持查询上下文。

### 4.2 Spatial Workspace 与 Precision Dual Elevation

```text
Spatial Rack [空间工作台 | 机柜阵列 | 规划/实装]             [数据能力]
[当前机柜 / Pod]  U高 / 设备 / 登记占位               [资产 | 空间 | 计算]
┌ 柜内对象 / 范围 ┬ [空间 | 双面 | 正视 | 背视 | 剖面] ┬ 选中设备 / 型号 / ID ┐
│ 搜索与U序清单  │ 柜体透视 / 聚焦 / 选择性分解       │ 资产、面板与端接、安装│
│                │ 设备安装坐标 / 参考面 / 开放边界   │ 容量、观测、来源      │
│ Pod机柜缩略索引│ 双面模式：同步 Front / Rear        │ 信息与证据按需展示   │
│ 上一柜/下一柜  │ U占用导航 / 缩放 / 草稿 / SVG      │ 关联拓扑与追踪入口   │
└────────────────┴───────────────────────────────────┴─────────────────────┘
```

- **整体**：Gallery 对比全部机柜。进入单柜后保留 Pod 分组的范围索引，不让用户丢失当前柜在授权资产范围中的位置；索引只是排版，不是机房坐标。
- **空间**：固定平行斜投影，参考 Front/Rear 面与 U 轴。柜侧板可隐藏；相机不影响真实位置。当前缺少宽深，参考后平面用细线表示，未知设备深度只画开放引导线，不画虚构实体机箱。
- **双面**：同屏 Front/Rear，共享 U、选择和缩放。Front 显示资产类别/型号/数量；Rear 显示可点击登记端口索引与端接数量。点击 Front 设备打开资产，点击 Rear 设备打开端接，点击端口直接定位 Cable 与双端证据。安装面缺失时均为明确标记的占位投影，不声称这是实际正反面板。
- **正视 / 背视**：正视用于核对设备、型号、资产与安装位置，切换时右侧打开资产；背视用于核查端口、速率、连接和对端，切换时右侧打开面板与端接。两者保留同一资产；端口与线缆上下文在同设备切换时保留。背视图上的端口是带文字说明的可操作索引，不是实际网口坐标。
- **剖面**：按 U 展示设备高度与深度范围。缺少深度用未定界区间；选中设备集中显示、其他设备弱化。只有设备/机柜深度同时已知时才计算相对长度；仍需安装面/对齐才能作真实净空结论。
- **聚焦**：弱化不相关对象，保留当前设备及选定 Cable 两端。**分解**只分离选中设备展示平面，保留原位置和引导线；不是实际位置变化、拆卸指令或维护空间验收。
- 画布点击、柜内树、U 导航和右栏端口索引共用稳定 ID。切换设备保留当前详情 Tab；切换正/背视按业务目标打开资产/端接，剖面打开安装，双面点击哪一侧就打开对应工作区；切换机柜清理不属于新柜的选中端点。
- 当前 1440px 桌面采用约 168px 对象栏 / 剩余空间画布 / 385px 工程栏；1600px 以上右栏 430px。窄屏先收起对象栏，再上下排列，不能通过缩小全部文字维持三栏。
- 草稿沿用半 U 吸附、冲突/越界检查、撤销/重做与明确确认。保存只写隔离浏览器存储。0U、缺位置或缺高度进入未定位托盘，不从画布丢弃。
- SVG 导出当前空间或双面视图，带 SYNTHETIC、规划/草稿和非实装测量说明；不对其他未实现格式提供假按钮。

### 4.3 设备详情与 Mapping

- 柜内对象：搜索，按 U 排序，型号、U 高、资产 ID；点击同步所有投影与右侧详情。
- 资产：已有型号、SN、登记 U、GPU/NIC/端口数量与组成摘要；安装：几何、来源与净空约束；容量：U 并集、连续空闲区间、计算资源与核算边界。设备符号不是产品照片。
- 面板与端接（保留 Mapping）：**当前设备 → 登记端口 → Cable → 对端设备/端口**。端口分页/搜索，选端口缩小关联范围；选线缆显示介质、长度、双端、来源与异常状态，可进入既有布线矩阵和链路追踪原型。选中线缆时画布仅画当前双端的关联引导线；端点落在设备范围，明确不是实体走线或实际端口坐标。
- 观测：显示当前能力、时间与历史登记证据；没有有效采集不展示热力图。来源：规划登记、几何字段、缺失实装与实时证据分层展示；保留 synthetic 来源和参考时间，不用参考时间冒充采集时间。
- 不另建设备抽屉。后续生产实现仍调用已有 ServerDetailDrawer 五 Tab / NetworkDeviceDrawer 八 Tab；本原型使用隔离证据面板，避免加载真实 Provider。

### 4.4 Dual-State Rack Comparison（数据接入后开放）

**当前入口隐藏**。已审计数据仅有 planned/BOM 登记，未发现可用于比较的已确认规划快照，已确认实装位置为 0。删除原先只有前提说明的空页面；历史 `view=compare` 链接返回工作台并给出一次提示。不能将普通设备状态 `planned`、展示草稿或当前设备复制当成规划基线。不为演示伪造批准版本和 As-built。

#### 比较资格与能力分级

首先验证左右两侧分别具备**已确认规划版本**和**合格实际数据**、授权项目及相同统计范围。某级字段存在不等于证据资格成立。数量对比也需要实际盘点范围与时间，不能把 BOM 数量再次当作实际数量。

| 经确认的规划数据 | 所需实际数据 | 开放的比较 | 禁止推断 |
|---|---|---|---|
| 只有 BOM 数量 | 同范围实际盘点数量、单位、时间 | 类别/数量差异列表 | 哪一台具体缺失或新增 |
| 有型号 | 同口径实际型号与数量 | 按型号的数量差异 | 型号相同就认定同一设备 |
| 有机柜编号 | 实际 rack_id 与归属证据 | 每柜分布差异；有身份映射时定位设备归属差异 | 跨柜移动对象的模糊匹配 |
| 有 U 位及高度 | 实装 U/高度、安装面、确认记录 | 可绘制两侧立面与位置差异；未配对区域标未匹配 | 仅凭位置判断设备身份 |
| 有规划设备身份 → CMDB 稳定 ID 映射 | 对应正式资产与实装快照 | 设备级缺失、新增、换型、换柜、移位及完整追溯 | 使用名称相似度静默配对 |

采用各字段能力独立启用；缺少 U 时保留数量/型号比较，不绘制假位置。缺少身份映射时，不以几何靠近自动生成移动箭头。缺少实际证据是“无法比较”，不是“设备缺失”。

#### 页面与交互设计

- 顶部仅保留项目/机柜范围、左侧确认规划版本、右侧实装版本及证据时间，以及 `并排 / 差异覆盖` 切换。没有合格版本的范围不出现在可比较列表。
- **并排模式**：左侧已确认规划，右侧 CMDB 实际安装；使用同一 U 比例和几何引擎，缩放、滚动及设备选择同步。只在双方有 U/高数据时显示精确立面。任一侧未知的安装面/深度继续采用明确的参考轮廓。
- **覆盖模式**：同一立面中规划为虚线、实装为实体；移位用起止 U 标签与短连接标注；缺失为空心规划轮廓并标“缺失”，新增实装设备标“新增”，型号差异用型号前后值。颜色为辅助，不能只靠红绿。前后面仍是空间视角，规划/实装是事实层，二者不能混成一个切换器。
- 画布下方是可筛选差异清单：全部 / 缺失 / 新增 / 型号 / 机柜 / U 位 / 无法比较。选择一行同时定位两侧和右侧证据，选择设备反向定位清单。
- 右侧显示**规划值 → 实际值**、各自来源/版本/确认时间/稳定资产映射。每条差异可以打开已有来源、ChangeSet、验收记录；不得通过“接受差异”直接覆盖 CMDB。修正走现有业务审核流程。
- 聚合层只显示已比较/不可比较数量与覆盖比例，不将未匹配对象判相同，不把数据缺失算为施工错误。换件保持旧、新实物身份与关系历史。

#### 最小接入条件

由现有 BOM Revision / CMDB 历史与来源 Adapter 提供规划 `revision_id、confirmed_at、evidence_refs、scope`，实际 `snapshot_id、observed/confirmed_at、evidence_refs、scope`，并按字段提供覆盖和身份映射。只派生 comparison view model，不建平行资产表或新审批规则。失去资格或版本变化使比较结果过期，必须重新选定版本。

验收必须包含：仅数量、型号、机柜、U 位、完整身份映射五种能力；无实际证据、未匹配身份、规划有实装无、实装有规划无、换型、换柜、移位、字段缺测。**本阶段只有完整设计，未开放运行时比较功能**；待合格数据接入后实施，不能把隐藏空页面报告成已完成 Diff。

### 4.5 Smart Overlays

| 层 | 启用条件 | 当前行为 | 不能推导 |
|---|---|---|---|
| Asset | 存在授权设备身份与类别 | 服务器蓝灰、交换机暗青；选中紫色，颜色只表示类型/选择 | 设备健康 |
| Capacity | 有柜高与有效占位 | 并集 U、斜线未占用区间 | 可安装深度、功率或承重 |
| Compute | 有 GPU 数量或一致的组成关联 | 登记 GPU 数；未知保留未知 | GPU 可用率、算力、温度、功耗 |
| Power | 可靠测点、单位、时间、保鲜、作用范围 | 当前不出现在覆盖菜单 | 用 BOM 预算冒充运行功耗 |
| Health | 有当前设备健康证据、能力状态 | 当前不出现在覆盖菜单 | 用登记 active / healthy 历史值判正常 |
| Thermal | 测点位置、单位、时间与覆盖范围 | 当前不出现在覆盖菜单 | 无传感器插值生成热力图 |

可用菜单由数据适配层判断；缺失原因集中于“数据能力”，不在主工具栏堆叠大量禁用按钮。部分覆盖时显示已知数量和覆盖比例，不能补零。

## 5. 确定性几何模型

### 5.1 ViewModel 建议（不是新资产表）

```ts
type RackGeometry = {
  rack_id: string;
  height_u: number;
  depth_mm: number | null;
  // width_mm / rail geometry: only when verified
};
type DeviceGeometry = {
  id: string;                 // CMDB identity
  u: number | null;           // lowest occupied U
  height: number | null;      // 0 / 0.5 / integer
  face: 'front' | 'rear' | 'both' | 'unknown';
  fullDepth: boolean | null;
  depth: number | null;
};
// Source envelope belongs to Adapter: fact_layer, revision, evidence refs,
// source, observed_at, freshness, support, approval status.
```

内部以半 U 为最小离散单位：`start=(u-1)*2; end=start+height*2`，区间 `[start,end)`。绘制 `y=top+(rackHeight-(u-1)-height)*unit`。相同输入生成相同位置。支持实际 42/45/48U 及有效自定义高度，不将 42 写成业务常量。

- 0U 不占竖向 U。只有已确认侧挂位置和方向时才放纵向侧栏；当前缺少这种数据，进入 0U/未定位托盘。
- 0.5U 保留精度；不能向整数舍入挤入相邻设备。负数、越界、缺高/位置进入异常，不自动修正。
- 已知相同安装面且区间相交为冲突；已知 full-depth 与对面重叠也冲突。
- 对面共用 U 只有在双侧明确非 full-depth 且已知深度与机柜深度兼容时，才可确认；面或深度未知时为“兼容性待确认”。
- Capacity 计算所有有效登记竖向区间的并集，不累加前后两次，也不把“未占用 U”说成已确认可安装空间。
- 只有确定尺寸才允许尺寸碰撞检查；未知不标绿。模型不从型号数字推导高度、端口数或速率。
- 原型半 U/0U/正反兼容性的几何例子仅用于独立检查，没有添加到 CMDB 或 Gallery。

### 5.2 设备素材与视觉细节

三层细节：Gallery 占位 → 空间/Dual 标准资产轮廓与端口索引 → 经确认型号面板。选择哪个层由素材与几何能力决定，不由缩放触发虚构硬件。

未来素材注册建议记录 model_revision、manufacturer、part_number、front/rear、SVG viewBox、anchor-to-port-template、license、source URL、revision/hash。素材不保存实例 SN、MAC、健康或实际端接；实例数据从 CMDB 叠加。端口模板与实例端口必须能明确映射，否则仅展示静态面板并关闭热点。

服务器、交换机、配线架、PDU 与挡板的绘制可通过可复用领域组件扩展；本期只画数据中实际存在的类型。未引入图片资产，不新增 UI 框架或图形依赖。

## 6. 视觉与状态规范

继承 DESIGN-SYSTEM 字体、紫色主操作、背景/边框/间距令牌，新增样式只作用于 `.rack-viz`、`rv-*`、`sr-*` 和机柜路由布局。正式 DESIGN-SYSTEM 令牌、全局 CSS 与其他四个拓扑页面不变。

- 页面标题 18px；工程栏设备标题 21px；正文 11–12px；证据/来源 10–11px；ID 和 U 使用等宽。缩略 SVG 不承担完整文本阅读。
- 控件高 28–34px；工作区分栏以细边界相隔；无多层卡片阴影。Gallery 采用两组 Pod 并排，每组四柜，窄屏顺序折叠。
- U 标尺每 U 可读，选择有描边和文本反馈；颜色是类别/选择，不伪装健康。
- 已知 U 占位实边；安装面缺失明确注明，未知深度用开放虚线；未占用细斜线；冲突红色并有文字。维护态若有授权有效状态则遵循现有蓝色暂停规范，与健康分开。
- SVG 可键盘选择；编辑时上下键每次 0.5U；Dialog 沿用 shadcn 焦点管理。缩放不改变业务位置。
- Loading/Error/Permission 在生产 Adapter 接入时用既有状态组件；本原型固定本地快照，不用定时器伪造加载过程。未知对象 ID 显示找不到，不静默替换资产。

## 7. API / 数据与布局边界

复用 `GET /api/v1/cmdb/visualization` 的授权资产/关系投影，以及已有资产详情、字段来源和 ChangeSet 接口；本阶段不修改 OpenAPI 或 handler，不创建平行 Rack API。

| 现有能力 | 后续最小兼容增量 | 边界 |
|---|---|---|
| visualization 资产/端口/线缆 + rack_id | 返回每个位置/几何字段的 fact_layer、来源、版本、support / freshness；分页/范围沿用现有约束 | 不从 fields 推测证据层 |
| rack_u / u_height / face 等字段映射 | 明确 half-U、0U、full_depth、安装面枚举与单位；补实际输入资料 | 不用零表示缺失 |
| 型号版本 | 可选几何与素材引用、许可和版本 | 不复制资产身份 |
| 字段来源 / 历史 / ChangeSet | 比较前验证批准规划与实装快照的资格 | 不把普通 revision 当实装 |
| 外部监控关联 | 观测范围、采样时间、单位、阈值来源、保鲜 | 不把原始时序塞到资产表 |

查询/缓存按授权项目集合、rack ID、CMDB revision 与视图参数分开。Gallery 可使用聚合摘要；单柜按需查设备和关联，Mapping 按端口取证；动态观测按现有加载协议订阅。授权变化清理缓存，不因清除项目筛选放大权限。

布局存储只包含视图模式、缩放、选中 ID、展开状态和展示草稿。正式位置修改应进入已有 CMDB ChangeSet：预览 → 校验 → 确认申请 → 审核/发布 → 新 revision。拖动和保存原型草稿不触发这些业务步骤。

## 8. 独立原型入口与运行

| 能力 | 入口 |
|---|---|
| Spatial Workspace 默认工程视图 | http://localhost:5181/rack-detail |
| Rack Gallery 整体范围 | http://localhost:5181/rack-detail?view=gallery |
| Precision Dual Elevation | http://localhost:5181/rack-detail?view=spatial&rack=rack-0001&camera=dual |
| 选择性分解 | http://localhost:5181/rack-detail?view=spatial&rack=rack-0001&asset=srv-0001&explode=1 |
| 安装剖面 | http://localhost:5181/rack-detail?rack=rack-0001&asset=srv-0001&camera=section&tab=installation |
| Capacity | http://localhost:5181/rack-detail?rack=rack-0001&overlay=capacity&tab=capacity |
| Mapping / 端接 | http://localhost:5181/rack-detail?rack=rack-0001&asset=srv-0001&camera=rear&tab=mapping |
| Dual-State Comparison | 暂不开放入口；数据资格与分级设计见 §4.4 |

实现：[页面](topology/prototype/pages/rack-detail.tsx)、[空间工作台](topology/prototype/rack/SpatialWorkspace.tsx)、[空间投影](topology/prototype/rack/SpatialScene.tsx)、[共享几何](topology/prototype/rack/geometry.ts)、[双面/Gallery](topology/prototype/rack/Elevation.tsx)、[Mapping/来源](topology/prototype/rack/Details.tsx)、[数据适配](topology/prototype/rack/model.ts)。React + SVG，没有增加图形依赖。

```bash
node docs/topology/prototype/run.mjs        # 独立 5181
node docs/topology/prototype/run.mjs check
node docs/topology/prototype/run.mjs build
```

URL 保留 `view/rack/asset/port/cable/tab/face/overlay/q/pod/camera/explode/isolate/cutaway`；旧 `view=dual` 链接继续打开双面，不失效。缩放/平移为当前相机局部状态，切换相机适配；未冒充 CMDB 位置。草稿继续隔离于 `keel.topology.prototype.rack-v2.*`，没有真实 CMDB 请求或写入。

## 9. 实施顺序与验收标准

1. 本期：审核当前文档、独立 Gallery/Spatial/Dual/Overlay/Mapping 原型；核对实际数据边界。
2. 用户确认后：最小扩展现有投影的来源/安装几何字段；通过 Adapter 接现有页面，保持原详情抽屉与权限。
3. 资产资料补齐后：型号素材、准确 front/rear 面板、0U 侧挂与实装确认；缺一项则只开放对应受支持细节。
4. 有一对合格快照后：Planned vs As-built 正式 Diff；遥测单独满足证据条件后启用相应 Overlay。

验收：Gallery、空间、剖面与 Dual 的同一资产 U/高完全相同；并集计数不重复；筛选、选择、前后同步、URL 恢复、Mapping 到端接/追踪有效；半 U/0U/越界/安装面冲突准确；未知面/深度/实时监控不伪装已知；草稿保存不改事实；差异前置缺失时不显示空对比页面；所有原型证据标记 synthetic。

V1 历史检查保留在 [旧验证报告](rack-visualization/validation.md)。当前 V2 检查与截图见 [空间工作台验证记录](rack-visualization/validation-v2.md)。只证明独立原型与几何行为，不证明生产页面、实际设备、三维碰撞、完整工程验收或千卡性能。

## 10. 当前缺口与设计决策

- **已决定**：安装面未知时双面占位参考、明确标注；深度未知保留开放边界；空间和双面互补而非互相替换；Power/Health/Thermal 不启用；空间“未占用”不等于“可安装”；Mapping 保留；不复制旧 fixture 的默认面作为正式事实。
- **待资料**：安装面/深度/full-depth、精确型号面板与端口锚点、PDU/供电关系、宽度/轨距、批准规划快照、现场实装记录、有效监控链。
- **需后续业务确认**：现场位置确认的责任主体/审批权限、何种证据可签署 As-built、旧规划版本选择方式。由现有 CMDB/交付审批体系补齐，不在原型中自造批准规则。
- **本阶段未实现**：生产 API 扩展、真实遥测、精确设备素材、可执行的规划/实装比较。未把这些能力标成已完成。

## 11. V2 设计决策记录

- **D-RACK-02**：综合 NetBox 的资产事实、Rackula 的坐标/编辑、Sunbird 的空间/连接/容量；不是选一家替换另一家。保持整体范围、精确双面、空间关系、端口细节四个层级。
- **D-RACK-03**：当前不使用 WebGL 增加无数据体积。React + SVG 的平行斜投影足以验证安装/选择/剖面交互；未来毫米几何齐备、明确需要自由相机时再评估渲染适配器。
- **D-RACK-04**：事实/设计/观测/验收/展示五层互不转正。所有基于未标定宽深的引导面为展示坐标，导出亦明确注明；不从视觉模型反写资产。
- **D-RACK-05**：当前 default 为单柜工程工作台，但 Gallery 与双面完整保留。用户要求“兼顾整体和细节”，因此单柜左侧保留所有机柜的分组范围索引；不是另一张伪楼面图。

## 12. 参考资料自审与本轮修正（2026-10-31）

| 依据与方法 | 发现的偏差 | 本轮修正 |
|---|---|---|
| Rackula 渲染/布局分离；NetBox 同一设备身份 | 正反视角主要换图，右栏业务目标没有改变 | Front 查设备与安装，Rear 查端口与连接；双面点击、端口操作与证据工作区同步，保持同一资产 ID |
| NetBox Device 的安装面与 full-depth 定义 | 反面全深度被误画为未知；背装深度从正面起算 | 共享 `faceProjection` / `sectionProjection`；明确安装面、对面占用、未知与 0U；背装几何从 rear 基准定位 |
| 数据事实层与展示层分离 | 草稿 U 被标为“登记位置” | 草稿与登记 U 并列，树/详情/导出/容量标识保持一致，未写入 CMDB |
| NetBox Visual Explorer 的关系与位置关联 | 跨柜连接落到本柜虚构位置；缺失端点画成完整线 | 柜外端点独立标注机柜/设备/端口并可定位；缺失端点虚线终止，不伪造第二端 |
| Heer / Shneiderman 的语义缩放与协调视图 | 缩略图文字难读；选择设备跳离当前详情上下文 | Gallery 只保留几何，文本在可读摘要；选设备保持 Tab，按面切换才改变业务焦点 |
| 确认规划与实际事实各自可追溯 | 缺两侧证据仍保留空对比页面 | 隐藏入口，保留 §4.4 分级比较与 Dual-State 设计，不生成伪差异 |

复用架构保持 `CMDB 投影 → 几何模型 → Gallery / Dual / Spatial SVG → 证据详情`。设备型号素材应挂在已有型号版本，当前仅标准资产轮廓和登记端口索引；未复制 Rackula 代码/运行时，未新建设备库或资产事实源。

本轮研究直接核对 [Rackula 碰撞计算](https://github.com/RackulaLives/Rackula/blob/main/src/lib/utils/collision.ts)、[Rackula 架构](https://github.com/RackulaLives/Rackula/blob/main/docs/reference/ARCHITECTURE.md)、[NetBox Device](https://netbox.readthedocs.io/en/stable/models/dcim/device/)、[Visual Explorer](https://netboxlabs.com/docs/visual-explorer/)、[Sunbird 可视化](https://www.sunbirddcim.com/product/data-center-visualization) 与 [Interactive Dynamics](https://idl.cs.washington.edu/files/2012-InteractiveDynamics-CACM.pdf)。它们分别约束几何、数据来源、空间表达与视图协同；不会因参考产品有照片或实时传感器就假定 Keel 也有。

必要验证及截图见 [本轮自审记录](rack-visualization/review-v3.md)。精确面板素材、真实遥测、合格规划/实装配对、完整毫米级空间验收仍未具备；没有声称完成生产数字孪生。
