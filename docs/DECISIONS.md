# DECISIONS.md — 待决策与冲突登记

只记录会改变业务行为、数据契约、安全边界或实施成本的未决事项。普通实现细节由开发者按现有架构处理，不要把此文件变成 TODO 清单。

| ID | 决策/冲突 | 影响范围 | 可选方案 | 默认处理 | 状态/负责人 |
|---|---|---|---|---|---|
| D-001 | 外部 AI 服务是否允许接收脱敏后的开局字段 | BOM AI mapping / 数据安全 | 内部模型；批准的企业 API；完全关闭 AI | 未批准前不发送原始工作簿，只用本地规则并进入人工审核 | 待确认 |
| D-002 | 生产部署的身份提供方、Secret Manager、对象存储和队列产品 | 部署/认证/密钥 | 按现有企业标准；无标准时由运维评估 | 不把开发 Compose 配置视为生产方案 | 待确认 |
| D-003 | BOM 样本与字段准确率目标的正式阈值 | 验收 | 依据风险分层与 PoC 数据制定 | 未批准前不宣称达到某个准确率；关键关系必须规则通过或人工确认 | 待确认 |
| D-004 | 业务域完整 OpenAPI 的分阶段冻结范围 | API 开发 | 已由 D-06 和后续全量 Mock 授权确定：全页面契约先行、Go 分模块接入 | 当前逐操作范围见 API-OPERATION-INDEX；不表示真实后端已完成 | 已被后续决策取代 |


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

## IA v2.3 · 双平面与统一抽屉

### IA v2.3 实施范围冲突
历史 v2.3 阶段记录，已由后续明确的 v2.4 前端实施授权取代，不再阻塞当前 Mock。原始冲突：需求前文要求前端Mock实现，但第8节明确“本次仅更新设计规范和前端 IA 实现计划”。按较具体的范围限制先冻结文档与契约，不声称前端实现或验收通过；如需执行前端开发，以后续明确任务启动。单项目种子不构成跨项目验收证据。

### IA v2.4 待决冲突
- IA24-01：本轮 §6.3 将 maintained 的 Redfish 列为可用；既有 PRD FR-9、OPS-D02、SECURITY 和 AGENTS 明确维护态 **IPMI-only**、禁止 Redfish/SSH/Agent。待产品与安全负责人明确批准变更前，按现有 IPMI-only 执行，展示 Redfish policy_denied；不阻塞无关页面。
- IA24-02：本轮要求实际检查 `TX B300 Layout-new.xlsx`；工作区检索未找到工作簿。现有 Excel Profile 是设计资料而非本次实测；不能根据文件名或旧描述声称已检查真实文件。待提供受控文件后只读取结构并脱敏记录，不复制真实行或凭据。
- IA24-03：是否集成 Kestra 执行引擎尚无部署、许可证、隔离和授权决策；本轮使用明确标记的前端 Mock，不声称实现或接入成熟调度器。


### IA24-04 最新数据包授权与维护策略冲突
新包project_members的user-admin/user-delivery/user-network可按唯一角色定位既有身份，但user-ops不存在且角色DE不能唯一推出SRE；在明确之前不授予该记录的权限。当前以users.project_scopes及可解析成员记录为授权事实，admin可以查看全部项目。新包maint-001/002允许Redfish，与PRD FR-9/OPS明确的IPMI-only冲突，保留历史源记录但操作仍拒绝Redfish。metadata计数/摘要陈旧属于可确定修复，不修改资产身份。


## IA v2.4 Mock 前端实施记录

导航树、完整页面清单、路由/权限/作用域/加载策略、五/八Tab抽屉与API对照统一见 [IA24-IMPLEMENTATION-MATRIX.md](IA24-IMPLEMENTATION-MATRIX.md)。原FR-0…24、门禁、BOM与维护态规则保留。当前实现为前端Mock，不表示已实现Go/真实设备/真实AI。权威种子为 `fixtures/demo-full-64-b300/demo-seed.json`；原包83集合，补齐与规范化后集合数量以当前manifest为准，主项目硬件数量不变。结构检查、数据修复、契约与交互结果以IA24-*报告为准，无法采集/缺少证据保持unknown或阻断。

## BOM-REAL-01 · 历史 CSV 附录兼容范围

当前 PRD 明确七 CSV 目录，并指向 API Schema，但仓库与已检查文档包未提供独立完整附录 A 列清单。本轮按现有 BOMItem Schema 输出，版本标记 keel-bom-openapi/v1；不声明未提供的旧外部包格式已经认证。真实 API 提交与 CMDB 发布不依赖该旧格式，不阻塞当前 Excel 闭环。

## IDENTITY-NH-2026-10-30 · 待确认：Critical 网络签署资格

PRD §2 已定义 NH 业务职责，但本次要求的六个稳定默认身份角色中没有 NH，也没有规定哪种角色/资源授权可获得 NH 签署资格。保留 NE+DE+NH 三签；实现 blocked_policy，禁止 admin 替代。待产品选择：增加独立签署资格（不改变六个默认角色），或明确新增 NH 角色及分配范围。仅阻断 Critical 网络审批，不影响普通 NE+DE 双签、本地登录和 BOM。

## RACK-VIS-2026-10-31 · 安装面、几何与实装来源

实查 CMDB 88 条 active located_in 为 planned/bom，安装面与深度未记录；旧 synthetic fixture 有 front 默认值和布局版本。决定不跨来源补写、不把登记规划冒充实装。独立原型双面显示明确标注的虚线占位，Comparison 等待批准规划与实装快照；不阻塞 Gallery、U 立面与 Mapping。现场安装确认责任人/权限、可接受实装证据、规划比较版本的选择需后续业务确认；沿用既有 ChangeSet 审批，不自行授予新签署权。完整数据审计与设计见 [rack-visualization.md](rack-visualization.md)。


## VIS-2026-10-31 · 批准可视化落地与分支隔离

用户批准 topology.md V3 与机房原型进入正式前端；先落查询契约和页面逻辑，真实导入由用户后续统一联调。沿用批准 SVG/矩阵/Metro，不为模仿技术名称强制重写为 G6；Three.js 只用于已批准的机房空间模式。规划/观测位置保留分层来源，局部直接物理段不推导交换转发。

开发固定在 `feat/datacenter-visualization-approved` 独立 worktree 和 5182/8082，未经用户指令不合并/发布 main。原 5180/8081 恢复至开发前版本；本轮无 migration 或资产导入。独立服务暂复用原数据库读取，真实写入联调需后续独立配置。实现和未实现边界统一记录于 `docs/topology/implementation-report.md`，不创建第二主设计。


## 2026-10-31 · Flows 引擎与作用域最小兼容决策

- 已采用：Kestra2.0.7原生Loop/If/HTTP组合受控Job模板，不假定已安装Keel Java插件；`fromJson(item.value)`与`toJson`经实际运行确认。
- 已采用：项目namespace使用稳定摘要避免字符清洗碰撞；旧binding编辑保持原namespace。Agent任务接受后未知只能对账，不能重复经SSH执行。
- 暂受限：CMDB目标Schedule尚无作用域凭据/目标冻结触发适配，不能直接启用；原生无设备Flow调度仍归Kestra。后续需增加受控触发身份/目标组解析，而非Go自建调度器或长期暴露dispatch ticket。
- 暂受限：原生SSE不是完整持久游标重放，使用当前状态重取和日志去重；每目标多步骤设备插件、生产Bootstrap/审批与规模验收见实施记录，不静默假装实现。
- 运行隔离：原环境BOM Agent集成源码不在本功能分支，后续部署前需保留这些修复；本轮不合并、不替换原5180/8081。


## 2026-10-31 · Flows 开发补齐（覆盖前述暂受限决策）

- 目标批次调度采用Kestra Cron标记Flow+Go幂等消费，独立Schedule API管理授权目标。不是把长期dispatch ticket写入触发器，也不自建Cron。
- 依赖步骤由目标作业顺序执行，实际状态与有界输出持久化；跨设备批次/Flow编排仍由Kestra负责。
- 原生SSE代理替换为Keel持久事件日志，提供递增ID重放。资源版本加入独立Namespace文件表及实际Agent分发，CMDB仍不存原始文件内容。
- 审批复用既有记录/签名，冻结全文附在审核理由中供审阅，SHA/设备版本绑定一次新执行；不改变领域角色规则或开放未批准网络写操作。
- 批次Loop必须显式`transmitFailed:false`，否则单设备失败可能取消其他已接受目标。设备结果与引擎结束状态分开显示。依据[Kestra Loop失败传播](https://kestra.io/docs/best-practices/loop)，已通过本地真实引擎SSH协议检查。
- 当前待项是生产设备/安装/规模验收；功能边界和数据上限见实施记录，不把未运行的环境验收当成代码缺口，也不反向伪造通过。

## 2026-10-31 作业平台命名更正
用户最新明确界面名称为「作业平台」，覆盖压缩包中固定 Flows 名称的旧约束。保留 /flows 路由、API、权限标识及领域 Flow/Execution 名称，不另建入口。
