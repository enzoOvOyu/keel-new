# Keel 当前交接

更新：2026-11-03。这里只保留当前事实；开发先读[模块目录](docs/README.md)，然后目标模块及API，不沿历史PRD重新做全平台。

## 工作区与版本

| 范围 | 当前事实 |
|---|---|
| 主开发 | `/home/ubuntu/keel-dev`，分支`dev`，应用目录`dev/`；文档定版已整合；本轮历史资料理解已提交dev并按用户要求发布 |
| 当前主环境 | 5180前端/8081 API，前端发布`1e5b298088b8a8d54577325d59097bb0de7c596e`；API沿用12af6e4，未重启；依据[DEPLOYMENT](docs/kickoff/DEPLOYMENT.json)，本轮已发布并完成启动、来源及数据核对接口定向检查 |
| Fabric | `/home/ubuntu/keel-fabric-dev`，`feat/fabric-rebuild@6a70041`；5197/8100分支预览，未并入dev；**用户要求暂停，等待上游接入修复通知** |
| 原型 | Kickoff5195、拓扑/机柜5181、Fabric5194仅设计参考，不是正式事实或执行来源；本次不启停服务 |
| main与旧目录 | main保留稳定基线；旧Flows/Kestra/visualization detached目录仅残留原型/运行依赖，不在那里继续开发；本次不合并或推送main |

## 智能开局持续部署授权

用户已明确要求：后续智能开局修改完成必要检查后，自动部署到现有5180/8081，无需再次等待发布指令。保留可回退发布，沿用端口、配置和存储；只改前端时不重启未改的API。此授权不等于合并main、导入业务数据、重跑AI或发布其他模块。

## 五个模块当前结果

- [Kickoff](docs/modules/kickoff.md)：9页/23表、非Agent范围发布和CMDB回执已上线；最新分页/来源/旧BOM退役也已上线。dev 新增历史资料理解报告、区域/对象分页和独立状态，已部署并通过定向真实HTTP检查；新Agent未开发，analyze503。数据核对五栏优化已上线：身份去重、独立规格列、异常影响与全量筛选；五栏及待定位筛选真实HTTP检查通过。Compact Workbench布局与10/25/50/100条分页已发布；真实MM页面只读交互检查通过。标题/主切换/右侧统计卡调整已自动发布，真实页面核对26px标准标题、统计卡位置及实体切换通过。
- [CMDB](docs/modules/cmdb.md)：统一身份、关系、版本、来源、ChangeSet及真实分页已上线；观测/采集未知不能显示健康。
- [数据中心拓扑](docs/modules/topology.md)：五工作区使用真实CMDB；最新查询/端口上下文修复已上线。缺几何/观测保留限制。
- [作业平台](docs/modules/jobs.md)：原生Kestra界面、Job Service、Agent/SSH及生命周期已上线；生产批次/NOS/规模验收未完成，ZTP事件消费者待Fabric联调。
- [Fabric](docs/modules/fabric.md)：真实输入采用/Blueprint/Profile/意图/预检持久化；没有NetworkPlan/原生Renderer/设备能力/真实回采，不能批准配置执行或NetworkReady。能力读取未提交草稿不计完成、不加载到运行API。

当前全平台仍遵守六阶段，但后续按模块授权实现和发布，不把这些剩余事项展开成一次全量开发。

## 验证和接续

[真实API入口](docs/REAL-API-CONTRACTS.md)区分源码、部署和验证。最新主环境证据记录Kickoff/CMDB/拓扑只读API及少量真实页面；建议1s/500KB摘要与所有查询2s预算未全部达到，无P95/FPS/并发或现场全链验收。

[文档整理记录](docs/DOCUMENTATION-REORGANIZATION.md)保留归档依据。本轮历史资料理解新增3个API及前端区域预览，复用原件和现有证据生成 MinIO 派生报告，不调用AI、不修改已确认输入或CMDB；Go定向测试、前端构建/类型检查、来源页协议交互回放及167项接口登记检查通过；验证及剩余字段证据缺口见[定向记录](docs/kickoff/HISTORY-UNDERSTANDING-CHECK.json)。已部署5180/8081；未迁移、未重新发布资产。本轮数据核对只修改读取投影与UI，按用户要求更新现有5180/8081，未发布资产或更换端口/存储配置；定向Go测试、最终前端构建、五栏合成协议交互与视觉核对通过；结果见[核对优化记录](docs/kickoff/REVIEW-IMPROVEMENT-CHECK.json)。最新Compact Workbench调整仅涉及Review页面、作用域样式和DataTable可选分页控件，没有修改API/模型/后端；证据见[紧凑布局检查](docs/kickoff/REVIEW-COMPACT-CHECK.json)，前端构建、四档分页及五栏定向协议交互、1440/900px视觉检查通过；前端已发布1e5b298，实际MM页面默认10条/切换25条、60px行高、无侧栏溢出检查通过；API未重启。待办在模块页与[未决项](docs/DECISIONS.md)维护。

后续修改只读目标模块及调用链；完成后替换当前快照，不再向HANDOFF堆叠里程碑。历史全文在[归档](docs/archive/2026-11-03/previous/HANDOFF.md)。
