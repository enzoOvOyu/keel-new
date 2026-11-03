# Kickoff · 当前 API 使用契约

基准：dev/部署 `686e9ea`。产品行为见[模块定版](../modules/kickoff.md)。[OpenAPI](../openapi.yaml)定义唯一请求/响应/枚举；[注册表](real-api-registry.json)链接实现及证据。

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

当前网络输入及CMDB版本引用不等于完整Fabric NetworkPlan；缺口见[决策记录](../DECISIONS.md)。[部署证据](../kickoff/DEPLOYMENT.json)、[后端定向集成](../kickoff/BACKEND-CHECK.json)、[性能与在线读取](../kickoff/PERFORMANCE-RESULT.json)各自限定验证范围。本轮新增历史资料理解为源码及定向检查结果，未更新部署。

## 历史资料理解派生读取（本轮源码，未部署）

- GET `sources/{sourceId}/understanding` (`getKickoffSourceUnderstanding`)：读取当前工程 head 的派生报告；未构建返回 unsupported/pending，不调用 AI。
- POST `sources/{sourceId}/reuse-history` (`reuseKickoffSourceUnderstanding`)：需要 bom:import；复用现有原件、Proposal Columns/Records 与对象证据生成 MinIO 派生报告。同 source/head 重复操作返回同报告，不修改工程修订、输入、审批或 CMDB。
- GET `sources/{sourceId}/regions/{regionId}/preview` (`getKickoffSourceRegionPreview`)：需要 bom:read；offset/limit（上限100），view=source|normalized。源行使用真实 Excel 坐标；normalized row=0 不是 Excel 行号。revisionId 是工程 head UUID，失效409；兼容 revision_id。
- 报告按项目/source/head/v1隔离，原件只在显式构建时由现有敏感单元格脱敏 Parser 读取；区域、预览分对象持久化，不把整个工作簿放入 workspace 或无界内存缓存。
- 结构使用现有 WorkbookIR 空白行分隔/256行分块，不宣称完整业务表边界。区域语义仅取精确单元格对应对象类型；同区域可含多类，不根据 Sheet 名猜测。
- 历史 reviewed_model 为对象级、无单元格坐标。恢复唯一 SN/MAC 相等或有效历史 Proposal 的映射仅为 `_candidate`，不计入已确认字段覆盖；未知和敏感单元格不做映射。结构、区域、语义、关系、字段分别报告计数、依据与缺口；历史发布不使字段步骤通过。
- 派生报告可重建，不是新事实层。不新增 Agent、映射向导或资产发布；analyze 仍503。读取存储故障503，缺报告/区域404，构建失败500，不伪造成功。

定向结果见 [HISTORY-UNDERSTANDING-CHECK](../kickoff/HISTORY-UNDERSTANDING-CHECK.json)：MM 恢复54个结构分块、29,583个候选定位单元格、7,154条同原始行双端物理端口精确组合候选；明确字段证据仍为0。关系完成量是有坐标引用且确定性校验通过的连接数，不以已有连接总数填充；字段分母仅为明确映射的对象字段，标题/备注/重建候选不算必填字段。步骤按覆盖和校验计算 complete/partial/pending；无适用连接为 unsupported。workspace 不额外读取 MinIO 报告，报告和预览只由来源页按需加载。
