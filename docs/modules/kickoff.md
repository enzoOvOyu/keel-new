# 智能开局 Kickoff · 当前实现定版

基准：主环境5180前端/8081 API为 `fdd86dd`；历史资料理解已部署，定向真实HTTP检查通过。本页以已实现结果定版，不再沿旧 BOM 四步/五步方案开发。

## 产品与闭环

资料接入 → 工程事实与输入建模 → 对象核对 → 有效范围确认入库 → CMDB 回执与版本追溯 → 持续补充。

核心是把有来源的工程对象正确录入 CMDB。布线计算、功率、完整几何和网络配置可以后补，不作为所有资产的统一前置门槛；身份、端点等关键冲突只阻断受影响范围。规划数量不能伪造成实物设备。

## 页面与交互

入口 `/p/:projectId/kickoff/overview`，统一 Shell、ProjectContext、RBAC。旧 BOM 菜单、页面与 URL 已退役，旧地址显示应用404，不重定向；历史后端和资产不删除。

| 页面 | 已实现行为 |
|---|---|
| overview | 工作区摘要、真实入库版本、缺口/建议动作、活动；机柜/Fabric/装机区域链接既有模块，不伪造后置完成 |
| sources | MinIO 资料、摘要复用、附件移除；已上线显式复用历史资料、五项独立识别状态、区域来源/标准对象分页及来源 Drawer；新文件仍等待 Agent |
| review | 已上线五栏分页与来源Drawer；已上线身份去重、端口/速率分列、工程异常及影响、全量计数快捷筛选与URL属性筛选，五栏定向HTTP检查通过；Compact Workbench紧凑布局及10/25/50/100条分页已发布 |
| publish | 服务端完整范围预览、冻结闭包、Outbox及CMDB回执；本轮调整范围说明/设备选择、右侧增补记录与同版本覆盖防重，确认按钮和弹窗保持原样，已发布并完成真实页面只读核查 |
| models/facility | 场地/机房/机柜、型号几何、设备位置；缺高度/容量保持未知 |
| models/network | 网络域/角色、管理接入、接口、地址/ASN、协议/策略的版本化工程输入 |
| models/provisioning | 启动身份、镜像、磁盘与架构等装机准备输入 |
| models/cabling | 路径/规格/规则输入、确定性路径/引线/余量/标准长度计算；无规则不造线长 |
| models/power | 功率、回路和容量输入；逐柜规划负荷、剩余容量、缺失与超限报告 |

23 张输入表支持编辑、XLSX/CSV 预览/导入导出和未知列反馈。保存、计算、分页和 Drawer 按需加载；summary 不携带全部端口/连接，切换修订后清旧分页，不混版本。预览按服务端完整闭包计算，不以当前浏览器页作为总范围。

### 数据核对紧凑布局（已部署）

页头沿用其他Kickoff页面的标准标题字号与内容起始位置。三项真实入库统计集中在右侧核对事项上方的小卡片，详细含义放入Tooltip。五个实体Tab采用清晰的主切换样式，与次级状态筛选区分；全部核对统计保留；搜索、类型、型号、机柜、来源和导出共用单行工具栏。表格行高目标58–64px，区域滚动并固定表头；侧栏问题可滚动，长文字不撑开列。

表格最多显示两个问题标签，其余通过可操作的 `+N` 菜单查看，保留相应处理入口。原有详情、来源、排序、筛选、选择、确认及导出不变。每页10/25/50/100条保存在前端URL `page_size`，转换成既有API的limit/offset；变更数量回到第一页，筛选和选择保留。没有修改业务规则、模型、HTTP契约或后端。

定向交互与视觉证据见[REVIEW-COMPACT-CHECK](../kickoff/REVIEW-COMPACT-CHECK.json)。核对页布局证据来自1e5b298，真实MM页面标准标题、右侧三项统计卡、实体切换、默认10条/切换25条及60px行高检查通过；当前前后端fdd86dd已包含这些功能。

### 入库与版本页

主区域只确认设备范围：条件说明并入范围卡片；支持全选、取消、清空和表头部分选中。列表身份去重，独立展示类型/型号/真实入库情况，详细核对通过入口返回上一页。已有回执的设备不默认选中，但可显式选择增补；等待回执和身份冲突不可选。原确认按钮与确认弹窗保持原样，成功后清理已完成选择。右侧原条件卡位置展示版本、时间、确认人、说明、数量和实际回执，保留详情/差异/导出与后续完善入口。

后端在创建新发布前检查同项目/工程修订/全部输入版本的已覆盖范围，拒绝重复创建记录或Outbox；不是完整字段级差异计算。证据见[PUBLISH-SCOPE-CHECK](../kickoff/PUBLISH-SCOPE-CHECK.json)。

## 事实、版本与集成

- 复用 `eng_entity`、类型化工程版本、稳定资产 ID 和已有 ProjectEngineering 投影，不建第二 BOM/资产身份。
- 源文件在 MinIO；工程输入不可变版本化。范围发布冻结 device/port/component/connection/placement/facility/location 引用，经同事务 Outbox 幂等投影，读取 CMDB 回执才标 published。
- 已知机柜/start-U 可独立以 planned 发布，缺高度/柜容量仍为 needs_input，不计完整几何占位。后续输入变化不改写历史回执。
- 历史复用限同项目不可变资料和已保存候选，保留原证据/修订；不再次调用 AI，不等于新资料识别能力。
- 网络/装机/电力输入当前保留版本和规划关联；这不等于 Fabric 所需完整 Published NetworkPlan，更不等于设备已配置。

## API 与代码入口

[当前 API](../api/kickoff.md)；[OpenAPI](../openapi.yaml) 的 Kickoff*；[真实登记](../api/real-api-registry.json)。

- [页面](../../dev/src/features/kickoff)、[Provider](../../dev/src/lib/api/kickoff)。
- [HTTP 入口](../../dev/backend/cmd/api/kickoff.go)、[发布事务](../../dev/backend/cmd/api/kickoff_publication.go)、[领域计算](../../dev/backend/internal/kickoff)。
- 相关迁移048–051；旧工程存储复用边界见[工程契约](../api/engineering.md)。
- 批准的详细页面/输入基准：[原型说明](../../kickoff/OPTIMIZATION.txt)、[接入边界](../../kickoff/CONTRACTS.txt)。原型本地 Provider 不是正式后端。

历史资料理解按 source/head 修订在 MinIO 保存可重建派生报告与区域预览，不修改工程输入、审核、发布或 CMDB。结构分块复用 WorkbookIR；精确身份和双端连接匹配只作为候选，不能代替原始字段映射或人工确认。区域语义和字段覆盖独立报告，未识别内容保留并显示缺口。

## 已验证和剩余

| 已有证据 | 可以确认的范围 |
|---|---|
| [BACKEND-CHECK](../kickoff/BACKEND-CHECK.json)、[UI-CHECK](../kickoff/UI-CHECK.json) | 定向隔离集成、9页/23表/交互协议回放；不能当作全部真实页面写入验收 |
| [HISTORY-UNDERSTANDING-CHECK](../kickoff/HISTORY-UNDERSTANDING-CHECK.json) | 本轮只读数据库/派生 MinIO 验证：54结构区域、29,583候选定位单元格、7,154双端连接候选；明确字段证据0，workspace修订不变；已部署，真实HTTP报告/复用/两类预览通过 |
| [MM-HISTORY-CHECK](../kickoff/MM-HISTORY-CHECK.json) | 历史候选入库：971设备、14308端口、7154双端连接；967已知位置关联59柜；精确几何占位0 |
| [DEPLOYMENT](../kickoff/DEPLOYMENT.json)、[PERFORMANCE-RESULT](../kickoff/PERFORMANCE-RESULT.json) | 12af6e4 为当前部署（五栏核对证据见 REVIEW-IMPROVEMENT-CHECK），af63cb3为历史资料理解证据；来源报告/复用/分页及过期版本拒绝为本轮HTTP证据，其他性能与页面为686e9ea历史证据；非 P95/FPS/并发验收 |

新 Agent 未开发，analyze 明确503；历史候选组件0、4台通用设备待补类型，不声称新资料覆盖。布线 needs_input、电力 partial、缺几何继续如实显示。建议1s/500KB摘要和全部拓扑2s目标未全部达到。以上为后续明确缺口，不自动扩展本次任务。
