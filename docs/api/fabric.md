# Fabric API · 首个纵向切片

权威Schema：docs/openapi.yaml。UI原型基准：dev/prototypes/fabric-studio；FABRIC-DESIGN-CONTRACT.md尚未在本机找到，未宣称通过该文档验收。

## 已实现的读取能力

GET /api/v1/projects/{projectId}/fabric/inputs，operationId=getFabricPublishedInputs。

- Session鉴权，项目级fabric:read与cmdb:read均必需。
- 只消费新Kickoff的kickoff_publication（state=published）、publication_object及publication_input；不调用创建工作区的上游GET，不重导BOM，不运行Agent，不写任何表。
- 读取选定发布范围的授权CMDB实体、当前资产版本、来源及已保存NetworkPlan。未完成回执的ID明确列入missingAssetIds，不补造资产。
- 显式区分WAITING_FOR_KICKOFF（上游发布迁移未部署）、EMPTY（没有已发布版本）、AVAILABLE。不会读取fixture替代正式事实。
- before游标，按发布revision倒序，每页最多20个发布记录。nextCursor为下一页before值。
- digest标识publication/model/input版本及当前CMDB资产版本向量；不是全局CMDB修订号。assets是当前CMDB投影，不把它伪装成发布时硬件观测快照。
- 403项目/权限拒绝；500读取失败。无设备、配置或审批副作用；无新数据库实体。

## Provider边界

Mock与Http都向页面提供同一FabricFixture/FabricState和领域DTO，页面不读取fixture或直接fetch。网络传输DTO FabricInputPage/FabricPublishedInput由OpenAPI生成；Http Adapter只投影，不猜测NOS、Plane、逻辑边或观测值。

VITE_FABRIC_PROVIDER=mock只用于隔离的交互/视觉验收；数据明确SIMULATION_ONLY，使用独立sessionStorage项目键，身份取主应用Session，不展示模拟身份切换器。默认real只读已发布输入。真实配置采用/Blueprint写入、编译、ZTP、校验、审批、部署、回采和回滚未实现时明确返回不可用，不回退模拟执行。

真实读模型依赖另一窗口开发中的Kickoff publication契约及迁移049；本分支没有复制其未提交代码，没有对在线库应用该迁移。实际发布/联调必须先同步并部署该依赖。

## 后续切片，尚未实现

1. 输入采用及Blueprint修订持久化，冻结发布ID、目标集合和版本向量。
2. Profile/能力匹配、逐设备编译与产物。
3. 验证、NE+DE双签、不可变目标/产物/Flow版本绑定。
4. 复用现有Flow/Execution，经Fabric受控Adapter进行备份/下发/回采。
5. 漂移、回滚与限定范围Network Ready。

不新建CMDB、Kickoff、Execution或Monitor事实库；不放宽SWITCH_CHANGE_REQUIRES_APPROVED_FABRIC_ADAPTER。Critical/NH及维护态限制保持原规则。
