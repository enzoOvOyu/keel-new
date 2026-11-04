# Keel 当前模块文档

更新：2026-11-03。按模块交付和上线，不再以旧全量 PRD 作为实施清单。通常只需读本页、目标模块和对应 API 契约。

| 模块 | 当前设计与实现入口 | API 语义 | 当前交付边界 |
|---|---|---|---|
| 智能开局 Kickoff | [kickoff](modules/kickoff.md) | [kickoff API](api/kickoff.md) | 端接驱动 Agent、名称身份采用/核对/发布已部署；真实 Provider 验收待执行 |
| CMDB | [cmdb](modules/cmdb.md) | [cmdb API](api/cmdb.md) | 统一身份、关系、来源、版本和 ChangeSet 已实现上线；设备观测不因此视为完成 |
| 数据中心拓扑与机柜 | [topology](modules/topology.md) | [visualization API](api/visualization.md) | 五工作区及真实 CMDB 查询已上线；缺几何/观测的能力如实不可用 |
| 作业平台 | [jobs](modules/jobs.md) | [flows API](api/flows.md) | Kestra 原生界面、统一 Job Service 与 Agent/SSH 已实现上线；生产设备批量验收未完成 |
| Fabric | [fabric](modules/fabric.md) | [fabric 分支契约](api/fabric.md) | 独立分支持久化切片；当前暂停；未合入 dev 或发布到 5180/8081 |

## 从哪里继续

- 运行版本、工作区和当前交接：[HANDOFF](../HANDOFF.md)。当前主环境记录为 `24bd44d`，不是通过本次文档任务重新测得。
- 新需求/后续上线流程：[模块维护规则](MODULE-WORKFLOW.md)。只更新受影响模块，不重新展开全量 PRD。
- 字段、枚举和 HTTP 定义：[OpenAPI](openapi.yaml)；实际实现登记及证据：[API 入口](REAL-API-CONTRACTS.md)、[注册表](api/real-api-registry.json)。Schema 存在不代表接口可用。
- 未决冲突及真实缺口：[DECISIONS](DECISIONS.md)；模块自身的待办见模块页，不复制到多份总表。

## 按需参考，不属于开工必读全集

| 触发条件 | 参考 |
|---|---|
| 修改界面 | [DESIGN-SYSTEM](DESIGN-SYSTEM.md)、[UI-REFERENCE](UI-REFERENCE.md)；以最新 Kickoff 视觉基线为准，作业平台保留原生组件 |
| 修改鉴权/秘密 | [IDENTITY-RBAC](IDENTITY-RBAC.md)、[SECURITY](SECURITY.md)；实际权限来自统一后端 |
| 修改 HTTP 协议 | [API-CONVENTIONS](API-CONVENTIONS.md)；具体 DTO 仍以 OpenAPI 为准 |
| 触及全平台阶段 | [六阶段主链](DELIVERY-LIFECYCLE.md)；阶段顺序不等于本轮全部实施范围 |
| 修改项目/AI 配置 | [PROJECT-AI-CONFIGURATION](PROJECT-AI-CONFIGURATION.md) |
| 处理旧工程数据 | [保留工程接口](api/engineering.md)；不恢复旧 BOM 或五步工作台 |
| 需要演示/解析规则 | [MOCK-DATA-CONTRACT](MOCK-DATA-CONTRACT.md)、[EXCEL-IMPORT-PROFILE](EXCEL-IMPORT-PROFILE.md)；不覆盖当前真实 Kickoff 契约 |
| 后续运维模块立项 | [OPERATIONS-DESIGN](OPERATIONS-DESIGN.md)；属于待逐模块确认范围，不自动视为已交付 |

## 历史和证据

[归档目录](archive/README.md)保存旧 PRD、阶段方案和原长篇交接，只有追溯决策时才读。旧文档中的“当前”“下一步”“允许部署”均是当时语境，不能覆盖本目录。

原有 JSON 验证报告、截图、原型代码和发布记录保留原路径；旧报告只证明记录的版本及范围。裁剪方式和变更清单见[本次整理记录](DOCUMENTATION-REORGANIZATION.md)。
