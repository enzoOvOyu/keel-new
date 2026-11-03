> 分支快照：`feat/fabric-rebuild@6a70041`，2026-11-03；未合入当前dev，不能向5180/8081假定存在这些接口。来源为该提交的 `docs/api/fabric.md`。暂停事项见[模块页](../modules/fabric.md)。本文件不在dev真实API注册表中冒充已部署契约。

# Fabric 真实 API · 已发布输入与持久化切片

机器可读权威：[该分支OpenAPI](https://github.com/enzoOvOyu/keel-new/blob/6a70041/docs/openapi.yaml)。原型与集成依据：[CONTRACTS](../fabric-provisioning/CONTRACTS.txt)、[IMPLEMENTATION](../fabric-provisioning/IMPLEMENTATION.txt)。设计不由旧 Mock 代码反向定义。

## 当前实现

主应用 Fabric 仅使用真实 Http Provider，没有 Mock 开关、fixture 回退、浏览器业务状态存储或模拟结果注入。独立 5194 原型不作为生产数据来源。

|接口|用途|权限|副作用|
|---|---|---|---|
|GET /projects/{projectId}/fabric/inputs|已发布 Kickoff 范围、版本向量、授权 CMDB 回执、已有 NetworkPlan|fabric:read + cmdb:read，且项目在会话授权列表|无|
|GET /projects/{projectId}/fabric/workspace|持久化工作区、输入、既有 Flow 任务能力与执行投影|同上|无|
|POST /projects/{projectId}/fabric/commands|版本化 Fabric 领域操作|同上；另按操作校验 edit/execute/approval 权限|仅 Fabric 状态、既有审批和审计；不新增资产|

请求和响应为资源专用 FabricStudio* / FabricWorkspace Schema，前端传输使用生成类型。错误遵循既有 error.code 格式。

## 作用域、并发与重试

- 项目从路由取值，不接受跨项目 body 覆盖。管理员也不能访问不存在或未出现在会话授权列表中的项目。
- 读取仅限 state=published 的新 Kickoff publication 与 CMDB 授权回执。按稳定 ID 去重；同设备同时作为 device/placement 发布不能重复计数。
- inputs 的 before 游标每页最多 20 个发布记录。assets 是当前授权 CMDB 投影，assetVersions/digest 不是发布时硬件观测，也不是全局 CMDB 修订。
- POST 必带 Idempotency-Key 与 expectedRevision；数据库锁、命令回执、工作区更新和审计同事务提交。同 key/同请求重放不重复变更；异请求拒绝；旧版本 409。
- 采用输入冻结发布引用和资产版本摘要。CMDB 事实变化后旧摘要阻断后续操作，必须重新采用；历史版本保留。在途操作不允许切换输入。
- 页面按需加载；存在在途执行时每 5 秒读取既有 Execution 状态，无客户端计时推进。

## 已落地命令

adopt、commitBlueprint、reviseBlueprint、revertBlueprint、publishProfile、compile、validate、createChange、submit、approve、advance、control、collect、ready 具备服务端路径及门禁。命令存在不表示所有前置条件已经满足。

- 输入采用、Blueprint、Profile 修订、逐目标结构化意图、差异与预检报告保存 PostgreSQL（迁移 052）。不修改 CMDB 资产或上游规划。
- compile 当前生成 application/vnd.keel.fabric-intent+json；内容、源引用及 SHA256 真实保存。它不是可执行 NVUE 配置。没有已发布 NetworkPlan 时明确缺失，绝不从设备名推断 IP/ASN/VLAN/QoS。
- 当前没有原生 NOS Renderer、精确设备能力及独立观测适配器。Build=INVALID；必需证据缺失/不支持使预检 BLOCKED。不能提交变更、批准下发或生成 READY。
- ChangeSet 只能基于当前有效可执行 Build/预检创建。任务目录从现有 Flow 定义读取 variables.keel_fabric 元数据；没有能力声明、精确修订或超时不能执行。不造 Task/Execution。
- 审批复用 approval_record 和原有签署 API：NE + DE；申请人不得自签；Critical/NH 不降级。approve 命令只是同步统一审批状态，不能凭 Fabric 请求直接批准。
- advance/control 读取/控制已绑定的真实 Execution；不生成模拟日志和输出。没有已绑定执行时拒绝。
- collect 当前仅保存证据缺口核对报告，observedAt=null、状态 UNKNOWN；没有真正发起设备采集。界面不可将其解释为成功回采。

## 明确未完成的设备适配

ZTP 发现/引导、型号/NOS/ASIC 能力采集、厂商命令 Renderer、批准的逐设备配置执行 Adapter、任务修订完整执行绑定、配置事件到CMDB的Outbox关联、真实回采/漂移与回滚尚未实现。相应路径返回 FABRIC_*_REQUIRED / NOT_CONFIGURED，不能通过传入场景参数获得成功。

不放宽现有 SWITCH_CHANGE_REQUIRES_APPROVED_FABRIC_ADAPTER，不改变 maintained 通道限制，不重复建设 CMDB、Kickoff、作业引擎或 Monitor。现有环境 NetworkPlan 为空；真实配置链路还需要上游发布该事实及后续适配切片。

## 环境与证据

仅 Fabric 分支预览：前端 5197 → Go 8100；后台通用 Worker 显式关闭，避免与 8081 抢任务。迁移 052 为增量新表，已应用开发库；未修改已发布资产和 5180/8081 服务。上线仍需单独发布，不把分支预览当作现网发布。

定向检查脚本：dev/scripts/check-fabric-real.mjs，真实 Session/API/PostgreSQL，无路由拦截和设备执行。实际结果及限制见 [集成记录](../fabric-provisioning/INTEGRATION.md)。
