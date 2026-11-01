# 智能开局工程真实接口

更新：2026-11-01。S1代码已实现；当前环境未部署。所有路径使用 `/api/v1` 前缀及现有身份Session、项目授权和CSRF。精确DTO以 `openapi.yaml` 的 `IDE*` 为准。

统一前缀 `/projects/{projectId}/engineering-imports`，下表 `{id}` 为工程来源ID，不是旧BOM批次ID。

| 方法与后缀 | 作用 | 权限 |
|---|---|---|
| GET / | 最近50项任务 | bom:read |
| POST / | 关联已解析的原始BOM文件，重复关联返回已有工程任务 | bom:import |
| GET /{id} | 摘要、模块适用性、校验、目标状态、任务和最近40条工具记录 | bom:read |
| POST /{id}/analyze | 对指定版本排队分析，返回202；同来源只允许一个活动任务 | bom:edit |
| GET /{id}/model | 当前或指定历史版本的类型化模型及单元格证据 | bom:read |
| PUT /{id}/model | 人工修订并创建新版本，需修改依据 | bom:edit |
| POST /{id}/review | 精确版本、摘要的逐目标批准/驳回 | bom:review |
| POST /{id}/publish | 入事务Outbox；要求已批准及Idempotency-Key，返回202 | bom:execute |
| GET /{id}/revisions | 不可变版本记录 | bom:read |
| GET /{id}/diff | 指定历史版本与当前版本的差异 | bom:read |
| GET /{id}/source | 本地来源查看；每次最多32行64列，过滤敏感单元格 | bom:read |

首批领域为EquipmentPlan、DeviceAssembly、FacilityModel、RackPlacement，分别以明确表和关系存储。`eng_revision.proposal`仅保存AI区域/字段建议协议，不代替工程领域表。原文件继续复用不可变对象和BOM解析结果。

AI只调用资料工具并提出区域、字段、模块建议。严格Pydantic Schema和本地确定性提取独立于模型；不按固定Sheet名、型号、机柜U数解析。缺失尺寸保持空值并记录问题；未知资料保留。公式缓存与重新计算结果区分。P3已接入隔离Calc和受限规则表达式；未配置Provider时明确返回不可用，不回退到缓存。

审核目标为CMDB_PLANNED_EQUIPMENT、CMDB_PLANNED_FACILITY、CMDB_PLANNED_PLACEMENT，各有状态、版本、摘要、审核和回执。设备数量组不能当成实例发布，已有实物引用须关联本项目CMDB身份。落位发布依赖设备和场地成功发布或同批排队。BLOCK仅阻断受影响目标。人工确认/已审核数据在重新分析时保留，冲突要求人工修订。

`expected_revision`冲突、活动分析或重复在途发布返回409；缺少证据、审批、依赖或类型无效返回422；越权403，跨项目资源按作用域404。同幂等键同源/目标/摘要重放不重复投影；变更请求使用新键。202只代表排队，PUBLISHED只在消费者成功提交后出现。失败由已有Outbox重试；过期事件不会发布。

CMDB复用bom_item和稳定ID，写入planned事实及字段证据。已有Observed payload保持不变，机柜可视化查询读取规划位置并明确来源。项目删除清理专属工程模型/任务；审核与发布审计保留。旧导入使用`?legacy=1`保留入口。

定向检查使用隔离PostgreSQL合成资料，覆盖审核门禁、幂等、分域投影、现有机柜图查询及Observed保护；AI协议使用本地合成模型服务。没有真实AI服务调用、真实XLSX入库、浏览器交互验收、参考Calc对比或第二客户资料验收。连接、线缆、电力、物料、工作量及网络域尚待纵向实施。


## P3：计算与规则

新增8项操作，同样复用项目授权和Session：

| 路径（省略/api/v1/projects/{projectId}） | 方法 | 权限与行为 |
|---|---|---|
| /engineering-rules | GET / POST | bom:read / bom:edit；最近100个规则版本，创建经案例校验的候选版本 |
| /engineering-rules/{ruleId}/{version} | GET | bom:read；精确版本 |
| /engineering-rules/{ruleId}/{version}/review | POST | bom:approve；精确摘要批准、驳回或停用，保留独立人类审计 |
| /engineering-imports/{id}/calculation-provider | GET | bom:read；当前配置的Provider能力 |
| /engineering-imports/{id}/calculations | GET / POST | bom:read / bom:edit；最近30次运行 / 持久排队；相同输入在途幂等 |
| /engineering-imports/{id}/calculations/{runId} | GET | bom:read；按offset/limit分页公式，最多200条；摘要最多100条错误，完整错误保留在逐公式记录 |

原始来源附加时建立空的不可变来源版本，AI可以先计算再提出工程模型。多轮工具可读取命名区域/范围/循环/动态依赖、提出规则草案、调用计算和确定性校验；无规则批准或正式发布权限。实际物理字段依赖公式时，确定性阶段统一重算；没有兼容计算记录的公式字段阻断受影响目标。新的单元格证据关联CalculationRun，人工修改无关字段不会清除来源冲突。

迁移028分别保存规则、输入输出、版本依赖、案例、审批、运行、逐单元格结果、依赖边和错误。表达式AST与单个TypedValue是局部类型化值，未以一个JSON保存整个工程域。规则版本不可变，只有审批状态由受权人改变；AI的案例仅用于确定性一致性检查，不是独立Ground Truth。

表达式支持带单位的输入/常量、加减乘除、最小/最大、绝对值、取整、标准规格上取整和多个输出；不执行Python/JS。取整须显式提供步长对应的无量纲比率；标准长度/采购参数来自规则输入或经审核常量，未预设业务规格。内部使用有理数运算，输出十进制表示最多18位小数。当前不支持复合单位相乘/相除（标量及同维比值除外），明确拒绝；未知/外链/动态依赖不能标为完全兼容。

## Calc运行配置（未应用到当前服务）

构建：`docker build -t keel-engineering-calc:local dev/deploy/calculation`。API运行用户需能调用本机Docker；显式设置`KEEL_CALC_IMAGE=keel-engineering-calc:local`。未设置时Provider报告available=false。其他引擎实现同一个SpreadsheetCalculationProvider接口，不自动替换或假装成功。

每次计算启动独立、无网络、只读根文件系统、去除capabilities的容器；只挂载本次只读输入和独立输出目录。限制CPU、内存、PID和三分钟执行时限，取消后清理容器。LibreOffice通过UNO以NEVER_EXECUTE/NO_UPDATE打开文档，calculateAll后将结果写入副本，原件摘要先校验。API只保存需要的类型化结果与依赖；临时副本在读取后删除。启动方式参考[LibreOffice参数文档](https://help.libreoffice.org/latest/en-GB/text/shared/guide/start_parameters.html)。

已实际检查：隔离Calc合成跨表公式结果20及除零错误；范围/命名区域/循环/动态依赖；规则单位/参考案例/审批；隔离PostgreSQL规则版本/审批/去重/任务执行/缓存与结果分离/证据关联/项目隔离。未执行真实客户资料公式对比或浏览器交互验收。
