# 数据中心可视化 · 真实 API 使用契约

核对基准：代码 `4541850`，2026-10-31。Base URL 为 `/api/v1`。字段/类型权威为 [OpenAPI](../openapi.yaml)，机器登记见 [注册表](real-api-registry.json)。本文记录语义和使用方式，不复制另一套完整 Schema。

## 使用边界

- 真实 Go 查询现有 CMDB；页面没有用 fixture 补齐缺失资产。数据库中带 synthetic 来源的记录仍是合成证据，不能当真机采集。
- Cookie Session + `cmdb:read` 和获授权项目/资源范围。`project_id` 省略表示当前用户获授权范围，不表示绕过权限。
- POST trace 是只读查询，也必须带当前 Session 的 `X-CSRF-Token`；浏览器通过现有 client 获取，不保存 Cookie/Token 到 localStorage。
- 本模块不创建资产、不导入 BOM、不提交设备操作。实际位置变更单独走既有 CMDB ChangeSet。
- 响应为 `{data, request_id}`；错误为既有 ErrorEnvelope。不要将 `integrity=complete` 或 HTTP200 变成 PASS。

## 操作、页面与定位

| 方法与路径 | operationId | 响应 data Schema | 页面/Provider |
|---|---|---|---|
| GET `/cmdb/visualization/subgraph` | `getCMDBVisualSubgraph` | `InfrastructureSubgraph` | 五工作区范围、目录、搜索；`infrastructureApi.subgraph` |
| GET `/cmdb/visualization/matrix` | `getCMDBVisualMatrix` | `InfrastructureMatrix` | 全局/网络/布线矩阵与束计数；`infrastructureApi.matrix` |
| GET `/cmdb/visualization/connections` | `getCMDBVisualConnections` | `InfrastructureSubgraph` | 成员、端口、Cable、Mapping；`infrastructureApi.connections` |
| POST `/cmdb/visualization/trace` | `queryCMDBPhysicalTrace` | `InfrastructureTrace` | Metro/Tree/Table；`infrastructureApi.trace` |
| GET `/cmdb/visualization` | `getCMDBVisualization` | `InfrastructureSnapshot` | 兼容 summary/full；新页面不默认下载 full5000 |

实现定位：

- [HTTP 路由与项目校验](../../dev/backend/cmd/api/cmdb.go)：`cmdbRoutes`、`cmdbScope`。
- [受控查询](../../dev/backend/internal/cmdb/visual_queries.go)：`VisualSubgraph`、`VisualMatrix`、`VisualConnections`、`VisualTrace`；统计优化在 `visualGroupSQL`。
- [统一投影](../../dev/backend/internal/cmdb/visualization.go)：字段和类型化关系来源。
- [机柜质量几何](../../dev/backend/internal/cmdb/visual_placement.go)：`visualRackPlacementFindings`，0.5U/0U/安装面/未知尺寸。
- [统一前端 API](../../dev/src/lib/api/infrastructure.ts)、[生成类型](../../dev/src/lib/schemas/openapi.ts)。
- [加载/缓存/路由外壳](../../dev/src/features/infrastructure/approved/ApprovedTopologyWorkspace.tsx)、[Context](../../dev/src/features/infrastructure/approved/model.tsx)。
- [四页实现](../../dev/src/features/infrastructure/approved/pages)、[机柜领域组件](../../dev/src/features/infrastructure/approved/rack)。

## 公共查询与分页

GET 新接口通用参数：`project_id`、`root_id`、`fabric_id`、`q`、`unplaced`、`cursor`、`snapshot_id`、`limit`，各操作实际支持项以 OpenAPI 对应 operation 为准。

- `root_id` 是已授权实体的稳定 ID；`unplaced=true` 表示未定位范围，不创造“未分组资产”。
- `fabric_id` 只匹配明确成员关系，不能根据物理连线推断逻辑 Fabric。
- `q` 在服务端分页前检索 ID、名称及适用的 SN/Hostname/IP/MAC 等字段，同时兼容原有顶层和 `payload.fields` 存储格式。
- 新接口 `limit` 默认100、范围1–200。这是种子或矩阵单元格数量；结构上下文可能有额外父实体/双端，不能要求 `assets.length === limit`。
- `page={limit,has_more,next_cursor}`；末页 `next_cursor=null`。翻页保留原查询和 `snapshot_id`，将返回 cursor 原样发送。
- Cursor/member 引用绑定用户、授权版本、范围、过滤和数据修订。不能解码后自行改字段、换用户、换项目或当作授权凭据。
- 服务进程重启可能使签名引用失效；清除旧引用后重新取当前范围。
- `snapshot_id` 是当前修订指纹，不是历史快照；`read_at` 是查询时间，不是设备采集时间。
- 数据修订变化返回409；前端清 cursor/member 引用并重新获取基础范围，保留适用的稳定对象 ID，不拼接不同修订的数据。

上下文上限5000资产/20000关系，分组上限200；达到限制会保留 partial 标记。不能把当前返回量称为全域数量，也不能用没加载到证明实体或线缆不存在。

## subgraph

专用参数：`view=hierarchy|fabric|physical`，默认 hierarchy；`depth=1|2`，默认1。

- depth1 用于结构和机柜几何摘要；depth2 加载登记组成/端口等局部上下文。
- Gallery、单柜和邻接机柜使用同一身份。邻接机柜可能只是摘要，不能展示成完整空柜。
- `groups[].counts` 是当前授权查询范围的登记统计，独立于当前种子页，不是现场验证的实物数量。
- `rack.fields.registered_device_count`、`placement_coverage={loaded,total,complete}`、`visualization_summary` 用于辨认机柜几何完整度。缺 U 位/U 高时，前端不计算已确认空闲空间。
- GPU 实体计数与服务器声明的 `gpu_count` 是不同覆盖口径；不因分组存在就把缺数变成0或完整硬件清点。

## matrix

`axes`：`server_switch`（默认）、`leaf_spine`、`rack_leaf`、`port_pair`。

- rows/columns 是稳定 ID 实体；cells 有 `row_id,column_id,count,member_query_ref,groups`。
- `groups` 按事实层、介质、登记速率拆分，分别保留 `count/member_query_ref`。线宽是记录数编码，不是带宽或吞吐量。
- `rack_leaf` 表示柜内服务器到 Leaf，不能把 Spine 上联混进计算组。
- `included_count` 为该轴适用的完整授权 Cable 记录总量；`excluded_count` 保留缺端或不适用记录。排除不等于故障，也不能静默丢弃。
- 覆盖率使用 `returned_cells,total_cells,limit,complete`，单位是单元格，不能与 Cable 数或资产数混用。
- 点单元格或分类束后，用其 `member_query_ref` 请求 connections；不要在浏览器凭节点位置生成边。

## connections

专用参数：`asset_id`、`port_id`、`member_query_ref`、`status=all|incomplete`、`sort=id`。

- `member_query_ref` 保留生成它时的范围/分类条件；传入冲突过滤会拒绝，不能将一个格的成员引用用于另一个范围。
- 当前服务端全范围状态过滤只支持 all/incomplete，排序只支持 ID。UI 中其他观测状态或局部排序需明确“当前窗口”，不冒充全域过滤。
- 已登记但无线缆的设备/端口仍保留对象上下文；“无线缆记录”不是“没有端口”或“已验证空闲”。
- Cable 保留 `a,b,device_a,device_b,fact,source,fields`。端点字符串为空表示缺失或当前范围不可用，不能创建一个“未知设备”补齐它。
- 一端多条 Cable 保留歧义供用户选择，不静默取第一条；分页完整性与物理完整性分开。

## trace

请求 Schema：`InfrastructureTraceRequest`。必须 `mode`、`from_port_id`；可选 `project_id,to_port_id,candidate_cable_id,snapshot_id`，不接受任意额外字段。

- body/query 项目同时给出时须一致；有效项目重新执行 `cmdb:read` 授权。
- from/to 必须是可见 PhysicalPort；不存在、不可见或类型不适用返回不泄露存在性的404。
- 合法端口没有 Cable 返回 `integrity=no_record`；多个候选返回 ambiguous，用户可通过 `candidate_cable_id` 显式选择，服务端校验附着关系。
- `segments` 仅含证据允许的登记物理段。到交换机端口停止；没有 patch-through/FIB 证据不跨内部端口拼接路线。
- `integrity=complete|incomplete|ambiguous|no_record`；当前 `verification` 仅 `unverified`，不代表实际连通、健康或验收 PASS。
- `breakpoints` 给出对象、原因和缺口；逻辑路径缺 FIB/VRF 等证据时返回422，不伪造路径。
- 当前没有历史回放；GET 的 at/evidence_at 不支持，客户端不要添加未声明的历史参数。

最小调用顺序（值为占位符，不是测试资产）：

```http
GET /api/v1/cmdb/visualization/subgraph?project_id=<project>&view=hierarchy&limit=100
GET /api/v1/cmdb/visualization/matrix?project_id=<project>&axes=server_switch&snapshot_id=<returned-snapshot>
GET /api/v1/cmdb/visualization/connections?project_id=<project>&member_query_ref=<returned-ref>&snapshot_id=<same-snapshot>
POST /api/v1/cmdb/visualization/trace
Content-Type: application/json
X-CSRF-Token: <current-session-reference>

{"project_id":"<project>","mode":"physical","from_port_id":"<registered-port>","candidate_cable_id":"<selected-cable>","snapshot_id":"<same-snapshot>"}
```

Cookie 由受控 Session 处理；不要把凭据复制进样例、日志或 Git。

## 错误处理与能力边界

| 状态 | 代表 code | 调用方处理 |
|---|---|---|
| 401 | UNAUTHENTICATED / SESSION_EXPIRED | 走统一身份体系，不在 BOM/机柜嵌套登录 |
| 403 | CMDB_SCOPE_DENIED / CSRF_DENIED | 保持权限拒绝，不回退到其他项目或 Mock |
| 400 | VISUAL_CURSOR_INVALID、VISUAL_CURSOR_SCOPE_MISMATCH、VISUAL_MEMBER_SCOPE_MISMATCH | 清理不适用引用，重新取当前范围；不自行改 token |
| 400 | VISUAL_LIMIT_INVALID、VISUAL_VIEW_INVALID、VISUAL_DEPTH_INVALID、VISUAL_AXES_INVALID、VISUAL_STATUS_UNSUPPORTED、VISUAL_SORT_UNSUPPORTED、VISUAL_TRACE_INPUT_INVALID、VISUAL_PROJECT_SCOPE_MISMATCH | 修正当前请求；不循环重试同一参数 |
| 404 | VISUAL_PORT_NOT_FOUND / VISUAL_ASSET_NOT_FOUND | 显示对象不可用，保留来源与返回上下文 |
| 409 | VISUAL_SNAPSHOT_EXPIRED | 清旧分页/成员引用，显式重载新修订 |
| 422 | CAPABILITY_UNAVAILABLE / VISUAL_TRACE_BRANCH_INVALID | 显示证据缺口或重新选分支，不伪造成功 |
| 504 | CMDB_QUERY_TIMEOUT | 保留上下文，缩小范围或手动重试；不拿部分数据充当完整 |

具体响应以 ErrorEnvelope 和返回标志为准；不要仅凭状态码自动扩大重试或认为所有失败都可重试。

## 与资产写入、UI 的关系

位置、组成、物理端接、逻辑成员、运行观测使用不同关系和事实层。`source/source_ref/revision/observed_at/freshness/support` 以具体对象提供的证据为准；未提供保持 unknown，采集失败不补零。

观察页仅查询。拖动修改展示草稿；实际位置变更沿用已有 ChangeSet 的项目/权限、预期修订和审批。规划/实装比较、实时热力与功耗、PDU、精确面板缺证据时保持不可用，不能为了原型效果构造事实。

## 已验证到哪一步

- [部署证据](../topology/deployment-validation.json)：代码4541850、原5180/8081、4接口启动200、5页面加载，无 pageerror/失败可视化请求。
- [代码检查](../topology/code-validation.json)：类型/构建、Go 编译和必要纯逻辑用例，保留当时范围。
- [实施报告](../topology/implementation-report.md)：查询超时修复、性能观察与资料缺口。
- 未做完整真实导入、全部交互闭环、生产规模基准。启动200和单次延迟不是这些能力的验收。
