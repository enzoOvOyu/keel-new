# Kickoff · 当前 API 使用契约

基准：dev/部署 `c6d5bc4`。产品行为见[模块定版](../modules/kickoff.md)。[OpenAPI](../openapi.yaml)定义唯一请求/响应/枚举；[注册表](real-api-registry.json)链接实现及证据。

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
