# 智能开局工程真实接口

更新：2026-11-01。工程Agent修复已在原5180/8081上线，迁移038已应用；前端保留原版本页面，只改变工程上传分流。下方阶段检查是历史开发记录，当前部署证据见[部署记录](../intelligent-deployment/deployment-agent-repair-20261101.json)。所有路径使用 `/api/v1` 前缀及现有身份Session、项目授权和CSRF。精确DTO以 `openapi.yaml` 的 `IDE*` 为准。

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

定向检查使用隔离PostgreSQL合成资料，覆盖审核门禁、幂等、分域投影、现有机柜图查询及Observed保护；AI协议使用本地合成模型服务。没有真实AI服务调用、真实XLSX入库、浏览器交互验收、参考Calc对比或第二客户资料验收。电力、物料、工作量及网络域继续纵向实施。


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

## Calc运行配置（当前服务已启用）

构建：`docker build -t keel-engineering-calc:local dev/deploy/calculation`。API运行用户需能调用本机Docker；显式设置`KEEL_CALC_IMAGE=keel-engineering-calc:local`。未设置时Provider报告available=false。其他引擎实现同一个SpreadsheetCalculationProvider接口，不自动替换或假装成功。

每次计算启动独立、无网络、只读根文件系统、去除capabilities的容器；只挂载本次只读输入和独立输出目录。限制CPU、内存、PID和三分钟执行时限，取消后清理容器。LibreOffice通过UNO以NEVER_EXECUTE/NO_UPDATE打开文档，calculateAll后将结果写入副本，原件摘要先校验。API只保存需要的类型化结果与依赖；临时副本在读取后删除。启动方式参考[LibreOffice参数文档](https://help.libreoffice.org/latest/en-GB/text/shared/guide/start_parameters.html)。

已实际检查：隔离Calc合成跨表公式结果20及除零错误；范围/命名区域/循环/动态依赖；规则单位/参考案例/审批；隔离PostgreSQL规则版本/审批/去重/任务执行/缓存与结果分离/证据关联/项目隔离。未执行真实客户资料公式对比或浏览器交互验收。

## S3：连接和线缆

新增 PUT /engineering-imports/{id}/connectivity（bom:edit），精确版本、修改原因和四选一的端口/端接/线缆/光模块需求对象；沿用查询、版本差异、审核及发布接口。OpenAPI 为 IDEConnectivityEdit，不能提交任意对象。

迁移029、030分别增加类型化端口/端接/路径/长度/光模块需求表和CMDB规划属性/配线贯通关系。新增CMDB_PLANNED_CONNECTIVITY及ENGINEERING_CABLING目标，前者依赖设备发布，后者依赖连接发布。线缆工程与采购需求仅发布工程版本，不生成实物光模块或写实际线长。

多轮Agent通过任意区域绑定提取端接两端、父/Breakout子端口、规格、长度与单位、标签、来源路径；按父物理端口去重光模块。配线中间端口必须为有明确依据的入口/出口对，同一节点两端属于同一设备。CMDB复用既有端口/线缆身份和端接关系投影每个外部线段，内部贯通为patch_passthrough关系，不是另一根采购线缆。现有图查询可读取各段；原有单Cable Trace接口仍仅证明单段端接，不宣称多跳追踪验收完成。

Estimated/Calculated/Confirmed/As-built分别保存；公式结果关联独立运行。单位不明、端口重复占用、Breakout循环、规格冲突或标准长度不足只阻断对应域。工作台支持对象修订、配线节点排序、路径段、长度状态与来源证据；规则计算结果应用及重新导入差异在后续统一发布链中接续。

实际通过：合成端口去重/长度状态/速率单位/Breakout与配线缺端检查，隔离PostgreSQL分域审核、Outbox投影、配线外部两段拓扑查询，未写实际线长或采购光模块资产。没有生产迁移、真实资料AI或浏览器验收。

## S4：电力、冷却与计算结果应用

新增PUT /engineering-imports/{id}/power（IDEPowerEdit）及POST /engineering-imports/{id}/calculation-application（IDECalculationApplication），均要求bom:edit、精确版本和修改依据。既有calculations请求增加可选operation=power_balance；不能同时指定任意规则或输入参数。迁移031以计划、设备功率、机柜容量/结果三类关系表保存。

功率档案明确planned/nameplate/measured，冷却方式保持来源语义；W/kW做确定性单位换算。内置power_balance/v1只做明确规划功率求和及容量相减，无需客户业务假设。缺少唯一落位、规划功率或容量时返回逐机柜问题和partial/failed，不把缺失当零；不自动应用铭牌、实测值、冗余系数或同时率。客户工程规则仍走独立规则版本和人工审批。

计算输出按明确绑定批量应用到线缆计算长度/标准规格或机柜负荷/余量，创建待审核修订。来源、项目、运行状态、输入版本及输出数值受校验，调用方不能挂一个运行ID并写入任意计算值。调整电力输入或设备/机柜归属后清除过期负荷；保留历史运行和修订。ENGINEERING_POWER独立发布工程版本，不写CMDB观测功率或生成资产。

工作台提供功率依据、设备功率档案、容量/冷却、参数编辑、负荷计算/批量应用、证据及版本校验。通过合成精确2kW负荷/1kW余量、实测分离、缺失不当零、冷却超限；隔离DB验证持久运行/输入输出、应用、审批/发布、拒绝过期和篡改、参数变化清除旧值。严格AI Schema与前端增量TypeScript通过；没有真实资料或浏览器验收。

## S5：采购与施工工程量

新增PUT /engineering-imports/{id}/estimates（bom:edit，IDEEstimateEdit），只接受物料或工作量对象之一。迁移032分别保存物料规格/需求/已采购/备件/缺口和施工类型/数量/标准长度/工时，单字段的需求/数量计算来源与缺口/工时计算来源独立。ENGINEERING_MATERIAL、ENGINEERING_WORKLOAD各自审核发布工程版本，不生成实物资产。

Agent支持任意material/workload区域、列位置和显式单位。重复需求不擅自相加，缺少数量保留来源和阻断；采购缺口及工时来自明确源数据或经过人类审核的项目规则，不内置备件政策或采购差额公式。统一计算结果应用增加material_required/material_gap/workload_quantity/workload_hours绑定；数量输入变化使旧缺口/工时失效。

既有计算请求增加operation=workload_summary，统计规划设备安装数量、物理外部线段数量、明确标准规格的长度分布。统计不生成实物SN、不代表施工完成、不计算默认工时。缺标准规格、配线中间节点未给出逐段规格时保留partial问题，不将估算长度作为标准规格。生成记录与来源工作量分别保留，重新应用完整统计替换之前同来源的派生记录；上游数量改变导致旧统计BLOCK，不能直接重新批准旧数值。

实际检查：合成设备组数量、标准规格单位换算、缺规格partial、缺口/工时不补造、物料单位不匹配；隔离PostgreSQL统计3→4、持久输入/输出、结果应用、独立审核/发布、无CMDB实物资产、旧统计失效和重算替换。增量TypeScript通过；未部署或执行真实资料/浏览器验收。

## S6 网络规划

`PUT .../network` 保存一个独立网络规划聚合，包含Fabric/Plane、角色、接口映射、IP/ASN、BGP/BFD、QoS/RoCE、Adaptive Routing和EVPN/VXLAN。核心对象存放在迁移033的明确类型表中。来源识别只映射显式规划依据，不从物理连接推测IP、ASN或邻居。未确认字段保留缺失，校验问题阻断对应网络目标。

`GET /projects/{projectId}/engineering-physical-revisions` 提供当前项目有效的已发布物理版本。网络计划可引用其他导入的物理版本，接口映射复用其设备/端口身份，修改物理依赖后不得继续消费旧版本。

`GET .../golden-input` 只返回当前已审核发布且物理依赖仍有效的NetworkPlan Revision；未发布/失效返回409。此接口提供Golden规划输入和工作台下载，不声称已实现厂商配置渲染或网络下发。网络发布仅生成工程发布记录，不新增CMDB资产。

验证：隔离合成数据库的十类子模型往返、物理依赖发布前阻断/发布后读取/依赖变更后拒绝、CMDB资产不增加；合成跨表顺序变化及不补造实体；TypeScript通过。真实AI、真实资料、浏览器交互、厂商配置或真机验证未运行。未部署，033仅应用隔离库。

## S7 资料理解与模块计划闭环

`GET .../plan` 返回当前持久ImportPlan、可信模块定义、输入依赖与可独立处理批次。模块状态从实际审核目标、分析作业及计算任务投影；不另建可绕过人工审核的状态来源。缺少某域资料只影响该域及必要消费者，AI不适用建议不能忽略已有模型。

分析保存模型后，根据适用模块和输入就绪状态，将内置电力平衡/施工量计算加入原持久任务队列。结果仍是候选，应用和发布分别操作；不会自动应用未审核项目规则或正式发布。

`GET .../intelligence?offset=0` 返回工作簿区域、合并区域/多行表头位置与分页语义事实（每页100）。来源绑定、ImportPlan实例/输入、语义事实、生产者溯源均采用明确关系表，迁移034。源事实建议与人工工程模型分开保留。证据包含原始source_id，避免跨资料引用丢失来源。

Agent region支持可选records单元格引用，每条record按明确的sheet/cell/field/unit引用提取；适用于横向参数、网格、跨Sheet对象。不能与同一区域的columns混用，所有值仍由本地读取，敏感单元格拒绝提取。现有列式提取、来源常量、公式重算继续复用。

工作台收敛为资料理解、工程总览、来源证据、计算、问题、差异及审核发布；领域面板从总览进入。领域修订冻结打开表单时的版本，避免陈旧表单覆盖新版本。

验证：合成横向/跨Sheet记录、敏感来源拒绝、缺失电力不阻断设备；隔离DB计划/输入/区域/事实/溯源往返和人工修订保留来源，按适用模块排队施工量且显示处理中；AI合成协议和TS通过。真实AI/文件、HTTP/浏览器验收未运行，未部署。

## P5 新版资料、差异决策与规划撤回

`POST .../reimport` 将新上传资料与当前精确版本关联，原件和基准版本均保持不可变。`POST .../difference-review` 由具有bom:review权限的人选择采用新版、保留基准或明确保留重新分析前的确认值；每个决定记录目标、摘要、版本、操作者及原因。接受差异后仍须经过原有分域审核和发布。基准域后来变化时阻断旧基准覆盖。

新版与基准之间的差异在各自目标产生BLOCK；不把未处理差异直接同步CMDB。空目标的明确移除建议也支持审核发布。保留基准时复制原source_id证据，并允许引用祖先来源的已验证计算记录，来源不会误指向新文件。

同一资料版本链、同一发布目标的旧规划证据标记superseded（不删除历史），旧关系结束有效期；撤回且没有其他有效拥有来源的纯规划资产退役。Observed/其他来源值不改写。现有机柜/拓扑查询排除退役资产。新版分域发布令祖先相同目标失效，Golden不能继续消费失效物理版本。迁移035仅隔离库。

验证：TestEngineeringReimportReviewedRemoval 在隔离合成库发布旧设备/端口/线缆，再导入新版；处理保留/采用决定，保留原始来源；撤回线缆后现有VisualConnections返回0；进一步审核空连接目标并撤回两个端口；Observed payload逐字节未变、历史证据仍在、祖先发布STALE。增量TypeScript通过。没有真实XLSX/模型或浏览器验收，没有部署。

## P4 分段线缆规格

CablePlan新增可选segment_index（1起）。直连缺省为唯一外部段；含配线架的路径必须明确每个CablePlan属于哪一段，不能将整条路径长度当作每段长度。持久化和人工编辑支持此字段，Agent可绑定来源段序号。标准线长分布逐段统计；缺失段保持partial。迁移036。合成两段3m/5m存储、审核发布与现有拓扑查询，以及核心缺失规格校验和TS通过；真实文件未验收。

## P4 版本化类型扩展

项目级`/engineering-attributes`提供定义查询/不可变版本草拟，`/{attributeId}/review`由bom:review人工批准或驳回。定义限定对象类型、text/decimal/boolean/reference、明确单位、文本枚举和必填；批准前不能应用。核心身份字段不通过属性覆盖。

值按定义ID/版本与对象版本逐条关系化保存，typed slots不能混用；false保留为明确值。Required缺失、引用不属于项目或定义不可用均阻断对应领域。定义变更不执行动态数据库迁移，不改写旧版确认值。对物理对象，发布后将类型化扩展值关联到CMDB planned来源；Observed仍不覆盖。

Agent收到项目属性定义，只能引用批准版本，使用`attribute:<id>@<version>`绑定明确来源列/单元格；本地提取校验类型、单位和来源。未知定义/冲突不静默接受。工程总览提供定义审核及对象属性修订，采用现有模型修订接口。

迁移037。TestApprovedTypedAttributeExtraction与TestEngineeringTypedAttributesApprovalAndPublication通过：真实类型/明确单位、草稿定义拒绝、false往返、项目必填阻断、批准后CMDB规划关联；合成Agent协议与TS通过。真实模型/真实资料/浏览器未验收，未部署。

## P6 输入变化与工作台收口

保存工程新版本时，仅相关物理输入变化使沿用的线缆计算值失效；估算、计算或确认长度变化使未更新的规则标准长度选型失效。清除对应派生字段的当前计算证据，历史版本/计算运行继续保留；来源标准规格、确认和As-built不被替换为推算值，无关机柜变化不清空该线缆结果。其他域仍独立审核发布。

扩展属性编辑展示对象值实际使用的批准定义版本，不用最新定义的单位重新解释旧值。排队计算不显示零时间；分析、差异审核、计算与属性失败显示中文处理提示。未增加接口、迁移或新的产品菜单。

定向线缆输入检查及增量TypeScript通过；真实源文件、真实Provider与浏览器验收未运行，当前环境未部署。当前开发状态统一见HANDOFF与实施报告，不以历史S编号判断P1–P6完成度。

## 当前环境部署

运行代码5abb94f，原5180/8081与身份/AI/对象存储/Flows配置沿用；Calc以镜像摘要固定版本。四个工程列表GET和Provider选项已通过真实登录后的只读启动检查；33个操作均已随二进制发布，但没有逐个执行写入或完整业务验收。没有导入样本或运行外部AI。部署/备份/迁移/回退证据见上述部署记录。

## 真实资料理解修复：原生工具、MinIO产物和上传分流

工程工作台上传显式传`import_mode=engineering`，解析后通过原`/engineering-imports`关联和`/analyze`排队；不再并行启动旧BOM识别。默认`ai`仍供原BOM流程使用。OpenAPI与前端生成类型同步。

Agent原生工具参数按Pydantic Schema校验，读取脱敏IR，以逐Sheet校验的提案形成可恢复检查点。已完成部分不依赖进程内存；暂时性上游错误保留HTTP状态与白名单原因，不暴露任意错误正文。120秒Retry-After不会被忽略或立即失败。

原件继续存MinIO，结构化工程事实继续存现有关系表。`eng_analysis_artifact`索引MinIO压缩工具结果、检查点和最终提案；按项目目录归属，删除项目沿用原Outbox清理前缀，审计与共享型号保留。作业恢复核对产物SHA256。大对象通过临时文件流式写入，工作上下文只保留最近工具结果。

验证截至本里程碑：原生工具/恢复协议/上游错误分类、Go构建通过；真实上传未启动旧Agent，MinIO产物已读取验证。真实MM任务因上游HTTP 402结束；当前账户余额不足，理解尚未验收。恢复机制已补到未完成Sheet的每个工具步骤，修复版本f98c8ea已上线。详见[真实复测记录](../intelligent-deployment/agent-repair-test-20261101.json)。

## Agent工具字段绑定与纠错

原生工具以字段名为唯一键表达columns（值为列号及可选单位），服务端规范化为现有ColumnBinding数组后执行严格Schema与来源校验；公共HTTP模型不变。重复JSON键、未知字段和来源冲突不得静默覆盖。完整原始工具请求/结果保存MinIO，当前模型上下文只保留active_sheet；批次首错立即返回纠错反馈。大范围读取返回有界样本及实际范围；区域覆盖不要求把辅助列强行绑定为工程字段。

针对已拒绝候选的字段/覆盖错误，Agent内部map_semantic_fields支持按区域索引局部设置/移除列绑定或常量、修订bounds。未修改字段保留，整个候选仍完整校验；复杂结构错误继续整份提案。工具原始局部请求留MinIO，检查点保存规范候选。该内部工具协议不新增HTTP操作，也不授予正式发布权限。

工程理解内部纠错轮仅携带当前未批准草案、来源与验证错误；`map_semantic_fields`支持基于来源修正区域分类（set_kind），完整Schema/领域/来源检查保持。此为内部工具协议，不改变HTTP契约。

## 五步标准输入契约（开发中，未部署）

`GET /projects/{projectId}/engineering-inputs` 返回版本1的五步模板目录、列名、类型、必填及条件说明。`GET .../engineering-inputs/{tableId}?format=csv|xlsx` 下载仅表头模板；XLSX使用文本表头，不包含真实资料。八列端接模板固定为a端SN/a端MAC/a端端口/b端SN/b端机架位/b端MAC/b端端口/端接类型。

`POST .../engineering-inputs/preview` 接受IDEInputPreviewRequest，返回逐行原值和字段问题。SN等身份字符串不转数字；MAC校验48位格式，数值按类型检查。表头、列序不符明确报错。该接口只做格式预览，不持久化、不认定工程模块就绪、不发布资产，不执行模型调用或规则计算。条件必填及跨表关系尚需后置建模校验。

作用域为项目，读取/下载要求bom:read，预览要求bom:edit并沿用Session/CSRF。查询可按项目缓存目录；预览无缓存。请求沿用8MiB限制。前端提供五步选择、字段说明、XLSX/CSV下载、CSV读取与手动修改、问题表和前20行预览；当前仅标准格式预览已实现，补充入库、XLSX读取及新Agent仍在开发。错误包含403/未知模板404/非法格式400/Schema400；预览中的业务输入错误以200响应issues返回。未运行HTTP或浏览器验收。

`POST /projects/{projectId}/engineering-imports/{sourceId}/standard-input`（开发中未部署）接收IDEStandardInputRequest，当前仅支持terminations八列表。权限bom:edit，沿用Session/CSRF；绑定expected_revision及确认依据。先验证完整CSV，再在项目CMDB范围按SN、设备/端口名解析已有身份；歧义、MAC冲突、端口占用、类型或同向位置冲突阻断。没有现成身份时创建规划草稿；其他工程域增量保留。原始CSV存MinIO，索引与新修订在同事务保存（迁移039）；失败不提交模型。反向重复保持原连接方向，反向位置声明保留在原始补充资料。前端确认后刷新任务版本，仍需现有审核/发布。

迁移039新增端口MAC、端接类型、原始B端位置声明和标准输入来源索引。MAC与类型由工程发布投影为planned事实，Observed保护沿用；声明不等于正式机柜落位。此入口不自动补造型号/高度/网络参数。当前补充来源尚未接入Evidence Explorer逐字段查看，后续四步确认导入和完整闭环仍未完成；这些不以预览成功代替验收。新增错误码STEP_*定位输入、身份、端口、类型、位置或源存储问题；并发版本或运行中分析返回409。未进行数据库、HTTP、浏览器和CMDB发布验收。
