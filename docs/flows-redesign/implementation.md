# 作业平台重构实施记录（技术域 Flows）

## 最新整改状态 · 2026-10-31（覆盖下方“仅待真机验收”的旧结论）

本轮复用 Kestra、Job Service、审批、CMDB、Agent/SSH 和幂等底座，调整现有 `/flows` 页面，不增加入口或启动额外服务。用户最新命名：**作业平台**，不改变 `/flows` 与 Flow/Execution 领域标识。

| 本轮修改 | 实际结果 |
|---|---|
| 页面视觉 | 统一现有设计令牌、紧凑标题与工具栏、三栏Builder、专属节点、结构化日志/输出/审计、Agent详情；去掉旧宽松拼装布局 |
| Builder | Visual/YAML/Topology同源；原生Loop/Parallel/If/Subflow/Log/Output添加、嵌套删除与分支顺序调整；设备步骤真正添加到inputs.steps；依赖连线及循环拒绝；脚本/输出绑定编辑；自动布局避开嵌套分支重叠 |
| 执行监控 | 执行原Revision拓扑叠加TaskRun状态、Gantt/迭代筛选；日志按设备/stdout/stderr/任务/级别/文本筛选、跟随及下载；Outputs/Audit结构化呈现 |
| 目录与调度 | URL筛选、状态/namespace/触发类型、近期执行链接、来源/通道/发起人/时间过滤；修复ISO超时解析及调度覆盖类型/超时/并发；重载不覆盖在编辑的Flow与Schedule |
| Agents | 状态/版本/能力筛选，多选心跳/证书/能力检查；批量按设备SSH安装及逐台结果；真实版本0.2.0；独立身份/任务/连接/证书/审计面板。没有配置安装包/TLS就明确阻断 |
| 后端增量 | 显式列表分页、解除内部目标500条截断；Execution时间过滤；固定时间窗分页同步；Quick Run也可读取冻结执行定义。新增迁移025尚未应用 |

### 验证与运行边界

- `npm run build`（TypeScript + Vite）通过；已有共享seed体积、大chunk和Zod注释警告未改动。
- `go test ./internal/flows ./cmd/api`通过；未启用真实设备/数据库集成环境的用例不算运行验收。
- `node dev/scripts/check-flow-definition.mjs`通过：嵌套分支布局与删除、同分支排序、设备依赖和循环拒绝、ISO时长及运行默认值、YAML结构。
- 现有5180浏览器实际通过：作业平台导航、Agents页面、新Flow草稿、添加设备任务、脚本编辑与canonical YAML联动；无pageerror。仅读取API和编辑未保存草稿，没有执行设备命令/创建测试资产/保存业务数据。截图见 `screenshots/correction-builder.png`。
- 本轮后端增量未覆盖8081的BOM v5集成版，迁移025未执行，新执行定义接口与时间筛选未进行运行联调。部署需先在当前BOM集成基线上纳入本增量。

### 仍未实现，不能归类为“只缺真实设备”

- Agent自动证书续签/轮换流程仍缺失；当前只具有注册发证、证书展示与撤销。
- 受控设备Flow目前支持手动与Cron；Webhook/事件触发的作用域Job Adapter尚未完成。原生无设备Flow可用Kestra触发器，但不等于设备事件调度已实现。
- ZTP真实业务后端尚无调用Job Service的消费者；source_module字段不能冒充已接通。
- Builder为已实现插件子集，不宣称任意Kestra插件的动态表单均可编辑；设备多步骤仍按依赖串行执行，不等于设备内任意并行DAG。
- 批量SSH安装UI逐台提交；服务端持久化批量安装任务/断点恢复未实现。真实发行版安装和长期生产规模未验收。
- 本轮没有用合成完整页面冒充真实API端到端；实际原型逐页视觉/交互验收仍以用户审核为准。

以上问题明确保留，修正旧记录“全部开发完成，仅待生产设备”的不准确结论。

基准：包内 build-spec 与 draw.io 八页，保留全局 UI、身份、CMDB、SSH 信任入口。独立工作区 `/home/ubuntu/keel-flows-dev`，分支 `feat/flows-redesign`，基于 `d8a91b4`；不替换正在运行的 5180/8081。当前原环境后端来自另一集成分支，重新部署时必须保留其 BOM Agent 修复。

## 定向盘点与归属

| 现有文件/能力 | 实际情况 | 本轮处理 |
|---|---|---|
| `features/FlowWorkspace.tsx` | 纯 Mock、纵向卡片、自有模拟生命周期 | 替换 Flows 路由消费者，旧专属页面停用 |
| `components/FlowDefinitionEditor.tsx` | 仍需确认共享调用 | 保留，新的 Kestra Builder 不依赖旧定义 |
| BlueprintWorkspace/MediaWorkspace | Provision 等有共享消费者 | 保留共享实现，移除 Flows 独立菜单，迁移上下文 |
| `lib/api/domain.ts`、统一 Mock | 其他交付模块仍依赖 | 保留，不批量删除 |
| 身份/权限 | Go/PostgreSQL 已实现 flows 权限点 | 复用逐请求授权，不写散落角色判断 |
| CMDB | 真实资产与稳定 ID 已有 | 只查询与保存目标快照，不复制 Inventory |
| Kestra / Agent / SSH 后端 | 改造前未实现；SSH 页面为 Mock 元数据 | 新增模块化底座；现有 SSH 入口增加真实适配，不将 Mock 当作信任 |

## 设计边界

- 菜单仅 Flows、Executions、Agents；旧蓝图/介质/调度路由重定向至上下文。
- Kestra 2.0.7 固定版本；以官方该标签 Controller 为 REST 适配依据，Flow/Execution 状态来自引擎。
- maintained 严格 IPMI-only；Agent Preferred 不得绕过维护态或在执行状态未知时自动切换 SSH。
- SSH 只使用受控文件秘密引用及严格 known_hosts，不读取前端 Mock 凭据。
- 缺引擎、CA、SSH 信任或插件返回明确不可用；不能用合成成功代替真实执行。
- 运行状态、实际检查和剩余缺口随实现更新，不提前宣称完成。

## 交付实现状态 · 2026-10-31

代码提交：初始底座`63ca2c9`，核心底座`826b964`，开发补齐`afe8cb9`。此后文档提交不改变运行二进制。运行环境为独立5190/8096/8095，原5180/8081未重启、未迁移、未导入资产；未合并main、未push。

| 能力 | 实际实现 | 验证/边界 |
|---|---|---|
| 导航与迁移 | Flows父入口，仅Executions/Agents子入口；旧资源路由转上下文，保留其他模块共享代码 | 浏览器列表/详情可达 |
| Flow目录与详情 | 搜索、类型筛选、排序/本地分页、Run/Edit、Enable/Disable、Copy as draft；七Tab | 真实Kestra定义，不依赖旧mock |
| Builder | 三栏库/DAG/Inspector，Visual/YAML/Topology同canonical YAML，嵌套控制、Finally、参数、并发、校验、Draft/Publish | 当前原生安全插件子集；Keel目录项进入受控模板，非假定Java插件 |
| Quick Run | CMDB筛选/多选、Command/Bash/Python、通道、真实预检、版本冻结、提交；Save as Flow Draft | 浏览器验证至确认并实际保存草稿；API另有真实执行验证 |
| Execution | 六Tab，实时状态/日志，Targets输出、Gantt/Topology、原生输出/产物、Audit/Parent | 实际Pause→Resume成功；重试失败目标新建执行 |
| Agent | 实际非root二进制、单次TLS注册、mTLS网关、身份审核、心跳/能力、撤销、持久化任务日志、实时输出/取消/对账 | 本地实际Agent通过，非生产systemd沙箱安装验收 |
| Agent任务 | Command/Bash/Python、受控文件上传/下载、硬件采集、本人作业进程控制；allowlist用户服务控制 | 文件边界/任务journal用例通过；无硬件能力时unsupported |
| SSH | 系统管理原SSH信任入口改真实受控引用、严格known_hosts、公钥、非root登录、Command/Bash/Python | 真实SSH协议双合成端点通过；没有连接生产交换机 |
| Bootstrap | HTTPS包/摘要、严格SSH、固定root可信helper、非root服务用户、一次Token注册 | 代码可用；未配置真实包/信任前返回503，不显示成功 |
| Files/Revisions/Dependencies/Triggers | 实际Namespace文本文件、版本差异/恢复草稿、授权依赖、canonical触发器草稿 | 文件读写/版本冲突验证；原生Schedule由Kestra拥有 |
| API与存储 | 49个FlowsReal操作、生成TS类型、迁移021…024、绑定与冻结目标、游标、幂等、审计、CMDB Outbox | 独立库已迁移；注册表不冒充49项逐端点验收 |

## 数据与执行链

CMDB `cmdb_entity` + `cmdb_asset_scope` → 授权目标/能力与SSH预检 → 保存冻结目标、版本与通道 → Kestra SECRET票据/JSON输入 → Loop子执行 → 原生If区分Agent/SSH → Go受控Dispatch → 实际Agent或严格SSH → 结果/Outbox → 统一Execution/CMDB关联。

Kestra2.0.7兼容修正包括Loop替代不存在的ForEach、`fromJson(item.value)`读取Loop对象值、`toJson`替代已移除json过滤器、SECRET输入启用引擎加密。不是只通过YAML校验就认定可执行。

## 实际检查

- `go test ./internal/flows ./cmd/agent ./cmd/api -count=1`通过；包括通道/维护态、跨项目namespace、已接受Agent失联、交换机写操作阻断、任务journal、文件边界和实时输出。未配置隔离DSN时集成用例明确SKIP。
- 显式隔离DSN运行`TestKestraSSHIntegration`通过：真实引擎、独立PostgreSQL、两个真实SSH协议合成服务，部分失败、失败目标单独新Execution、成功目标不重复、幂等及严格Host Key；最终重试引擎SUCCESS。
- [运行记录](runtime-validation.json)：实际本地非rootAgent/mTLS执行、结束前stdout、退出码0、票据脱敏；Execution游标/筛选绑定、跨项目403；原生Pause→Resume→SUCCESS、Namespace文件读写、旧Revision409。
- [浏览器记录](ui-validation.json)：8项定向交互通过，页面无pageerror或Flows失败请求。截图见[screenshots](screenshots/)；实际查看Builder与目录截图并修正对比度/单节点过度缩放。
- `npm run build`（含TypeScript）通过。遗留全局共享mock seed约31MB、既有大chunk/Zod annotation警告仍在，不为本任务重构其他模块。
- `node dev/scripts/check-real-api-docs.mjs`通过54个登记项（5可视化+49Flows）；只证明Schema/引用一致，不证明所有接口闭环。

## 本轮开发补齐 · afe8cb9

| 之前缺口 | 当前完成的代码与交互 | 实际证据 |
|---|---|---|
| CMDB目标批次Schedule | Kestra原生Cron标记Flow + 幂等触发消费；每次重检当前owner授权/Flow版本/目标和通道，Triggers提供独立调度编辑器 | 原生Cron实际触发，创建真实Agent作业成功，检查后停用测试计划 |
| 每目标多步骤 | Command/Bash/Python、依赖拓扑排序、缺失/循环校验、环境变量传递前序stdout、fail-fast、实时步骤与持久结果；Quick Run编辑/保存、Task Runs查看 | 本地Agent两步骤及输出传递实际通过；依赖循环用例拒绝；浏览器实际保存带steps默认值的Flow |
| 二进制资源 | 版本/SHA256/并发检查/历史下载/指定版本分发至Agent uploads；Files上下文UI | 含NUL/非文本字节的真实文件保存、实际分发成功 |
| 持久事件续传 | PostgreSQL事件日志、递增ID、Last-Event-ID/after、history JSON、权限验证与200项分页补齐 | 两次真实SSE重连均只收到更新ID；跨执行游标400 |
| 审批执行对接 | 复用现有审批/签名；申请附完整冻结作业快照供审核，执行校验SHA/目标版本/领域权限/有效期/当前签署资格；单审批只绑定一次新执行 | 待审批拒绝、自批403、独立DL签署、改脚本拒绝、批准作业成功、重复使用批准409 |
| 终止链路 | 引擎停止调度 + 精确Agent Task取消/SSH连接中断；未知副作用保留RECONCILING | 实际Agent睡眠任务被终止，后续命令未执行，target FAILED/engine KILLED |
| 部分失败一致性 | 批次Loop显式transmitFailed=false，不因某目标失败取消其他设备；设备结果与Engine状态分开展示 | 修复定向SSH检查暴露的取消竞态后，部分失败/失败目标新执行/幂等检查通过 |

新增检查：[运行证据](completion-runtime-validation.json)、[6项页面交互](completion-ui-validation.json)、[确认页截图](screenshots/completion-confirm.png)。本轮未运行全平台回归、未导入原环境资产、未连接生产设备。

## 当前能力边界与待设备验收

- 开发补齐已完成上述五项缺口及终止链路，不能把下列部署/适配验收标成真机通过。
- 每目标最多32步，按确定性顺序执行，不承诺步骤内部并行；单步骤结果/传递值32KiB，超限标注。Agent版本0.2.0/`job.pipeline`能力；旧Agent缺能力时按预检阻断或在接受前选择批准的SSH。
- Namespace资源≤1MiB；这不是OS镜像仓库。文本编辑128KiB限制保留。事件当前持久保存，没有擅自制定生产保留周期/SLO。
- 高危作业使用每次冻结范围的独立审批，不能把批准记录当作长期无人值守调度授权；网络变更继续遵循Fabric批准闭环，NH未决规则不变。
- 生产SSH Bootstrap、真实NOS/发行版、8目标混合设备、生产规模与长期运行验收尚未执行；真实设备与运维预置条件由后续联调提供。
- 原BOM/拓扑环境8081包含本分支尚未合入的Agent修复，未来部署必须保留该修复；当前仍只运行独立5190/8096，没有合并main。

## 快速接续索引

- [真实API](../api/flows.md)、[OpenAPI](../openapi.yaml)、[机器注册表](../api/real-api-registry.json)。
- [本地运行](../../dev/deploy/flows/README.md)、[非秘密配置示例](../../dev/deploy/flows/.env.example)、[原生Flow示例](example-native-flow.yaml)。CMDB批次模板从实际`GET /flows/templates`获取，避免手写第二套协议。
- `dev/backend/internal/flows/{service,transport,gateway,kestra}.go`：执行主链；`cmd/agent/`：目标端；`cmd/api/flows.go`：路由；`dev/src/features/flows/`：业务页面；`dev/src/lib/api/flows.ts`：唯一页面数据入口。
- 本文区分开发完成、明确能力边界与未运行的生产设备验收。


## 用户授权发布 · 9f16065

已部署到现有5180/8081；兼容纳入原BOM修复4403c63/9f16065，不修改其实现语义。原数据库应用021…024前保留完整私有dump，未导入隔离合成行。原admin登录/5项只读API/3页面渲染共9项通过，容器回调可达且无票据403。证据[deployment-validation.json](deployment-validation.json)。临时5190/8096与合成Agent关闭；当前运行/回退以HANDOFF顶部发布节为准，不再使用先前隔离环境描述作为当前状态。

## 原生Kestra UI接入 · feat/jobs-kestra-ui · 开发中

- 用户已授权实施，替换方案及LC-01…14为当前范围。新目录`/home/ubuntu/keel-kestra-ui-dev`，原5180/8081没有切换。
- 已固定上游commit49b62a2，纳入原生KsDataTable/KsTableColumn/KsButton/KsInput/KsExecutionStatus及19个直接依赖文件；许可和来源见`dev/src/vendor/kestra/`。不是完整原生应用已迁移。
- Vue3挂载到React模块内，原生SFC由Vite Vue插件编译；样式通过PostCSS限定在`.keel-kestra`，主题令牌也作用于模块范围。上游完整UI要求Node24，本次实际证明的是所引入组件子集可在现有Node22/Vite6编译，不代表完整编辑器依赖已验证。
- 作业列表与Execution列表接入原有Provider，原生分页/搜索/状态/行跳转，运行和QuickRun仍接既有流程。当前是已加载数据的本地分页，不冒充已完成服务端total/统计；执行历史仍通过原游标加载更多。
- `npm run build`通过；浏览器借助请求拦截加载新分支dist并读取现有API，两原生列表挂载无pageerror/console error。没有新服务/端口，没有改测试数据。检查发现空态翻译键显示已补中文消息，字体按上游16px根字号换算为模块内12/14px，避免Keel根字号压缩原生组件。
- 尚待：原生多面板编辑器/执行详情及生命周期页、完整KSFilter、URL筛选/列管理/批量动作、服务端目录分页与最近执行摘要。当前不能称为完整前端替换。
- 保留并迁入此前未发布修复：QuickRun保存真实Revision、历史Execution精确Revision恢复、完整原生模板转换、Files载入任务脚本；Go定向测试及编辑器回归通过，尚未覆盖当前BOM集成二进制。
- 接入上游RunTimeline.vue（保留许可证，标注Keel导入/日期/类型及popover隔离改动），从真实Execution.state.histories展示状态历史；不填造历史。此组件当前仅完成接线和编译检查，未在真实有历史的执行记录上完成交互验收。
- Native列表搜索/状态保留在URL；当前仍是过渡目录与已加载执行数据筛选，服务端完整分页/total/聚合尚待实现。
- 最新分支构建（TypeScript+Vite）通过；[三项浏览器证据](native-ui/validation.json)与[当前原生目录截图](native-ui/catalog.png)已保存。搜索URL/刷新恢复检查通过，无页面或console error；不是编辑器、真实设备或完整生命周期验收。

## 原生编辑器接入增量（开发中）

- 接入上游KsEditor/Monaco和MultiPanelGenericEditorView/MultiPanelTabs，当前YAML与只读结构面板；不是No-code已完成。多面板作用域按项目/Flow隔离，仅持久化布局UID，不保存执行目标或秘密。
- Vue编辑器通过既有React回调更新同一YAML；只读与保存草稿快捷键受权限/忙状态约束。原生面板组件去除上游外部统计调用，并连接实际快捷键说明面板；弹层保持在模块内。
- 修复接入时monacoSetup的ASI初始化错误；一次完整构建被137终止，裁减到本期YAML实际使用的generic Worker后构建通过。JSON/TypeScript语义Worker未开放，不暗示支持；源文件语法配色/编辑与Go侧校验继续独立。
- 浏览器三项检查通过：原生Monaco挂载、实际键盘输入更新YAML、Visual/YAML切换保留内容，无pageerror。证据：[editor-validation.json](native-ui/editor-validation.json)、[截图](native-ui/editor.png)。结构面板动态渲染的间距样式随后定向修正，最终视觉需下一次页面检查；未声称全编辑器验收。
- Monaco版本0.56与上游一致，附LICENSE/ThirdPartyNotices。新引入的DOMPurify依赖已锁定到修复补丁3.4.17；未进行无关全平台依赖升级。

## 暂停接续 · 用户要求记录后暂停

本次停止开发，当前完整范围约20%（功能估算）。最新接续入口移至HANDOFF顶部，列出全部未完成与环境边界。

暂停前Outputs增量：复用上游SidebarList/explorerSearch和原生搜索/折叠/标签组件，接入当前Execution实际outputs；支持搜索、选择、值预览、复制和完整返回值JSON下载。不是完整Inputs/Variables表达式调试器，现有Artifact授权下载链接继续保留。

`npm run build`通过；浏览器仅注入明确synthetic执行，挂载、搜索选择、下载三项通过，无页面错误，未写业务库/操作设备。证据：[outputs-validation.json](native-ui/outputs-validation.json)、[截图](native-ui/outputs.png)。不是实际引擎产物闭环。
