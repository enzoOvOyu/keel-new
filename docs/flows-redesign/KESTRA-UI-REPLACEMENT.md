# 作业平台：Kestra 原生前端替换方案

状态：方案待用户审阅，尚未实施界面替换。分支 `feat/jobs-kestra-ui`，工作目录 `/home/ubuntu/keel-kestra-ui-dev`。现有5180/8081不因本方案切换。

## 依据与结论

业务范围沿用 `keel-flows-module-build-spec.md` §2–10，页面任务沿用同目录 draw.io 01–07；用户最新指令覆盖其中“只借鉴、不复制UI”和固定Flows名称：产品名为作业平台，直接复用Kestra开源前端。原型规定业务覆盖，上游规定组件实现与交互方式。

推荐：Keel React外壳 + 作业平台独立Vue模块 + Keel API适配层。直接复用上游源组件；不再用React另写一套相似界面。Vue只用于此模块，其他业务不迁移。无需引入通用微前端平台或更换Keel构建体系。

源码核验：Kestra tag `v2.0.7`，实际检出commit `49b62a2c820d37e98a1c7b63aa35fd83f960d4ec`。源码检查位置 `/tmp/keel-kestra-ui-source/ui`；此临时目录不是交付依赖，实施时固定源码快照/版本并纳入许可证记录。其package使用Vue、Pinia、Vue Router、Monaco及上游design-system/topology包，声明Node>=24/npm>=11.7；不能直接放进现有Node22/React工程运行，也不能仅复制几个.vue文件便声称完成。实际构建依赖闭包需在实施首步验证。

源码许可证为Apache-2.0；复用时携带LICENSE，保留适用的署名/NOTICE，修改文件加说明。限开源组件，不依赖Enterprise专有授权能力。

## 页面替换清单

| Keel页面 | 上游实际源码 | 替换方案与Keel扩展 |
|---|---|---|
| 作业平台 `/flows` | `components/flows/Flows.vue`、`KSFilter`、`KsDataTable` | 直接复用列表、筛选、列配置、批量动作和URL分页交互；主按钮创建，次按钮Quick Run。列含名称/ID、说明、Namespace、目标类型、触发器、Revision、最近执行、最后结果、启用状态；最近执行由服务端摘要提供 |
| Flow详情 | `FlowRoot.vue`、`FlowRootTopBar.vue`、`useFlowRoot` | 复用标题/状态/动作和上下文Tabs；Overview、Builder、Executions、Revisions、Triggers、Dependencies、Files。只显示本阶段具备数据和操作能力的内容，缺口明确禁用 |
| Builder | `MultiPanelFlowEditorView.vue`、`MultiPanelGenericEditorView.vue`、`panelDefinition.ts`、No-code/BlockEditor、`FlowFileEditorTab`、`LowCodeEditorWrapper` | 复用多面板布局、分栏调整、标签拖动/关闭、Monaco、节点选择、属性面板、Topology及Docs/Files/Blueprints。默认并排Visual与Topology，YAML随时打开；具体默认布局以依赖验证后的上游组件能力为准，不另造固定三栏画布 |
| Executions | `components/executions/Executions.vue` | 复用筛选/表格/状态/批量操作；增加项目、业务来源、CMDB目标、实际Agent/SSH通道。统计图仅在有真实聚合接口时启用 |
| Execution详情 | `ExecutionRoot.vue`、`ExecutionRootTopBar.vue`、`Gantt.vue`、执行详情组合组件 | 复用任务树、Gantt、拓扑、日志、输出与执行控制；扩展Targets标签和选中设备上下文。引擎状态与设备结果分别展示 |
| 日志 | `components/logs/LogsWrapper.vue`、LogLine、TaskRunDetails、LogLevelSelector | 复用任务/Attempt关联、等级过滤、日志行、跟随与详情；接入设备stdout/stderr及重连补齐 |
| Quick Run | 上游FlowRun/输入、对话框、表格组件 | 业务向导为Keel扩展，使用上游组件组合；目标→脚本→通道→预检→确认。按项目/机柜/角色/Agent状态筛选；显示每台设备通道与阻断原因 |
| Agents | 上游表格、筛选、详情和步骤组件 | Keel专有领域，没有可直接搬来的Kestra Agent页面。沿用上游设计语言实现身份、连接、能力、任务、证书和审计，安装显示服务端真实进度 |

## 核心交互

### 定义与编辑

Flow详情进入Builder后，节点目录、文件、模板和Docs通过上游编辑面板打开。添加任务→选择节点→属性编辑→YAML同步→校验定位→保存Revision→启动，是一条连续链。

取消现有“设备步骤/完整编排”两套操作入口。界面中的设备任务、循环与条件必须来自同一份定义。设备步骤在真实执行中只有串行依赖能力时，应显示为对应任务内部的步骤，不能画成已具备任意并行调度的原生Kestra节点。

CMDB目标解析、通道策略、Agent任务、SSH命令、文件传输作为明确的Keel适配任务提供领域属性。底层如果仍用受控HTTP wrapper执行，编辑器的展示元数据只负责显示与编辑该真实wrapper；不存在的插件不得编造为引擎插件。必须有YAML→图→YAML往返校验，保留未知任务字段。无法无损编辑的任务可用YAML编辑，不能静默删除。

目标选择以CMDB稳定ID和版本为准；通道选择显示Agent Preferred/Only与SSH Only。并发区分Flow并发、目标并发和Worker资源。重试根据任务幂等能力提供，明确“重试次数+首次执行=最大Attempt”，禁用时说明原因。

文件从原生File Explorer选择，脚本内容/资源版本随Flow Revision或Execution快照冻结。模板是完整合法定义，切换前处理未保存修改，不能保留旧模板票据/回调/触发器。

### 执行与排障

全局列表→执行详情→失败TaskRun/目标→指定Attempt日志→失败原因→只重试失败目标→新Execution，保留父执行返回路径。点击节点同步过滤任务日志；选择设备同步其任务、通道、耗时、退出码和stdout/stderr。

详情固定Overview、Targets、Task Runs、Logs、Outputs & Artifacts、Audit/Context。字段来自原始所属域，不将设备部分失败改写成Kestra原生SUCCESS/FAILED。Quick Run也保存真实引擎Revision，历史定义不可用时明确提示。

### 页面空间与视觉

保留Keel唯一侧栏、顶栏和ProjectSwitcher；模块内不再渲染第二套Kestra全局导航。作业平台只有默认列表、Executions、Agents三个入口。

内容区域采用上游字体层级、状态组件、密度、表格、筛选器、按钮、编辑器与日志样式；不再保留旧`.fl-*`视觉体系混搭。Keel项目和审批扩展也使用同一套上游组件。浅/深色跟随Keel主题。上游全局CSS需经过作用域隔离验证，不能影响BOM/CMDB。

1440px以上优先：页面头部单行标题/动作，筛选器紧凑，列表主体占余下高度；Builder最大化利用可用高度。ID可复制并作为次级信息，业务名优先；不将内部wrapper ID当唯一节点标题。运行按钮只在明确Revision、权限与目标范围后生效。

## 技术接入边界

React路由在`/flows/*`挂载一个Vue应用，提供mount/update/unmount接口；外壳传递当前身份、permissions、项目过滤、主题、受控导航和API客户端。卸载或切换项目时清理请求、订阅、编辑器与布局监听。项目范围不通过跨标签页共享localStorage维护。

模块独立构建，采用上游兼容工具链。使用受限Vue Router子路由或内存路由，与React Router建立单一URL同步；浏览器前进/后退、刷新、深链接和Tab状态必须可恢复，不能两个Router争抢history。

浏览器仅访问Keel Go API，不直接暴露Kestra凭据，也不通过iframe开放原生引擎管理后台。保留HttpOnly Session/CSRF、逐请求项目授权、审批及维护态约束。

上游Store/SDK调用经集中Adapter替换：namespace+flowId映射Keel Flow绑定ID；engine execution ID映射Keel Execution绑定ID；TaskRun ID原样保留。不能把项目ID直接当Namespace，也不能用名称匹配资产。官方导入/删除/批量修改状态等动作只有Keel契约已支持且授权时才开放。

## API适配与明确缺口

| 数据/操作 | 已有基础 | 替换前必须补齐 |
|---|---|---|
| Flow列表/详情/版本 | `/flows`及详情、revisions | 服务端筛选/排序/分页total、逐Flow最后执行摘要；适配上游数据表需要的结构，不通过加载全量数据模拟服务器分页 |
| Builder任务目录与属性 | 模板、YAML validate、受控任务执行 | 受权限过滤的插件/任务Schema、文档与属性元数据接口、Keel任务适配定义；不能假定现有49/50个接口足够驱动完整原生UI |
| 保存/发布 | expected_revision、draft、KestraSave | 校验与错误定位、模板转换、取消未保存变更；不得对不兼容字段静默降级 |
| Execution/日志/输出 | ExecutionPage、SSE、日志、产物、审计 | 完整筛选与聚合、执行版本冻结、TaskRun关联/日志续传与分页、准确设备结果汇总 |
| 文件与模板 | 文本、二进制版本、分发、单个设备模板 | 上游FileExplorer所需目录操作的明确范围、脚本/资源绑定和模板目录；尚无接口的按钮不展示为可执行 |
| Triggers | 手动与作用域Cron | 设备Webhook/事件Adapter、触发器编辑与权限；先完成契约再启用 |
| Agents | 注册、mTLS、心跳、执行、撤销 | 证书轮换、Linux核验、持久化批量安装进度、Active Tasks、断点恢复 |
| 业务模块 | JobService已有 | ZTP等真实消费者仍缺失，不在来源筛选里出现ztp就宣称集成完成 |

保留Keel API权威契约；不为让原生UI“看起来可用”直接代理整个Kestra API。新契约逐项登记OpenAPI、Go、类型和适配器，接口权限始终由Keel后端校验。

## 替换顺序与回退

1. 固定上游源码/许可证，验证最小Vue页面及其依赖在独立模块构建；梳理真实依赖闭包、CSS和路由隔离。技术不可行项先报告，不改用自制近似页面冒充复用。
2. 优先接通Flow列表和Execution列表，用这两个页面验证原生组件、项目权限、深链接、主题和真实Adapter。
3. 接入Flow详情/原生多面板编辑器，完成Keel任务元数据、文件/模板、校验保存和运行。
4. 接通原生执行详情、Gantt/拓扑/日志/输出及Keel Targets；再完成Quick Run和Agents同体系界面。
5. 按八页任务逐项验收后，才将`/flows`入口统一切换；旧实现作为回退保留至确认。开发预览沿用现有机制，不新增一级菜单、端口或覆盖正在测试环境。

本轮只交付方案。下一阶段先做原生组件可构建与两个列表接入，验证后再扩展，不将“源码已拉取”计为UI已复用。

## 验收

- 可展示复用的上游组件路径、固定版本、改动清单及许可证；不是仅色彩相似。
- 原生表格/筛选、多面板编辑器、任务属性、Gantt与日志操作实际可用。
- 一个设备作业能从创建、编辑、校验、保存、目标选择、预检进入执行，并从失败任务定位日志和重试；有真实状态证据。
- Vue模块与React外壳项目/权限/主题/URL一致；卸载无遗留订阅，BOM/CMDB视觉与运行不受影响。
- 无生产设备时可用明确标识的协议模拟端点验证执行链，不冒充真实设备验收。
- 前端替换完成、后端能力补齐和生产设备验收分别记录；没有实现的能力不归为“只等真机”。

## 上游来源

- https://github.com/kestra-io/kestra/tree/v2.0.7/ui
- https://github.com/kestra-io/kestra/blob/v2.0.7/LICENSE
- `ui/src/components/flows/{Flows,FlowRoot,MultiPanelFlowEditorView}.vue`
- `ui/src/override/components/flows/panelDefinition.ts`
- `ui/src/components/executions/{Executions,ExecutionRoot,Gantt}.vue`
- `ui/src/components/logs/LogsWrapper.vue`

## 增补：执行生命周期与运维信息页（用户最新纳入范围）

本节扩展前面的六个Execution标签与七个Flow标签，不删除原有能力。依据同一上游commit的 `executionTabs.ts`、`flowTabs.ts`、`useExecutionRoot.ts`、`RunTimeline.vue`、`FlowTriggers.vue`、`FlowConcurrency.vue` 及相关动作组件，已实际查阅源码。以下均为规划纳入，尚未开发或部署。

### 页面入口与密度

仍保留作业平台、Executions、Agents三个侧栏入口。新增能力进入列表视图、Flow上下文或Execution上下文。常用标签固定，低频标签放同一标签栏的“更多”，每项仍有独立URL和深链接；不把十几个标签挤成多行。

- 作业平台默认Flow目录；支持进入“源码搜索”。Flow详情固定总览、编辑、执行、触发器；更多：版本、依赖、日志、指标、并发；Files/模板/Docs继续嵌入编辑器。
- Executions提供记录、运行概况、异常/等待视图、跨执行日志；“异常/等待”是同一授权查询的保存视图，不另建执行库。运行概况消费真实聚合，不新增与全局运维Dashboard重复的主菜单。
- Execution详情固定总览、Targets、Gantt/任务、日志、变量与输出；更多：Metrics、Dependencies、Audit/Context。Topology与Gantt互相切换并保留所选任务。原有Artifacts进入变量/文件浏览。

### 新增页面与交互矩阵

| 编号 | 纳入能力与入口 | 对Keel的具体价值及交互 | 上游依据/性质 | 数据与实现前提 |
|---|---|---|---|---|
| LC-01 | Execution总览：状态历史时间线 | 显示创建、排队、运行、暂停、继续、终止与结束的真实发生时间；区分排队时长/执行时长。选事件定位相关Attempt、日志或审计 | `overview/Overview.vue`、`RunTimeline.vue`，原生能力 | 引擎state.histories与Keel操作审计分别保留，禁止前端时间动画生成状态 |
| LC-02 | TaskRun/Attempt详情 | 点击任务打开迭代值、开始结束时间、每次Attempt、错误、超时、Worker引用与对应日志；循环中的单设备失败可精准定位 | `TaskRunLine.vue`、`TaskRunActions.vue`、`logs/TaskRunDetails.vue`、`WorkerInfo.vue` | 引擎TaskRun和设备Task需显式关联；Worker不是设备Agent，不能混为同一身份 |
| LC-03 | 恢复与重跑操作工作区 | 暂停/恢复/终止、失败目标重试、全部重跑、修改输入后新运行分开；确认页显示设备范围、Revision、前后输入差异、审批和重复执行风险 | Pause/Resume/Restart、`ReplayWithInputs.vue`、`ResumeFromBreakpoint.vue` | 设备执行必须走Keel预检及审批。Replay/从节点恢复/跳过任务只有具备幂等和明确目标恢复契约后开放，不直通引擎API |
| LC-04 | 执行变量与文件浏览 | 树形查看Inputs、任务输出、迭代输出、类型和文件；支持搜索、复制、文件预览/下载，定位产出任务和源Revision | `outputs/ExecutionVariableExplorer.vue`、`FilePreviewDrawer.vue`、`VarValue.vue` | 变量先脱敏；密钥输入不展示原文。只下载属于当前授权Execution的资源，HTML等文件采用安全预览 |
| LC-05 | Execution Metrics页 | 展示任务上报计数、耗时、结果数量等带单位指标，按任务/时间筛选，并链接日志。无样本显示未上报 | `ExecutionMetric.vue`、Metrics/MetricsTable | 只消费引擎或任务实际指标；不把它替代Fleet GPU/BMC监控，也不伪造CPU/显存曲线 |
| LC-06 | Execution Dependencies页 | 展示父子执行、Subflow、重试/重跑谱系、触发来源；点节点查看对应执行，保留返回路径 | `dependencies/Dependencies.vue`、执行Dependencies标签 | 项目授权逐节点过滤；未知外部节点不泄露标识。重试谱系是Keel扩展，不能只凭时间相近推定关系 |
| LC-07 | Flow日志页 | 跨该Flow多次执行检索错误/任务日志，筛选后跳到精确Execution/TaskRun/Attempt，减少逐个打开执行 | `flowTabs.ts`的logs、`LogsWrapper.vue` | Go提供授权范围内跨执行日志查询、游标/时间窗；不让浏览器遍历全部执行拼接结果 |
| LC-08 | Flow指标与运行趋势页 | 成功/失败/警告分布、执行次数、耗时分布/趋势、目标失败数、重试次数，按时间/Revision查看 | `FlowMetrics.vue`、Flow Overview；目标汇总为Keel扩展 | 分母与时间窗明确；Kestra成功率与设备成功率分开。P50/P95只有足够实际样本时呈现，不设未经确认的SLO |
| LC-09 | Flow并发与排队页 | 展示正在运行、待运行、Flow并发限制、目标并发及阻塞原因；链接排队执行，查看限制变更 | `FlowConcurrency.vue`、`admin/ConcurrencyLimits.vue` | 项目级视图默认只读；变更需独立授权和审计。取消排队与强制越过并发是不同动作；不默认开放Force Run/Unqueue绕过控制 |
| LC-10 | Triggers详情及历史补跑工作区 | 触发类型、启停、上次/下次触发、触发日志、关联执行、错误；Cron补跑选择历史区间并预览次数，支持暂停/恢复/取消 | `FlowTriggers.vue`、`BackfillBanner.vue`、TriggerVars、TriggerEnableDialog | 设备补跑每次生成当前授权范围的独立作业，重新预检审批；不能用引擎原生backfill绕过JobService。下次时间来自引擎，不前端猜算 |
| LC-11 | Executions运行概况与异常/等待视图 | 运行中、失败、暂停、排队、超时、RECONCILING的紧凑汇总，点击进入对应筛选；批量处置先显示每项可操作性和结果 | `Executions.vue`统计与批量动作；RECONCILING为Keel扩展 | 不强行把通道失联改成引擎失败。批量权限/目标/版本分别核验，不以一项成功代替全批 |
| LC-12 | 跨执行日志检索 | Executions内“日志”视图，按项目、Flow、执行、设备、任务、等级、时间检索并跳转详情；支持导出当前授权范围结果 | 上游 `/logs` 与 `LogsWrapper.vue`；Keel将其收进Executions | 替代旧“仅单Execution日志”的范围限制；独立日志查询API，不新增侧栏Logs菜单 |
| LC-13 | Flow源码搜索与插件Docs | 查找脚本/任务类型/输入引用，打开对应Flow Revision；节点选择时显示实际插件说明、参数及示例 | `FlowsSearch.vue`、PluginListWrapper、编辑器Docs面板 | 检索须排除秘密；只展示真实可用的任务Schema和插件，不将整个上游插件目录等同Keel可执行能力 |
| LC-14 | 执行历史快速导航与版本追溯 | 同一Flow上一条/下一条执行、相同筛选返回、源Revision跳转；新运行可从历史输入生成待确认草稿 | `overview/components/main/PrevNext.vue`、FlowRevisions、ReplayWithInputs | 时间相同需稳定ID排序；每个相邻执行重新授权。不能把原审批、票据或凭据复制进新运行 |

### 不直接复制的页面及原因

源码 `executionTabs.ts` 的Audit/Assets和 `flowTabs.ts` 的Promote/Audit，以及routes中的部分IAM、Instance、Policies等，指向 `components/demo/*` 并标注locked。这些是上游开源前端的演示入口，不是可直接交付的开源完整能力。

- Audit保留Keel真实审计；Assets保留Keel CMDB/设备/报告关联，用原生组件展示，不引入第二资产库。
- Promote/跨环境发布可保留后续产品议题，本次不开放生产环境发布流程；用户当前要的是执行生命周期管理。
- Instance/Worker全局管理不作为普通用户页面。TaskRun已有的Worker只读诊断信息纳入LC-02；完整引擎节点管理需管理员权限、实际运行数据与能力核验。
- 不开放手工Change Status伪造设备通过、强制运行绕过并发、删除原始执行证据或清空设备计数器。界面可展示原生状态与错误，但状态结论必须来自真实执行/验证。
- Apps、MCP、AI聊天、企业租户/IAM、通用KV和完整Namespace管理与本次作业替换无直接关系；Keel已有AI/RBAC/项目/凭据域，不重复引入。

### 新增API影响清单（设计，尚未冻结OpenAPI）

沿用 `/api/v1/flows`，资源专用Schema需在实现之前写入OpenAPI及生成类型。本节只登记缺口，不宣称接口已存在。

| 查询或操作 | 必要返回/约束 |
|---|---|
| execution lifecycle / task-runs / attempts | 状态历史、时间、迭代、错误、每Attempt、worker引用及目标映射；不能仅用当前状态推算历史 |
| execution variables / files / metrics / dependencies | 脱敏变量树、文件授权引用、带单位的实际指标、带关系类型的执行谱系 |
| scoped log search / flow source search | 项目/Flow/Execution/TaskRun/设备/时间过滤，服务器分页、游标与授权导出 |
| flow execution statistics / concurrency | 明确时间窗和分母的聚合、运行/等待队列、限制、排队原因；区分Flow/目标/Worker |
| trigger details / histories / backfill preview & jobs | 实际触发记录、下次计划、关联Execution、补跑范围/数量/任务状态/逐次预检；不把一次审批当长期补跑授权 |
| replay request / scoped batch actions | 新的冻结目标/输入/Revision/审批/幂等键；逐项结果，已接受且结果未知时不重复执行 |

高频更新仍按Execution SSE、日志流、页面按需或低频聚合查询分工。历史指标/日志查询不按所有页面统一5秒轮询；没有已确认保留策略/SLO时不自行规定生产默认。

### 开发与验收次序增补

1. 与核心Execution详情一起实现LC-01/02/04/14，形成“看状态→找Attempt→看日志/输出→追溯Revision”的排障路径。
2. 接入LC-05/06/07/08/12/13，完成指标、依赖和跨执行检索，聚合与查询接口先明确再启用界面。
3. 实现LC-03/09/10/11的处置和调度管理，先补齐状态/权限/幂等/审批契约，再开放有副作用的操作。

验收额外覆盖：暂停恢复历史、循环某次Attempt失败、排队原因、RECONCILING不盲目重发、变量脱敏/文件权限、失败目标重试与全部重跑区别、历史补跑不绕过审批、跨项目日志与依赖隔离。用户闭环验收与开发必要检查分开记录；不以演示页或图表渲染证明后端能力完成。
