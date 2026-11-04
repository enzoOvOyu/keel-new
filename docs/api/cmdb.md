# CMDB · 当前 API 使用契约

基准：dev/部署 `686e9ea`。职责见[模块定版](../modules/cmdb.md)；字段以[OpenAPI](../openapi.yaml)为准。下表仅索引实际cmdbRoutes/领域服务，不将早期设计端点认作可用。

## 操作索引

| 方法 | 路径（省略 `/api/v1`） | operationId |
|---|---|---|
| GET | `/cmdb/inventory` | `getCanonicalCMDBInventory` |
| GET | `/cmdb/dossier/assets/{asset_id}/provenance` | `getCmdbDossierprovenance` |
| GET | `/cmdb/dossier/catalog` | `queryCmdbDossierCatalog` |
| GET | `/cmdb/dossier/tree` | `getCmdbDossierTree` |
| GET | `/cmdb/dossier/relations` | `getCmdbDossierRelations` |
| GET | `/cmdb/dossier/assets/{asset_id}/configuration` | `getCmdbDossierconfiguration` |
| GET | `/cmdb/dossier/assets/{asset_id}/lifecycle` | `getCmdbDossierlifecycle` |
| GET | `/cmdb/dossier/quality` | `getCmdbDossierQuality` |
| PATCH | `/cmdb/dossier/manual-fields` | `patchCmdbManualFields` |
| GET | `/cmdb/dossier/saved-views` | `listCmdbSavedViews` |
| POST | `/cmdb/dossier/saved-views` | `createCmdbSavedView` |
| GET | `/cmdb/dossier/evidence/{resource}/{record_id}` | `getCmdbDossierEvidence` |
| GET | `/cmdb/schemas` | `getCMDBRegistry` |
| POST | `/cmdb/schemas` | `installCMDBSchema` |
| POST | `/cmdb/schemas/validate` | `validateCMDBSchema` |
| GET | `/cmdb/schemas/versions` | `listCMDBSchemaVersions` |
| GET | `/cmdb/schemas/{schema_version}` | `getCMDBSchemaVersion` |
| GET | `/cmdb/dossier/assets/{asset_id}` | `getCMDBDossierAsset` |
| GET | `/cmdb/dossier/assets/{asset_id}/drawer/{tab}` | `getCMDBDeviceDrawer` |
| GET | `/cmdb/dossier/assets/{asset_id}/revisions` | `listCMDBAssetRevisions` |
| GET | `/cmdb/dossier/assets/{asset_id}/diff` | `diffCMDBAssetRevisions` |
| GET | `/cmdb/changesets` | `listCMDBChangeSets` |
| POST | `/cmdb/changesets` | `createCMDBChangeSet` |
| GET | `/cmdb/changesets/{changeset_id}/diff` | `previewCMDBChangeSet` |
| POST | `/cmdb/changesets/{changeset_id}/submit` | `submitCMDBChangeSet` |
| POST | `/cmdb/changesets/{changeset_id}/approve` | `approveCMDBChangeSet` |
| POST | `/cmdb/changesets/{changeset_id}/reject` | `rejectCMDBChangeSet` |
| POST | `/cmdb/changesets/{changeset_id}/publish` | `publishCMDBChangeSet` |
| GET | `/cmdb/relationships/{relationship_id}/revisions` | `listCMDBRelationshipRevisions` |

可视化五接口另见[visualization](visualization.md)；旧`published-assets`属于保留工程/BOM发布投影，不是当前Kickoff回执的替代。

## 作用域和写入权限

Session + `cmdb:read`；project_id省略只读取获授权范围。资产及关系双端/关系所属项目再次授权；浏览器Query Key包括permission_revision。写操作CSRF保持统一。

ChangeSet创建/提交复核create/edit/delete等逐操作授权，批准/拒绝需cmdb:approve，发布需cmdb:execute；Schema安装/校验需admin:edit。manual-fields只改授权说明，不绕过来源所有权更新设备身份/观测。保存视图限当前用户和可访问项目。

正式资产写入通过ChangeSet，创建带Idempotency-Key，后续动作带expected_revision；审核人与申请人分离，发布重新验证Schema/来源/版本/Cable双端。同key重放不能重复写资产；失败保留可追溯状态。Schema历史与资产/关系历史不可覆盖。

## 查询/响应与副作用

目录、树、关系、详情、来源、配置、生命周期、质量、版本、抽屉、证据均按需查询。各operation的envelope不完全同形，Adapter按生成类型读取，不能统一猜测为裸数组。

inventory默认1000、每页1–2000，游标按稳定ID，返回has_more/truncated；旧“固定10000完整装载”已废弃。目录limit≤100，关系节点≤120/展开输入≤10；这些是当前查询限制，不是生产SLA。可视化分页是另一组专用契约，不能混用cursor。

GET只读；POST Schema安装/ChangeSet/action、PATCH说明、POST保存视图分别变更其领域记录及适用审计，不触发设备操作。证据读取经授权索引访问对象存储，不能作为任意对象下载接口。

## 错误/限制与证据

400非法参数、403项目/对象/动作权限拒绝、404对象/证据不可用、409并发修订冲突、422Schema/来源/业务约束；具体code和payload以实际operation为准。5秒查询期限，不循环重试或把截断结果冒充完整。

历史[后端定向记录](../CMDB-REAL-BACKEND-CHECK.json)覆盖资产/关系/Schema/ChangeSet典型场景，最新[部署记录](../kickoff/DEPLOYMENT.json)只覆盖目录与inventory等读取。此次新增登记只核对源码路由及服务，不声称对每个接口重新运行验收，也不声称字段一致性审计全部完成。

早期`/cmdb/entities*`/`/cmdb/events*`仍存在OpenAPI设计声明，但当前cmdbRoutes没有对应分支，统一未知资源处理；见[差异](../DECISIONS.md)。不删协议、不新增兼容路由来掩盖这一事实。

## 工程发布批量写入

工程Outbox投影采用本次任务内的批量资产、字段来源、关系及版本写入；保持同事务和现有稳定身份，不改变HTTP契约。单对象编辑继续使用原接口；规划与Observed/人工事实分层。阶段日志记录SQL数量和耗时，详见[实施记录](../kickoff/CMDB-WRITE-PERFORMANCE-IMPLEMENTATION.md)。来源公共元数据拆表及发布版本切换未实施。
