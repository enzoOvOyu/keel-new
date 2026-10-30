# CMDB 全链路资产档案中心

本轮保留双平面与统一API Provider，跨项目/cmdb项目过滤可清除，只查询授权数据。借鉴[Infrahub对象](https://docs.infrahub.app/objects/overview)、[类型化关系](https://docs.infrahub.app/schema/relationships)及[NetBox设备/组件/布线](https://netboxlabs.com/docs/netbox/features/devices-cabling/)的展示方法，不引入其后端或图数据库。

## 页面与职责

左侧机房→Pod/区域→机柜→设备树，可切换类型分类。上方搜索SN/hostname/IP、类型/状态/项目与保存视图。URL保存asset、q、type、status、rack、project_id和视图路由；切换五视图保留上下文。设备行使用既有五/八Tab Drawer，非设备在当前工作区展示，禁止抽屉嵌套。

五视图：资产目录（按类型字段和独立状态）、资产关系（一跳/按需展开，位置/组成/物理/逻辑/业务分层）、配置与来源（baseline/intended/actual/operational分开，字段来源与版本Diff）、生命周期（六阶段到运维/维修/换件/退役）、数据质量（定位对象/字段/来源与修正入口）。原项目资料保留辅助入口。

## 单一事实边界

目录是servers/switches/gpus/nics/ports/switch_ports/optics/cables/racks的只读投影，ID保持原值，不从cmdb_entities任意业务记录生成第二设备库。BOM负责物料/规划，配置、采集、作业、Issue各自持有事实；CMDB关联引用与摘要，不复制所有日志、时序或报告。

位置/组成来自稳定外键，物理关系引用Cable双端，LLDP观测独立保留并显示冲突；逻辑关系来自明确membership，不能凭Cable推断。默认一跳、限制节点、逐次展开，关联不代表流量路径或证实影响。物理/逻辑深挖跳Fabric Topology，位置跳RackView。

字段来源记录source/revision/observed_at/freshness/support/evidence_refs、BOM批次/Sheet/单元格/采集/人工/派生；没有来源位置就unknown。待审核候选单独标记，不能覆盖正式资产。人工仅operator_notes且需权限/项目/版本校验，写入审计；采集值、SN和批准基线不允许直接编辑。

六阶段摘要使用既有delivery_stage_records，生命周期聚合stage_events/lifecycle_events/report/Execution/Issue及handover。换件事件保留旧实物身份和有效期，替代件使用不同ID；历史实物不是当前库存重复副本。维护态不发起SSH/Agent或自动采集。

数据质量动态检查适用身份缺失、型号未绑定、位置/U冲突、Cable双端、LLDP与规划冲突、陈旧及待审核歧义；定位具体对象/字段/来源，不自动生成硬件故障工单。已有硬件健康与数据完整性分开。

## API

/cmdb/dossier/catalog（分页/过滤/排序/分类计数）、tree、relations（root/expanded/type/limit）、assets/{asset_id}/provenance、configuration、lifecycle、quality、manual-fields，及saved-views。全部授权项目过滤，写操作固定project/asset/revision并校验字段白名单；400/403/404/409/422明确，request_id贯穿。专用CmdbDossier* schema，不使用GenericObject。查询on-demand，关系渐进展开，Query key包含项目/用户/资产/过滤，离开可取消。真实Go实现仍后续逐模块接入。

## 验收链路

搜索服务器→部件→端口→Cable→对端交换机；配置Diff→字段来源→作业/Issue；生命周期→验机/NCCL/退场证据；质量问题→导入或采集；维护/unknown/stale/无权限；列表/抽屉/机柜/拓扑稳定ID一致。实际结果记录CMDB-DOSSIER-VALIDATION.json；未执行不标通过。


## 本轮实际验证

最终TypeScript+Vite生产构建通过。浏览器完成资产→部件→端口→Cable→对端及五/八Tab抽屉、配置版本/来源、验机/NCCL/历史退场归档、Execution、质量导入入口、保存视图、人工备注/审计与权限。10类真实返回response通过JSON Schema校验，维护态/stale/unknown与无授权项目已检查。结果见CMDB-DOSSIER-VALIDATION.json；不等于正式Go、原生采集或全平台回归通过。
