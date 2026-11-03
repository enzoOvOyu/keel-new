# Fabric · 当前切片与保留设计

**状态：开发中且暂停。** 已提交 `feat/fabric-rebuild@6a70041`；工作区 `/home/ubuntu/keel-fabric-dev`。用户要求等上游设备接入问题修复通知后再继续。分支5197/8100仅预览，未合入dev、未替换主环境5180/8081。

本页冻结“已实现切片”，下方保留未实现设计；不把开发目标当成完成。该分支 `HANDOFF.md` 中能力读取草稿未提交、未编译、未补契约，不纳入定版。

## 产品目标与保留的完整设计

从智能开局正式发布数据生成Fabric配置交付，复用CMDB、作业平台和Monitor，不另建资产/调度/监控系统。

Input → Blueprint → Onboarding → Configuration → Validation/ChangeSet → Deployment → Assurance。

八工作区保留：Overview、Blueprint、Onboarding、Configuration、Validation、Deployments、Assurance、Settings。真实分支路由 `/p/:projectId/deploy/fabric/:workspace`。原型 `/p/:projectId/fabric/*` 路径不是正式路由。

完整原型需求和未开发设计继续保留在 [DESIGN](../fabric-provisioning/DESIGN.txt)、[CONTRACTS](../fabric-provisioning/CONTRACTS.txt)、[IMPLEMENTATION](../fabric-provisioning/IMPLEMENTATION.txt)、[参考研究](../fabric-provisioning/REFERENCES.txt)、[生命周期图](../fabric-provisioning/lifecycle.mmd)。它们中的Mock实现只描述独立5194原型；真实切片依据下表和[分支API](../api/fabric.md)，不恢复localStorage/fixture回退。

## 已实现结果与后续边界

| 能力 | 6a70041 已实现 | 保留待开发部分 |
|---|---|---|
| 输入 | 读取published Kickoff范围及授权CMDB回执，稳定ID去重，采用后冻结版本摘要 | 完整已发布NetworkPlan/字段来源、正式能力证据；当前环境无NetworkPlan |
| Blueprint | 八页移植统一Shell，拓扑来自实际资产/端口/Cable，修订/提交/回退持久化 | 已确认角色、任意Plane/Tier/冗余域逻辑模型，不能从设备名/线缆推导 |
| 纳管 | 真实数据页及缺口表达 | DHCP/引导/身份精确匹配、NOS适配ZTP、失败证据恢复；Day-0不等于生产FabricReady |
| 配置 | Profile、逐目标结构化意图/摘要/来源/Diff入PostgreSQL | 精确型号/NOS/ASIC能力、可执行NVUE/Host/DMS Renderer；当前intent JSON不可下发，Build INVALID |
| 校验/变更 | 版本化预检、证据缺失阻断、既有统一审批路径 | 完整七层校验、拓扑影响/冗余安全、可执行产物和任务绑定；命令存在不代表允许执行 |
| 部署 | 读取既有Flow目录及Execution、服务端门禁 | 获准Fabric任务、精确Flow修订和输入Schema、批次与依赖、安全执行/恢复/回滚Adapter |
| 验收 | 保存证据缺口核对，observedAt=null、UNKNOWN | 独立设备回采、四态一致性、漂移与修复变更、真正NetworkReady/交付证据 |

统一模型/设备执行分离：Switch使用NVUE适配；Host独立DOCA/NIC/PF/VF/eSwitch/GDR/SPCX-CC等配置域；DMS保留Prepare/Configure及所需重启/验证流程。Plane数/硬件Profile来自项目，不固定示例B300或4Plane；OpenConfig不能代表全部厂商能力。

## 领域约束

InputSnapshot绑定发布/CMDB/物理/网络版本；Blueprint、Profile、能力快照、Build/Artifact、Validation、ChangeSet、Rollout/Execution、Observed/NetworkReady保留来源和历史。正式DTO以**该分支**OpenAPI为准，不将原型DTO复制成第二份协议。

上游变化使受影响下游STALE；审批后配置/目标变化重新校验审批。在途执行不切输入。PASS/WARNING/BLOCKED/UNSUPPORTED/NOT_RUN/STALE分开；任务SUCCESS、配置应用成功和运行验证成功分开。

审批复用现有记录/签署；当前分支NE+DE、申请人不自签、Critical/NH未决不降级。没有任务声明/设备能力/可执行产物时阻断，不造Flow或执行日志。修复漂移也须新ChangeSet，不自动改设备。

## 接续、证据与依赖

[分支 API 契约](../api/fabric.md)给出固定提交来源、权限/版本/幂等/真实限制。迁移052仅Fabric工作区及命令回执；据分支记录已用于开发库，不由本文授权再次迁移。

分支报告记录4项Go检查、真实Session/API/PostgreSQL与8页读取；不是设备执行验收。旧 `fabric-provisioning/integration/result.json` 在分支中是合成拦截历史，不当真实API证据。

恢复顺序：收到用户修复通知 → 核对最新Kickoff身份/发布/观测契约 → 同步明确依赖 → 确认一个下一切片 → 更新该分支OpenAPI/Adapter/注册表 → 按用户节奏验证。任务由用户授权选择，本次不自行启动开发、测试或部署。
