# Keel Rack Visualization · 机柜可视化设计

更新：2026-10-31。状态：**产品设计与独立可运行原型，待用户评审；不是生产功能交付**。

本文件是本轮机柜可视化的设计入口，接替 `topology.md` 中“机柜冻结”的阶段性决定。四个拓扑原型保持原设计；正式 `/racks`、CMDB、数据库及业务 API 均未修改。现有业务规则继续以 PRD、RACK-DIAGRAM、DATA-DICTIONARY 和 OpenAPI 为准。

## 1. 定位与交付边界

用一套确定性几何模型，将 CMDB **已经登记且有来源**的机柜和设备关系自动投影为 Rack Gallery 与前后双面立面。两种尺度共享身份、U 区间、选择上下文与证据，不维护第二套资产库。

第一期核心：Gallery、Dual Elevation、Asset / Capacity / Compute、设备 Mapping 与来源追溯。规划与实装比较保留明确入口，但当前数据不足时只展示缺口；Power / Health / Thermal 不因字段名称存在就自动启用。

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

## 4. 信息架构与交互

### 4.1 Rack Gallery

```text
现有 Sidebar / 顶栏（保持）
机柜可视化                   保存 / 恢复视图
[机柜阵列 | 双面立面 | 规划/实装]        数据能力
[设备/型号/SN 搜索] [Pod]        叠加层：资产 / 空间 / 计算
范围摘要：机柜、登记 U、服务器、登记 GPU、交换机
┌ Pod 1：4 个缩略立面 ┐ ┌ Pod 2：4 个缩略立面 ┐
└ 柜号 / U / 数量    ┘ └ 同一几何模型       ┘
┌ Pod 3              ┐ ┌ Pod 4              ┐
紧凑图例：设备类别、未知面；不是实际机房坐标
```

- Gallery 使用同一 `layoutRack` 和 `Elevation`。缩略尺度降低标签密度，设备占位不变；不按设备数平均排布。
- 每柜显示名称、U 高、占用并集、未占用 U、服务器/GPU/交换机登记数量与“实装/健康未核验”。缺记录显示未知。
- 搜索设备、型号、SN、资产 ID 或机柜；命中设备后打开其所在柜并选择该设备。Pod 筛选写 URL。
- 不绘制假行列坐标；响应式阵列只是视图排版。
- 零匹配显示空状态，保留筛选。返回阵列保持查询上下文。

### 4.2 Precision Dual Elevation

```text
[返回] [柜号] [上柜/下柜]          [叠加层] [展示草稿] [SVG]
柜号 / Pod / 总U / 登记占位 / 未占用 / 设备 / GPU
┌ Front ─────── Rear ───────┐ ┌ 选中设备 / 稳定ID ┐
│ 同一 U 刻度，双面同步选择 │ │ 设备 | 资产       │
│ 未知面使用虚线登记参考   │ │ Mapping | 来源    │
│ 缩放 / 0U未定位托盘      │ │ 按当前资产加载    │
└ 同一位置，不复制资产     ┘ └ 来源与跳转        ┘
```

- 1440px+：画布为主、右栏 340px，1600px 以上留足双柜阅读宽度；窄屏上下堆叠。不要同时塞入一组完整机柜阵列。
- 一处选择同时高亮两面；右侧设备清单同样定位。使用精确 ID，不通过设备名模糊匹配。
- 当前安装面缺失时，两面显示**登记占位参考**并统一虚线提示。不是已知 Front+Rear，也不重复统计 U、设备或 GPU。
- Front / Rear 使用相同身份与 U 区间。精确面板未绑定时，仅绘资产轮廓、类别、型号与已有数量；没有虚构盘位、电源、风扇或端口数。
- 后面板中的点阵框表示“几何未绑定”，不作为实际接口。端口仅在右栏 Mapping 以登记名称清单呈现。
- U 方向按实际 U 数据成比例。**横向为固定阅读宽度，当前未投影可靠 mm 宽度，因此不是物理尺寸施工图**。后续有型号宽度与轨距证据才启用毫米等比。
- 双面缩放统一，超过容器可滚动；SVG 导出保留两面、数据性质、图例语义与选择样式。

### 4.3 设备详情与 Mapping

- 设备：柜内搜索，按 U 排序，型号、U 高、资产 ID；点击联动立面。
- 资产：已有型号、SN、登记 U、GPU/NIC/端口数量；安装面/深度/当前健康缺失明确标示。型号轮廓不是产品照片。
- Mapping：**当前设备 → 登记端口 → Cable → 对端设备/端口**。端口分页/搜索，选端口缩小关联范围；选线缆显示介质、长度、双端、来源与异常状态，可进入既有布线矩阵和链路追踪原型。
- 来源：规划登记、几何字段、缺失实装与实时证据分层展示；保留 synthetic 来源和参考时间，不用参考时间冒充采集时间。
- 不另建设备抽屉。后续生产实现仍调用已有 ServerDetailDrawer 五 Tab / NetworkDeviceDrawer 八 Tab；本原型使用隔离证据面板，避免加载真实 Provider。

### 4.4 Planned vs As-built

当前显示数据前提而非假 Diff：有规划登记，但缺明确批准的比较快照，且没有已确认实装位置。`生成差异` 禁用并说明原因；“查看数据依据”“查看登记立面”可操作。

未来前提：明确项目/范围、已批准规划版本 ID、实装快照 ID、各字段来源、确认人与时间，双方使用同一稳定资产 ID。比较设备集合、型号、rack_id、U 位与高度；结果为缺失、新增、移位、换型及无法比较。缺测不判一致。设备换件保留旧身份，不通过型号和名称猜匹配。

批准规划和现场事实由各自领域负责；CMDB 关联查询。展示草稿、未审核 BOM 与当前位置的复制都不能作为比较基线。原型不生成假时间、批准人或版本。

### 4.5 Smart Overlays

| 层 | 启用条件 | 当前行为 | 不能推导 |
|---|---|---|---|
| Asset | 存在授权设备身份与类别 | 服务器淡蓝、交换机淡青；选中紫色 | 设备健康 |
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

三层细节：Gallery 占位 → Dual 标准资产轮廓 → 经确认型号面板。选择哪个层由素材与几何能力决定，不由缩放触发虚构硬件。

未来素材注册建议记录 model_revision、manufacturer、part_number、front/rear、SVG viewBox、anchor-to-port-template、license、source URL、revision/hash。素材不保存实例 SN、MAC、健康或实际端接；实例数据从 CMDB 叠加。端口模板与实例端口必须能明确映射，否则仅展示静态面板并关闭热点。

服务器、交换机、配线架、PDU 与挡板的绘制可通过可复用领域组件扩展；本期只画数据中实际存在的类型。未引入图片资产，不新增 UI 框架或图形依赖。

## 6. 视觉与状态规范

继承 DESIGN-SYSTEM 字体、紫色主操作、背景/边框/间距令牌，新增样式只作用于 `.rack-viz` 与 `rv-*`。不改全局 CSS 或其他四个拓扑原型。

- 页面标题 20px；柜/设备标题 15–17px；正文 12px；证据/来源 10–11px；ID 和 U 使用等宽。缩略 SVG 不承担完整文本阅读。
- 控件高 28–34px；画布/证据栏间距 16px；无多层卡片阴影。Gallery 采用两组 Pod 并排，每组四柜，窄屏顺序折叠。
- U 标尺每 U 可读，选择有描边和文本反馈；颜色是类别/选择，不伪装健康。
- 已知占位实边、未知面虚线；未占用细斜线；冲突红色并有文字。维护态若有授权有效状态则遵循现有蓝色暂停规范，与健康分开。
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
| Rack Gallery | http://localhost:5181/rack-detail |
| Dual Elevation | http://localhost:5181/rack-detail?view=dual&rack=rack-0001 |
| Compute | http://localhost:5181/rack-detail?view=dual&rack=rack-0001&overlay=compute&asset=srv-0001 |
| Capacity | http://localhost:5181/rack-detail?view=dual&rack=rack-0001&overlay=capacity |
| Mapping | http://localhost:5181/rack-detail?view=dual&rack=rack-0001&asset=srv-0001&tab=mapping |
| Planned / As-built 数据前提 | http://localhost:5181/rack-detail?view=compare&rack=rack-0001 |

实现位置：[prototype/pages/rack-detail.tsx](topology/prototype/pages/rack-detail.tsx)、[共享几何](topology/prototype/rack/geometry.ts)、[数据适配](topology/prototype/rack/model.ts)、[SVG](topology/prototype/rack/Elevation.tsx)、[局部样式](topology/prototype/rack/rack.css)。

```bash
node docs/topology/prototype/run.mjs        # 独立 5181
node docs/topology/prototype/run.mjs check
node docs/topology/prototype/run.mjs build
```

URL 恢复 `view/rack/asset/port/cable/tab/overlay/q/pod`；从旧拓扑携带 rack/asset 进入时自动开 Dual。顶栏保存视图复用已有原型行为；草稿隔离于 `keel.topology.prototype.rack-v2.*`。没有真实 CMDB 网络请求或写入。

## 9. 实施顺序与验收标准

1. 本期：审核当前文档、独立 Gallery/Dual/Overlay/Mapping 原型；核对实际数据边界。
2. 用户确认后：最小扩展现有投影的来源/安装几何字段；通过 Adapter 接现有页面，保持原详情抽屉与权限。
3. 资产资料补齐后：型号素材、准确 front/rear 面板、0U 侧挂与实装确认；缺一项则只开放对应受支持细节。
4. 有一对合格快照后：Planned vs As-built 正式 Diff；遥测单独满足证据条件后启用相应 Overlay。

验收：Gallery 与 Dual 的同一资产 U/高完全相同；并集计数不重复；筛选、选择、前后同步、URL 恢复、Mapping 到端接/追踪有效；半 U/0U/越界/安装面冲突准确；未知面/深度/实时监控不伪装已知；草稿保存不改事实；差异前置缺失时不能生成结果；所有原型证据标记 synthetic。

实际检查结果与截图见 [验证报告](rack-visualization/validation.md)。只证明独立原型与几何行为，不证明生产页面、实际设备、三维碰撞、完整工程验收或千卡性能。

## 10. 当前缺口与设计决策

- **已决定**：安装面未知时双面虚线参考；Power/Health/Thermal 不启用；空间“未占用”不等于“可安装”；Mapping 保留；不复制旧 fixture 的默认面作为正式事实。
- **待资料**：安装面/深度/full-depth、精确型号面板与端口锚点、PDU/供电关系、宽度/轨距、批准规划快照、现场实装记录、有效监控链。
- **需后续业务确认**：现场位置确认的责任主体/审批权限、何种证据可签署 As-built、旧规划版本选择方式。由现有 CMDB/交付审批体系补齐，不在原型中自造批准规则。
- **本阶段未实现**：生产 API 扩展、真实遥测、精确设备素材、可执行的规划/实装比较。未把这些能力标成已完成。
