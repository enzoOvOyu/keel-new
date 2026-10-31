# Flows 重构实施记录

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
