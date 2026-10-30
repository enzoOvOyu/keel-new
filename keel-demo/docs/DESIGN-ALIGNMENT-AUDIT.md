# 真实开发前设计一致性核查

核查基点：`221c2ea`；核查范围是当前仓库设计、OpenAPI、IA registry 与 Mock 路由映射，不是全平台业务验收。已实现的页面、Provider、fixture、运行配置均未改动。

## 结论

当前发现的可确定文档冲突已修正；未发现需要修改已实现功能或 HTTP 载荷才能解决的冲突。OpenAPI 保持原样。不能据此宣称全部真实后端、设备适配和生产策略已准备完毕；下述真实开发前置仍按原决策执行。

## 修正与保留

| 核查对象 | 结果与处理 |
|---|---|
| PRD FR-0…28 | 原规则与编号保留；六阶段、独立BOM、Topology、无损证据、CMDB档案均已有定义，不重写PRD |
| IA v2.4 | 主表移出Provision中的BOM，补独立入口、Topology和CMDB五视图；保持Fleet/Fabric/Flows各一个主入口 |
| 当前路由/加载矩阵 | 保持registry页面路径/权限；更新陈旧验证说明、无损抽屉描述与新增领域精确API链接 |
| 项目与抽屉 | 项目路径绑定、跨项目授权过滤、五/八Tab、按需加载及稳定身份规则一致，原文保留 |
| 六阶段/NP/维护态 | 六阶段枚举与PRD相同，旧gate接口deprecated；NP仅网络断言；maintained仍IPMI-only，不替产品决定开放Redfish |
| OpenAPI | 原样保留；全部操作索引见API-OPERATION-INDEX。无断裂本地引用、operationId重复、缺失必需路径参数或GenericObject引用 |
| 文档目录/架构 | 修正旧fixture目录、web/src/mocks实际路径误导；旧web规划明确为规划，禁止因此搬动现有工程 |
| 通用API说明 | 分页上限及时间/版本字段按资源专用Schema，不把通用示例强加到CMDB投影或证据响应 |
| 实施/决策 | D-004已由全量契约先行决策取代；历史设计阶段限制与专项验证范围明确区分 |
| 双文档副本 | 根docs和keel-demo/docs同名Markdown与OpenAPI一致；不触碰历史JSON验收事实 |

## 业务域对照

| 业务域 | 权威需求/领域定义 | IA/契约入口 |
|---|---|---|
| 项目/阶段 | PRD FR-25、DELIVERY-LIFECYCLE | overview、BOM独立菜单；DeliveryStage* |
| BOM/装机 | PRD FR-2、EXCEL-IMPORT-PROFILE | bom / provision；BOM与操作专用schema |
| 验机/NetTest/NCCL/Cleanup | PRD、DELIVERY-LIFECYCLE | validation / nettest / nccl / cleanup；子任务和逐目标证据 |
| Fabric/Config | PRD FR-0、DOMAIN-API-CONTRACTS | fabric / config；基线、Doublecheck、PushJob及观测 |
| 拓扑 | PRD FR-26、FABRIC-TOPOLOGY | fabric/topology；physical / logical / details / filters |
| 无损 | PRD FR-27、LOSSLESS-EVIDENCE | fabric/lossless及网络抽屉；Lossless*证据、结论、采集与复验 |
| 机柜 | RACK-DIAGRAM、RACK-WORKSPACE | racks全局和单柜；Mapping为所选设备关联，GPU-NIC矩阵保留拓扑职责 |
| Fleet/Monitor/Issues | PRD FR-18…22、OPERATIONS-DESIGN | 统一抽屉、观测与工单闭环；Ops/IA专用schema |
| Flow/Execution | PRD FR-1、SYSTEM-DESIGN | flows；Flow、Execution、目标/日志/产物与生命周期 |
| CMDB | PRD FR-28、CMDB-DOSSIER | 五主视图与项目资料；CmdbDossier* |
| AI/系统管理 | PRD、IA-V2.4、DOMAIN-API-CONTRACTS | ai / admin；模型配置、用户权限、审计及治理契约 |

## 真实开发前置与尚未验证事项

- API当前定义不等于HTTP实现。通用Mock Provider仍有同步资源读取/写入，不能仅更换base URL就得到异步Go客户端；逐模块实现HTTP Adapter与错误/取消/并发处理，保持页面业务行为。此次不动代码。
- 真实事务、Outbox、幂等、RBAC、SSE恢复/权限重验、数据库迁移和设备副作用对账需后续真实集成验收；Mock状态不构成其证明。
- IA24-02：未提供真实Excel；33 Sheet结构元数据不能代替真实解析测试，敏感字段处理不能伪造验证。
- IA24-03/D-07：Kestra集成、真实AI与凭据流程仍待确认；不阻塞无设备的基础API实现。
- OPS-P01…04：生产观测存储、保留与恢复策略、适用阈值/能力确认保留待决。不能将演示参数直接升为生产默认。
- 原生NVUE字段/解析器、NOS/ASIC版本认证仍未接入；现有无损反例是synthetic结构化Mock证据。独立Intended缺失保持unknown。
- IA24-01如未来要开放维护态Redfish须另行明确批准；当前安全规则已经明确为IPMI-only，本轮不放宽。

## 实际检查

结构性检查：227路径、264操作、512 Schema；本地引用解析、operationId唯一性、路径参数必需声明、Mock路由映射、六阶段枚举、旧gate废弃标记、Markdown本地链接与文档镜像均通过。未改API Schema或运行代码，不重复生产构建/浏览器回归/规则测试。本次结果不能代替已有专项报告，也不将其扩大成全平台验收。
