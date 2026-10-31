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
| Kestra / Agent / SSH 后端 | 尚未实现；SSH 页面为 Mock 元数据 | 新增模块化底座；现有 SSH 入口增加真实适配，不将 Mock 当作信任 |

## 设计边界

- 菜单仅 Flows、Executions、Agents；旧蓝图/介质/调度路由重定向至上下文。
- Kestra 2.0.7 固定版本；以官方该标签 Controller 为 REST 适配依据，Flow/Execution 状态来自引擎。
- maintained 严格 IPMI-only；Agent Preferred 不得绕过维护态或在执行状态未知时自动切换 SSH。
- SSH 只使用受控文件秘密引用及严格 known_hosts，不读取前端 Mock 凭据。
- 缺引擎、CA、SSH 信任或插件返回明确不可用；不能用合成成功代替真实执行。
- 运行状态、实际检查和剩余缺口随实现更新，不提前宣称完成。

## 交付实现状态 · 2026-10-31

代码提交：初始底座`63ca2c9`，当前功能`826b964`。此后文档提交不改变运行二进制。运行环境为独立5190/8096/8095，原5180/8081未重启、未迁移、未导入资产；未合并main、未push。

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
| API与存储 | 41个FlowsReal操作、生成TS类型、迁移021/022、绑定与冻结目标、游标、幂等、审计、CMDB Outbox | 独立库已迁移；注册表不冒充41项逐端点验收 |

## 数据与执行链

CMDB `cmdb_entity` + `cmdb_asset_scope` → 授权目标/能力与SSH预检 → 保存冻结目标、版本与通道 → Kestra SECRET票据/JSON输入 → Loop子执行 → 原生If区分Agent/SSH → Go受控Dispatch → 实际Agent或严格SSH → 结果/Outbox → 统一Execution/CMDB关联。

Kestra2.0.7兼容修正包括Loop替代不存在的ForEach、`fromJson(item.value)`读取Loop对象值、`toJson`替代已移除json过滤器、SECRET输入启用引擎加密。不是只通过YAML校验就认定可执行。

## 实际检查

- `go test ./internal/flows ./cmd/agent ./cmd/api -count=1`通过；包括通道/维护态、跨项目namespace、已接受Agent失联、交换机写操作阻断、任务journal、文件边界和实时输出。未配置隔离DSN时集成用例明确SKIP。
- 显式隔离DSN运行`TestKestraSSHIntegration`通过：真实引擎、独立PostgreSQL、两个真实SSH协议合成服务，部分失败、失败目标单独新Execution、成功目标不重复、幂等及严格Host Key；最终重试引擎SUCCESS。
- [运行记录](runtime-validation.json)：实际本地非rootAgent/mTLS执行、结束前stdout、退出码0、票据脱敏；Execution游标/筛选绑定、跨项目403；原生Pause→Resume→SUCCESS、Namespace文件读写、旧Revision409。
- [浏览器记录](ui-validation.json)：8项定向交互通过，页面无pageerror或Flows失败请求。截图见[screenshots](screenshots/)；实际查看Builder与目录截图并修正对比度/单节点过度缩放。
- `npm run build`（含TypeScript）通过。遗留全局共享mock seed约31MB、既有大chunk/Zod annotation警告仍在，不为本任务重构其他模块。
- `node dev/scripts/check-real-api-docs.mjs`通过46个登记项（5可视化+41Flows）；只证明Schema/引用一致，不证明所有接口闭环。

## 明确限制及后续项

1. CMDB目标批次Flow的自动Schedule需要额外的作用域Job触发适配器；当前启用会422拒绝。原生安全Flow的Schedule/Webhook/Flow事件可保存并由Kestra执行，最近100项增量发现，不宣称完整历史同步。
2. 受控批次当前每目标一个任务，可用原生DAG控制包裹；尚非任意多步骤设备插件框架。任意Worker脚本/外部HTTP/引擎秘密上下文不开放，防止绕过项目与凭据边界。
3. Namespace提供128KiB文本资源，非完整二进制介质版本管理。SSE断线采用状态重取/日志去重，不具备完整持久Last-Event-ID重放保证。
4. 未执行生产SSH Bootstrap、真实交换机命令验证、生产8目标混合批次、生产规模性能或完整审批集成。普通执行权限不能代替高危审批；交换机写配置仍回Fabric。
5. 页面表格本地分页与目录上限、Execution游标边界见[真实API契约](../api/flows.md)。不能把上限内演示当作无限规模能力。
6. 旧BOM/拓扑环境8081包含不在本分支的Agent修复。未来部署需先兼容合入该修复，禁止直接用此分支旧基础覆盖。

## 快速接续索引

- [真实API](../api/flows.md)、[OpenAPI](../openapi.yaml)、[机器注册表](../api/real-api-registry.json)。
- [本地运行](../../dev/deploy/flows/README.md)、[非秘密配置示例](../../dev/deploy/flows/.env.example)、[原生Flow示例](example-native-flow.yaml)。CMDB批次模板从实际`GET /flows/templates`获取，避免手写第二套协议。
- `dev/backend/internal/flows/{service,transport,gateway,kestra}.go`：执行主链；`cmd/agent/`：目标端；`cmd/api/flows.go`：路由；`dev/src/features/flows/`：业务页面；`dev/src/lib/api/flows.ts`：唯一页面数据入口。
- 本文记录实现与检查事实，包中尚受限/未验收项不能标为全部完成。
