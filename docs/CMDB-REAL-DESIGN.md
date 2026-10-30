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

## 当前真实数据接口对照

| 能力 | HTTP | 所有权/版本 |
|---|---|---|
| Schema及兼容性 | GET/POST `/cmdb/schemas`、POST `/cmdb/schemas/validate`、GET `/cmdb/schemas/versions`、GET `/cmdb/schemas/{schema_version}` | 平台 Schema 管理权限；expected_version；历史不可覆盖 |
| 目录、资源树、搜索 | GET `/cmdb/dossier/catalog`、`/tree` | 项目/资源授权，cursor、limit≤100、sort；跨项目只含获授权资产 |
| 详情、来源、配置、生命周期 | GET `/cmdb/dossier/assets/{asset_id}` 及 `/provenance`、`/configuration`、`/lifecycle` | 保留原 DTO；来源分层，不补零 |
| 一跳关系/受控展开 | GET `/cmdb/dossier/relations` | root_id、expanded_ids≤10、limit≤120；两端分别授权 |
| 字段与关系历史 | GET `/cmdb/dossier/assets/{asset_id}/revisions`、`/diff?from=&to=`；GET `/cmdb/relationships/{relationship_id}/revisions` | 不覆盖历史；关系失效保留版本 |
| 资产创建/更新/换件/退役、关系新增/结束 | POST `/cmdb/changesets` | asset.create/update/replace/retire、relationship.upsert/end；不另开绕过审核的 CRUD 写入口 |
| 审核、发布、Diff | POST `/cmdb/changesets/{id}/submit|approve|reject|publish`；GET `/cmdb/changesets/{id}/diff` | 冻结 schema/project/asset/version；审核人与申请/发布职责分离；409并发冲突 |
| 人工说明、保存视图、数据质量 | PATCH `/cmdb/dossier/manual-fields`；GET/POST `/cmdb/dossier/saved-views`；GET `/cmdb/dossier/quality` | 说明为授权注释，不改身份与观测；保存视图按用户隔离 |
| 原抽屉及跨模块资产投影 | GET `/cmdb/dossier/assets/{id}/drawer/{tab}`、GET `/cmdb/inventory` | 保留原五/八Tab；统一ID的只读投影，其他尚未接入领域仍明确Mock |

所有路径前缀 `/api/v1`。写请求使用 Cookie Session、CSRF，ChangeSet 创建要求 Idempotency-Key。具体严格响应以 openapi.yaml 为准。领域专用快照在 MinIO，索引记录摘要、哈希、资源类型、项目和关联资产；不得作为任意对象存储浏览接口。

## 迁移、启动与数据

显式执行 `dev/backend/migrate.sh`，014–017演进既有CMDB，不重建数据库。`cmd/cmdb-seed`仅限隔离开发环境：导入既有合成数据并保留其原始ID，重复执行不重建身份。正式BOM仍由现有提交事务/Outbox发布；先按既有关联、再按唯一精确SN匹配同类型资产，禁止名称近似匹配。物料UID通过bom_item_uid关联统一资产UID，二者不必相同。

Schema 初始文件 `dev/backend/internal/cmdb/schemas/core-v1.json`；增量示例 `schemas/extensions/operations-v2.json` 由 `cmd/cmdb-schema`执行兼容性校验及历史校验后安装。核心表、外键、关系索引和授权字段固定；新字段/类型/关系经Schema扩展。

真实Adapter默认启用，`VITE_CMDB_PROVIDER=mock`可恢复既有Mock。正式BOM发布通知刷新统一资产投影；该投影不是新资产库。原来的设备操作和观测样本不会因此变成真实采集或真实执行能力。

## 本地查询预算与边界

目录每页≤100，邻接图≤120节点、展开输入≤10，设备归属递归≤8层，层级循环校验≤32层，单请求截止5秒；这些是当前实现限制，不是生产SLA。inventory为授权资产投影，达到10000条上限返回truncated，不可推定所有规模都已完整返回。大型生产资产规模需分页投影/缓存，当前不承诺万级全量首屏性能。

本轮新增真实能力是资产/关系/来源/版本/审核和BOM发布；交换机、Agent、BMC采集及监控仍只读取明确标记synthetic的已有证据，不连接设备，不放宽maintained IPMI-only约束。

## 授权与事实一致性补充

关系查询除分别校验两端资产，还校验关系所属项目；共享设备不会暴露另一未授权项目的关系和来源。CMDB Query Key包含permission_revision。ChangeSet创建和发布分别检查create/edit/delete与execute，审批权限独立；跨项目按SN复用资产时还需原资产读取及目标BOM执行授权，不能仅凭知道SN认领未授权资产。

无损证据、监控、BMC和工作流仍由各领域所有；本轮不改变已批准的采集和维护态规则。

目录列与全目录排序保存在URL的cols/sort参数，保存视图复用同一URL；快速连续操作读取最新URL以保留前次选择。设备观测缓存按用户/权限版本、项目、设备和资源隔离，返回上一设备不会误用另一设备的Mapping记录。生命周期关系两端均能打开关联ChangeSet深链。
