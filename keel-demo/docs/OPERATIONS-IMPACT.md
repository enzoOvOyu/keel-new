# 交付后运维全局影响分析

变更标识：OPS-2026-10-29。本轮交付的是设计与契约，非运维功能实现报告。

| 范围 | 现有能力 | 缺口 / 本次处理 |
|---|---|---|
| PRD FR-0、3、12、15 | 网络采集、硬件采集、监控告警、BMC/固件 | 缺持续纳管与独立运维页面；增补 FR-18…FR-22，保留所有原 FR |
| FR-9、14 | 交付清理与 maintained、统一资产历史 | 明确交付不是终点；维修、变更、复验、退役链路继续保留原身份 |
| CMDB | 稳定身份、BOM来源、Outbox投影 | 区分统一身份/关系事实源与业务域记录所有权；禁止第二套资产写入源 |
| 配置 | NetworkBaselineRevision、ConfigSetRevision、PushJob | 缺 baseline / intended / observed 的独立事实及差异版本，补齐引用链 |
| 观测 | 原始JSON、零散日志及能力标识 | 缺统一快照上下文、FDB/ARP/DDM字段、历史/保鲜/失败语义 |
| 身份模型 | Device、BOMItem、DeviceModelRevision、PhysicalPort、Cable、Rack | Machine、Switch、Component 是已有对象的角色/类别，不新增同名资产表 |
| 权限/维护态 | FR-9 仅写“批准的BMC带外能力”，FR-3/15泛指Redfish/IPMI | 按本次用户明确要求收紧 maintained 为 IPMI-only；禁止据旧泛述恢复 SSH/Agent/Redfish |
| API | 项目设备/网络设备、端口、快照、工单/告警既有路径 | 保留路径；新增类型化运维详情/配置/观测/固件/采集契约，不使用 GenericObject 代替新资源 |
| 前端/Mock | 64节点交付合成数据 | 补运维页面与完整Mock契约；现有演示未因此自动具备新增能力 |

## 已解决的语义冲突

1. **CMDB事实源与投影**：CMDB 是跨域统一资产身份、有效关系与生命周期的权威查询入口。BOM保留原始/标准物料事实，业务域持有配置/作业/观测写入事实，通过同事务Outbox幂等投影；不存在第二份手工录入的物理资产或任意改写的CMDB镜像。
2. **维护态通道**：本次用户明确约束优先。maintained 的Keel设备直连仅限批准的IPMI用途；SSH、Agent、Redfish不可因运维入口、审批、刷新、回滚或固件功能重新开放。外部观测只可接入既有批准的独立来源，不得作为代理绕过禁用通道。能力不支持/策略不允许时只读历史并显示原因。
3. **退场与退役**：交付CleanupRun完成进入maintained，持续运维；物理/服务生命周期最终退役另经审批、停止采集与凭据撤销，历史不可删除。二者不能混为同一个“结束”。

## 未决定事项与安全默认

- 时序/日志专用产品、保留天数、备份RPO/RTO、采样间隔、各型号阈值由部署/工程负责人确认；设计使用适配器和版本化策略引用，不臆造数值或更换数据库。
- maintained/IPMI-only 无法读取的GPU利用率、NIC细粒度指标、交换机FDB等只展示历史/外部批准来源，不能承诺实时全量。
- 不新增维护态到“临时恢复带内访问”的隐含转换；如未来需要，此变更必须单独修改批准的安全要求，本轮不提供该入口。

## 文档权威与同步

根目录 docs/ 与 AGENTS.md 是本次规范源；keel-demo/docs/ 同步其设计文档副本，保留演示已有的检查报告与截图。API定义见openapi.yaml，页面及领域行为见OPERATIONS-DESIGN.md。新增契约不代表已有Go/Mock Handler已实现。


## 当前六阶段主链路修正

本轮用户明确废弃数字门禁作为主链。当前唯一业务顺序、状态机、逐目标放行、安全交接、独立BOM菜单与API约束以[DELIVERY-LIFECYCLE.md](DELIVERY-LIFECYCLE.md)为准。历史编号只按[DELIVERY-MIGRATION.md](DELIVERY-MIGRATION.md)追溯，不得用于解锁。验机失败关联统一Issues并定向复验；NCCL不反写验机；清理/归档/客户签署齐全后才进入维护态。NP仍是网络内部断言。当前仅前端Mock与契约实现，没有真实设备后端。
