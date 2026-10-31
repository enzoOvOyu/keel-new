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
