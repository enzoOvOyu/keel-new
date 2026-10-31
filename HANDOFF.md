# Keel 当前开发交接

更新：2026-10-31 UTC。先读本文件，再按下列索引定位；无需重扫全仓。历史交接可用 `git show 6601444:HANDOFF.md` 查阅。

## 2026-10-31 后端 Agent 部署更新（优先于下方历史版本）

当前8081已切换为 `/tmp/keel-api-agent-topology-1031`，源码 `ad8918b`，来自独立集成分支 `deploy/bom-agent-topology-1031`（工作区 `/tmp/keel-agent-deploy`）。该版本保留拓扑基线 b184645，并集成 Astra 的 e3f6cd3、74ab238 Agent 修复。启动配置仍使用 `/home/ubuntu/keel/dev/backend/.env`，前端5180仍使用本工作区。本分支没有合入这些后端代码，后续重新部署时必须保留集成修复，不能直接从旧后端源码重建覆盖。

定向 Go 检查、构建和部署启动检查通过；没有新启动真实识别/入库验收。证据见 `docs/topology/agent-deployment-20261031.json`；旧二进制 `/tmp/keel-api-4541850` 保留用于回退。下方4541850部署描述是历史状态。

## 当前分支与运行环境

- 仓库目标：`enzoOvOyu/keel-new`。工作区 `/home/ubuntu/keel-visualization-dev`，分支 `feat/datacenter-visualization-approved`；未合并 main。
- 已部署业务代码 `4541850`；后续交接/契约文档提交不改变运行代码。
- 前端 http://localhost:5180 ，tmux `keel-dev-5180`，目录为本工作区 `dev/`。
- 后端 `127.0.0.1:8081`，tmux `keel-cmdb-api`，运行 `/tmp/keel-api-4541850`。配置和存储工作目录仍为 `/home/ubuntu/keel/dev/backend`，不要复制或输出 `.env`。
- 原目录 `/home/ubuntu/keel` 的源码仍在 `keel-new-main` / `978db7f`。5181 批准原型未调整，额外 5182/8082 已停止。
- 用户已授权本次部署；本轮没有数据库迁移、资产导入或主分支合并。后续文档维护不隐含再次部署授权。
- 启动与回退命令见 [DEPLOYMENT](docs/DEPLOYMENT.md) 末节；不要为了接续开发重复启动或重启。

## 已完成及入口

| 页面 | 当前入口 | 实现位置 |
|---|---|---|
| 全局拓扑 | `/topology?view=global` | `dev/src/features/infrastructure/approved/pages/global-topology.tsx` |
| 网络架构 | `/topology?view=network` | 同目录 `network-architecture.tsx` |
| 物理布线 | `/topology?view=cabling` | 同目录 `physical-cabling.tsx` |
| 链路追踪 | `/topology?view=trace` | 同目录 `link-tracing.tsx` |
| 机柜可视化 | `/racks?view=gallery` | 同目录 `rack-detail.tsx` 及 `approved/rack/` |

页面共享范围、URL、缓存由 `approved/ApprovedTopologyWorkspace.tsx` 维护，组件不能各自创造资产身份。服务器与交换机详情继续复用现有抽屉。

## API 与代码定位

首先查 [真实 API 契约总入口](docs/REAL-API-CONTRACTS.md) → [可视化使用契约](docs/api/visualization.md) → [机器注册表](docs/api/real-api-registry.json)。字段权威为 [OpenAPI](docs/openapi.yaml)，不在交接里重复定义 DTO。

- 真实接口：GET subgraph、matrix、connections，POST trace；兼容 GET visualization。完整路径、operationId、参数及错误见上述契约。
- Go 路由：`dev/backend/cmd/api/cmdb.go`；查询 `dev/backend/internal/cmdb/visual_queries.go`，兼容投影 `visualization.go`，位置校验 `visual_placement.go`。
- 前端 Adapter：`dev/src/lib/api/infrastructure.ts`；生成类型 `dev/src/lib/schemas/openapi.ts`。
- CMDB 是唯一资产事实源。布局草稿不修改安装事实；实际位置变更走现有 ChangeSet。物理/逻辑/运行观测分层，缺数据不能伪造连接或正常状态。

## 实际检查与限制

- 已有构建/类型生成、Go 编译及定向纯逻辑检查通过，见 [代码检查记录](docs/topology/code-validation.json)。纯文档收尾不重复运行。
- 已部署四个新接口启动 HTTP 200，五页面加载，无 pageerror 和失败可视化请求；兼容旧接口不在本次四接口探测范围。见 [部署证据](docs/topology/deployment-validation.json)。
- groups 查询瓶颈已修复：单次观察 6568ms → 447ms；未修改超时或数据库配置。这不是生产性能 SLO。
- 未完成真实导入验收、全部关键交互闭环或千卡/大型生产规模基准。真实数据用户后续统一联调，不能把页面加载当作完整验收。
- 逻辑/历史 Trace 缺能力时明确拒绝；物理路径存在不等于已验收通过。规划快照、精确面板、实时监控和 PDU 缺少事实时不制造展示数据。
- BOM 完整自主工具调用 Agent 的历史未完成状态保持，不因本次可视化交付宣称已解决；相关现状查 [BOM 实施状态](docs/BOM-REAL-IMPLEMENTATION-STATUS.md) 和 [工作台设计](docs/BOM-WORKBENCH-V2.md)。

## 后续修改顺序

1. 用户指出要调整的页面/行为后，先查模块契约与 [实施报告](docs/topology/implementation-report.md)，再定向读取涉及的代码。
2. 契约变化同步 OpenAPI、生成类型、Go/Adapter 和注册表；UI 调整保护批准原型及既有身份/权限模型。
3. 只运行受影响的必要检查，更新本文件与证据；不重复全仓 review，不提前导入或合并 main。
4. 文档/登记一致性：`node dev/scripts/check-real-api-docs.mjs`（只读，不调用服务）。

## 2026-10-31 用户要求暂停后的提交

- 仅调整机柜页现有视图按钮，区分“3D 机房 / 机柜阵列 / 单柜工作台”；3D 仍使用已有 `view=gallery&overlay=capacity`，没有新增独立路由、Sidebar 入口或渲染引擎。
- 用户随后纠正方向并要求停止开发、提交；未继续实现独立 3D 页面，后续需求需重新确认。变更位置 `dev/src/features/infrastructure/approved/pages/rack-detail.tsx`。
- TypeScript `tsc --noEmit` 与 `git diff --check` 通过；本次没有浏览器交互验证，没有重启服务或合并 main。5180 使用该 worktree 的 Vite，源文件修改可能已被热更新读取，不应将未重启等同于界面未变化。

## 当前 Flows 重构分支（2026-10-31）

最新任务包见 `docs/flows-redesign/`，实施索引 `docs/flows-redesign/implementation.md`。本任务已隔离到 `/home/ubuntu/keel-flows-dev` / `feat/flows-redesign`，基线 d8a91b4；不要在运行5180的可视化 worktree 继续写本任务。包内指定真实 Kestra/Go/Agent/SSH 落地，覆盖旧 Mock-only 的 Flows 阶段限制；其他业务不扩展。新底座 `keel-flows-*` 独立容器，Kestra 仅127.0.0.1:8095，未替换现有5180/8081，未修改原数据库。
