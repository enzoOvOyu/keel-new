# 真实 API 契约与实现台账

> 当前API部署`ceab753`/v40（8081），前端5180保持，迁移038；见[部署证据](intelligent-deployment/deployment-agent-repair-20261101.json)。真实MM资料已完成15张表候选，首轮理解验收未通过，见[理解审查](intelligent-deployment/SOURCE-UNDERSTANDING-REVIEW-20261101.txt)。下方旧模块运行证据保留为历史。

更新：2026-10-31。用途：后续迭代先从这里确认接口是否真实实现、部署到哪里、哪些行为已经验证，再定位变更涉及的代码。没有登记的操作不自动判为未实现，也不自动判为可用。

## 固定入口与权威边界

| 要查什么 | 固定入口 | 使用规则 |
|---|---|---|
| 当前分支、环境、进度和下一步 | [HANDOFF](../HANDOFF.md) | 当前事实入口，不从聊天或旧计划推断状态 |
| 请求、响应、参数、枚举 | [OpenAPI](openapi.yaml) | 机器可读的字段契约唯一来源，前端类型由它生成 |
| HTTP 通用规则 | [API-CONVENTIONS](API-CONVENTIONS.md) | 身份、错误、分页、幂等、CSRF 等公共规则 |
| 领域业务语义 | [DOMAIN-API-CONTRACTS](DOMAIN-API-CONTRACTS.md) | 事实分层、门禁和领域规则，避免各页面各算一套 |
| 本轮真实接口使用说明 | [可视化契约](api/visualization.md) | 端点、页面、Schema、错误处理、来源和限制的对照 |
| 可机器核查的登记 | [真实接口注册表](api/real-api-registry.json) | 状态和引用，不复制完整请求/响应 Schema |
| 全部设计操作目录 | [API-OPERATION-INDEX](API-OPERATION-INDEX.md) | 仅设计索引；操作存在不代表 Go 已实现 |

本台账不新建一套契约，也不更改现有 API。发现文档、OpenAPI 和运行行为冲突时，登记差异并定向核对，不能拿任何一份旧文档静默覆盖另一份。

## 当前登记范围

本次已逐项核对 **5 个可视化操作**：4 个新查询接口与1个兼容读取接口。运行代码版本为 `4541850`，部署事实来自 [deployment-validation.json](topology/deployment-validation.json)，不是根据分支 HEAD 推断。

| 操作 | 代码状态 | 部署状态 | 本次运行证据 |
|---|---|---|---|
| GET `/cmdb/visualization/subgraph` | 已核对 | 已部署 | 启动检查200 |
| GET `/cmdb/visualization/matrix` | 已核对 | 已部署 | 启动检查200 |
| GET `/cmdb/visualization/connections` | 已核对 | 已部署 | 启动检查200 |
| POST `/cmdb/visualization/trace` | 已核对 | 已部署 | 启动检查200；不等于路径验收通过 |
| GET `/cmdb/visualization` | 兼容实现已核对 | 随同版本部署 | 未纳入这次4接口启动探测，不能补写200 |

“代码已核对”“已部署”“已验证”分别维护。启动200、页面加载、交互闭环、真实导入、规模性能是不同证据等级，不能互相代替。台账描述本地开发环境，不是生产发布认证。

## 其他模块从哪里继续

| 模块 | 已有记录入口 | 本次台账状态 |
|---|---|---|
| 身份、Session、RBAC、项目成员 | [IDENTITY-RBAC](IDENTITY-RBAC.md) | 已有真实后端记录，本次未逐操作复核 |
| 项目创建与 AI Provider 配置 | [PROJECT-AI-CONFIGURATION](PROJECT-AI-CONFIGURATION.md) | 已有真实实现记录；秘密不登记到契约样例 |
| BOM 导入及 Agent | [BOM 工作台](BOM-WORKBENCH-V2.md)、[BOM 实现状态](BOM-REAL-IMPLEMENTATION-STATUS.md)、[HANDOFF](../HANDOFF.md) | 有真实后端；历史未完成能力以交接为准，不宣称完整 Agent 验收 |
| CMDB、Schema、ChangeSet、档案与抽屉 | [真实设计](CMDB-REAL-DESIGN.md)、[档案中心](CMDB-DOSSIER.md) | 已有真实实现记录；仅本轮可视化查询逐接口入账 |
| 其他交付、设备执行、采集、监控与工作流 | [实施计划](IMPLEMENTATION-PLAN.md)、OpenAPI 与各域文档 | 不能由 UI 或设计路径推定真实后端；修改到哪个模块，再补该模块的真实登记 |

这是一份有明确覆盖范围的真实台账，不是“全平台全部接口都已审核通过”的声明。

## 后续迭代工作流

1. 读 HANDOFF 的当前状态和本台账对应模块；只读受影响契约及业务规则。
2. 在需求中列出受影响的 operationId、请求/响应字段、页面、权限/作用域、兼容性和验收行为。
3. 契约不变的 UI 调整沿用已有 Provider。契约变化先修改 OpenAPI 与领域语义，再生成类型、改 Go/Adapter；不另造页面内协议。
4. 接口、枚举、默认值、分页、错误、权限、数据所有权或副作用变化时，同一提交同步登记和说明。
5. 运行文档校验和风险对应的必要检查，记录真实结果；仅代码变更不擅自部署，仅部署不自动合并 main。
6. 更新 HANDOFF、实施报告及部署证据，保留“代码版本”和“运行版本”的区别。

定向审查仍覆盖实际变更与直接调用链；权限、事务、并发、迁移、兼容性等风险需要对应代码审查。文档的作用是免去重复遍历与猜测，不是跳过正确性检查。

## 只读一致性校验

在工作树根目录执行：

```bash
node dev/scripts/check-real-api-docs.mjs
```

需要已安装的 `dev/` 依赖，复用现有 YAML 包。检查注册表中的方法/路径、operationId、请求/响应 Schema、实现文件、说明链接及部署证据。它不启动服务、不请求数据库、不调用 AI，不证明 HTTP、权限、SQL语义或性能正确；这些必须有独立验证记录。

新增模块逐步登记，未登记的设计操作不会因此被判定为失败或已实现。


## Flows 功能分支真实实现

新增 [Flows 实际契约](api/flows.md)。登记表中的Flows操作只表示独立开发分支实现，运行与检查见[实施记录](flows-redesign/implementation.md)；不继承上方可视化部署4541850的证据，不代表已替换原5180/8081。

Flows当前登记49个操作（开发补齐afe8cb9），加原5个可视化共54项。新增Schedule/文件版本与分发/步骤/审批/持久事件的契约均在Flows使用文档与OpenAPI；不是逐端点生产验收声明。

## 智能开局工程

[P1设计](intelligent-deployment/P1-DESIGN.txt)保留三层模型和分域边界。P1–P6已实现并部署，新增33个工程HTTP操作；契约见[工程接口](api/engineering.md)，检查及缺口见[实施记录](intelligent-deployment/implementation.json)。本次四个工程只读GET启动检查通过；真实AI、真实XLSX及跨客户泛化验收未完成。
