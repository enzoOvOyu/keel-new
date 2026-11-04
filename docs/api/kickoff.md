# Kickoff · 当前 API 使用契约

基准：dev；当前部署见[发布记录](../kickoff/DEPLOYMENT.json)。产品行为见[模块定版](../modules/kickoff.md)。[OpenAPI](../openapi.yaml)定义唯一请求/响应/枚举；[注册表](real-api-registry.json)链接实现及证据。

## 操作索引

| 方法 | 路径（省略 `/api/v1`） | operationId |
|---|---|---|
| GET | `/projects/{projectId}/kickoff/publications/{publicationId}/status` | `getKickoffPublicationStatus` |
| POST | `/projects/{projectId}/kickoff/history` | `replayKickoffHistoricalSource` |
| POST | `/projects/{projectId}/kickoff/calculations` | `calculateKickoffEngineering` |
| POST | `/projects/{projectId}/kickoff/publications` | `confirmKickoffPublication` |
| GET | `/projects/{projectId}/kickoff/publications/{publicationId}` | `getKickoffPublicationReceipt` |
| POST | `/projects/{projectId}/kickoff/issues/{issueId}/resolve` | `resolveKickoffIssue` |
| POST | `/projects/{projectId}/kickoff/sources/{sourceId}/analyze` | `analyzeKickoffSource` |
| GET | `/projects/{projectId}/kickoff/sources/{sourceId}/tasks` | `listKickoffSourceTasks` |
| GET | `/projects/{projectId}/kickoff/sources/{sourceId}/tasks/{jobId}` | `getKickoffSourceTask` |
| GET | `/projects/{projectId}/kickoff/workspace` | `getKickoffWorkspace` |
| PUT | `/projects/{projectId}/kickoff/models/{tableId}` | `saveKickoffModelInput` |
| POST | `/projects/{projectId}/kickoff/sources` | `uploadKickoffSource` |
| DELETE | `/projects/{projectId}/kickoff/sources/{sourceId}` | `detachKickoffSource` |
| GET | `/projects/{projectId}/kickoff/review` | `getKickoffReviewPage` |
| POST | `/projects/{projectId}/kickoff/preview` | `evaluateKickoffPublicationScope` |

## 作用域与权限

所有接口路由中的 projectId 经过统一 Session 项目授权；沿用后端 `bom:*` 权限标识，不因前端改名另建权限。GET及POST preview为 `bom:read`；sources/history为 `bom:import`；publications写入为 `bom:execute`；其余修改为 `bom:edit`。所有写请求沿既有CSRF，不保存秘密到浏览器业务状态。

路由当前会幂等确保 kickoff_workspace 锚点存在，因此首次GET并非严格无数据库写入；不产生资产发布、设备执行或AI调用。不可将读取历史版本误报为新发布事件。

## 加载、版本、发布

- GET workspace 默认摘要；`section` 明细、局部 `entity_id` context 与 `revision` 参数以OpenAPI为准。GET review服务端过滤/排序/分页；分页绑定同一修订，冲突409后重读，不拼接旧页。
- 请求与缓存按项目/修订/授权范围隔离，切项目取消旧请求。Drawer/历史差异按需；typed read cache是有界投影，数据库仍为事实源。
- PUT models保存输入版本；POST calculations执行确定性计算，不执行网络/装机任务。输入字段缺失保留needs_input/partial，不补零。
- POST preview只计算完整选中范围及依赖闭包，不批准、不发布。publications冻结对象及输入引用；202只表示请求被接收，轻量status轮询后以CMDB回执判断published。
- 同一范围的幂等/并发、字段和支持参数以具体operation及服务实现为准，不能把其他模块Idempotency-Key约定套给所有Kickoff操作。
- 历史回执按保存的闭包重建，不被新输入/选择逻辑改写；history复用同项目已保存候选，保持原身份/版本/来源。

## 错误、限制和验证

403为项目/权限拒绝；409为版本或当前操作冲突；422为输入/范围不满足规则。字段级错误和实际响应仍以ErrorEnvelope为准；不把所有错误自动重试。

`analyzeKickoffSource` 是已实现的明确拒绝入口：503 `KICKOFF_AGENT_NOT_CONFIGURED`。它没有成功分析结果，不因登记到真实接口表就成为Agent能力。

当前网络输入及CMDB版本引用不等于完整Fabric NetworkPlan；缺口见[决策记录](../DECISIONS.md)。[部署证据](../kickoff/DEPLOYMENT.json)、[后端定向集成](../kickoff/BACKEND-CHECK.json)、[性能与在线读取](../kickoff/PERFORMANCE-RESULT.json)各自限定验证范围。本轮历史资料理解已部署并通过定向真实HTTP检查。

## 历史资料理解派生读取（已部署）

- GET `sources/{sourceId}/understanding` (`getKickoffSourceUnderstanding`)：读取当前工程 head 的派生报告；未构建返回 unsupported/pending，不调用 AI。
- POST `sources/{sourceId}/reuse-history` (`reuseKickoffSourceUnderstanding`)：需要 bom:import；复用现有原件、Proposal Columns/Records 与对象证据生成 MinIO 派生报告。同 source/head 重复操作返回同报告，不修改工程修订、输入、审批或 CMDB。
- GET `sources/{sourceId}/regions/{regionId}/preview` (`getKickoffSourceRegionPreview`)：需要 bom:read；offset/limit（上限100），view=source|normalized。源行使用真实 Excel 坐标；normalized row=0 不是 Excel 行号。revisionId 是工程 head UUID，失效409；兼容 revision_id。
- 报告按项目/source/head/v1隔离，原件只在显式构建时由现有敏感单元格脱敏 Parser 读取；区域、预览分对象持久化，不把整个工作簿放入 workspace 或无界内存缓存。
- 结构使用现有 WorkbookIR 空白行分隔/256行分块，不宣称完整业务表边界。区域语义仅取精确单元格对应对象类型；同区域可含多类，不根据 Sheet 名猜测。
- 历史 reviewed_model 为对象级、无单元格坐标。恢复唯一 SN/MAC 相等或有效历史 Proposal 的映射仅为 `_candidate`，不计入已确认字段覆盖；未知和敏感单元格不做映射。结构、区域、语义、关系、字段分别报告计数、依据与缺口；历史发布不使字段步骤通过。
- 派生报告可重建，不是新事实层。不新增 Agent、映射向导或资产发布；analyze 仍503。读取存储故障503，缺报告/区域404，构建失败500，不伪造成功。

定向结果见 [HISTORY-UNDERSTANDING-CHECK](../kickoff/HISTORY-UNDERSTANDING-CHECK.json)：MM 恢复54个结构分块、29,583个候选定位单元格、7,154条同原始行双端物理端口精确组合候选；明确字段证据仍为0。关系完成量是有坐标引用且确定性校验通过的连接数，不以已有连接总数填充；字段分母仅为明确映射的对象字段，标题/备注/重建候选不算必填字段。步骤按覆盖和校验计算 complete/partial/pending；无适用连接为 unsupported。workspace 不额外读取 MinIO 报告，报告和预览只由来源页按需加载。

## 数据核对信息与筛选（已部署）

GET review 保留既有分页协议，新增明确类型的 `facts` 和 `signals`。设备身份、型号、端口名称、速率、MAC、角色及位置分别返回；不再以 description 的 UI 拼接作为字段语义。signals 包含 code/label/severity/impact/action，是当前工程核对的只读发现，不新增审批门槛，也不是观测健康结论。

- `filter` 选择身份阻断、待定位、占位待核对、型号缺失、未连接、多次引用、端点缺失、待入库或已入库；旧 issues=true 仍支持，明确 filter 优先。
- `type/model/rack/source` 分别按类型、型号、机柜ID和来源ID筛选；q搜索身份、端点、MAC和位置。连接栏的类型/型号/机柜属性筛选当前针对本端设备；完整枚举见OpenAPI。
- counts 在当前 category + 搜索 + 属性筛选的完整结果上计算，再应用快捷 filter 和分页；不同缺口可重叠，数量不能相加当作总数。facets 是当前栏目全量候选选项。
- 已入库按相应对象的 publication_object 回执判断，不由父设备状态推定子对象。缺机柜、U位、高度、容量、占位重叠/越界分别展示；这些只读发现不改变既有发布校验。
- 前端筛选/搜索/排序/分页保存在URL；Drawer关闭后保持原位置。每次只返回当前页，不下载全量端口/连接到浏览器。导出沿同一筛选逐页读取。
- 右侧事项区仅显示当前页对象的发现及处理入口，不罗列整个项目无关事项。稳定ID收纳于抽屉“内部标识”；SN与名称相同只显示一次。设备抽屉端口表明确分为端口名称、速率、角色；不拼接速率到端口名称中。

验证范围见 [REVIEW-IMPROVEMENT-CHECK](../kickoff/REVIEW-IMPROVEMENT-CHECK.json)。当前已部署版本为12af6e4，五栏及待定位筛选的真实HTTP只读检查通过。

## 入库范围确认与重复提交（已部署）

页面以选择设备和确认范围为主。入库说明并入范围卡片，右侧展示版本与增补记录；原确认按钮及确认弹窗的文案、结构与回调保持不变。列表展示设备身份、类型、型号、真实入库情况和核对入口，SN不重复展示。已成功入库设备不默认勾选；身份冲突或正在等待回执的设备不可选，已有回执设备可按需选择增补。全选、取消、清空及表头部分选中状态完整；成功后清理已完成选择。范围预览绑定设备选择及工作区修订，切换选择时不把上一范围计数当作本次计数。

POST publications 保留原权限、事务、同 expectedRevision 幂等与 Outbox；新增写入前的覆盖检查：同项目、同不可变工程修订、全部输入版本相同，且历史记录覆盖所选设备范围时，published 返回422 KICKOFF_NO_CHANGES，pending 返回409 KICKOFF_PUBLICATION_PENDING，不创建新的发布/Outbox/审批/审计事件。使用既有ErrorEnvelope，前端统一错误映射提供中文说明。请求响应Schema未变。

这是针对已发现重复确认的严格版本覆盖检查，不是完整字段级增量计算：工程修订或输入版本改变、或新设备范围不被单条记录覆盖时，仍可按原规则发布。不会因设备已经入库就永久禁用其增补。没有新增数据表、迁移或CMDB写入渠道。

证据见 [PUBLISH-SCOPE-CHECK](../kickoff/PUBLISH-SCOPE-CHECK.json)；合成UI测试不等于真实MM发布验收，真实数据仅用于只读SQL覆盖核查。

## 可选设备判断与核对入口（已部署）

GET publication-options（getKickoffPublicationOptions，bom:read）在同项目只读快照中返回revision及逐设备new/changed/unchanged/blocked/pending，不返回整本工作簿或完整拓扑。比较现有类型化设备、所属组件/端口、关联连接、机柜/落位和相关工程输入与已成功发布范围；忽略工程修订/证据ID、输入版本号和行顺序，仅内容变化才标changed。发布内容基线按历史对象累积，避免后续小范围发布使原有连接被误判为新增。物理删除仍走CMDB变更流程，不在本接口隐式删除。

入库页只允许new/changed选择和全选；unchanged显示“已同步·无变化”并禁用；加载失败不默认放行。已有pending及身份冲突禁用。首次数据、修订和回执状态变化时读取判断，成功后清理选择；不修改确认按钮与确认弹窗。后端在创建发布前复用实际内容判断，全部所选对象无变化时拒绝创建新记录。

核对链接优先以SN定位；数据核对“处理”列不再留横线：有事项调用原处理流程，无事项提供“查看详情”。这不把没有待处理事项解释为运行健康。

实际MM只读检查：971台中808台无变化禁用、163台待增补可选；核对跳转后处理列有有效操作。项目修订12/发布记录3未变；没有真实发布写入验收。

### 2026-11-03 性能实现说明

HTTP字段与业务校验不变。按项目、工程head及workspace修订复用有界派生读模型（最多3份，空闲10分钟）；publication-options与capabilities还核对发布ID/回执状态，避免pending→published后复用旧结果。历史区域按source/head复用。发布事务继续实时验证，批量SQL不放宽项目隔离、来源、Outbox及回执门槛。真实验证和冷/热耗时见[性能记录](../kickoff/PERFORMANCE-20261103.json)。

### 首次打开派生快照（802a001）

GET workspace（无section）可读取同项目MinIO私有派生摘要；命中前用数据库一致性读校验名称、修订、模型head、资料状态/head及发布状态。一个对象覆盖旧派生版本，不作为正式工程事实；失配/不可用回源。响应根据Accept-Encoding支持gzip，JSON Schema未变化。快照实现版本变更须升级workspace-read版本标识；测试覆盖失效与进程缓存清空。见[冷打开验证](../kickoff/COLD-OPEN-PERFORMANCE.json)。

### 设施增量与分域 Agent（66ce278已部署，待统一测试）

publish/preview 新增可选 `facilityIds`，设备可为空但总范围不能为空。设施选项返回实际差异和 `label/kind`；空机柜/机房、容量坐标可独立发布，祖先设施进入冻结闭包。回执增加 `facilityIds`，counts区分 `facilities`、`locations`、`placements`。引用同项目已发布端口不重复发布对端设备，端点不明仍阻断对应连接；比较基线只推进实际发布的连接事实，不能覆盖对端其他事实。幂等同时比较设备/设施请求范围，继续使用原Outbox事务。

Agent操作：
- `POST sources/{sourceId}/analyze`：`bom:import`，请求 `{expectedRevision,domains?}`，返回202任务ID；仅排队，不在HTTP请求中调用模型。域为equipment/facility/cabling/network/power/provisioning。
- `GET sources/{sourceId}/analysis?jobId=`：`bom:read`，返回持久状态、覆盖总数/截断提示、每表最多20行候选及其证据；完整结果在MinIO。前端仅pending/running时轮询，离开取消读取，不取消后台任务。
- `POST sources/{sourceId}/analysis/adopt`：`bom:edit`，请求 `{jobId,expectedRevision,tableIds?}`。人工采用只新增/补空，分别校验工作区版本、工程源版本与表版本；不自动发布CMDB。已采用表不重复默认选择。

原understanding/region-preview按同项目新Agent任务返回真实结构、区域、语义、关联、字段五步证据；无新任务保留历史读取。`origin=agent`不代表候选已确认。新Agent复用现有Parser、Provider和eng_job/checkpoint，23类工程输入与四类物理实体显式区分，旧StepInputCatalog候选不自动转换。仅支持XLSX，不宣称OCR/PDF解析或真实MM识别效果已经验收。

机房机柜仍为四类输入：真实覆盖率与缺口筛选、实体选择、批量预览保存。坐标ΔX/ΔY是同一参考点的坐标步距，不是机柜净距，无默认现实距离；型号高度补齐保护实例已有不同高度。机柜预览复用原CMDB工作台，入口明确只展示已入库数据，没有新建草稿画布。

接口字段以OpenAPI为准。常见失败：400域/表/任务参数，403授权，404资料/任务，409版本变化/任务活动/结果未就绪，422候选或依赖不成立，503MinIO不可用。模型失败保留异步失败状态和来源；单域缺输入保留其余域候选。代码已编译并部署，仅启动/静态资源检查通过；业务测试、Provider、真实资料识别及入库验收均NOT RUN。

发布选项基线：已成功独立发布的设施事实会同步推进引用该facility事实的既有对象基线；只更新已发布设施键，不覆盖其他设备、关系或工程输入的未发布差异。HTTP契约不变。统一范围UI仍使用deviceIds/facilityIds，依赖对象随范围核对。

## 拓扑前置资料检查

机房与机柜页复用CMDB hierarchy读取已入库机房/机柜/设备，connections(status=incomplete,limit=10)按需返回缺项，另以limit=1判断是否有已登记连接；不加载全量端口/线缆。五视图资料状态不是观测验收。补录复用原工程输入与CMDB关系变更入口；角色工程输入到CMDB角色投影尚有缺口，不将草稿显示为完成。权限、项目、snapshot_id和查询失败沿用原API。

## Agent最小修复：分域复用与Excel机柜图

同指纹checkpoint中的其他域计划参与当前候选结果，局部重跑只请求所选域；源文件/Provider/提示词指纹不匹配时不复用。Provider中途失败时停止后续模型调用，若已有可展开候选则保留候选并返回对应域缺失原因；无候选时任务仍失败。失败计划不视为已完成checkpoint。取消、存储失败和claim失效仍终止，不绕过人工采用/版本检查。

Excel机柜图复用现有映射与证据：支持明确U刻度容量、U区间，以及占位块与U轴对齐的起始U/高度。裸数字刻度必须由Agent提出rack_u_axis分类，并通过双侧一致刻度、连续1..N及中间合并占位块的本地验证；普通行号、像素尺寸和无依据高度不被接受。单个未合并标记不推测高度。无固定Sheet/列/容量。使用原normalizationEvidence记录占位范围和刻度，不新增接口。

Understanding步骤新增not_applicable；总状态可为recognized，仍只表示分析结果，不能替代采用或发布。此次更新后提示词指纹改变，旧指纹不会冒充新规则分析结果。MM落位图一个真实区域本地验证52U及9个占位块；未调用真实Provider，不表示完整MM智能识别验收。

## 机柜逐行补录与供电检查

质量状态后仅一个补充信息入口，弹窗按缺项切换rooms/racks/placements/power_profiles/rack_capacity，复用原ModelEditor在线表格及XLSX导入。规划功率窗口限定planned行，保留其他依据；每表应用后可继续切换，未保存切换仍需明确放弃。保存工程输入不替代确认入库。

发布选中机柜时，rack_capacity.capacity_kw随原事务记录为CMDB planned_power_capacity_kw；设备planned_power_kw继续复用既有发布记录。供电不足或未知只影响容量判断，不扩大成全域入库门槛。

检查结果使用现有React Query内存缓存，按身份/权限/项目/修订隔离；完整查询结果不写入浏览器持久存储。页面重载后的缓存复用仍待修复，不宣称已完成。工程保存修订变化与手动刷新继续更新检查。

补录入口位于五个消费视图各行的资料状态后，已满足时隐藏；全局拓扑仅编辑归属，机柜与机房按实际空间/供电缺项切表，网络架构提供角色及连接缺项切换，物理布线和追踪显示双端缺项及现有CMDB关系变更入口。查询失败/不完整显示重新检查，不伪装成已知可补录缺项。

## 资料页任务与采用状态

资料包左侧紧凑展示，Agent任务在右侧；五步资料理解、区域预览与来源证据通过日志详情抽屉读取。区域预览仅在打开抽屉时请求，分析轮询不再重复触发理解报告加载。执行阶段使用不定进度条与活动图标，未提供服务端百分比时不生成百分比。

前端采用使用全局React Query Mutation，key含项目与资料ID；切换菜单后返回仍识别当前待完成请求，阻止重复提交。采用请求不绑定被销毁Provider的AbortController；成功后当前工作区及候选重新读取。此状态仅跨当前应用内路由保留，不表示已有服务端异步采用任务或浏览器重载恢复。原POST采用仍在同事务中执行，性能瓶颈和后端建议见[1103定位](../kickoff/AGENT-ADOPTION-1103.md)。

## 后台采用任务（覆盖上文同步采用描述）

POST analysis/adopt返回202与KickoffAdoptionTask，不等待合并；任务复用eng_job队列、claim、心跳和恢复，无新表。GET analysis/adopt为轻量状态读取，不加载候选文件；GET analysis.adoption也包含最近任务编号、pending/running/succeeded/failed、错误和成功回执。浏览器只在执行中读取状态；切页或浏览器刷新后从后端恢复，不依赖本地持久化。存在同资料活动采用时返回原任务；工作区修改冲突由后台明确失败，不绕过版本校验。后台重新核对提交者bom:edit权限，保留事务、证据、人工值和非自动CMDB发布规则。任务成功回执与采用事实同事务提交。

证据改为候选行摘要索引，设备SN及设备/端口名索引保留歧义处理，避免采用期间平方级重复扫描与哈希。失败可查看原因后重试，重试不重新调用AI。

用户要求回退：资料页已恢复d881421布局，撤回左右任务栏与日志抽屉设计；后台异步采用接口及必要前端任务状态保留。上文资料页新版布局描述仅历史，不作为当前页面说明。

## 后台 Agent 任务历史与详情（本轮源码，未部署）

- `GET /projects/{projectId}/kickoff/sources/{sourceId}/tasks`，operationId `listKickoffSourceTasks`：返回 `{items,offset,limit,total}`。默认 offset=0、limit=20；offset须为非负整数，limit为1..50，非法值返回400 `KICKOFF_TASK_PAGINATION_INVALID`，不静默截断。按createdAt倒序、id倒序稳定排序；空列表为items=[]、total=0，越界页仍返回真实total。
- `GET /projects/{projectId}/kickoff/sources/{sourceId}/tasks/{jobId}`，operationId `getKickoffSourceTask`：返回 `{task,steps,understanding?}`。task为 `KickoffSourceTask`；id、kind、state、createdAt、updatedAt必填，errorCode和analysisJobId按实际记录可选。kind仅analysis（kickoff-domain-agent/v1）或adoption（kickoff-adoption/v1）；analysisJobId仅采用任务的proposal.body.jobId，不推断最新分析任务。
- 两接口均要求统一Session及当前项目 `bom:read`，同时限定未移除的kickoff_source、同项目eng_source及eng_job.proposal.sourceId，不因共用工程源/文件摘要而串资料历史。无授权403；资料不存在/已移除404 `KICKOFF_SOURCE_NOT_FOUND`；详情任务不属该项目/资料/类型404 `KICKOFF_TASK_NOT_FOUND`。
- 详情steps仅该jobId已持久化的eng_step，按sequence升序返回，包含sequence/tool/state/detail/createdAt；没有日志返回[]，不借用分析任务日志、不合成采用步骤。失败码来自eng_job.error_code，不用错误码伪造日志。
- understanding复用现有SourceUnderstanding（OpenAPI `KickoffUnderstanding`）结构：仅读取当前分析任务、或采用任务显式关联且同项目/同资料的分析任务已成功持久化的kickoff_domain_understanding artifact。没有该证据就省略，不生成pending报告、不回退最新任务或历史报告；既有artifact读取/摘要校验失败按原接口返回500/503，不伪造结果。旧分析任务仅有结构artifact而没有理解报告时也省略。
- 列表在同一只读快照计算total和当前页；详情按需读取日志与可选报告，不加载候选/工作簿，不新增表、不初始化workspace、不调用AI或业务写入，无服务端派生缓存。前端缓存必须按身份、授权、projectId/sourceId/jobId和分页隔离；活动任务刷新由调用方控制。任务状态可以在下一次请求变化，分页不是跨请求冻结快照。
- 本轮仅实现及契约同步：按用户要求未运行Go编译/测试、HTTP或数据库验收；未部署、未提交，交由用户统一构建验证。完整日志不设分页；采用日志若未记录，后端不能补造。

本轮任务记录接口已部署d2f9acc，真实项目列表及指定分析/采用详情GET通过。已有MM后台采用成功恢复；10类采用、4类保留，未运行新的AI分析或CMDB发布。

### 关系推理Agent更新

沿用分析启动/任务历史/候选/采用API。后台新增独立AI对象性质与关系调查，工具检验全行跨表候选后才展开事实；调用过程及简短依据进入既有任务日志。字段证据新增可选`relationshipEvidence`（sheet/cell）与`relationshipReason`，类型以OpenAPI为准。连接候选行可保留线缆型号/规划长度/介质及两端模块来源；人工采用才关联CablePlan，不写实装长度。详见[实施说明](../kickoff/RELATIONSHIP-AGENT.md)。本轮真实Provider/MM验收NOT RUN，部署不自动重跑历史任务。


### 2026-11-04 候选采用与版本同步修复
- 分析候选增加`applicableRowCount`：全量候选依据原问题记录、输入Schema及实际合并关联键过滤，不能用预览20行或rowCount判断可采用；预览与执行共用资格函数。无可采用行保留来源/问题，不可勾选。
- 采用提交前刷新工作区版本；后端排队前拒绝陈旧版本，执行事务仍校验工作区/工程源/表版本，不自动重试写操作。
- 已完成采用回执由HttpKickoffProvider统一同步工作区；清理页面重复刷新effect。只读核对409刷新工作区后最多重读一次，旧refresh响应不得回滚版本；手动重新加载先刷新工作区。
- 缺关联/格式无效候选继续保留待修正，不将跳过描述为已采用。权限/事务/Outbox保持不变。
- 验证：项目001原采用结果r7及五分类只读Handler200；真实候选1156条落位/7条规格均无可采用行；前端Provider模拟409恢复/完成回执/异步刷新顺序测试，Go资格单测通过。未重跑AI、采用或CMDB发布。


### 来源事实闭合修复
分析请求可传`cableLengthUnit`（m/cm/mm/ft），只作为本项目不可变资料的人工确认，保存于已有持久任务proposal；后续同资料分析复用，参与checkpoint指纹，不作为其他资料默认值。端接区域经AI判断、完整关系求值和显式SN来源校验后可以作为设备身份依据，不强制独立库存清单。位置通过标签关联端接SN；明确U起始写法规范化，不猜高度。连接保留同行线长及单位换算证据。候选缺SN/端口/落位关联时记录问题及needs_input，不能只按已展开行数报完整。旧checkpoint只复用同来源已读区域边界作为提示，身份关系重新求值。

### 2026-11-04 端接理解五步（源码契约，未部署）

`KickoffUnderstanding.flowVersion`为可选字段；已产出报告仅实际Agent结果`flow_version=termination-v1`生成新语义；未产出报告时按入队`eng_job.proposal.flowVersion=termination-v1`显示新五步pending。没有该版本的既有artifact、历史复用报告及无结果任务保持旧语义，不用`origin=agent`或任务成功推断新版、不改写历史存储。报告仍由已有同项目/资料权限与artifact读取路径返回，无新增操作、副作用或自动采用。

技术step ID保留，`count=completed`，分母仅对应当前步骤的实际核对对象：

| ID | termination-v1语义 | 实际判据/分母 |
| --- | --- | --- |
| structure | 端接资料定位 | WorkbookIR区域中有实际Agent路由范围重叠且有coverage记录的区域数/全部IR区域；Parser成功不能单独完成此步 |
| regions | 设备与端口识别 | 有来源支持且身份/归属唯一的设备与端口候选行/该类全部候选行；设备来自端接候选投影，报告自身不生成设备 |
| semantics | 关系推理与佐证 | 两端身份、端口均有来源且唯一引用已有候选、非自身连接的连接行/全部连接行；公式必须有独立重算记录，模型自评不计完成 |
| relationships | 位置与线缆关联 | 已提供的位置行与线缆属性的有效关联数/该类已提供信息数；未提供U位、长度、型号等不猜测，零对象为not_applicable |
| fields | 候选完整性核对 | 关键身份、已提供字段来源、类型和候选问题通过的候选行/全部候选行加未展开的needs_input覆盖记录；无证据的候选不算完成 |

设备使用`device_ref`、端口使用`device_ref+port`、连接使用`a_device_ref/a_port/b_device_ref/b_port`；明确来源的serial及a_serial/b_serial保留身份兼容。可选serial、型号、位置或线缆字段缺失仅保留缺口提示，不记为任务失败；已有值错误、身份冲突、未解析区域仍不能标为完成。自由文本missing保留于limitations，不将其数量直接当作必填缺口数。步骤只有complete/partial/pending/unsupported/not_applicable，不制造执行失败状态。`recognized`仅指以上证据核对通过，不代表采用、发布、设备健康或真实模型验收。

验证记录：定向`TestKickoffAgentUnderstandingNotApplicable`、`TestKickoffTerminationUnderstanding*`通过（可选缺失、未解析端点、重复身份、缺来源、未重算公式、覆盖缺口及旧版本隔离）；OpenAPI类型生成与限定diff检查通过。整合后`check-real-api-docs.mjs`通过。部署以DEPLOYMENT.json为准；未运行全量测试或真实Agent；无报告的进行中/失败任务读取入队flowVersion，旧任务无版本不补标；只将eng_step中route/graph/reason_relationships/kickoff_step_1/2/3真实succeeded记录加入对应步骤note，计数保持0、步骤保持pending，不伪造完成比例。候选issues按row索引，全局问题按field去重，避免每行重复遍历全量问题。

## 当前流程隔离与字段贯通纠正

当前analysis/understanding/region-preview/tasks及采用仅接受flowVersion=termination-v1；旧识别结果不再返回或允许采用。无新任务时返回空候选及新版待分析步骤，不回退旧报告。reuse-history及history入口退役返回410 KICKOFF_LEGACY_ANALYSIS_RETIRED，前端入口删除。历史原始存储不再进入当前工作链。

KickoffConnection与KickoffReviewFacts新增cableModel/cableLengthM，单一连接的源线缆规格和估算米数进入核对列表、详情抽屉；多段线缆不冒充单根线长，安装实测长度保持独立。
