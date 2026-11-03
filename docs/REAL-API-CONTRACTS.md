# 当前真实 API 入口

更新：2026-11-03。主环境前端`3c0ac07`，API沿用`fdd86dd`，发布证据[DEPLOYMENT](kickoff/DEPLOYMENT.json)。精确字段/请求/响应/枚举以[本分支OpenAPI](openapi.yaml)为准；实现、部署及验证按[机器注册表](api/real-api-registry.json)逐项查询。

| 模块 | 当前语义契约 | 登记范围 | 真实性边界 |
|---|---|---:|---|
| Kickoff | [kickoff](api/kickoff.md) | 16 | 非Agent闭环已部署；包含明确返回503的analyze入口，不能计为AI能力 |
| CMDB | [cmdb](api/cmdb.md) | 29 | 统一资产/关系/Schema/ChangeSet；新增27项为源码补登记，未逐项重验HTTP |
| 拓扑 | [visualization](api/visualization.md) | 5 | 已部署真实查询；partial、unknown及trace未验证状态不得变PASS |
| 作业平台 | [flows](api/flows.md) | 70 | 已部署；引擎/本地Agent/协议仿真/UI与生产设备验收分开 |
| 保留工程后端 | [engineering](api/engineering.md) | 47 | 旧数据/服务仍供复用；旧前端退役，不恢复旧五步产品 |
| Fabric | [分支契约](api/fabric.md) | 不计入dev表 | 6a70041独立分支，3个领域接口；未合入dev或部署主环境 |

合计167项登记不是全平台完整接口数，也不是167项验收通过。未登记不代表没有代码；OpenAPI里早期设计端点不代表已实现。新增登记从现有Handler/Service和Schema交叉定位，没有变更协议或服务。

## 接入前只核对这几项

1. 明确目标分支和运行环境，选择同版本OpenAPI与生成类型。不能拿Fabric分支接口调用主环境。
2. 查operationId及Handler/Source/Adapter，确认作用域、权限、加载/缓存、错误、副作用、当前限制。
3. 查该操作及模块的部署/verification_evidence，注意只读启动、模拟回放和真实业务写入的差别。
4. 如协议与实现/当前要求不一致，登记[差异](DECISIONS.md)，在后续受影响模块任务中统一修正；本次不擅自修改代码或OpenAPI。

## 更新与检查

按[模块维护规则](MODULE-WORKFLOW.md)在相关变更中同步。历史operation的deployed_revision可能表示首次专项证据版本，主环境最新版本看表顶发布记录，不静默将旧验收升级到当前版本。

`node dev/scripts/check-real-api-docs.mjs`核对operation/Schema/源码/文档引用及部署revision的一致性；不调用HTTP、数据库或AI，不证明运行行为。文档整理的检查结果见[整理记录](DOCUMENTATION-REORGANIZATION.md)。
