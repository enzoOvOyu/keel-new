<a id="prototype-parity-fixes-20261102"></a>

## 2026-11-02 原型复核后的实际修复（未部署）

用户要求继续开发，以下覆盖下一节原始审计中的“尚未修复”状态；原审计保留为发现依据，不删除失败证据。

| 审计项 | 当前状态 | 实际证据 / 保留限制 |
|---|---|---|
| JUI-01/02 | 已修复并定向检查 | dirty含元数据修改，禁止直接运行；源版本读取Execution冻结定义，失败不回落最新 |
| JUI-03/05 | 已修复并定向检查 | Topology、TaskRun与日志共享URL上下文；缺失设备关联显示不可定位，绝不拿其他任务日志代替；循环多实例选择器已实现，未做全量循环场景验收 |
| JUI-04 | 已修复并定向检查 | 60条Agent以25条切片，第二页实际变化；历史/目标列表分页同样绑定；修复切片数组触发表格选择事件的循环 |
| JUI-06 | 新增/替换交互已修复 | 当前任务默认替换候选，保留其他任务；固定版本来源已持久化至Flow变量，可重开原文件；动态跟随最新版仍不支持 |
| JUI-07 | 已实现，关键返回检查通过 | 筛选、分页、列设置与return_to，内存滚动；非原型像素级或全场景滚动验收 |
| JUI-08 | 已实现 | 全部/运行中/需要关注+真实聚合；关注由服务端过滤，Go隔离SQL检查通过；不把加载数量当总量 |
| JUI-09 | 已实现适用交互 | 受控步骤按依赖拖动；Log多行；JobStep描述/时限已按同一契约接通，复用Task时限与能力选择 |
| JUI-10 | 已实现并定向检查 | 输出逐层、源任务、冻结定义、授权文本预览；仅小文本，非任意二进制/HTML预览 |
| JUI-11 | 已实现并定向检查 | 执行名称实际持久化；批准候选、待审提示、目标revision、重跑输入Diff；后端仍重新判定批准 |
| JUI-12 | 已修复已确认的布局问题 | 五核心面板顺序、右区复用、START/END视觉终点、原生主题、输出main填满；已查看截图，不宣称全页面像素100% |
| JUI-13 | 已实现适用交互 | 指标task_id/当前页时间筛选→日志、状态历史事件→日志时间窗；未提供全量指标时间范围查询 |

验证共7组关键修复与3组增强（synthetic API拦截，零后端/设备写入、零pageerror）；稳定ID映射和返回路径检查通过。输出工作区布局定向测量主区宽1032px、顶部偏移0，修复全局main/Sidebar样式污染；截图`parity-correction/output-layout.png`。修复URL连续事件相互覆盖、冻结定义Worker缺初始化。隔离数据库测试通过；最终TypeScript/Vite生产构建通过（Vite1m34s），保留既有chunk体积提示；不重复全仓或无关BOM检查。

协议：新增Run/Execution可选name、目录attention布尔参数；其余复用既有Provider。没有migration、没有部署、没有合并；Identity审批列表登记为复用接口而非本轮新后端。ZTP在Fabric联调。

<a id="prototype-parity-audit-20261102"></a>

## 2026-11-02 原型一致性复核：仍有偏差，尚未修复

**审计基线**：`dff0a8a` / `feat/jobs-kestra-ui`。本次只核对原型、现有代码和当前页面，不改业务实现、不部署、不合并。用户已明确ZTP事件匹配在Fabric模块开发时联调，不列为作业平台本轮未完成项。先前“整改已完成”仅能表述为该批基础交互接通，不能代表整体原型一致性已验收。

依据：本目录prototype/{Prototype,EditorPreview,ScriptFilesPreview}.vue、KESTRA-UI-REPLACEMENT.md的页面及LC矩阵。四个当前问题已通过隔离浏览器复现，证据`parity-audit/result.json`及截图；API全拦截synthetic，零后端/设备写入、零pageerror。不把没有页面崩溃当成交互正确。其余项目是定向源码确认，未冒充浏览器逐项验收。

| ID / 优先级 | 与批准设计的偏差 | 核对结果及位置 | 最小整改方向 |
|---|---|---|---|
| JUI-01 / P1 | dirty定义仍可运行；原型dirty时禁用运行 | **浏览器复现**：NativeFlowHeader运行按钮未判断dirty；FlowsWorkspace的onRun传入已保存flow | 禁止混淆草稿与执行版本：保存/发布后运行，或显式选择运行已发布版本并清楚说明；不静默执行旧定义 |
| JUI-02 / P1 | Execution“源版本”跳到Flow的普通Revisions列表 | **浏览器复现**：NativeExecution只带tab=Revisions，不带冻结revision，也不打开现有executionDefinition接口 | 使用该Execution实际冻结定义，展示Flow Revision/引擎版本；缺失时明确不可用；不回落到最新版本 |
| JUI-03 / P1 | Execution Topology与Task Run/Attempt/日志有两套选择状态 | **浏览器复现**：TaskRuns.tsx本地selected未接外部URL context；点resolve_targets后右栏仍validate_topology | 统一选中定义节点与具体TaskRun实例的映射；循环多实例需明确选择，不随便取第一个 |
| JUI-04 / P1 | Agent分页控件没有实际页码/切片绑定 | **浏览器复现**：NativeAgents配置pageSize=25，但60条fixture全部渲染；history列表也使用固定pageSize未绑定状态 | 按现有完整列表在UI切片并绑定分页/筛选重置；若后端分页则明确同一契约，不重复分页 |
| JUI-05 / P1 | 目标设备→日志并未准确定位到设备TaskRun | **源码确认**：NativeExecution的filteredLogs仅过滤task_run/attempt，不过滤或解析context.target；“查看设备日志”只改Tab，旧任务上下文仍在 | 复用稳定asset_id与真实TaskRun/JobStep关联，缺失映射时明确无法定向定位；不按名称推测 |
| JUI-06 / P1 | Files缺少“新增/替换所选任务”的明确选择及返回编辑来源 | **源码确认**：NativeFiles只有快照确认；FlowsWorkspace.scriptTarget只有“从Files载入”动作设置，不跟随当前Visual选择；未保存路径/版本的任务级来源绑定 | 补目标任务选择和适用性校验；先保留快照语义，再用现有版本引用扩展明确来源。不要承诺文件修改自动同步任务 |
| JUI-07 / P1 | 列表→详情→返回不能恢复筛选、分页与视图 | **源码确认**：FlowsWorkspace的onOpen和NativeFlowHeader/NativeExecution返回动作使用裸路由；列配置是Vue临时ref，保存视图仅存URL过滤 | 保存授权作用域内的return context，并恢复URL/页码/列选择/滚动；不把目标范围写入全局可变存储 |
| JUI-08 / P2 | Executions页并非原型“全部/运行中/需要关注+紧凑汇总” | **源码确认**：NativeCatalog使用记录/运行概况/跨执行日志/失败/排队/暂停；记录固定active；业务Flow名称已在API但列表仍主要显示engine_flow_id | 保留已有查询能力，按原型重组页签、当前态和汇总；展示业务名及引擎ID，不另造总数 |
| JUI-09 / P2 | 受控Shell/Python任务缺少原型拖动排序；脚本属性较原型退化 | **源码确认**：NativeVisual受控卡片draggable=false且无移动动作；Log message通用单行input，而原型为多行textarea；JobStep当前没有独立description/timeout字段 | 补符合依赖语义的排序交互及多行编辑；对JobStep属性先核对/补契约，不能只增加无效字段或重排后破坏DAG |
| JUI-10 / P2 | 变量/输出尚为顶层列表+JSON，文件预览与产出任务定位不完整 | **源码确认**：NativeOutputs仅Object.entries展开一层；ArtifactLinks只有下载，未实现LC-04的递归/文件安全预览/源任务关联 | 保留脱敏和授权下载，按真实结构增加递归浏览、定位和安全预览；不能把任意HTML直接内嵌 |
| JUI-11 / P2 | QuickRun少任务名称、确认信息与审批交互不够产品化 | **源码确认**：原型配置runName；正式只有可选“保存Flow名称”；最终逐台决策不显示冻结revision，审批要求手输记录ID，输入差异未呈现 | 明确一次执行标题的真实契约；最终展示冻结范围/版本/差异；复用已有审批记录选择，不把ID输入当完整审批体验 |
| JUI-12 / P2 | 视觉语言和多面板空间利用仍未完全还原 | **截图+源码**：正式节点/START-END/子任务组织与原型不同；toolbar顺序多出模板，部分面板仍使用旧React Panel/表单；Visual+Topology+Files三开时Files过窄 | 保留原生多面板能力，统一标题/属性/节点/留白/状态样式，Files优先复用右侧面板或明确最大化；不为一致外观伪造依赖 |
| JUI-13 / P2 | LC扩展仍有交互缺口，不能全标完成 | **源码确认**：Execution Metrics只有分页表、无任务/时间过滤及日志定位；状态历史仅展示、未接事件定位；重跑已有预检但缺前后输入Diff | 使用已有指标/审计/重跑契约补定向交互；缺字段时扩展同一API，禁止仅加空按钮 |

**允许且必须保留的差异**：正式应用保留Keel全局Sidebar/ProjectContext/RBAC，不复制原型的隔离侧栏；真实预检、独立审批与维护态限制不能换成原型模拟成功。原型展示的Namespace动态引用/Worker脚本没有获准的正式执行契约，当前禁用并使用设备脚本快照是边界，不列为可通过改UI直接消除的缺陷。库与语言差异本身不是问题，具体页面质量和行为才是验收对象。ZTP属于Fabric后续联调。

**整改依赖**：先JUI-01/02/03/05（执行版本及上下文正确性）→ JUI-04/06/07（目录/文件/返回闭环）→ JUI-08/09/12（原型视觉与布局）→ JUI-10/11/13（需核对接口字段的增强）。前两组多数可复用现有API，不需要真机；TaskRun与资产/文件来源、执行标题等缺少完整字段的部分需先收紧契约。没有证据前不估计“原型还原百分比”。

<a id="prototype-parity-correction-20261102"></a>

## 2026-11-02 原型一致性整改与后续交互补齐

用户要求：先完成当前原型一致性整改，再完成下列新增交互整改；直接连续执行，不逐项等待确认。ZTP/设备事件仍按已记录安排暂缓，不扩大到该业务域。

**当前呈现整改已接通，后续专项交互继续：**
- Flow目录恢复原型状态页签、保存视图、近期执行历史条、触发方式及更新时间；真实数据从统一Provider/API取得，不造历史或时间。
- Builder移除重复表单/动作，隐藏底层wrapper实现细节，保留同一canonical定义、多面板、领域节点和授权执行。
- Overview/Revisions/Triggers等替换残留旧呈现；执行详情恢复任务、Attempt、内联日志同屏组织。保留既有生命周期和业务能力。
- 同视口、同一份synthetic数据对照原型验证，分别记录视觉还原与业务联动，不沿用工作量估算充当原型还原率。

呈现整改证据：parity-validation/result.json使用批准原型同一份data.ts、1600×1000视口，5组交互通过、零后端写入；已查看目录/Builder/Execution/Revisions截图。保留正式Keel全局壳层，不复制原型隔离侧栏。子任务默认收起和目录行密度随后做了定向呈现修正，最终截图随交互补齐一并更新；不宣称像素级100%一致。Go查询集成通过，TypeScript通过首轮检查。

**追加交互已实现并通过定向浏览器检查（synthetic API，不是真机验收）：**
1. 新建交付任务五模块：按批准原型Visual / YAML / Topology / Files / Docs逐项核对完整性、切换保留、同源联动及保存/运行路径。
2. Files输入交互：新增/导入、路径与语言、在线编辑、校验、保存版本、未保存处理、使用脚本、来源版本和错误反馈。已改为原生文件树/Monaco/新建与导入弹窗；新建Flow文件草稿绑定后延续，保存冲突保留内容。
3. 添加任务显式提供Shell Script、Python Script，实际接入受控设备任务与脚本编辑；已添加独立类型选择器及选中任务属性联动，不开放未授权Worker执行或伪造插件。
4. Quick Run：目标→脚本/通道→预检确认，覆盖前后返回、修改后预检失效、输入校验、审批、冻结目标/版本、提交结果与失败反馈。三步前后切换、脚本修改后旧预检失效、冻结目标/版本提交和维护态禁用已检查；审批最终由后端重新授权。

专项证据：`parity-validation/scripts-result.json` 与 formal-files/formal-quick-run 截图，5组交互、零浏览器异常、零后端/设备写入。覆盖新建Flow文件草稿转移、导入、保存冲突后重试、已保存快照使用、Shell/Python任务、设备Docs和QuickRun变更后重新预检及版本冻结提交。`TestDeviceTaskDocumentationUsesJobContractWithoutEngine`通过；客户端校验检查合法流水线、数值范围、重复/缺失/循环依赖和输出绑定。文件草稿仅保留内存，未保存时刷新会丢失并提示；动态Namespace引用仍不支持。没有执行全仓/真机/规模验收，完整构建结果见最新HANDOFF。

所有条目分别记录实现、验证、限制；在feat/jobs-kestra-ui继续，不合并/发布到当前测试环境。代码索引：flows/{FlowsWorkspace,Builder,NamespaceFiles,QuickRun}.tsx，jobs-native/Native{Catalog,Visual,Workbench,Execution,QuickRun}.vue；API契约仍以docs/openapi.yaml为准。

<a id="ztp-event-follow-up"></a>

## ZTP / 设备事件后续事项（2026-11-02，暂缓）

用户最新明确：ZTP事件匹配在Fabric模块开发时联调；作业平台这里只保留接续点，不计为本轮缺口。本事项不阻塞当前作业平台UI交付，也不表示相关能力已完成；本轮不新增消费者、事件接口或部署。

- **现状**：受控设备作业已有手动、Cron及历史补跑入口；ZTP业务消费者、设备Webhook/事件到Job Service的受控Adapter尚未接通。`source_module=ztp`和列表来源筛选仅为关联信息，不是事件接入证据；界面继续明确未启用。
- **接续前提**：先核对届时ZTP真实后端、事件生产方、事件类型与载荷、稳定事件ID、项目/设备归属、服务身份及重试约定，再冻结OpenAPI/领域事件契约。未确定的内容不自行补造。
- **接入路径**：事件生产方 → 受控消费Adapter → 现有统一Job Service → Execution；复用CMDB稳定资产ID、当前项目授权、目标快照/版本、预检和审批，不另建调度器或执行库。
- **可靠性与关联**：事件ID与项目绑定幂等；重复投递不得重复创建设备执行。来源事件、ZTP记录、Execution及审计可双向追溯；结果未知先对账，不能盲目重发；maintained设备不得恢复SSH/Agent。
- **届时必要验收**：有效事件创建并关联一个执行、重复事件幂等、跨项目/未授权事件拒绝、目标变化或维护态阻断、失败/未知结果恢复。没有对应运行证据前保持未完成。
- **定位入口**：`dev/backend/internal/flows/service.go`（Start/Preflight）、`schedule.go`与`backfill.go`（现有受控触发参考）、`cmd/api/flows.go`、`docs/api/flows.md`及OpenAPI。这里只提供接续索引，不预先确定新的接口或状态。

## 2026-11-02 剩余核心交互补齐（未部署）

- 执行目录补原生批量暂停/继续/终止、逐项适用性/状态/授权检查与结果；失败重试保持独立预检和审批。来源/实际通道/时间筛选从旧React页面接回原生列表，列选择按页面类型展示。
- Flow/Execution统计增加时间窗/精确Revision筛选及保持上下文的下钻；时长只取已采集终态证据，缺失单列，样本不足不显示经验百分位；daily为真实创建日UTC计数。未推定运行SLO或未采集历史。
- 不变：无设备Webhook/事件受控Adapter时明确未启用，真实ZTP消费者仍待相关业务域接入统一Job Service；不在UI任务中重建ZTP，不将source筛选当集成。Namespace动态Worker执行依旧受既有策略禁止；保存脚本通过设备steps快照使用。
- 定向批量与聚合筛选浏览器检查通过，隔离DB统计/版本过滤及样本不补零检查通过。最新全量TypeScript/Vite构建通过（1m13s）；构建产物的metrics-result.json两组检查通过，已查看页面截图。最后单日柱宽调整仅做SFC编译检查，不重复整包构建。107项契约引用检查通过。

## 2026-11-02 业务批次和身份生命周期增量（未发布）

本轮在feat/jobs-kestra-ui继续，既有521013b之后补齐：

- 服务端持久Bootstrap批次，逐项状态/暂停恢复/取消/阻断重试/UNKNOWN对账，浏览器不再串行持有SSH安装循环；安装成功仍等注册与身份批准。
- 历史Cron补跑窗口/时区/准确时刻预览及冻结hash；每项独立目标版本和approval_context，不复用旧Schedule批准，授权缺失不调用引擎。批次创建与新Execution实际成功分别展示。
- 失败目标或全部历史范围的新运行工作台；清除旧批准，目标不能超出源范围；Flow Revision改变要明确确认，再预检和申请新审批。
- Agent两阶段证书续期：旧证书申请、CSR持久化、新公钥证明激活、原子generation切换；在执行中/维护态拒绝。旧serial不再可派发；默认未启用，需新Agent参数和受控CA/HTTPS。
- 迁移044仅隔离库验证，当前环境不变；OpenAPI/生成类型/107项台账和模块契约同步。部署说明见dev/deploy/flows/README.md新增节。

实际检查：隔离DB的持久任务/UNKNOWN、PKI轮换、历史补跑审批隔离与引擎零写入，以及查询/秘密脱敏用例通过；Agent本地mTLS续期故障恢复检查通过。正式原生核心8组synthetic页面检查之前已通过，新补跑/重试/纳管交互与最终构建正在核对。浏览器拦截响应不等于真实HTTP/设备验收；旧BOM全包失败保留，不扩大无关改动。

新增定向检查：补跑预览/冻结hash提交/逐次审批、当前revision恢复、失败目标新预检、Bootstrap缺配置禁用4组通过；主题检查复现并修复ia-dark与Kestra原生令牌不同步。Trigger运行态/消费历史使用固定v2.0.7实际DTO，隔离查询与项目拒绝通过。调度页收拢为原生按钮、Monaco与目标折叠区，避免旧大表单。

最终TypeScript、Vite生产构建、Go API构建和107项契约引用检查通过；Vite先前内存失败在关闭仅本任务临时5193服务后成功重跑。临时迁移038因origin/dev已占用改为044，隔离库只更正本任务版本记录，不覆盖其他迁移。

尚待本轮继续：提交同步、剩余软件缺口及实际设备验收。历史Sunday=7/Quartz等Cron变体明确unsupported；批次列表最近50条。设备事件/ZTP生产接入、动态Worker Namespace引用未实现，不归类为“只缺真机”。没有覆盖未配置底座或扩大Worker插件权限。

# 作业平台重构实施记录（技术域 Flows）

## 2026-11-02 原生核心工作台接通（持续开发中）

原生多面板替换旧三栏Builder；Visual、受控设备步骤、YAML、Topology、Files/模板/Docs共享同一Flow定义，节点拖动限定同分支、未知字段保留。Flow头部和常用/更多标签、执行详情、三步QuickRun及Agents目录/抽屉已转原生组件；Keel外壳、项目上下文与审批保持原样。新增6个查询资源，详见docs/api/flows.md。没有发布或合并。

验证：正式路由8组synthetic浏览器交互通过，截图及结果在native-validation/；修复Monaco定位缺失导致遮挡下一步。隔离PostgreSQL+synthetic Kestra查询集成通过，含跨项目拒绝、SECRET变量、日志与相邻执行。上述不是真机或生产验收。后续持久化批量安装/恢复、证书轮换与调度补跑尚在开发；不据核心页面运行宣称全部完成。


## 2026-11-02 正式开发恢复：脚本文件工作台第一批

分支仍为 `feat/jobs-kestra-ui`，工作区 `/home/ubuntu/keel-kestra-ui-dev`。用户已要求按原型继续正式开发；本批未发布、未合并，不改变5180/8081/5181。

- 正式 `NamespaceFiles` 替换textarea：新增Shell/Python、多文件文本导入、搜索、Kestra Monaco在线编辑、保存版本、下载/历史、重新加载。复用已有namespace-artifacts API，保存带expected_revision，不走原来无版本的PUT。项目/用户/权限版本隔离的内存草稿和选择在面板重新挂载时保留，不写localStorage；刷新前提示未保存。
- 普通引擎Flow可将已保存脚本内联为新任务，内容进入待保存的Flow定义；任务ID避重。受控设备任务保持原来的steps.command与QuickRun/审批链，不静默变成Kestra Worker脚本。Namespace动态文件引用绑定尚未完成。
- 将原型验证过的Element Plus基础样式同步到`.keel-kestra`作用域，修正原生组件图标/加载等基础尺寸。保留二进制历史与Agent文件分发，折叠时不提前加载。
- 修复本工作树dev/fixtures旧链接仍指向已删除keel-demo，改为仓库根fixtures；不复制数据、不恢复旧应用。
- 验证：TypeScript、Vite生产构建通过（既有大chunk/依赖注释警告）；定向浏览器4组检查通过，无pageerror，使用拦截的synthetic契约响应，非真实Go/DB联调；内联脚本ID/快照检查通过。未做全量回归、真机测试或完整视觉验收。临时本地验证入口与服务已移除/停止。

继续位置：`dev/src/features/flows/NamespaceFiles.tsx`、`jobs-native/NativeScriptEditorHost.tsx`、`flows/script-task.ts`、`lib/api/flows.ts`。后续优先完成原型多面板Visual/Topology/Files整体替换、服务端Flow目录分页/total/近期执行聚合、Execution生命周期页。此前约20%只属于旧原生替换估算，本批没有重新量化整个任务；不能把文件切片当全平台完成。后端既有缺口（证书轮换、可恢复批量安装、设备事件/ZTP消费者、新聚合查询）仍保留。



## 2026-11-02 脚本原型闭环

补齐此前未完成的新增/导入/Monaco在线编辑/保存/引用闭环，入口为原型编辑器顶部及Files / 脚本。Namespace文件与内联任务明确区分，切换面板保留原型文件；实际上传和设备执行仍未接入。详见 [原型说明](prototype/README.md) 与prototype/scripts-validation.json。本次只改变独立原型，保持正式作业平台实施状态及5180/8081不变。


## 2026-11-02 独立原型UI修复

按用户要求精简演示侧栏，仅保留三个作业平台入口。补齐原生组件依赖的kel图标/选择框/加载等基础样式及作用域主题，修复巨型SVG；补齐Flow、目标、Agent分页绑定和列表滚动。只在jobs-preview展示，不改正式作业平台完成状态。证据见 [UI遍历记录](prototype/ui-audit.json)。


## 2026-11-01 独立样式原型预审

用户对UI/UX要求先看原型。入口及范围见 [原型说明](prototype/README.md)，现有5181新增`/jobs-preview/index.html`；正式5180和Go API未变。原生组件来源、有限任务编辑/布局/交互与synthetic边界均明确记录。原型不会提高正式产品完成率；用户确认前不把它替换到正式页面。


## 2026-11-01 作业平台接续检查点

本节只记录用户要求记住的暂停进度，不恢复开发或发布。已核对main合并c6822e8：智能开局工程后续合并没有修改Flows、jobs-native及Flows后端目录，因此原生UI重构仍约20%（该轮完整范围估算，非全平台或后端完成率）。

- 已完成：React/Vue桥与原生样式隔离；Flow/Execution基础列表、搜索与状态URL；RunTimeline接线；KsEditor/Monaco及多面板基础（YAML+只读结构）；Outputs搜索/选择/预览/复制/下载；QuickRun锁定执行修订及模板修复。
- 下一步起点：原生No-code节点/属性与Topology、Files/Docs；补服务端分页/total/最近执行摘要，然后接完整Execution详情（Gantt、TaskRun、Attempt、日志、变量、指标、依赖）及生命周期页面。
- 后续软件缺口：完整筛选/列管理/批量操作、并发/排队/补跑/跨执行及源码检索、QuickRun/Agents替换；后端证书轮换、持久批量安装/恢复、设备事件与ZTP消费者。不得称为仅等真机。
- 已有构建/浏览器检查证据见native-ui目录。Outputs为明确synthetic浏览器数据；不代表真实引擎闭环或完整视觉验收。原生UI未发布到5180。
- 恢复前先看HANDOFF最新部署节，核验当前main/运行进程。当前前端是工程工作树，API为engineering-5abb94f，不能从旧feat/jobs-kestra-ui基线覆盖工程模块和新迁移。后续按2026-11-01全局约定统一从最新dev继续，复用现有原生UI及有效后端，不重做已完成基础、不重复旧检查、不擅自部署。


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


<a id="prototype-file-source-restoration-20261102"></a>
## 2026-11-02 · Files 与任务属性原型还原

范围仅作业平台原型，沿用既有React/Vue壳层、canonical YAML、文件版本接口和JobStep；不扩展ZTP、不新建执行引擎、不发布服务。

- 导入脚本增加候选文件/大小预览、目标目录、改名、显式确认/取消；确认前不创建草稿。
- 文件使用支持 Namespace固定版本引用 / 内联快照，显式新增或替换兼容设备步骤。路径、版本、摘要保存至同一Flow.variables；固定引用内容随Flow冻结，不跟随“最新”。
- 从任务打开脚本定位原版本；历史只读；明确转为内联后允许独立编辑并保留来源。后端Save使用现有文件表核对项目、Namespace、版本与内容。
- Visual/QuickRun补齐步骤说明和超时；复用已有Task时限与Agent能力声明，不新增调度器。保存保持当前任务/返回路径等URL上下文。
- OpenAPI增加FlowsScriptSource和可选JobStep属性，生成类型、模块API说明与台账同步；没有新增DB表或API操作。

定向浏览器覆盖导入预览改名、替换选中任务并保留描述/时限、保存来源、重开固定历史版本和转内联，使用拦截synthetic API，零设备/后端写入、零pageerror。证据 `parity-correction/file-source-result.json` 与 `file-source-restoration.png`。隔离PostgreSQL `TestWorkspaceQueryIntegration`（来源/内容/跨项目）与 `TestStepOptionsUseExistingDeadline` 通过。

仍不宣称像素级100%或真实设备验收；Metrics时间筛选为已返回样本，不是全量时序查询。固定版本是本轮已实现语义，动态跟随最新版不在范围。ZTP由Fabric后续联调。未保存草稿仍为当前会话内存。

本批最终 `npm run build`（TypeScript + Vite）通过，Vite1m22s；API台账108项及diff空白检查通过。保留既有chunk体积和第三方注释提示。仅本任务临时5193已关闭，未改部署服务。

## 2026-11-02 授权部署

组合发布9441a89保留dev ab1bd85并接入jobs12f8b80，独立deploy/jobs-ui-20261102推送；未合并dev/main，未修改其他工作区。5180/flows和8081已上线，增量044迁移/私有备份完成。组合构建、5项认证GET及3个真实页面启动检查通过；不含设备/业务写测试。证据及回退见deployment-20261102.json。
