# 数据中心可视化 · 功能分支实施报告

更新：2026-10-31。状态：代码版本 `4541850` 已按用户新指令部署到现有 5180/8081；四接口启动检查及五页面加载通过。真实导入、完整交互与规模验收仍待用户安排。记录见 [部署验证](deployment-validation.json)。

批准依据是 `docs/topology.md`、`docs/rack-visualization/room-validation.md` 及其引用的五个交互原型。没有重新设计产品，没有新建资产库、数据库表或迁移。

## 分支和运行环境

- 开发分支：`feat/datacenter-visualization-approved`。
- 工作目录：`/home/ubuntu/keel-visualization-dev`。
- 原目录 `/home/ubuntu/keel` 保持 `keel-new-main` / `978db7f`，源代码未修改。当前前端从功能分支 worktree 运行，API 使用新版本 binary，配置和存储工作目录保留原位置。
- 不合并 main、不改变服务地址。用户随后授权部署，已更新现有 5180 前端和 8081 API；额外 5182/8082 保持停止。
- 原代码开发阶段没有运行数据库测试；部署阶段为修复查询超时执行了定向只读查询和启动检查。没有导入资产或执行迁移；登录与 Session 审计正常产生。

## 已完成页面和交互

| 页面 | 已落实内容 |
|---|---|
| `/topology?view=global` | Cluster Map / Hybrid，Pod/未定位范围下钻，机柜×Leaf与服务器×交换机矩阵，服务端搜索、分组登记统计、成员展开与分页 |
| `/topology?view=network` | Fabric Flow、分层语义束、Bipartite、交换机目录；按真实 Fabric 成员过滤，按事实层/介质/速率分组；选束加载成员 |
| `/topology?view=cabling` | Connection Matrix、BioFabric、Port Panel、Cable Inventory；矩阵单元格关联真实成员查询，缺端保留，搜索/分页/窗口筛选明确范围 |
| `/topology?view=trace` | API 驱动的端口—Cable—端口 Metro Route、关联 Tree 和证据 Table；显式选择多 Cable 分支；无端口、无线缆、缺端、歧义和不支持分开显示 |
| `/racks`、`/racks/:rackId` | Gallery、机房 3D 空间、Spatial/Dual/Front/Rear/Section，设备/端口/线缆 Mapping、双向定位、展示草稿/撤销/恢复、SVG/PNG/打印 PDF |

Sidebar 仍是唯一模块导航，不在内容区复制一套菜单。服务器五 Tab 和交换机八 Tab 复用原 DrawerDetail。URL 保存稳定 ID、筛选、相机和 Tab；机柜 `rack_tab` 与设备抽屉 `tab` 分开。关闭/返回不通过名称重找资产。

### 逻辑修正

- 快速切换基于当前 URL 合并，不让旧闭包覆盖新视图。
- 未授权 project_id 保持拒绝，不自动改成默认项目。
- Query Key 含用户、项目过滤、授权版本、查询和快照；切范围取消旧请求，异步设备抽屉检查用户/范围/路由身份。
- 快照过期时明确清除旧 cursor/member 引用后重新加载，不拼接不同版本的分页。
- 不再默认下载整份 `fullSnapshot5000`。先限定种子、范围和分页，再取结构上下文；选中对象/单元格后才取端口与成员。
- 已登记但未接线的端口仍可查询；无线缆不能显示为没有端口。
- 端口多 Cable 不自动取第一条；没有机柜位置不展示伪定位按钮。
- 展示草稿按用户/项目/机柜和版本保存；实际位置变更要求限定项目，并冻结项目、目标和版本，通过既有 ChangeSet 审核发布。
- GPU、占用与空闲 U 的完整性分开。组件实体数量只作有来源的登记下界；缺失数量/几何不显示为零或完整空柜。
- 后端机柜质量检查与 SVG 使用相同的半 U 区间、0U 和安装面语义；未知几何不是已证明的重叠。

## API 与统一关系模型

复用 `cmdb_entity(item_uid)`、`cmdb_asset_scope`、`cmdb_relationship`、既有 Schema/来源/Revision/ChangeSet。没有平行 Machine/Switch/Cable 身份，布局不写安装事实。

| 新增查询路径（均在 `/api/v1` 下） | 契约与用途 |
|---|---|
| GET `/cmdb/visualization/subgraph` | InfrastructureSubgraph；位置/Fabric/物理范围、搜索、未定位范围、深度与分页；分组统计独立于当前窗口 |
| GET `/cmdb/visualization/matrix` | InfrastructureMatrix；四种双轴、单元格计数、事实层/介质/速率分类组、成员查询引用、矩阵专用覆盖率 |
| GET `/cmdb/visualization/connections` | InfrastructureSubgraph；Cable 分页、对象/端口/单元格筛选、授权双端与来源；无连接对象仍保留自身上下文 |
| POST `/cmdb/visualization/trace` | InfrastructureTrace；可见 PhysicalPort、指定目标和显式候选 Cable、段/断点/完整度/未验证状态 |

原 GET `/cmdb/visualization` 摘要及 full 模式保留兼容，新的工作区不把 full 模式作为默认加载方式。

- 所有查询重新执行项目/资源授权；POST trace 的 body/query 项目需一致，使用同一范围及现有 CSRF。
- Cursor/member 引用绑定用户、授权版本、过滤和当前数据修订；篡改/换范围拒绝，修订变化返回409。进程重启会使临时签名引用失效，明确重载即可。
- `snapshot_id` 是当前修订指纹，不是已保存的历史快照。`read_at` 不是 `observed_at`。
- 单页种子/单元格上限200，结构上下文上限5000资产/20000关系，分组上限200；任何限额损失明确 partial。没有把局部量当全域资产数。
- `status=incomplete` 为服务端范围过滤；其他观测状态明确标成当前窗口筛选，不伪装全域结果。
- 规划与观测关系保留分层来源；位置、端点和归属不按任意返回顺序选择。矩阵排除无法确认的双端，连接目录仍保留缺口。
- Fabric 成员仅来自明确字段及类型化成员关系，不由物理线缆推断 VLAN/VRF/VNI/BGP。
- 精确字段、错误码与枚举以 OpenAPI 为准；前端响应 DTO 已改为生成 Schema 别名，没有第二份手写响应模型。

## 关键代码与同步文档

- `dev/src/features/infrastructure/approved/`：五工作区、Context、共享详情、机柜几何与渲染。
- `dev/src/lib/api/infrastructure.ts`：统一查询 Provider 与生成类型别名。
- `dev/backend/internal/cmdb/visual_queries.go`：受控子图/矩阵/连接/追踪查询。
- `dev/backend/internal/cmdb/visualization.go`：复用授权投影和字段/关系来源。
- `dev/backend/internal/cmdb/visual_placement.go`：与机柜几何一致的质量判定。
- `dev/backend/cmd/api/cmdb.go`：接口路由、项目校验和错误返回；没有更换服务地址。
- `docs/openapi.yaml`、生成 TypeScript、API-CONVENTIONS、DOMAIN-API-CONTRACTS 同步。
- PRD、ARCHITECTURE、SYSTEM-DESIGN、DATA-DICTIONARY、ACCEPTANCE、TEST-STRATEGY、IMPLEMENTATION-PLAN、topology.md 和 HANDOFF 更新实施映射，不重写批准设计。

图形技术沿用批准原型：四页采用语义 SVG、HTML 矩阵/表格与专属路径表达；机柜使用 React+SVG 共享几何，空间模式使用按需 Three.js 0.187.0。没有为技术名称更换图形引擎；既有 G6/ELK 依赖保持。

## 本轮实际验证

| 检查 | 实际结果 |
|---|---|
| `npm run generate:api` | PASS；OpenAPI 生成专用前端类型 |
| `npm run build` | PASS；TypeScript 与 Vite 生产构建完成 |
| `go test ./internal/cmdb -run 'TestVisual(Query\|Placement)' -count=1` | PASS；4组顶层用例，含SQL参数/括号、scope/token、限额/不支持能力及6种机柜几何反例 |
| `go build -o /tmp/keel-visualization-contract-check ./cmd/api` | PASS；只编译，未执行 binary |
| `git diff --check` | PASS |
| 新4接口启动检查 | 部署阶段四接口200；完整业务联调仍未运行 |
| 新查询接线后的页面加载 | 部署阶段五页面加载通过，无 pageerror/失败可视化请求；完整交互闭环未运行 |
| 千卡/万节点性能与真实导入 | NOT RUN；不声明生产规模达标 |

仍有已有大于500kB分包和Zod注释提示，未修改阈值隐藏。Three.js 按需加载。其他尚为演示实现的业务资源没有在本轮无关重构；本工作区不读取 fixture 作为生产数据。

## 性能和数据边界

本轮完成 SQL 范围/分页、服务端矩阵汇总、局部成员查询、索引、缓存和按需渲染代码。尚未执行运行时数据库计划/耗时、传输、布局、FPS、内存或大规模基准，不能将编译或限额当性能结论。

历史 [API记录](api-validation.json)、[交互记录](interaction-validation.json) 和截图来自较早 full 投影接入阶段，包含本地库中的 synthetic_fixture 数据。它们不证明本轮新4接口、分页、真机或最终规模能力；本轮没有导入这些数据。

以下能力依赖尚无证据的资料，保持明确不可用：

1. Logical Trace 的同窗口 FIB/VRF/方向证据，及实际业务故障影响判断。
2. 配线架内部贯通与多段物理连续证据；目前只证明已记录的直接 Cable 双端，不穿越交换芯片。
3. 已确认规划/实装配对快照与完整 Dual-State Comparison；当前入口继续隐藏。
4. 测绘机房 XY、机柜宽深、精确面板、实时功耗/温度、PDU 端接。3D 排列与代理宽深明确为示意，不能用于施工。
5. 可回放历史拓扑快照与生产性能 SLO；当前修订指纹不冒充历史存储。

## 用户后续联调清单

在用户安排的环境启用功能分支后，验证：真实导入后的资产与端口检索 → 矩阵单元格成员 → Cable 双端与分支 → 单柜背面/Mapping → Trace 与来源；切换项目/用户/修订/分页；缺端与无权限；空柜与缺几何；位置变更草稿及既有审批；最后进行实际规模性能验收。

用户后来已授权部署现有环境：当前 5180/8081 运行功能分支版本 `4541850`，未合并 main、未迁移数据库。最初启动检查发现分组SQL超时，已先回退，再修复为分离聚合和正确主键关联后重新部署；未调高超时或修改JIT配置。详见部署验证记录。
