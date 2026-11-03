# Keel 当前交接

更新：2026-11-03。这里只保留当前事实；开发先读[模块目录](docs/README.md)，然后目标模块及API，不沿历史PRD重新做全平台。

## 工作区与版本

| 范围 | 当前事实 |
|---|---|
| 主开发 | `/home/ubuntu/keel-dev`，分支`dev`，应用目录`dev/`；本轮文档定版通过`docs/module-current`整合到dev；不改变运行版本 |
| 当前主环境 | 5180前端/8081 API，固定发布`686e9ead0c977d29baeabbbe1c082965ba77e312`；依据[DEPLOYMENT](docs/kickoff/DEPLOYMENT.json)，本次没有重新部署或运行探测 |
| Fabric | `/home/ubuntu/keel-fabric-dev`，`feat/fabric-rebuild@6a70041`；5197/8100分支预览，未并入dev；**用户要求暂停，等待上游接入修复通知** |
| 原型 | Kickoff5195、拓扑/机柜5181、Fabric5194仅设计参考，不是正式事实或执行来源；本次不启停服务 |
| main与旧目录 | main保留稳定基线；旧Flows/Kestra/visualization detached目录仅残留原型/运行依赖，不在那里继续开发；本次不合并或推送main |

## 五个模块当前结果

- [Kickoff](docs/modules/kickoff.md)：9页/23表、非Agent范围发布和CMDB回执已上线；最新分页/来源/旧BOM退役也已上线。新Agent未开发，analyze503。
- [CMDB](docs/modules/cmdb.md)：统一身份、关系、版本、来源、ChangeSet及真实分页已上线；观测/采集未知不能显示健康。
- [数据中心拓扑](docs/modules/topology.md)：五工作区使用真实CMDB；最新查询/端口上下文修复已上线。缺几何/观测保留限制。
- [作业平台](docs/modules/jobs.md)：原生Kestra界面、Job Service、Agent/SSH及生命周期已上线；生产批次/NOS/规模验收未完成，ZTP事件消费者待Fabric联调。
- [Fabric](docs/modules/fabric.md)：真实输入采用/Blueprint/Profile/意图/预检持久化；没有NetworkPlan/原生Renderer/设备能力/真实回采，不能批准配置执行或NetworkReady。能力读取未提交草稿不计完成、不加载到运行API。

当前全平台仍遵守六阶段，但后续按模块授权实现和发布，不把这些剩余事项展开成一次全量开发。

## 验证和接续

[真实API入口](docs/REAL-API-CONTRACTS.md)区分源码、部署和验证。最新主环境证据记录Kickoff/CMDB/拓扑只读API及少量真实页面；建议1s/500KB摘要与所有查询2s预算未全部达到，无P95/FPS/并发或现场全链验收。

[本次文档整理记录](docs/DOCUMENTATION-REORGANIZATION.md)记录归档与文档检查；本轮仅文档，未改代码/OpenAPI/SDK、未跑应用测试、未迁移/导入数据。待办只在各模块页与[未决项](docs/DECISIONS.md)维护。

后续修改只读目标模块及调用链；完成后替换当前快照，不再向HANDOFF堆叠里程碑。历史全文在[归档](docs/archive/2026-11-03/previous/HANDOFF.md)。
