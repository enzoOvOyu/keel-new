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

## 当前实现进度（第一切片）

- 已解压完整任务与八页原型；新增独立 worktree，当前运行中的前端不会热更新本分支。
- 已新增 Go Kestra Adapter、Flow/Execution/CMDB 目标/预检接口、业务绑定 Migration、严格 SSH 执行、Agent 注册/mTLS 网关及目标端程序；未接入真实设备。
- 已替换 Flows 专属旧页面，新增紧凑 Flow/Execution/Agent 列表、三栏 Visual/YAML/Topology Builder、Quick Run 向导与执行详情；共享媒体/Blueprint 组件仍保留给其他模块。
- Kestra 2.0.7 和独立 PostgreSQL 已启动，受控凭据认证请求200；未访问或修改原业务数据库。
- 当前 TypeScript 检查与 Go 编译通过。尚在开发集成和定向验证，不代表任务全部完成；Bootstrap、目标冻结、文件/触发器上下文及状态对账仍需补齐检查。
