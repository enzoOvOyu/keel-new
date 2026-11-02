# keel Design System — React + shadcn/ui / 作业平台 Kestra 原生 UI

> 作业平台特例（用户确认，2026-11-01）：本文件§10为该模块的当前规范，优先于本文件其他章节关于“所有组件使用shadcn”“只借鉴Kestra、不复制源码”和旧Flow线框的描述。其他模块继续沿用既有设计系统。完整页面范围和LC-01～14见 [Kestra原生替换方案](flows-redesign/KESTRA-UI-REPLACEMENT.md)，实施状态见 [作业平台实施记录](flows-redesign/implementation.md)。设计规定不代表功能已实现。

## 1. 技术约束

- React + TypeScript + Vite；shadcn/ui 为基础组件源，Tailwind CSS 管理样式。
- 路由：React Router；服务端状态：TanStack Query；表单：React Hook Form + Zod；图表按项目现有方案统一；大型表格使用虚拟滚动。
- 除§10作业平台特例外，业务组件基于shadcn/ui组合，不另造基础组件体系。作业平台内容区使用固定版本Kestra原生组件，React外壳保持不变；源码纳入仓库，避免运行时依赖外部CDN。
- 图标使用 Lucide；不使用 emoji 作为功能图标。色彩仅表达状态，不能只靠颜色传递含义。

## 2. 视觉原则

1. 工程控制台优先信息密度、层级清晰、操作可预测；避免营销型渐变、大面积装饰卡片、过多圆角和无意义留白。
2. 页面使用统一容器、间距、字号、边框、阴影、状态色和交互态 token；不在页面内散落硬编码色值。
3. 主要动作每个区域最多一个 primary button；危险动作使用 destructive variant 并二次确认。
4. 表格优先用于设备、端口、任务、问题等结构化信息；详情使用侧栏/抽屉，复杂配置使用分步表单和固定摘要。
5. 长任务必须显示状态、进度、开始时间、耗时、阶段、失败原因和可执行下一步；不得只显示无限 loading。
6. 状态标签使用统一枚举映射，并同时显示文字和图标/颜色。

## 3. 全局布局

- 左侧 Sidebar：项目切换、一级导航、折叠状态持久化；当前项目始终可见。
- 顶栏：面包屑、项目/批次上下文、全局搜索、通知、用户菜单。
- 主区：标题 + 简短说明 + 主要操作 + 过滤区 + 内容区。
- 页面状态统一：loading skeleton、empty state、error/retry、permission denied、stale data。
- 所有列表筛选、排序、分页、选中 tab 尽可能写入 URL query，便于刷新与分享。

## 4. 信息架构

- 总览
- 项目与交付流程
- BOM 与开局导入
- 网络底座：设备、基线/模板、ZTP、配置闭环、线缆校验、网络健康
- 服务器交付：PXE、资产、验机门禁、Mapping、批量配置、连通性、NCCL
- 作业流：Flow、执行记录、介质库、参数目录
- 故障工单
- 监控与告警
- 机柜图自动生成
- 型号库与项目模板
- CMDB
- 平台管理：用户权限、审计、系统设置

## 5. 页面规范

### 5.1 BOM/开局导入

步骤条：`上传文件 → 解析与映射 → 校验 → 人工审核 → 提交预览 → 完成`。

- 上传区明确支持格式、大小、模板版本和安全限制；显示文件 hash、上传人、时间。
- 解析阶段显示 Sheet、候选表区、识别进度和失败原因；不以模型思考文字作为 UI。
- 映射审核采用三列/分栏视图：原始资料（Sheet/单元格）｜标准字段和值｜校验/置信提示。支持点击问题定位源单元格。
- 问题列表支持 BLOCK/WARN/INFO 筛选、按文件/行/字段搜索、批量处理安全的低风险项、导出 CSV。
- BLOCK 数量大于 0 时提交按钮禁用；WARN 未确认时显示明确影响与确认责任；关键链路/设备身份歧义不可批量放行。
- “应用为模板规则”必须单独确认，并展示影响范围、版本差异和审批状态；AI 建议不自动保存成已批准规则。
- 提交前显示批次号、模板版本、文件摘要、物料计数、变更 diff、阻断数/警告数；确认提交后展示标准 BOM 版本。

### 5.2 BOM 资产列表

虚拟化数据表；固定关键列（业务码/类别/型号/序列号/位置/状态/来源批次）；支持列显隐、筛选、排序、导出。详情抽屉显示身份、关系、来源、版本历史、审计和相关工单。不要把所有字段挤进表格。

### 5.3 网络设备与端口

列表显示 hostname/SN/角色/机柜/管理 IP/NOS/ZTP/配置同步/健康/最后采集。详情抽屉 Tab：基础信息、端口、配置版本、配置事件、运行状态、无损链路、光模块、日志。Tab 懒加载；异常优先，原始证据可下钻。

### 5.4 交付总览与门禁

显示网络阶段和 六阶段交付主链路 的独立状态、前置断言、证据新鲜度、责任角色和阻断原因。状态四值统一：PASS/FAIL/BLOCKED/WAIVED；WAIVED 必须显示批准人、原因和有效期。

### 5.5 执行详情

Overview（阶段时间线/汇总）、Targets（目标矩阵）、Logs（实时/历史日志）、Outputs（产物/渲染变量）、关联（BOM/工单/上游任务/版本快照）。大型目标表虚拟滚动；日志支持过滤、复制和定位，不默认加载全部历史日志。

### 5.6 Flow/配置变更/PushJob

使用明确的“目标范围 → 参数快照 → 预检查 → diff → 审批 → 执行 → 验证”步骤。危险操作显示目标数、版本、回滚策略和不可逆影响。执行按钮不得与预览按钮混淆。

## 6. 组件规范

- `Button`: primary/secondary/outline/ghost/destructive；文字动词明确（导入、校验、提交、审批、下发、回滚）。
- `DataTable`: server-side pagination/filter/sort；选择行后显示批量工具栏；危险批量操作再次确认目标数。
- `StatusBadge`: 由后端状态枚举映射，统一文案、颜色、图标。
- `IssuePanel`: 严重级、规则 ID、定位、原因、建议、责任人和处理状态。
- `SourceCellViewer`: 原始 Sheet/坐标/表头/值和对应标准字段对照。
- `DiffViewer`: 结构化字段 diff，标注新增/修改/删除和来源。
- `EvidenceList`: 来源、采集时间、有效期、规则版本、能力状态。
- `ConfirmDialog`: 说明动作、影响范围和回滚限制；不得用通用“确定吗？”替代。
- `EmptyState`: 说明为什么为空和下一步动作；不只显示插画。

## 7. 交互和无障碍

- 表单有标签、帮助文本、字段级错误和提交级错误；键盘可操作，焦点可见。
- 异步任务采用轮询或 SSE，页面离开不取消已提交后台任务；用户可重新进入查看。
- 表单修改未保存时提示；重复点击提交需幂等。
- 不用 hover 作为唯一操作入口；危险动作不只靠颜色区分。
- 断网/接口失败保留可恢复上下文，并显示 request_id。

## 8. 前端质量门槛

- 统一 TypeScript API types 从 OpenAPI 生成或校验，禁止手工维护冲突的重复状态枚举。
- 所有主要页面覆盖 loading/empty/error/permission/stale 状态。
- 关键路径 E2E：上传导入、修正问题、提交 BOM、审批配置、运行任务、处理工单。
- 不为装饰性变化引入新依赖；组件优先复用 shadcn/ui 与仓库现有组件。

## 9. 视觉验收与参考实现

### 9.1 参考源（作业平台按§10直接复用Kestra，其余模块学习交互）
- Kestra UI 总览：<https://kestra.io/docs/ui>；重点学习左侧导航、Flows/Executions/Logs 分区、任务执行状态、Topology/Gantt、日志过滤与产物查看。
- Kestra Execution：<https://kestra.io/docs/workflow-components/execution>；重点学习执行/任务状态、重试、Outputs 与运行生命周期。
- shadcn/ui Blocks：<https://ui.shadcn.com/blocks>；优先参考 Sidebar、Dashboard、Data Table、Form、Dialog 的组件组合方式。
- shadcn/ui Data Table 指南：<https://ui.shadcn.com/docs/components/data-table>；列表筛选、排序、分页、列可见性与行选择按统一模式实现。

### 9.2 不可接受的页面表现
- 不得所有页面都是相同的“标题 + 4 张统计卡 + 大表格”模板。
- 不得大量无意义留白、过粗/过细字体、无层级的灰色文字、过多渐变、装饰性环形图或无业务意义的卡片。
- 不得使用没有动作的按钮、没有反馈的批量操作、仅 hover 可见的关键操作、只有颜色没有文字的状态。
- 不得把所有字段塞进一张宽表；用详情抽屉、分组 Tab、结构化 diff 和源数据下钻承载复杂信息。

### 9.3 交互线框必须先于细节开发
每个主页面在实现前至少用 Mermaid 或 ASCII 画出页面结构和主要交互路径，纳入 PR 或 `docs/ui-flows/`。最少包含全局导航、BOM 导入、机柜图、CMDB 详情、Flow 列表、Execution 详情、网络设备详情、六阶段交付主链路 门禁、工单详情。不得只用文字说“现代、美观”。

### 9.4 视觉验收
- 以 1440×900 桌面为主视口；关键页面检查 1280px 宽布局与窄屏降级。
- 同一页面必须检查正常/空/加载/错误/权限不足/部分失败/数据过期状态。
- 对表格密集页检查首屏信息密度、列宽、行高、固定列、筛选区和操作位置；对执行页检查状态、时间、耗时、频率、日志和失败动作是否无需猜测即可理解。
- 提交 UI 变更时附关键页面截图或浏览器检查结果；不能只报告“样式已优化”。

## 10. 作业平台：Kestra 原生 UI 实施硬规则

### 10.1 当前产品决策与权威依据

- 产品名称固定为 **作业平台**。Flow、Execution是资源术语，保留`/flows`及既有API命名兼容，不把整个模块重新命名为Flows。
- 用户已明确要求**直接复用Kestra开源前端组件、页面组织和UI/UX**，不是只借鉴颜色，也不是用React/shadcn重新画一套相似页面。不得再次以旧文档“不复制源码”为由退回自制界面。
- 固定上游为Kestra `v2.0.7`、commit `49b62a2c820d37e98a1c7b63aa35fd83f960d4ec`。只复用Apache-2.0开源内容，保留LICENSE、来源和修改说明；不将上游locked/demo企业入口冒充可用功能，不替换Keel品牌。
- [替换方案](flows-redesign/KESTRA-UI-REPLACEMENT.md)规定逐页替换、生命周期LC-01～14和上游源组件；[原任务](flows-redesign/keel-flows-module-build-spec.md)及同目录draw.io规定Keel业务覆盖。上游原生UI是实现与交互基准，不再按旧ASCII线框另造布局。
- 开发在独立`feat/jobs-kestra-ui`分支、`/home/ubuntu/keel-kestra-ui-dev`工作区进行，以最新dev为基线。其他业务在dev继续。用户未明确要求时，不合并dev/main、不部署、不覆盖5180/8081。

### 10.2 原生模块与Keel边界

- 保留Keel唯一Sidebar、顶栏、ProjectSwitcher、React Router和身份会话；作业平台挂载独立Vue模块。不增加第二套全局导航、iframe或临时用户入口，不迁移BOM/CMDB等其他模块。
- 表格、筛选、按钮、状态、弹层、多面板编辑器、任务属性、Gantt、日志、变量/输出等优先使用上游原生组件；Keel专属Targets、Quick Run、Agents、审批扩展也用相同原生组件组合。不得长期保留旧`.fl-*`视觉与新原生组件混搭。
- 集中Adapter连接Keel API；浏览器不直连Kestra、不携带引擎凭据。项目、权限、目标设备、审批、CMDB稳定ID、通道及维护态约束由Keel保留，后端逐请求授权。
- 上游样式限定在`.keel-kestra`模块作用域；浅/深色跟随Keel主题。修改导入组件标记适配内容，不让全局CSS、弹层、快捷键或布局监听影响其他模块。
- Flow与Execution使用一份URL状态，支持刷新、深链接和前进后退；切项目/卸载时清理请求和订阅。只持久化布局偏好，不在localStorage保存会话、秘密或可变执行范围。

### 10.3 导航与页面信息架构

- 侧栏沿用**作业平台、Executions、Agents**三个已确认入口，不为每项生命周期能力增加一级菜单。
- Flow目录采用原生列表/筛选/列管理/批量动作；创建为主动作，Quick Run为次动作。搜索、排序、分页total和最近执行摘要来自授权服务端查询，不能用已加载数据的本地分页冒充完整目录。
- Flow详情固定总览、编辑、执行、触发器；版本、依赖、日志、指标、并发放同一标签栏的“更多”。Files、模板、Docs进入编辑器面板，不堆多行Tabs。
- Execution目录提供记录、运行概况、异常/等待保存视图及跨执行日志。只显示实际聚合，不重复全局Dashboard。
- Execution详情固定总览、Targets、Gantt/任务、日志、变量与输出；更多包含Metrics、Dependencies、Audit/Context。每个页面保留独立URL，选择任务/设备/Attempt后联动日志和输出。

### 10.4 编辑器：同一份定义的连续交互

- 直接复用上游多面板编辑器、Monaco、No-code/BlockEditor及Topology；支持分栏、标签切换/关闭、拖动、最大化和布局恢复。默认Visual与Topology并排，YAML随时打开；不再设计固定三栏替代品。
- 节点目录 → 添加/选择节点 → 属性/脚本编辑 → YAML同步 → 校验定位 → 保存Revision → 目标预检 → 执行，必须形成连续路径。只读结构摘要不能称为No-code，普通textarea不能替代Monaco。
- Visual、YAML、Topology、Files及模板共用canonical Flow定义，不维护第二套图数据。不保留“设备步骤/完整编排”两套互相独立的编辑入口。
- 设备任务是Keel受控适配；当前只有串行依赖时，应在真实所属任务内部表达，不画成可任意并行执行的Kestra节点。未知插件参数不能因Visual编辑而丢失。
- 属性与Docs来自实际可用插件/任务Schema；不得用静态少量输入框宣称支持所有Kestra插件。未具备的能力明确说明限制，不使用可点击但无行为的按钮。
- 文件内容/版本、脚本、输入及目标随Revision/Execution冻结；套用模板处理未保存修改，不能保留旧票据、回调或触发器。快捷保存遵守只读、权限和忙状态。
- 脚本入口必须完整可见：新增脚本、导入本地文本脚本、文件列表、Monaco在线编辑、保存和下载。不能以预置文件或普通textarea冒充脚本工作台。脚本草稿在切换面板时保留，未保存内容不得被“在Flow中使用”静默采纳。
- 文件引用与内联快照明确区分：Namespace文件引用产生namespaceFiles/commands，内联使用已保存内容生成script。允许新增脚本任务或明确替换兼容脚本任务，禁止把Log等其他节点静默转换成脚本任务。引擎脚本任务不等于Keel设备执行通道；真实设备操作仍走目标/通道/预检/审批。

### 10.5 执行中心与生命周期

- 按LC-01～14完整纳入状态历史、TaskRun/Attempt、暂停/恢复/终止/重试、变量文件、指标、依赖、跨执行日志、并发队列、触发补跑、源码搜索及历史版本导航；未完成项保留在实施记录，不因主列表能打开就称为完成。
- 故障路径：Execution → 失败TaskRun/目标 → 指定Attempt日志 → 原因/产物 → 失败目标定向重试 → 新Execution；保留父执行和筛选返回路径。
- Kestra引擎状态与设备成功/失败/阻断分别展示；Worker与设备Agent分别建模。不能用引擎SUCCESS覆盖部分设备失败，不能用当前状态推算历史。
- 暂停/继续、终止、失败目标重试、全部重跑、修改输入后新运行是不同操作，确认时冻结项目、Revision、设备集合、输入与审批。权限不等于批准，不直通引擎绕过JobService。
- 日志使用当前Execution SSE/游标查询，支持任务、Attempt、设备、等级、时间与跟随；不加载全量目标日志或在浏览器遍历所有执行拼接跨执行检索。
- Inputs/Outputs先脱敏，文件按授权资源下载。没有真实指标、排队原因、触发历史或依赖证据时显示缺失，不编造趋势、百分比、下次触发时间或故障关系。

### 10.6 视觉与交互验收

作业平台独立高保真原型入口：[原型说明与页面](flows-redesign/prototype/README.md)。使用现有5181供用户先审阅，属于synthetic设计预览，不代表正式组件/API集成完成。

- 1440px及以上桌面优先，紧凑单行标题/动作、筛选栏和高利用率主体；编辑器与执行工作区占主要空间。去掉占据大面积的说明、重复统计卡、卡片嵌套和无意义空白。
- 字体、间距、行高、状态、按钮、图标、表格、编辑面板与日志遵循上游原生组件及其设计token；Keel专属部分也保持同体系。不能只换颜色、加卡片或用截图冒充原生接入。
- 同页覆盖loading、empty、error、permission、partial、unknown等适用状态；主要操作有可观察结果，无权限按既定规范禁用并说明原因。
- 检查真实来源组件、许可证、canonical定义同步、URL/主题/项目一致性和BOM/CMDB样式隔离。截图与关键交互按实际变更定向验证，不重复全仓测试。
- UI复用、后端能力、环境发布、真实设备验收分别记录；浏览器synthetic数据只证明对应UI行为。不得再把软件缺口写成“只等真机”。


## 11. Rack Diagram 机柜图工作区

```text
┌ 项目 / 机房 / 机柜选择器 ─ BOM版本 ─ 布局版本 ─ 状态 ─ 导出 ┐
├ 左侧：机柜/设备导航 ───── 主画布：正面/背面、多机柜、U刻度 ───┤
├ 搜索/筛选/图层控制 ────── 设备详情抽屉：型号/U位/端口/来源 ──┤
└ 底部问题面板：冲突/未分配/缺失属性/布局diff/审计事件 ────────┘
```

- 采用 Rackula 风格的直接画布交互：缩放、平移、拖放、撤销/重做、正反面切换、空 U 折叠；采用 RackView 风格的真实资产、端口、线缆端点、查找、影响关联与变更活动记录。技术实现使用 React + SVG + shadcn/ui，不移植 Svelte 组件。
- 设备拖动只修改草稿；落位前显示 U 位占用/越界预览；键盘可操作；不能只靠颜色表达冲突，需图标+文字+问题列表。
- 默认主画布不显示所有跨柜线缆，选择设备/端口后高亮相关链路，避免万卡级线缆遮挡机柜信息。
- 自动生成必须提供预览、校验摘要、未分配设备、diff 和发布动作。已发布图不可直接覆盖。
- 视觉基准与交互验收见 `RACK-DIAGRAM.md`、`UI-REFERENCE.md` 和 `UI-FLOWS.md`。

## OPS-2026-10-29 · 交付后运维设计增补

### 运维信息架构与统一详情
新增“运维管理”一级组，独立“服务器运维 / 交换机运维 / 配置漂移 / 采集中心 / 维护与退役”，保留交付导航。列表只呈现识别、位置、健康、配置/维护摘要；复杂配置、趋势、端口、证据分别放详情Tab，不做巨型Excel。

服务器详情：概览/基础配置/基线与实测/硬件监控/BMC/固件/网络端口/关联时间线。交换机详情：概览/配置/端口/LLDP/FDB/ARP/DDM/健康/无损/变更与时间线。详情可从抽屉切换独立路由，Tab、过滤、时间范围和snapshot_id写入URL，返回目录保留上下文。

每个观测面板固定显示来源、collected_at、freshness、capability、quality、适用阈值/规则版本；supported与policy_allowed分别表达。unknown/unsupported/stale/degraded用文字+图标和原因；null显示“无观测/未支持/策略禁用”，真实0显示单位与成功采样时间。趋势面板支持范围/样本源/阈值与断点，不把缺样插成0。

配置diff采用三栏批准基线/intended/observed与结构化差异，固定精确revision和采集时间，危险动作展示目标资产/项目/版本/审批/通道/维护窗口。禁止一个通用“确定”弹窗替代这些上下文。设备操作受拒绝时说明IPMI-only/不支持原因和可查看历史，不能出现“恢复SSH/启用Agent”按钮。

交互线框见UI-REFERENCE的OPS补充，端到端流程见UI-FLOWS；本次仅设计，待Mock契约冻结后实现。

## IA v2.3 · 双平面与统一抽屉

IA v2.3 顶栏品牌/环境/⌘K/通知/ProjectSwitcher/用户，左侧三分区；统一 DrawerDetail，服务器五 Tab、网络八 Tab，列表点击/状态直达、前后设备、键盘、URL、懒加载和滚动恢复。所有动态字段展示 source/observed_at/freshness/support；light/dark、焦点和窄屏状态列入验收。完整规格见 IA-V2.3.md。

## IA v2.4 实施增补

统一顶栏和三分区Sidebar；Fleet/Fabric紧凑列表+专属五/八Tab抽屉，RackView使用机房可视化和独立编辑器。


## IA v2.4 Mock 前端实施记录

导航树、完整页面清单、路由/权限/作用域/加载策略、五/八Tab抽屉与API对照统一见 [IA24-IMPLEMENTATION-MATRIX.md](IA24-IMPLEMENTATION-MATRIX.md)。原FR-0…24、门禁、BOM与维护态规则保留。当前实现为前端Mock，不表示已实现Go/真实设备/真实AI。权威种子为 `fixtures/demo-full-64-b300/demo-seed.json`；原包83集合，补齐与规范化后集合数量以当前manifest为准，主项目硬件数量不变。结构检查、数据修复、契约与交互结果以IA24-*报告为准，无法采集/缺少证据保持unknown或阻断。


## IA24-05 · 用户确认的GPU-NIC默认映射
用户于本轮明确：没有单独说明时，GPU1–8分别对应NIC1–8。对当前B300切片按此规则确认512条GPU-NIC关系，NIC9/10保留存储角色；不再把已有LLDP链路显示成全部unknown。矩阵行GPU、列NIC，点击显示物理端口、线缆、交换机、来源与观测；配对关系与线序/新鲜度判据分开，异常不能被配对规则覆盖。


## 当前六阶段主链路修正

本轮用户明确废弃数字门禁作为主链。当前唯一业务顺序、状态机、逐目标放行、安全交接、独立BOM菜单与API约束以[DELIVERY-LIFECYCLE.md](DELIVERY-LIFECYCLE.md)为准。历史编号只按[DELIVERY-MIGRATION.md](DELIVERY-MIGRATION.md)追溯，不得用于解锁。验机失败关联统一Issues并定向复验；NCCL不反写验机；清理/归档/客户签署齐全后才进入维护态。NP仍是网络内部断言。当前仅前端Mock与契约实现，没有真实设备后端。


## 单柜详情工作区增补

固定单柜窄画布、左侧层级导航和右侧八Tab为本轮布局要求；交互、API、元数据、Mock来源及待执行验收见[RACK-WORKSPACE.md](RACK-WORKSPACE.md)。资产/位置和既有安全模型不变。右侧按需加载，不以缺失指标显示正常。当前构建/浏览器验证按用户安排等待统一执行。


## Fabric Topology 增补

Topology采用左侧主画布+右侧详情。Physical/Logical双视图切换保留筛选、设备与线缆上下文。规划虚线，观测实线；healthy绿、warning黄、critical红、unknown灰、planned蓝，配合文本。选中线缆高亮双端/路径，无关对象降低亮度；端口详情内联，设备详情使用唯一共享Drawer。

完整导航、模型、交互、API表与实施边界见[FABRIC-TOPOLOGY.md](FABRIC-TOPOLOGY.md)。


### Rack Mapping：所选设备关联追溯（当前定义）

Mapping恢复重构前三列工作区中“设备与连接追溯”的内容，不再作为GPU-NIC矩阵页面。支持服务器和交换机：设备型号/SN/稳定ID/来源BOM、草稿与实际位置、“连接/端口”切换、线缆A/B两端设备/端口/介质/长度、MAC与状态、对端定位和统一设备详情抽屉。切换画布设备或定位对端保留Mapping上下文，清除旧设备的关联内容；连接和端口支持搜索、分批加载，缺失引用显示unknown。GPU-NIC矩阵继续由Topology/Validation承载。

workspace/mapping按需返回ports/cables及授权peer_ports/peer_devices；关联来自原有BOM/CMDB稳定ID，不创建资产副本。Cable双端与端口peer引用不同要明确提示冲突，不以端口状态或历史线序结果伪造当前验证通过。


## 无损网络分层证据 v2

Fabric无损、交换机无损抽屉、Topology健康详情、机柜Network共用LosslessEvidencePanel。规则与端口可选，五层Tab展示baseline/applied/operational/counters-test/provenance；显示缺口、来源、有效期及工单，配置匹配与运行测试结论分开。缺测/不适用不可用绿色替代。

规则/官方章节/命令/版本/判据/证据对照、历史迁移、真实缺口见[LOSSLESS-EVIDENCE.md](LOSSLESS-EVIDENCE.md)。


## CMDB 全链路资产档案增补

CMDB左树+主工作区，五视图共享asset/filter URL。目录按类型字段呈现独立交付/维护/健康/完整性；关系采用React Flow+右侧来源/逐层展开面板，不重建拓扑引擎。设备沿用唯一五/八Tab抽屉，原始证据用Dialog。质量条目直接定位来源，不堆统计卡。

唯一详细设计与模型边界见[CMDB-DOSSIER.md](CMDB-DOSSIER.md)。

## BOM 真实开发增补

用户已授权在 `dev/` 真实实现 BOM 模块。数据与权限边界、版本/审核/Outbox、真实六步工作台及运行验收统一见 [BOM-REAL-DESIGN.md](BOM-REAL-DESIGN.md)。FR-2.1 和唯一 BOM/资产身份保持；其他模块的 Mock 状态不因本模块接入而视为后端已完成。实际通过范围以 BOM-REAL-ACCEPTANCE.json 为准。

## 2026-10-30 · 统一登录与身份管理

/login 直接迁入用户指定旧仓库 LoginPage、Brand、KeelStructure、Preferences 与样式，不重设计。适配 React Router 与当前 shadcn，样式隔离避免覆盖控制台。全局用户菜单提供安全/会话/管理入口；BOM 无二次登录。权限不足显示说明并禁止操作；服务端仍复验。管理对话框内提供真实错误与变更结果，详见 IDENTITY-RBAC.md。


## BOM Intelligence Agent 当前实施（2026-10-30）

本轮仅 AI 智能识别：上传→真实分析进度→分类审核→确认入库。交换机/端接优先，单主编排器+Workbook IR 工具+独立 Schema/来源/覆盖校验。当前范围替代此前 BOM 双模式/六步 UI 描述，历史数据不删除；详细字段、状态、API、安全和验证边界统一见 [BOM-WORKBENCH-V2.md](BOM-WORKBENCH-V2.md)。保留 FR-2.1、项目权限、唯一资产身份和原事务 Outbox。完整导入入库闭环由用户验收；不以旧报告宣称当前通过。

## 2026-10-31 · 数据中心可视化 V3（设计评审）

四页采用“Cluster/Hybrid、Fabric Flow、Connection Matrix、Metro + Tree/Table”的差异化工作区。默认正文占满宽度，选择对象才显示证据栏；继承现有字体、颜色与间距，不增加一排重复 KPI 卡片。束的交叉不等于连接、矩阵空格不等于正常、证据状态不并入位置/数量。完整视觉编码、密度与选择规则以 [topology.md §4、§6、§14](topology.md#4-四个模块的页面设计v3-权威方向) 为准，**不改变全局设计令牌或正式页面**。机柜不在本轮设计范围。

## 2026-10-31 · 机柜可视化设计原型

后续独立机柜设计以 [rack-visualization.md §4–6](rack-visualization.md#4-信息架构与交互) 为准：Gallery、Spatial、Dual 与剖面共用几何；安装面未知标注、深度未定界；整体范围与端口细节联动；Asset/Capacity/Compute 依数据启用；Front 选择打开资产与安装，Rear 选择打开端口与连接，同一设备 ID 与上下文同步；规划/实装缺少合格快照时隐藏入口，按数据能力逐级开放，规则见该文档 §4.4。新增样式限定于机柜原型路由及 rack-viz/rv-*/sr-*，继承全局 token，未更改正式页面或其他四个拓扑原型。

机房空间模式使用主 3D 画布与紧凑右侧机柜详情；颜色仅映射有来源的 U 占用率，选择使用统一紫色。机柜标签与同一资产立面联动，未知坐标用“示意排布”明确标记。此模式替换阵列“空间”展示，单柜几何/双面/Mapping 与原主题令牌保持不变，详见 rack-visualization.md §4.1a。


## Flows 包重构实施增补 · 2026-10-31

Flows定向实施沿用现有token/shadcn；紧凑目录、三栏Builder、Execution六Tab、Quick Run五步弹窗。仅Flows/Executions/Agents菜单；其余资源归Flow上下文，不修改全局主题。

详见 [Flows 实施记录](flows-redesign/implementation.md)、[真实 API 契约](api/flows.md)。字段/Schema仅定义于OpenAPI。

Flows开发补齐（afe8cb9）：作用域Schedule、多步骤/输出绑定、Namespace二进制版本/分发、持久事件游标、审批单次绑定与任务取消已实现；规则和真实证据统一见[实施记录](flows-redesign/implementation.md)及[契约](api/flows.md)，不在本文件重定义。生产设备验收仍单列。
