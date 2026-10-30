# CMDB 真实实现交付记录

开发入口：http://localhost:5174/cmdb/assets 。Go API在127.0.0.1:8081；原5180 Mock演示保留。

## 完成

- 014–017版本化迁移复用原资产、关系、BOM与Outbox；稳定ID、外键、项目/资源作用域。
- 版本化Schema Registry、受控扩展、历史兼容性/约束检查；已安装v2，包含人工字段、新资产类型和关系示例。
- 类型化关系及其历史、来源、有效期；独立规划/实际事实层；线缆双端和端口占用校验。
- 字段/关系来源、资产Revision、Diff、生命周期、质量与原始证据引用；原始证据在MinIO。
- ChangeSet草稿/审核/批准/发布/拒绝、幂等、并发版本及职责分离；换件保留旧身份和历史。
- BOM已确认数据经同事务Outbox自动沉淀；重放无重复资产，投影失败可恢复。
- React通过同形Adapter接Go；既有导航、设计令牌、组件封装及服务器五/交换机八Tab保持。
- 资产目录、关系、配置来源、生命周期、质量、原设备抽屉，以及列选择/API排序/保存视图。

## 实际检查

- `KEEL_CMDB_INTEGRATION=1 go test ./internal/cmdb -v`：通过，包括BOM重放、线缆单端删除阻断、来源权限、约束与扩展不可变性。
- `go build -o /tmp/keel-cmdb-api ./cmd/api`、`npm run build`：通过。
- 32类真实响应对OpenAPI检查：通过，见CMDB-REAL-CONTRACT-CHECK.json。
- ChangeSet和权限/换件API交互：见CMDB-REAL-BACKEND-CHECK.json。
- 五视图、五/八Tab逐个打开及受保护文件哈希：见CMDB-REAL-VALIDATION.json和ui-baselines/cmdb-before、cmdb-after。
- 浏览器搜索、关系展开到物理端口、配置对比、来源证据、备注写入/恢复及报告跳转：见CMDB-REAL-INTERACTIONS.json。
- 列选择/API排序/保存视图见CMDB-COLUMNS-CHECK.json；设备往返切换与关系目标的ChangeSet深链见CMDB-DRAWER-CACHE-CHECK.json。

## 边界

64台B300、512GPU、24交换机和16机柜继续采用原稳定ID；目录同时包含此前正式BOM新增资产。Schema扩展验收使用明确合成的传感器身份，换件记录保留。采集与硬件执行未连接真实设备，观测证据仍标记synthetic；其他领域业务执行仍为SIMULATION_ONLY。

本地请求测量见后端检查JSON：普通资产/关系/来源请求约13–88ms，授权全量投影约436ms；这是本机数据规模下的结果，不是生产SLA。生产容量、时序存储选型和真机能力未验证。现有Mock包和主bundle仍较大，构建有体积提示；本轮没有为此重构无关模块。

详细模型、路由/接口、所有权及迁移方式见CMDB-REAL-DESIGN.md。未进行全平台重复冒烟。
