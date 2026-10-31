# Keel 开发交接

更新：2026-10-30（UTC）。本文件用于切换机器或 Agent 后快速接续，记录实际状态，不代替 PRD，也不代表全平台验收通过。

## 2026-10-31 后续任务变更

用户本轮转向数据中心拓扑与机柜一体化，当前优先级据此更新。新入口 `/topology`、`/racks`，实现/验证/缺口见 `docs/DATACENTER-VISUALIZATION.md`；代码在 `dev/src/features/infrastructure/`、`internal/cmdb/visualization.go`。BOM Agent 未完成状态保持，不因本次可视化开发声称已解决。

## 1. 先看这里

- 仓库：`git@github.com:enzoOvOyu/keel-new.git`，远端目标分支 **main**；本地分支 `keel-new-main`。
- 最近代码提交：`0784931`（分析进度加载修复）；前一功能提交 `405becc`（工作簿识别与审核）。本交接文档另行提交。
- **当前开发应用是 `dev/`**，后端 `dev/backend/`，权威设计文档是根目录 `docs/`。`keel-demo/` 是历史演示副本，不要改错目录。
- 当前机器前端 **5180**，后端 **8081**。Vite 配置默认仍为 5174，当前通过启动参数覆盖。
- 上一任务遗留：**BOM AI 智能识别 Agent** 尚未完成；当前优先级已由上方 2026-10-31 用户新任务替换。
- **完整自主工具调用 Agent 尚未完成。** 已有持久化任务、Workbook IR、语义提取方案、工具函数、来源追溯和覆盖对账；工具执行顺序仍由程序编排，不能把步骤名称当作完整 Agent 已落地。
- 本轮只整理交接，不修改业务功能、不重跑业务测试。

## 2. 必须保留的用户要求

1. 先读根 `AGENTS.md`，随后只按本文件索引读任务相关章节，不遍历全仓。
2. 本轮 BOM **只做 AI 模式**；固定字段模式、固定模板映射 UI 不开发。后端历史兼容类型仍可能出现 fixed，不能据此恢复入口。
3. 顺序：完整保留原始信息 → 理解分类 → 识别关系 → 独立确定性校验 → 人工确认 → 提交当前项目 CMDB。
4. 交换机、端口、端接与线序优先。物理与逻辑关系分开；公式不是设备 ID；无缓存公式、未知型号、缺失端点不能凭空补造。
5. AI 不能直接入库，未确认/未通过校验不能入库。沿用唯一 `bom_item` 和 CMDB 稳定身份、事务及 Outbox，不另建资产库。
6. 保留现有导航、ProjectContext、React 组件和设计令牌，真实数据通过 Adapter 接入。服务器五 Tab、交换机八 Tab 抽屉保持。
7. 不过度防御、不无故发散、不反复跑冒烟或全仓测试。仅开发和必要定向验证，完整闭环由用户测试；未经再次指示不要重跑两份 Excel、调用付费 Provider 或提交测试资产。
8. 每次修改提交 Git，目标始终 **keel-new**。不要覆盖用户新改动，不提交秘密和真实 Excel。
9. 主链路只使用：开局配置 → 网络就绪 → 装机 → 验机 → NCCL → 优雅退场；NP-1～NP-6 是网络内部断言，不恢复 G1～G6 主链路。

## 3. 当前实现及可信边界

| 范围 | 已有实现 | 尚未证明/完成 |
|---|---|---|
| 全局前端 | 双平面 IA、项目上下文、业务页面与统一 Provider；全局演示场景切换已移除 | 不代表所有模块已接真实服务或全量业务验收 |
| 身份/RBAC | 统一登录、Session、项目成员、角色/权限、审计的真实后端及管理入口 | 不重述历史测试为本轮验收；Critical NH 签署资格仍待产品确认 |
| 项目与 AI 配置 | 项目真实 API；`/admin/ai-provider` 服务端 Provider 配置、凭据引用、测试能力 | 配置和 Key 不随 Git 迁移，必须恢复数据库及秘密存储 |
| BOM | Go Excelize 解析、来源/合并/公式缓存、工作簿 IR、模型语义方案及本地提取、候选审核、校验、提交/Outbox 代码 | 完整自主 Agent、语义提取准确率、所有交互和正式入库闭环仍待完成/用户验收 |
| CMDB | Go/PostgreSQL Schema Registry、资产/类型关系/来源/版本/ChangeSet API，前端 Adapter 接入 | 不代表所有历史数据/关系、换件与发布场景已验收 |
| 其他域 | Fleet/Fabric/RackView/Flows 等保留已有 Mock 页面 | 不是真实设备后端，不连接生产设备 |

### 最近用户报告的问题与修复

用户报告“导入后点击 AI 分析卡崩溃”。已确认原页面每 4s 请求整份候选并触发全量校验；公共路由还加载整本工作簿。现有大批次有 33,616 条记录，工作区 JSONB 约 159 MB。

`0784931` 做了以下修复：
- 新增 `GET /api/v1/bom-imports/{id}/workbench/progress?project_id=...`，只返回状态、数量、Sheet、覆盖与最近 16 步，不返回候选/源单元格、不执行校验。
- 批次 summary 在路由层不再加载完整 workbook/draft/issues。
- 分析进度独立 Poll 4s，仅 recognizing 时轮询；第三步按需加载完整结果，不再自动跳到审核。
- 空结果数组保持稳定；查询可取消；审核写操作携带所查看版本。
- 已同步 OpenAPI、生成类型和开发说明。

实际验证：Go 编译、前端生产构建通过；真实已有批次进度接口 37,669 bytes，单次约 0.98s；浏览器切换上传/分析、打开结果/返回分析正常，分析页无全量候选请求；浏览器拦截的“零候选/运行中”场景也无 JS 异常。**未在已结束任务上稳定复现原 renderer 崩溃，因此没有证明所有崩溃原因均排除。** 本轮未调用模型重跑、修改候选或提交 CMDB。

## 4. 接下来从哪里改

按依赖顺序接续，避免重建已有能力：

1. 阅读 `docs/BOM-WORKBENCH-V2.md` 和 `docs/BOM-AGENT-VALIDATION.md`，理解现有边界。
2. 从 `runRecognition` 看现有编排，再看 IR、Provider 和来源校验。完成单主 Agent 根据真实工具结果选择区域、追查跨表证据、发现遗漏再补查的执行循环；复用已有工具/缓存/任务持久化，不新增一群互相聊天的 Agent。
3. 保留当前工作簿缓存和已完成方案，失败从未完成区域恢复；不能重复发送整本工作簿。工具必须实际访问 Workbook IR，进度必须来自执行状态。
4. 交换机设备/端口、A端→介质/中间段→B端关系和未知内容保留是质量重点。模型候选必须有真实来源依据，并经独立校验。
5. 前端继续四阶段：上传 → 真实分析进度 → 分类审核 → 确认入库。不要为了改后端重新生成页面；不要把全部来源行视为成功识别实体。
6. 只做本次变更必要检查，准确率只能依据人工基准，不能用 Agent 自报或覆盖 complete 证明。

### 关键代码索引

| 内容 | 入口 |
|---|---|
| 四阶段工作台/上传/样式 | `dev/src/features/RealBOMWorkspace.tsx`、`BOMFileUpload.tsx`、`bom-workspace.css` |
| BOM 类型化 API | `dev/src/lib/api/realBom.ts`；生成类型 `dev/src/lib/schemas/openapi.ts` |
| 主任务、继续分析、本地提取 | `dev/backend/cmd/api/bom_recognition.go`：`runRecognition`、`applySheetPlan`、`extractWithTools` |
| 进度、记录编辑/确认、提交门禁 | `dev/backend/cmd/api/bom_workbench.go`；公共路由 `main.go` |
| Workbook IR、区域、跨表引用、覆盖审计 | `dev/backend/internal/bom/intelligence.go` |
| 候选与校验模型 | `dev/backend/internal/bom/workbench.go`，同目录其他解析/校验实现按调用定位 |
| Provider 规划、分类、来源绑定 | `dev/backend/internal/ai/planning.go`、`completion.go`、`schema_validation.go` |
| Prompt / Pydantic 校验 | `dev/backend/internal/ai/prompts/`；`dev/backend/tools/requirements.txt` |
| AI 配置前后端 | `dev/src/features/AIProviderSettings.tsx`；`dev/backend/cmd/api/ai_providers.go` |
| CMDB UI / Adapter | `dev/src/features/CMDBWorkspace.tsx`；`dev/src/lib/api/cmdb-http.ts`、`cmdb.ts` |
| CMDB Domain/Schema/ChangeSet | `dev/backend/internal/cmdb/`；Schema 在该目录 `schemas/`；handler `cmd/api/cmdb.go` |
| 权限与会话 | `dev/backend/internal/identity/`、`cmd/api/authorization.go`、`identity_routes.go`；`dev/src/features/auth/` |
| 契约与设计 | `docs/openapi.yaml`、`DOMAIN-API-CONTRACTS.md`；UI 改动再按需读 `DESIGN-SYSTEM.md`、`UI-REFERENCE.md` |

## 5. 新环境启动

需要 Node/npm、Go、Docker Compose、Python venv。版本与依赖以 `dev/package-lock.json`、`dev/backend/go.mod` 为准。

```bash
# 仓库根目录：仅启动基础依赖，避免旧 web 占用端口
docker compose up -d postgres minio redis
# 先恢复或配置根 .env 与 dev/backend/.env，再执行未应用迁移
./dev/backend/migrate.sh
# 终端一：脚本读取 backend/.env
./dev/backend/run-local.sh
# 终端二：当前使用 5180，覆盖 Vite 默认 5174
cd dev
npm ci
npm run dev -- --port 5180 --strictPort
```

- `dev/backend/migrate.sh` 当前硬编码 Docker 容器 `keel-dev-postgres-1`、数据库/用户 `keel`，换环境先核对容器名；已应用迁移最高 **020**，不删库重建。
- `DEV_ORIGIN` 必须与实际前端 origin 一致；API 默认 `127.0.0.1:8081`，Vite `/api` 代理到该地址，可通过 `KEEL_API_URL` 覆盖。
- 默认 MinIO 镜像是本机镜像；新机器没有时按根 README 的 `MINIO_IMAGE` 配置兼容镜像。
- Pydantic 负责输出 Schema 校验，工作簿仍由 Go Excelize 解析。建立 venv 并安装 `dev/backend/tools/requirements.txt`，把 venv Python 绝对路径写到后端 `.env` 的 `KEEL_BOM_PYTHON`。当前机器路径 `/home/ubuntu/.local/share/keel/bom-tools/bin/python3`，迁移后不能直接假定存在。
- 登录 `/login`，AI 配置 `/admin/ai-provider`，BOM `/p/{projectId}/deploy/bom`，CMDB `/cmdb`。
- 空库首次通过 `KEEL_BOOTSTRAP_PASSWORD` 创建 admin，**重启不会重置现有密码**。`KEEL_LOGIN_PASSWORD` 是检查脚本使用值，不是覆盖密码的开关。密码不写进本文件或 Git。
- 当前 tmux：`keel-dev-5180`（目录 dev）、`keel-cmdb-api`（后端二进制 `/tmp/keel-cmdb-api`）。tmux 和 `/tmp` 二进制不随 Git 迁移。新机器用上述源码启动即可。
- 根 README 和 dev README 有历史“demo 5180 / dev 5174 / BOM 六步 / Rules 默认”等描述；当前状态以本交接和 BOM-WORKBENCH-V2 为准，不照旧文恢复错误流程。

## 6. 切环境时必须单独保留的状态

Git 只带代码，不带以下内容。使用受控方式迁移，不提交到仓库：

- PostgreSQL 数据（包括项目、用户/密码哈希、成员、Provider 配置、候选、任务、BOM、CMDB、Outbox）。
- MinIO 对象与 bucket（原始不可变文件及输出引用）；数据库和对象存储应保持同一交接时间点。
- 根 `.env`、`dev/backend/.env`；Provider Key 文件默认在 `dev/backend/.secrets/ai-providers/{provider-id}.key`，如配置 `KEEL_SECRET_DIR` 则以实际目录为准。只恢复数据库而漏掉 Key 文件，Provider 无法工作。
- 用户提供的根目录 `netbom.xlsx`、`bomv1.xlsx`，以及必要原始资料；均不在 Git。不要提交工作簿、原始真实行或凭据。
- Redis 是已备基础依赖，不代表当前 BOM 任务以 Redis 为权威状态；任务状态以 PostgreSQL 为准。

只切换 Agent/模型而仍使用本机时，不重新初始化数据库，不重复导入样本。真正换机器时若不恢复上述状态，旧批次 URL 不会存在。

### 可继续定位的已有批次（仅恢复原数据库后有效）

- 最近排查批次：`b359da1e-89fb-405c-819a-f0db10b7b1f3`，项目 `proj-0dd2eb36-f5de-4932-b6b0-a130f6bb5d48`，netbom.xlsx。最近只读检查：pending_review / review、33,616 条记录，仍有覆盖待处理；不是全部识别成功，也未由本轮提交。
- 早前隔离检查项目：`proj-f4d933ec-eabb-4856-81da-61dfb63b33a4`；bomv1 批次 `7881a5f5-cfb0-49e8-8348-610c5155d600`，netbom 批次 `dc466c3d-93d1-4f90-9ce8-07fd8392e08f`。
- 早前两个批次来源保留对账均 162,715/162,715；这只说明保留/扫描 complete，**不是识别准确率或入库许可**。详细限制见验证文档。
- 不要为了接续而重新调用这些任务。临时诊断脚本/截图位于旧机器 `/tmp/keel-bom-progress-*`，不是可迁移测试套件，不是运行依赖。

## 7. 必要检查与交接纪律

代码变化后按影响范围选择，不逐条无差别执行：

```bash
# Go 编译
(cd dev/backend && go build ./cmd/api)
# 前端构建（包含 TypeScript）
npm run build --prefix dev  # 从根目录执行
# 仅修改 OpenAPI 后生成类型
npm run generate:api --prefix dev
```

不要直接运行旧 `check:browser`、全仓 `go test ./...` 或历史导入脚本作为固定收尾；部分会创建项目、调用 Provider、写入数据。用户负责完整闭环测试。

提交前仅检查本次 diff 与秘密。沿用正确 remote；旧机器 SSH key 为根目录被忽略的 `deploy_key`，新环境需自行配置 Git 授权，不复制到 Git。提交后推送 `origin HEAD:main`。

给下一个 Agent 的开场指令：**先读 HANDOFF.md、AGENTS.md 和最新任务增补。当前从 DATACENTER-VISUALIZATION.md 接续拓扑/机柜剩余验收；若用户要求回到 BOM，再从现有 runRecognition 接续单主 Agent。不要自行切换任务、不重建 CMDB、不扩展范围，只做必要验证。**
