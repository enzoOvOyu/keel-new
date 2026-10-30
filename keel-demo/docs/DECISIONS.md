# DECISIONS.md — 待决策与冲突登记

只记录会改变业务行为、数据契约、安全边界或实施成本的未决事项。普通实现细节由开发者按现有架构处理，不要把此文件变成 TODO 清单。

| ID | 决策/冲突 | 影响范围 | 可选方案 | 默认处理 | 状态/负责人 |
|---|---|---|---|---|---|
| D-001 | 外部 AI 服务是否允许接收脱敏后的开局字段 | BOM AI mapping / 数据安全 | 内部模型；批准的企业 API；完全关闭 AI | 未批准前不发送原始工作簿，只用本地规则并进入人工审核 | 待确认 |
| D-002 | 生产部署的身份提供方、Secret Manager、对象存储和队列产品 | 部署/认证/密钥 | 按现有企业标准；无标准时由运维评估 | 不把开发 Compose 配置视为生产方案 | 待确认 |
| D-003 | BOM 样本与字段准确率目标的正式阈值 | 验收 | 依据风险分层与 PoC 数据制定 | 未批准前不宣称达到某个准确率；关键关系必须规则通过或人工确认 | 待确认 |
| D-004 | 业务域完整 OpenAPI 的分阶段冻结范围 | API 开发 | 先冻结 BOM/P0；按实施阶段扩展其他域 | 不为未进入实施阶段的模块臆造完整请求/响应契约 | 待确认 |


## 已确认的全局决策（v3.2）

- D-01 前端先行：全量 UI + API contract + mock 先完成，再逐模块接 Go 后端。
- D-02 Flow 首期范围：面向已纳管服务器/交换机的批量脚本执行、频率调度和执行生命周期监控；交互参考 Kestra，不复制其代码、不以自研完整通用 DAG 引擎阻塞首期。
- D-03 CMDB：从 BOM 导入任务开始全链自动沉淀；权威业务表 + transactional outbox + idempotent projector，CMDB 不作为第二套人工录入源。
- D-04 Excel：以 `TX B300 Layout-new.xlsx` 作为真实格式样本，具体 Sheet 分型和映射策略见 `EXCEL-IMPORT-PROFILE.md`。工作簿包含敏感凭据字段，原文件禁止进入文档包/Git/mock/外部模型请求。
- D-05 UI 质量：实现前参考 Kestra UI、shadcn/ui Blocks；每个主要页面提供线框和状态矩阵，禁止仅堆默认卡片。
- D-06 API：全页面 API 先在 OpenAPI 中设计并由 mock 实现，真实 Go API 按模块替换；mock 与生产共用 schema。
- D-07 尚未确定：是否选用 Kestra 作为外部独立执行引擎（当前默认不依赖）；真实 LLM provider 与数据处理边界；Excel 样本中敏感列的正式凭据录入流程。未确认前不阻塞前端 mock，但不得在生产环境猜测。

## Rack Diagram 已确定决策

- RD-01：首期用 React + SVG，不引入 3D 引擎。
- RD-02：BOM/CMDB 是资产和位置事实源；图纸是版本化投影/编辑工作区，已审批的布局修改回写权威位置并留审计。
- RD-03：缺少 rack/U/u_height 不得自动猜成实际布局；只可形成 planned 候选或未分配项。
- RD-04：自动重生成默认保留人工覆盖，展示 diff；清除覆盖必须由用户显式确认。
- RD-05：参考 Rackula 的画布体验和 RackView 的交付逻辑，但不复制源码、不增加其运行依赖。

## OPS-2026-10-29 · 交付后运维设计增补

### OPS已明确
- OPS-D01：按用户最新要求，CMDB为统一资产身份/关系/生命周期事实权威入口；保留既有BOM/业务域权威写入+Outbox，不另建平行资产源。
- OPS-D02：旧FR-9仅泛述“批准BMC带外能力”；本次用户明确为maintained/IPMI-only，优先适用。SSH/Agent/Redfish不因运维/审批/fallback恢复；本次不提供扩大通道的接口。
- OPS-D03：Machine/Switch/Component为已有Device/BOM类别视图；ConfigBaseline/ConfigRevision为已有基线与版本统一引用，禁止重复建表复制事实。

### OPS待负责人确认（不阻塞只读页面/Mock契约）
| ID | 未决项 | 安全默认 |
|---|---|---|
| OPS-P01 | 指标/日志存储产品、容量、归档位置 | 保留适配器，沿用已有存储技术；不宣称生产选型完成 |
| OPS-P02 | 各类别保留天数、legal_hold责任人与RPO/RTO | 不提前清理被引用证据；生产启用前确认 |
| OPS-P03 | 每型号指标阈值、采样间隔与freshness窗口 | 引用批准revision，缺失为unknown/BLOCKED，不臆造阈值 |
| OPS-P04 | maintained/IPMI-only设备可提供的实际指标及独立外部数据源 | 按能力与策略展示，无法读取为unsupported/policy_denied，只读历史 |
