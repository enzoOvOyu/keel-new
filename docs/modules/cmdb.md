# CMDB · 当前实现定版

统一资产与关系服务已实现上线；当前主环境 `686e9ea` 含库存分页、工程来源和未知状态展示整改。不是设备采集平台，也不保存所有领域原始日志。

## 产品与交互

`/cmdb` 提供五视图：资产目录、资产关系、配置与来源、生命周期、数据质量。机房→区域/Pod→机柜→设备树与类型分类共用稳定身份；搜索、筛选、排序、分页、保存视图、URL 返回保留上下文。

服务器五 Tab、交换机八 Tab 复用唯一 DrawerDetail；非设备对象在工作区展开，不能叠加多层设备抽屉。跨项目读取只在授权范围，清空过滤不等于全平台权限。来源支持对象/字段证据与版本 Diff，缺采集保持 unknown。

## 模型与写入

- PostgreSQL `cmdb_entity(item_uid)`、`cmdb_asset_scope`、`cmdb_relationship` 为统一资产身份、范围及关系。设备、组件、物理端口、Cable、Rack 不在业务模块复制身份。
- 固定核心字段/关系配合版本化 Schema 扩展；类型、属性约束、基数、所有权可演进，不提供任意动态数据库或图数据库。
- 位置、组成、物理、逻辑、业务关联分层；baseline/intended/applied/operational 分开。LLDP 是观测，不能覆盖计划 Cable；unknown 不是健康/已维护/零值。
- 工程发布以 Outbox/幂等服务沉淀资产及关系；保留 `bom_item` 与工程事实所有权。人工只改允许的 manual 字段，不覆盖上游身份/观测。
- 资产/关系正式变更走 ChangeSet：Draft → Review → Approved → Published，或 Rejected。冻结项目/目标/Schema/expected_revision，审批与申请职责分离，发布再次校验。
- 换件创建新身份、结束旧安装关系；退役和关系失效保留历史。原件/配置/日志在所属领域或对象存储，CMDB 只保留受控引用与摘要。

## 读取与集成

目录、详情、关系、来源和历史按需查询；inventory 是既有类型的兼容只读投影，不等于目录所有类型。已实现稳定 ID 游标分页，默认1000、上限2000，`has_more/truncated` 表示还有数据，不能显示为全量。

当前 MM 目录22492对象，兼容库存22488个适用对象；差额来自通用类型/项目锚点等投影范围，不能靠补造资产使数字相等。工程来源可追溯旧 eng_publication 与新 kickoff_publication；reviewed_model 是对象级审核依据，不能称为独立字段证据。旧来源在读取时链接新 Kickoff，不改原审计。

[API 使用契约](../api/cmdb.md)记录授权、写入、加载/错误；[拓扑 API](../api/visualization.md)只读相同事实。精确字段在 [OpenAPI](../openapi.yaml)。

代码：[HTTP](../../dev/backend/cmd/api/cmdb.go)、[领域服务](../../dev/backend/internal/cmdb)、[真实 Adapter](../../dev/src/lib/api/cmdb-http.ts)、[前端入口](../../dev/src/lib/api/cmdb.ts)。相关基础迁移014–017；后续按已有版本继续，不重新建库。

固定B300演示切片沿用已确认GPU1–8对应NIC1–8，NIC9/10为存储；配对规则与线序/LLDP新鲜度分别判断，不外推到未确认的真实设备。

## 验证与限制

[定向后端记录](../CMDB-REAL-BACKEND-CHECK.json)证明当时身份/关系、来源、项目隔离、ChangeSet/职责分离、换件及 Schema 扩展检查；[当前分页与来源记录](../kickoff/CMDB-PERFORMANCE-CHECK.json)、[最新部署](../kickoff/DEPLOYMENT.json)证明相应只读整改。旧合成记录不等于现场采集。

不宣称全量生产规模、BMC/交换机/Agent 观测已接通。维护态仍 IPMI-only。OpenAPI 中早期 `/cmdb/entities*` 等旧设计不能作为当前可用接口，见 [DECISIONS](../DECISIONS.md)；本次只记录差异，不删除协议或更改代码。
