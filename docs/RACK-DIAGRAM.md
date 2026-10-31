# Rack Diagram — Keel 自动机柜图生成规范

> 目标：从已导入并确认的 BOM/CMDB 数据自动生成可交互、可追溯、可导出的机柜正反视图。前端先用统一 Mock API 完成完整交互，再逐项接入 Go 后端。

## 1. 产品边界

- Rackula 只作为现代画布交互参考；RackView（`kursatbal/rackview`）作为真实机柜交付、端口/线缆追溯和现场核对参考；Keel 自己实现 BOM 驱动的生成、规则校验、版本差异和 CMDB 关联。不得直接照搬他人代码或把参考项目当作运行依赖。
- 图是已确认数据的可视化视图，不是第二套资产事实源。设备身份、机柜位置、U 位、设备高度、端口和线缆关系以 BOM/CMDB 为准。
- 区分 `planned`（规划建议）、`review_required`（待确认）、`approved`（已确认布局）、`as_built`（现场核实）。导入资料没有明确 U 位或设备高度时，不得 AI 猜测并标记为实际布局；只能生成待审核候选或标记缺失。
- 首期实现 2D SVG 机柜正面/背面、多机柜并排、U 位刻度、设备占位、设备详情、线缆/端口关系查看、自动布局、人工调整、撤销/重做、版本 diff、PNG/SVG/PDF 导出。3D、实时热力/功耗仿真不属于首期范围。

## 2. 从真实 Excel 导入识别机柜图所需数据

依据 `TX B300 Layout-new.xlsx` 的实际工作簿结构（33 个 Sheet），重点解析：

- `Device Information`：资产编号、设备角色/类型、品牌、型号、设备 U 数、SN、机房/机柜位置等资产属性；公式字段需要区分公式与缓存值。
- `Layout`、`Layout 110-120`：机柜/节点分布与布局网格候选。必须保留行列坐标、合并区域和原始单元格定位，不能只转成扁平表。
- `工作量`、`IT Power`、`IT Power (2)`：数量、U 数、功耗和每柜数量等汇总/容量参考；这些是交叉核验数据，不直接作为设备实际落位的唯一依据。
- `P2P design`、`P2P design-1k`、`Roce`、`Eth`、`GPU`、`CPU`、`CPU2`、`MGT CPU`、`SPX`、`ETH-2k`、`GPU-2k-*`、`Switch 2K`、`S9825-64D`、`SN5610`、`备线`：以不同格式记录设备端口、网络类型、A/B 端、端口号、光模块/线缆型号与长度等；用于连接追溯和端口关系展示，不可直接推导缺失的机柜 U 位。
- `Device OOB`、`Device OOB 2`：管理地址与设备命名等信息；用于匹配资产，不用于推断物理 U 位。
- `金石`：另一个设备/线缆记录区，可能包含敏感账号字段。账号、密码、Token 等不得进入 LLM 请求、Mock fixture、机柜图标签、CMDB 普通字段或导出图。

工作簿可能包含重复/版本化 Sheet、公式、空白延续行、跨表公式引用、不同粒度的汇总和端口明细。解析器必须为每条候选数据保留 `source_file/sheet/cell/row/column/header_context`；公式不可执行，优先读取 Excel 缓存值，缺失时标 `formula_value_unavailable` 并进入问题清单。不能将重复 Sheet 简单相加；必须先识别业务版本、覆盖关系和数据来源，无法判定时由用户确认。

## 3. 自动生成引擎

输入：项目、已确认 BOM revision、CMDB 实体与关系、DeviceModelRevision（含 `u_height`/外形/前后深度/端口定义）、机柜高度与位置、已确认的人工布局覆盖。

生成步骤：
1. 解析机柜实体和位置；用稳定 `item_uid` 关联设备，不使用显示名称作为主键。
2. 获取设备型号能力。设备高度优先使用型号库的已批准 `u_height`；其次使用明确来源的 BOM `u_height`；单位/来源不明则产生 issue，不猜测。
3. 读取已确认的 rack/U 位。对未指定位置的设备，可按项目模板中显式批准的 placement policy 生成 `planned` 建议；没有规则时放入“未分配设备”列表，不自动占用 U 位。
4. 校验 U 位范围、设备高度、占位冲突、机柜容量、前/后视图占位、同一设备跨面一致性，以及线缆端点/端口引用完整性。供电/重量/散热只有在型号与基线数据完整时才做校验；缺数据显示 `unknown`，不编造。
5. 生成可重复的 `RackDiagramRevision` 快照，记录 BOM revision、型号库 revision、placement policy revision、生成器版本、来源和校验摘要。
6. 与上一个版本比较，输出新增/移动/删除/属性变化及冲突；人工覆盖默认保留。重生成不能静默覆盖用户调整，必须提供“保留调整 / 查看差异 / 明确重置”选择。
7. 发布后将图版本、设备位置变更和来源事件关联 CMDB；设备位置权威字段按审批结果更新，不能只改 SVG 图层。

## 4. 交互规范

- 页面采用“项目/机房/机柜选择器 + 主画布 + 右侧详情面板 + 底部问题/变更摘要”的工作区。
- 画布：前/后视图切换、U 位标尺、缩放/平移、适配画布、选择设备、高亮占位、冲突标记、空 U 折叠（可展开）、多机柜导航。
- 设备交互：点击查看资产/SN/型号/U 位/高度/状态/端口/线缆/来源；拖动只产生草稿位置修改；放置前预览冲突；键盘可完成选择和移动；支持撤销/重做。
- 自动生成：选择 BOM 版本 → 生成预览 → 查看未分配/冲突/缺失属性 → 查看差异 → 人工确认 → 发布版本。不得点击“生成”就覆盖已发布布局。
- 连线：从设备端口查看关联线缆、对端设备/端口、介质/长度和来源；线缆密集时默认不在主画布绘制全部跨柜连线，使用按设备/端口筛选、高亮单链路或独立连接视图，避免视觉混乱。
- 导出：PNG/SVG 用于图纸，PDF 用于交付文档；导出包含项目、机房、机柜、布局版本、数据时间、状态（planned/approved/as_built）和图例。敏感字段默认不导出。
- 大规模场景：按机房/机柜分页或分区加载，不一次渲染数千机柜和全部线缆；单机柜 SVG 应可独立渲染和导出。

## 5. 数据模型

- `Rack`: `rack_id`, `project_id`, `site_id`, `room_id`, `rack_name`, `u_height`, `position`, `source_item_uid`, `status`。
- `RackPlacement`: `placement_id`, `rack_id`, `item_uid`, `u_start`, `u_height`, `face`, `placement_status`, `source`, `manual_override`, `source_ref`。
- `RackDiagram`: `diagram_id`, `project_id`, `scope`, `current_revision_id`, `status`。
- `RackDiagramRevision`: `revision_id`, `diagram_id`, `bom_revision_id`, `model_catalog_revision`, `placement_policy_revision`, `generator_version`, `content_hash`, `validation_summary`, `created_by`, `created_at`。
- `RackDiagramIssue`: `issue_id`, `revision_id`, `item_uid`, `rule_id`, `severity`, `source_ref`, `message`, `resolution_status`。
- `RackLayoutEvent`: 设备放置/移动/移除/生成/发布/导出事件，带 actor、before/after、原因、request_id，并投影到 CMDB timeline。

`RackPlacement` 仅表示项目布局与经批准的物理位置关系，不得复制/取代 `BOMItem` 的设备身份。对于 full-depth 设备，前后视图占位必须关联同一 placement；不允许前后面各放一台逻辑设备。

## 6. API 契约

- `GET /projects/{project_id}/racks`：机柜列表及容量摘要。
- `GET /projects/{project_id}/racks/{rack_id}`：机柜详情、当前布局版本和占用信息。
- `GET /projects/{project_id}/rack-diagrams`：图纸列表/状态/当前 revision。
- `POST /projects/{project_id}/rack-diagrams/generate`：基于冻结 BOM revision 和规则版本创建异步生成任务，返回 `202` + `generation_job_id`。
- `GET /projects/{project_id}/rack-diagrams/generations/{job_id}`：生成任务状态、阶段、进度、问题计数。
- `GET /projects/{project_id}/rack-diagrams/{diagram_id}`：图纸元数据与 revision。
- `GET /projects/{project_id}/rack-diagrams/{diagram_id}/revisions/{revision_id}`：读取可渲染布局数据。
- `POST /projects/{project_id}/rack-diagrams/{diagram_id}/revisions/{revision_id}/validate`：重跑确定性校验。
- `POST /projects/{project_id}/rack-diagrams/{diagram_id}/revisions/{revision_id}/placements`：创建/更新草稿 placement；用 `If-Match` 防止并发覆盖。
- `POST /projects/{project_id}/rack-diagrams/{diagram_id}/publish`：发布指定 revision；必须无 BLOCK，且关键位置缺失已解决或明确按规则保留为 planned。
- `GET /projects/{project_id}/rack-diagrams/{diagram_id}/export?revision_id=...&format=svg|png|pdf`：导出指定 revision。

所有接口均按项目 RBAC 授权、返回 request_id、记录审计。生成/导出为长任务时返回任务 ID；不得在请求中直接生成大图导致 HTTP 长时间阻塞。详细 Schema 以 `openapi.yaml` 为机器契约。

## 7. 验收

1. 使用真实工作簿的脱敏测试副本，正确识别 `Device Information`、`Layout`、`Layout 110-120` 与端口/线缆 Sheet；保留来源位置。
2. 已有明确机柜/U 位时生成与输入一致的布局；缺失位置时只产生待规划项，不猜成实际布局。
3. U 位越界、重叠、未知高度、重复设备、无效机柜引用和不完整线缆端点均生成可定位问题。
4. 重复生成相同输入和相同规则得到相同内容摘要；规则/输入变化产生新 revision 和可读 diff。
5. 人工拖动后再次生成，默认保留人工覆盖并显示差异；只有明确重置才清除覆盖。
6. 正反面、全深设备、多机柜、空 U 折叠、缩放、键盘操作、撤销/重做和 SVG/PNG/PDF 导出可用。
7. 所有图纸版本、位置修改、发布和导出在 CMDB timeline/审计中可追溯；敏感凭据在图、Mock、API 普通响应和导出中为零。
8. Mock 与真实 API 使用同一 OpenAPI 类型；前端阶段无需后端即可演示生成、审核、修改、差异、发布和导出状态。

## IA v2.4 实施增补

新增跨授权项目机房视角/机柜分布与健康利用率，单柜SVG编辑器保持独立；确定性生成、来源、高度、不覆盖已发布版本与版本diff规则不变。


## IA v2.4 Mock 前端实施记录

导航树、完整页面清单、路由/权限/作用域/加载策略、五/八Tab抽屉与API对照统一见 [IA24-IMPLEMENTATION-MATRIX.md](IA24-IMPLEMENTATION-MATRIX.md)。原FR-0…24、门禁、BOM与维护态规则保留。当前实现为前端Mock，不表示已实现Go/真实设备/真实AI。权威种子为 `fixtures/demo-full-64-b300/demo-seed.json`；原包83集合，补齐与规范化后集合数量以当前manifest为准，主项目硬件数量不变。结构检查、数据修复、契约与交互结果以IA24-*报告为准，无法采集/缺少证据保持unknown或阻断。


## 当前六阶段主链路修正

本轮用户明确废弃数字门禁作为主链。当前唯一业务顺序、状态机、逐目标放行、安全交接、独立BOM菜单与API约束以[DELIVERY-LIFECYCLE.md](DELIVERY-LIFECYCLE.md)为准。历史编号只按[DELIVERY-MIGRATION.md](DELIVERY-MIGRATION.md)追溯，不得用于解锁。验机失败关联统一Issues并定向复验；NCCL不反写验机；清理/归档/客户签署齐全后才进入维护态。NP仍是网络内部断言。当前仅前端Mock与契约实现，没有真实设备后端。


## 单柜详情工作区增补

固定单柜窄画布、左侧层级导航和右侧八Tab为本轮布局要求；交互、API、元数据、Mock来源及待执行验收见[RACK-WORKSPACE.md](RACK-WORKSPACE.md)。资产/位置和既有安全模型不变。右侧按需加载，不以缺失指标显示正常。当前构建/浏览器验证按用户安排等待统一执行。


## 2026-10-31 数据中心可视化增补

新增运维 `/topology` 与真实 CMDB 机柜工作台；投影接口、布局/事实边界、asset.relocate 审核发布、验证记录和未完成项统一见 [DATACENTER-VISUALIZATION.md](DATACENTER-VISUALIZATION.md)，HTTP schema 见 openapi.yaml。沿用原资产身份和权限，不把计划连接或 synthetic 数据视为真机验证。
