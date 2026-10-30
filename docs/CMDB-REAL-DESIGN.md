# CMDB 真实实现边界

## 固定模型与扩展

沿用 Go/PostgreSQL、cmdb_entity、cmdb_relationship、bom_item、project_asset 与 Outbox。CMDB 是已发布资产当前事实及关系的统一入口；BOM 保留物料和规划版本，配置/采集/工单等领域保留所有权，不产生另一套设备身份。演进现有 cmdb_entity 的稳定键为 text，旧 UUID 原值不变，原有 BOM 外键作为可空唯一关联保留，以承载既有演示文本ID。既有关系表增加类型定义、独立版本、有效期与外键；历史不删除。

固定核心列承载身份、类型、SN、项目授权、状态、版本；受控 JSONB 只承载 Schema 声明的扩展属性。Schema Registry 用 Git JSON 定义属性类型/单位/来源/可编辑/约束/展示分组和类型化关系。它不是动态数据库，不运行 Schema 中的 SQL 或脚本。不引入图数据库或通用插件平台。

## 领域、版本和所有权

BOM 通过现有事务 Outbox 调用 CMDB Domain Service，保证事件幂等、资产ID复用、关系双端外键及来源落库。人工写入通过带 expected_revision 的 ChangeSet；Draft→Review→Approved→Published 或 Rejected。审核与发布分离，申请人不得自批；发布事务对资产/关系版本、Schema、来源所有权及 Cable 端点重新验证。失败保留已批准 ChangeSet，可按相同幂等键重试。换件创建新身份、结束旧安装关系，保留旧部件和历史。

来源按 bom/agent/bmc/switch/manual/derived 分层；规划、批准基线、Intended、实际配置和运行观测独立保存。人工只修改声明为 editable/manual 的字段，不能覆写BOM身份或观测值。快照无需每次人工审批。原始日志、时序、配置原文及证据在对象存储/原领域存储，CMDB只存对象引用、摘要和资产关联。

## 查询与前端保护

保留现有 /cmdb/dossier/* 契约。SQL负责授权范围内目录分页/筛选/排序、资源树、邻接关系、来源、生命周期与质量；限定关系深度及节点数量，查询有截止时间。预留 GraphRepository 适配器，但 PostgreSQL 为唯一当前实现。

现有 React 组件结构、Sidebar、顶栏、五/八Tab Drawer、主题与设计令牌保持。只将 API 调用改成可等待的同形 Adapter；同步 device/保存视图读取使用该 Adapter 的已加载缓存，禁止由页面另造资产。ChangeSet 与 Schema 属性展示作为既有详情/生命周期区域的增补，不替换现有工作台。

UI 前置基线在 ui-baselines/cmdb-before：五视图、原抽屉和样式快照/哈希。接入后复用同一视口/资产/路由比较。只运行本任务指定的针对性契约、数据、权限和视觉检查，不重复全平台冒烟。

## 来源参考

只借鉴 [Infrahub Schema](https://docs.infrahub.app/schema/overview)、[类型化关系](https://docs.infrahub.app/schema/relationships)、[对象与来源](https://docs.infrahub.app/objects/overview) 的领域原则；不复制其技术栈、数据库或完整产品。
