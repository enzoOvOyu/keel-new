## 已完成里程碑事件：机房与落位标准输入代码切片（未部署、未业务验收）

四类表rooms/racks/equipment_specs/placements已接入同一确认导入通路：补充机房/机柜/设备规格、按稳定引用落位，保留设备与连接。服务端目录声明可确认的表，前端显示统一入口；U位与non_u位置分开，缺上级、规格高度冲突、重复位置有阻断，其他结构问题保留现有审核门禁。迁移040持久设施编码、设备高度和非U位置；CMDB投影保留规划关系并避免为非U填造U位。工程域测试、API/CMDB编译及TypeScript已通过；未部署或运行DB/HTTP/UI/机柜图验收。步骤3-5标准确认导入、分步Agent、补充资料证据查看及完整独立验收仍未完成。

## 已完成里程碑事件：设备关联增量导入代码切片（未部署、未业务验收）

已实现八列表确认导入服务与工作台按钮，按项目范围查找CMDB设备/端口并复用ID；纯函数增量合并处理重复/反向连接、MAC冲突、占口、类型和位置冲突，失败不修改输入模型。原始CSV存MinIO，标准来源索引随工程修订同事务保存。迁移039提供MAC、端接类型、B端位置声明及来源索引；发布沿用planned/Observed边界。新增端点字段同步OpenAPI、生成类型、Go读写及CMDB投影。工程域单元测试（含新增字段/合并冲突用例）、API/CMDB包编译、TypeScript和文档登记检查通过；未运行数据库/HTTP/UI验收，不代表整体完成。后续四步确认导入、新Agent、来源逐字段查看、数据库/HTTP/UI/真实资料闭环仍未完成；未部署或修改线上库，在线迁移仍为038。

## 已完成里程碑事件：五步模板契约与格式预览入口

用户已授权按方案持续开发。新增28类固定输入表的统一目录（设备关联八列、落位、布线、网络、电力），Go提供目录、CSV/XLSX模板下载和CSV逐行格式校验，OpenAPI及生成类型同步。现有工作台接入五步、模板下载、CSV读取/编辑与问题预览；页面明确预览不代表入库或关系正确。SN保留文本，MAC与正数/整数有定向用例。实际通过Go输入契约测试、API包编译检查；前端TypeScript类型检查及文档登记检查通过。尚未完成标准入库/增量关联、后置关系补齐、新Agent及独立业务验收；未部署，线上v40保持。变更入口为step_input.go、engineering_step_input.go及EngineeringStepInputs.tsx，API说明见docs/api/engineering.md。

## 已完成里程碑事件：五步开局的研究依据与详细实施设计

完成[研究与实施方案](docs/intelligent-deployment/RESEARCH-BACKED-IMPLEMENTATION-20261101.txt)，补充原生表格/视觉资料适配、分拣与分步Agent、证据关联、固定格式后置链、有限纠错和独立验收。引用一手论文/官方规范，区分预印本报告与本项目验证；无限范围全自动零错误不能保证，人工补齐不能计为AI识别成功，具体范围与自动化目标尚待确认。本次仅设计文档，未改变模型、业务实现、数据库或在线版本。实际检查：文档引用/登记检查与diff格式检查；不代表业务验收或论文实验复现。

## 已完成里程碑事件：五步固定输入与分步Agent整改方案

用户明确将开局内部收敛为设备关联、机房与落位、连线与布线、逻辑网络、电力五步。已核对现有源码并完成[整改方案](docs/intelligent-deployment/FIVE-STEP-REFACTOR-PLAN.txt)：先固定模板/字段/前置/输出和人工补齐，独立跑通无AI标准链，再新增资料分拣与五个受限提取任务。A端MAC=本端MAC，B端MAC=对端MAC，按对应端点保存。方案列出多来源增量合并、身份/位置/布线/网络/电力关系、UI、代码复用、缺口及各里程碑退出条件。端接类型与机架位示例尚待确认；不阻断其他设计。本次只完成设计与交接记录，未修改业务实现、调用模型、导入测试数据或部署；当前在线版本不变。

## 已完成里程碑事件：恢复前端失效的fixture资源链接

用户报告Vite无法导入demo-seed.json。根因是dev/fixtures/demo-full-64-b300仍指向已删除的../../keel-demo/fixtures/demo-full-64-b300；仓库与当前frontend-source部署均存在同一个断链。已将二者修正为../../fixtures/demo-full-64-b300，复用仓库现有唯一数据源，没有复制种子或修改API/数据库。实际失败的/src/lib/api/index.tsx及入口/src/app/main.tsx均恢复200；无头浏览器使用已有测试会话进入/dashboard，显示“全局交付与运维”，无Vite错误层、pageerror或HTTP失败。只验证本次页面启动故障，没有重跑Agent、平台冒烟或业务回归。

## 已完成里程碑事件：整本候选完成独立的首轮理解审查，结论未通过

v40任务c3788ecc-f911-465f-bb4c-bc667ca70d8d已保存15/15表、29区域和33问题，完整提案从MinIO读取。当前候选7154条端接×8来源字段及967条设备名称/型号匹配独立OOXML；这不证明最终对象关系正确。首轮理解仍未通过：数量说明不准确、适用模块漏关联来源、网格关系不完整、机柜对仍当设备连接、摘要重复。已区分理解与工程投影，不再追加字段门禁或启动新一轮分析。此前P0-P5代码实现不等于真实业务验收完成。具体证据和根因见[首轮理解审查](docs/intelligent-deployment/SOURCE-UNDERSTANDING-REVIEW-20261101.txt)。本里程碑完成的是审查，不是问题修复或产品验收。

## 已完成里程碑事件：待修草案按Sheet持久保存并跨重启恢复

定位到多表恢复缺陷：旧恢复把所有错误候选塞进共享history，切换当前Sheet会过滤掉其他表的草案，保存当前表又清空history，导致反复重新识别。检查点现新增类型化pending_drafts，保存尚未处理的表级草案（含失败历史中的未完成表），仅把当前表错误注入模型上下文；其他待修草案继续留MinIO检查点。新增两张错误候选的HTTP协议模拟：修正第一张、序列化检查点并停止、重启后直接修第二张，合计仅两次纠错且身份绑定不变。相关纠错/恢复/原生调用/重复失败用例和v40构建通过。当前任务仍未验收；准备部署并恢复revision2的原检查点。

## 已完成里程碑事件：拆分区域保留设备身份及线缆附加字段语义

v37独立复核五张端接7154条八字段再次全部匹配，但NS/OOB额外字段把线缆类型/端点标签当连接器、线长当路径距离；Network把名称仅放在placement，equipment无label会被提成匿名型号组。现明确要求有显式设备名表头的equipment区域绑定自身label，不能由另一个placement区域代替；端点标签、介质、角色、长度的已知语义冲突明确拦截。新增拆分设备/落位仍生成唯一planned_instance的用例及附加字段冲突用例，完整engineering和定向AI检查、v39构建通过。包含v38区域分类误报修复，准备在revision2检查点继续重验；源文件未改，未发布正式数据。

## 已完成里程碑事件：区分区域分类错误与字段漏映射

v37新一轮完成五张端接与Network候选后，电力表明细已经绑定rack，却因kind=placement被功率域门禁判为“缺少rack”，导致Agent重复修改正确列并NO_PROGRESS。现明确返回REGION_KIND_CONFLICT、区域索引及允许域，并将该轮工具限定为set_kind；分类修正后再检查真正缺失的功率/容量字段。新增真实HTTP协议模拟验证两轮“改分类→补功率/容量”完成，正确rack来源保持；定向AI、完整engineering及v38构建通过。v37候选设备功率表已经是label/model/power/quantity四个正确来源字段，稀疏纠错效果有改善，整本仍未通过。准备在revision2原检查点继续，不重跑已通过来源。

## 已完成里程碑事件：首个整本候选及虚构端口拦截

v35任务016d7561-3851-460f-96a5-a97c218b6a1e首次完成15/15 Sheet，完整提案已写MinIO，工程草稿revision2生成，技术任务succeeded。独立复核仍FAIL：电力误绑定、P2P机柜标识同时当设备/端口、网格误按行表等。草稿有3118条问题（含真实缺失与错误提案），1058设备/7492连接均不可作为真实资产数；没有发布。v37新增同源列不能同时充当同一端设备与端口的确定性约束，完整engineering及定向纠错检查、构建通过，包含v36稀疏纠错和功率门禁待部署。已异步询问是否允许用已配置testAI/DeepSeek做对照；答复前保持Ultra。

## 已完成里程碑事件：稀疏字段纠错与功率来源门禁

v35已推进至13/15候选Sheet，但独立检查发现电力提案把设备名称填入位置/尺寸/序列字段、数值功率填入单位/容量字段，并把机柜负载明细留成placement；这些不能验收。纠错set_columns改为稀疏数组，兼容旧对象读取，减少强制对象参数填满全部可选字段的现象。复用来源表头语义新增通用功率/容量与设备名称冲突检查，功率依据/单位使用已有领域枚举；明确机柜+功率+容量表必须绑定对应域关键字段。新增任意Sheet名/表位置的功率不能被落位吞掉用例，相关AI纠错及完整engineering检查、v36构建通过。当前v35任务继续剩余资料；v36待切换后重验已保存候选。

## 已完成里程碑事件：覆盖错误只编辑区域与排除行

v34最终在电力表仅遗漏标题A1时仍反复改无关字段，任务失败。覆盖纠错工具现在只暴露bounds/header_rows并要求显式填写；不再给这类错误开放所有工程字段。标题/小计等必须作为排除行保留来源，不能因扩大区域而提取成设备。新增合成用例验证完整来源覆盖后仅产生真实设备、不把标题当资产；相关纠错/恢复/重复失败用例及v35构建通过。原电力候选尚未验收，继续原来源。

## 已完成里程碑事件：纠错上下文收敛与区域分类修正

v33已真实纠正Server的站点/机房误映射，保留核心字段；电力仍在字段/常量错误之间循环并最终NO_PROGRESS。纠错轮现在只携带未批准草案、实际验证错误、绑定问题及当前来源预览，不重复整本/模块规划任务；保留常量错误的局部纠正。另允许基于来源修正单区域set_kind，解决把功率表误分为placement后只能删除正确功率字段的死路。完整AI检查发现旧上下文断言不再适用，更新为要求实际错误/索引且零错误assistant示例；受影响纠错、恢复、重复失败停止及分类保留证据用例全部通过，v34构建完成。真实整本仍未通过，准备继续同一来源。

## 已完成里程碑事件：纠错工具要求显式编辑参数

v31真实电力分析反复只返回sheet和region_index，未执行字段修改，最终NO_PROGRESS。纠错工具现在要求明确提供set_columns/remove_columns（未用项为空），并解释删除字段不会由set操作代替；保留已有完整候选校验。定向纠错/恢复检查及v33构建通过，包含v32设施层级检查，准备继续原来源；此变化尚不能宣称真实电力理解通过。

## 已完成里程碑事件：设施层级语义冲突检查

独立检查发现Server候选把Rack Number同时当site、Customer Rack Name当room。现复用已有通用表头语义，拒绝明确site/room/rack之间的错误替代；未知表述仍由Agent判断。补充对应合成关键用例，完整engineering包及局部纠错/反馈AI检查通过，v32构建完成待切换。空修正错误明确要求编辑操作，绑定错误统一使用region_index并说明remove_columns语义。v31已补回Network机柜；按U位置表示归一后967条设备的四个核心字段匹配，但Server额外层级字段仍需纠正，整本未验收。

## 已完成里程碑事件：重复表头关键关系门禁与恢复草案纠错

同表出现多个机柜/U位表头时，原覆盖检查跳过整个字段，导致Network提案遗漏rack仍被保存。现在要求至少选择一个有来源依据的候选列，保留语义选择空间；恢复检查点发现错误时保留未批准草案并提供局部纠错反馈，不重新生成全部已正确字段。新增重复表头及恢复候选修正用例通过，定向AI和完整engineering包检查通过，v31构建完成。v30真实任务在电力测算收到连续空修正后失败，整本仍未通过；准备继续原Ultra-03来源。

## 已完成里程碑事件：从整表重写改为候选草案局部纠错

v29真实任务e66f9d18最终仍因Network名称绑定反复丢失而NO_PROGRESS。根因是每次纠错都要求重新生成整份提案，模型修复元数据时又丢掉已正确绑定的核心字段。现在对字段/来源/覆盖错误保留明确未批准draft_to_correct，复用map_semantic_fields工具按区域索引设置/删除指定列或常量、调整范围；其他字段不变，完整候选仍经Schema和来源校验才保存。复杂结构变更仍使用完整提案，未放宽门禁。原始局部请求及完整候选检查点留MinIO。完整AI既有包及新增真实HTTP协议模拟、保留无关字段、重复键/未知参数/跨Sheet/空操作拒绝、可选数组恢复检查通过，Go构建通过；真实整本尚未验收，v30准备继续原来源。

## 已完成里程碑事件：电力多对象常量纠错

v27最终在电力测算中将多个对象的power_capacity/cooling_type/power重复放入单区域constants，旧INVALID_CONSTANT_EVIDENCE没有说明重复对象问题。现明确区分不支持字段、重复字段、缺失证据，并一次返回全部列/常量冲突，要求不同源对象拆区域或records。没有擅自删除重复业务事实或改变源文件。常量纠错/既有反馈定向AI检查、完整engineering包及Go构建通过；v29包含已上线v28设备语义校验，继续同一来源。

## 已完成里程碑事件：设备核心字段核对及额外语义冲突校验

Ultra-03七张核心表的独立对照达到设备967条×名称/型号/机柜/U位四字段全部匹配、端接7154条×八字段全部匹配。但Network额外绑定把Asset ID当深度、序号当数量/身份类型、U位置当高度、落位名当安装面；Server把配置名当设备类型，因此设备整体仍不能验收。新增通用来源表头冲突和既有分类值检查，一次返回多个语义冲突；未知元信息仍保留来源，不改样本，不按Sheet名/列号判断。常量不存在的引用现在明确返回字段/单元格。完整engineering/ai包、追加聚合冲突后工程包及Go构建通过，v28准备上线；恢复会退回错误设备提案，保留有效端接。

## 已完成里程碑事件：工具唯一键Schema简化与上游错误信封识别

v26任务92fa07b9收到HTTP200但正文error.code=502且无choices，原客户端误报SCHEMA_VALIDATION_FAILED，未查询400原因。现按正文错误码识别Provider临时错误，不把其当作AI提案；原生列映射Schema改用已有标准properties/additionalProperties形式声明明确字段键。唯一键、作用域、原生工具及HTTP200错误信封定向检查和构建通过，v27继续同一来源；先前五张端接检查点保留。

## 已完成里程碑事件：工具字段绑定改为唯一键结构

v24任务50a9ba49在Network Information反复输出label/model重复绑定后失败；五张端接7154条有效检查点保留。原生工具的columns现采用字段名到列号/单位的对象映射，再无损转换为既有规范数组；重复JSON键明确拒绝，未知属性仍由严格Schema拒绝，原始模型工具参数保存MinIO，不静默覆盖。内部模型、公共API、旧检查点数组结构不变。非法新结构的纠错上下文仍按Sheet保留。完整AI包、追加唯一键/历史隔离定向检查与Go构建通过；v26准备继续同一来源。

## 已完成里程碑事件：五张端接表7154条关键来源核对通过

Ultra-03已完成五张端接Sheet（EW2592、NS484、GB_OUT3456、OOB216、HDD/SSD/CPU406）。独立OOXML基线逐行核对A/Z设备、A/Z端口、A/Z光模块、线缆规格、长度八字段，7154/7154全部匹配；当前提案除medium外没有虚构额外列绑定，辅助列明确保留来源/other。尚未生成最终工程模型，967条设备及剩余10张资料仍在分析，不等于CMDB/拓扑验收。运行v24，v25文本反馈澄清已构建待后续切换。

## 已完成里程碑事件：三张端接表6532条关键来源核对通过

Ultra-03当前有效检查点为EW2592、NS484、GB_OUT3456，共6532条；独立原始来源对比八个关键端接/线缆字段全部匹配。NS旧错误额外字段已移除，NS/GB_OUT辅助列显式保留other。尚未覆盖剩余622条端接、设备表和其他工程资料，不代表整本完成。另已澄清工具Schema/提示及覆盖错误：扩大区域bounds不等于增加字段绑定，未知辅助列可保留在区域内；定向AI检查、来源覆盖检查和构建通过，v25已构建，当前v24任务继续运行避免无谓中断。

## 已完成里程碑事件：恢复已验证的Ultra工具调用设置

v22/v23在模型默认推理行为下持续返回非原生工具调用且JSON外层括号不完整，最终任务3ed13f1c-974d-418f-b356-4a88b0d949f1失败INVALID_JSON。已恢复此前可执行原生工具的reasoning=false设置，没有用字符串修补伪装合法输出；其余分页、批次纠错、来源语义和当前Sheet上下文修复保留。原生工具/非法输出纠错/历史隔离定向检查及Go构建通过，v24继续同一独立来源。

## 已完成里程碑事件：切换/恢复Sheet时隔离工具上下文

v22退回NS提案后，模型仍看到历史GB/OOB调用并输出跨Sheet提案。当前轮工具历史现仅保留active_sheet的来源/错误；完成一张表后清空同批剩余调用，完整历史仍留MinIO。已新增跨Sheet历史隔离检查，完整internal/ai与Go构建通过，准备v23继续原独立测试。不存在整本成功结论。

## 已完成里程碑事件：辅助列语义误映射拦截

Ultra-03的NS表484条八字段虽与来源匹配，额外却把纤芯数量绑定length_state、厂商绑定cable_label，故不能宣布该表正确。来源校验现拒绝厂商/品牌列冒充现有工程字段，以及任意值冒充四类线长状态；恢复检查点也重新校验并退回这种提案，不静默改值。原生工具调用不再强制reasoning=false，使用所选模型默认行为，旧JSON协议保持原样。AI与engineering定向包、Go构建通过；v22准备部署继续同一来源，未发布任何正式数据。

## 已完成里程碑事件：Ultra首张端接表独立核对通过

v20修复批量首错后，Ultra-03已提交Cabling_EW_SP-LF完整区域。独立OOXML基线逐行对照2592条，A/Z设备、A/Z端口、A/Z光模块、线缆规格、长度八字段全部2592/2592；额外绑定仅有来源medium，没有旧轮的辅助列冒充label。其余14张未完成，不代表最终模型通过。另补齐读取阶段收敛：连续六次成功来源读取后要求先提出理解，批量读取也不能越过此阶段；完整AI包和Go构建通过，准备v21继续。

## 已完成里程碑事件：修复批量工具首错未返回模型的问题

真实Ultra任务addd7201-c3ed-4f6b-b3b7-c2ec592f415d的最后三次CELL_NOT_FOUND均在同一响应批次中、约0.4秒内执行，模型没有收到中间反馈就被NO_PROGRESS终止，不能归因于模型三轮纠错失败。现任一Schema/工具错误立即清空尚未执行的批次，下一轮先返回错误证据；原进度上限和校验不变。新增三次错误工具同批返回后仅执行一次、模型纠正成功的用例。

## 已完成里程碑事件：资料读取分页及坐标纠错

v18真实Ultra任务c02c518e-a090-4fcd-814b-70b60acf562d因反复把end_row当行数而失败，尚无通过Sheet。工程读取工具现复用旧BOM Agent的有界分页方式：大范围返回最多32行/64列、实际returned_bounds及截断标记；反向坐标明确提示end_row为绝对坐标。完整读取判定使用实际返回范围，未放宽来源覆盖。internal/ai检查和Go构建通过；准备部署v19并在同一独立来源继续。

## 已完成里程碑事件：修复首轮理解纠错反馈

用户要求继续完成，恢复使用指定Ultra真实测试。Agent现在一次返回提案中全部不支持/重复列绑定，纠错轮聚焦重新提案，不再逃逸到无关读取；公式单元格不存在时返回实际可读的公式引用。未知列仍保留来源，没有自动删除提案或降低来源门禁。完整internal/ai检查和Go API构建通过；首次检查因未设置既有KEEL_BOM_PYTHON失败，改用现有专用Python环境后通过。代码准备部署v18并继续独立测试来源，整本理解尚未通过。

## 已完成里程碑事件：用户指定Ultra直接实测并停止

直接使用nvidia/nemotron-3-ultra-550b-a55b:free上传原MM文件到全新独立项目TEST-Agent-MM-Ultra-02，未继承旧模型完成结果。首次读取成功，但连续三次模型提案因connection.fabric、connection.label、cable.label等错误字段被拒绝；还观察到多列重复label。随后又三次查询不存在的公式单元格（CELL_NOT_FOUND），任务自行失败ENGINEERING_AGENT_NO_PROGRESS，停止后续重试。0/15表完成，未产生工程实体或发布记录；证据已持久保存。没有可用于量化“比DeepSeek差多少”的同条件成功基线，只能确定本轮未达到Agent可用标准。详见[Ultra实测记录](docs/intelligent-deployment/ultra-model-test-20261101.json)。

用户此前指定HeyGen已在一次合成调用确认其为语音合成模型、不能运行聊天/工具任务；随后用户切换Flint，再明确改为Ultra并要求不核查400。本轮按最后指令直接跑Ultra，未继续其他模型诊断。

## 已完成里程碑事件：原生工具Schema命名冲突修复

- v16真实调用HTTP400；独立合成请求精确复现上游错误duplicate $defs entry CellBinding has conflicting schemas。上游把不同工具的定义合并到同一命名空间，域收敛工具与全域分类工具发生冲突。
- 现仅为收敛后的映射工具定义及引用加独立命名空间；分类能力和本地校验不变。修改后同一上游协议检查HTTP200/tool_calls，定向Native/域约束测试和Go构建通过。
- v17准备上线继续同一来源，原v16失败没有新增完成Sheet。模型协议检查不代替真实整本理解验收。

## 已完成里程碑事件：按Agent域判断收紧纠错工具

- v15任务最终因NS表重复在connection里输出quantity而失败ENGINEERING_AGENT_NO_PROGRESS；同一检查点中EW2592、GB_OUT3456、OOB216共6264条的八个关键字段均匹配来源。EW另有cable_label指向Fiber core count的可疑额外映射，不能把八字段匹配扩称全部字段正确。
- 现在从Agent最近的区域分类限定下一轮映射工具的kind/field枚举；例如连接域不再提供quantity。分类仍由Agent提出，不由Sheet名或样例模板决定；classify_information保留以允许显式改判。检查点和强制来源校验保持。
- 完整AI包和域工具收敛用例、Go构建通过。额外真实模型表头探针：Gemma429；Super-VL原参数格式错误、展开Schema后仅4/8字段正确，未切换正式测试Provider，不冒称通过。

## 已完成里程碑事件：显式关键表头不能退为未知资料

- v14中GB_OUT 3456条与OOB216条的八个端接/线缆字段逐行匹配独立源基线；EW/NS却被模型退为other，故整本仍未通过。保留这种退化的真实记录，不用区域数量代替理解质量。
- 对已有明确A/Z设备+端口表头，或明确设备标识+型号+机柜/序列号表头的区域，完成门禁要求对应关键列实际绑定；不自动生成业务行，不限制未知客户表述，不按Sheet名选类型。旧检查点重验会退回这种other提案。
- 读取工具说明明确32行/64列只是采样上限，不限制整表映射。工程域/AI包检查与Go构建通过，新增打乱列序和任意Sheet名的端接不能被other吞掉用例。

## 已完成里程碑事件：消除分页边界造成的假表头

- v13真实任务结束ENGINEERING_AGENT_NO_PROGRESS；NS表虽覆盖全范围，仍把第二分页块第一行257误判表头，独立基线只能匹配483/484条。未宣称该表通过。
- 现在向Agent展示连续占用区域，合并IR的人工分页边界，真实空行分隔仍保留；没有按示例行号修数据。检查点增加解释版本，旧策略产生的提案不再自动恢复成已完成，原MinIO证据仍保留。
- 完整AI包检查和Go构建通过，新增不同起始位置/连续分页/真实空行分隔用例。下一轮需重新分析旧分块提案；此前第一张2592条通过记录保留为该次结果，不能移花接木到新任务。

## 已完成里程碑事件：读取分块与整表边界分离

第二张真实端接表反复只映射首个256行读取块。来源预览现明确给出full_sheet_bounds，并标记结构区域为读取分块而非业务表；范围工具复用现有读取校验，仅精简模型上下文中的重复API元数据。最新纠错反馈放在末尾，相同工具/Sheet错误三次明确失败（不受提案文本变化绕开）。完整AI包检查和Go构建通过；已通过源字段核对的第一张表保留，从MinIO继续其余Sheet。

## 已完成里程碑事件：真实端接表首个完整字段对比

API v12 / 743383f已部署；任务28d974d0-b1e0-4203-9b59-b8daa7d9dfdb使用独立Lightning版本2继续原资料。Cabling_EW_SP-LF已映射完整2595行区域，独立OOXML基线的2592条端接数据在A/Z设备、A/Z端口、A/Z光模块、线缆型号、线长八字段均逐行匹配2592/2592。该结论是区域映射与原始来源对比，不是最终模型/公式/CMDB验收；其余Sheet仍在分析。私有检查点与checkpoint-coverage.json位于既有测试目录。

## 已完成里程碑事件：整张来源覆盖门禁

- 精简提示后端接关键列映射正确，但真实结果只圈住预览的前2/5行。此为实际漏识别缺陷，未作为成功交付；v11测试已受控停止。
- 完成单Sheet前现在校验全部非敏感有效单元格是否被提案区域覆盖；遗漏返回具体范围和数量要求纠正，未知区域必须显式保留为other。旧检查点同样重新校验，避免恢复时沿用漏行提案。不要求未知字段强行映射，也不按示例Sheet或行数硬编码。
- AI/工程域定向检查通过，新增后部数据与公式区域不能被预览漏掉的用例，Go构建通过。继续原来源复测并逐字段比对。

## 已完成里程碑事件：首轮提示与错误上下文收敛

- 同一Lightning模型在独立、打乱列顺序的简短语义探针中映射正确；不能仅归因模型能力。真实工作流中长全域提示、重复错误提案污染上下文，导致按词表顺序编列号。当前v10任务已受控停止，状态TEST_STOPPED_REPEATED_SEMANTIC_ERROR，无成功模型。
- 首轮理解改用精简来源专用提示，复用注册表和字段能力；错误提案完整留MinIO，下一轮只带具体工具错误，不把错误大提案作为assistant示例。相同错误提案连续三次失败明确结束为ENGINEERING_AGENT_NO_PROGRESS，保留检查点。
- 完整AI包检查（精简提示/反馈）及新增重复错误停止用例通过，Go构建通过。继续同一独立来源真实复测，不把简短语义探针当整本验收。

## 当前在线状态（覆盖下方历史快照）

当前后续开发方向已由用户调整为五步固定输入先行、分步Agent后接入；方案见 `docs/intelligent-deployment/FIVE-STEP-REFACTOR-PLAN.txt`，研究和验收细则见 `docs/intelligent-deployment/RESEARCH-BACKED-IMPLEMENTATION-20261101.txt`。本轮仅设计，尚未实施，以下在线版本保持。

API v40 / ceab753已部署8081；5180沿用原页面，已在线修正fixture资源断链并验证页面启动。独立TEST-Agent-MM-Ultra-03，项目proj-18101842-d821-4583-9229-a708f995c1c8，来源71a5c797-319b-496d-a6ce-f67c1080439c。Provider仍为用户指定Ultra版本3；默认test/DeepSeek未变，未进行模型对照。

当前任务c3788ecc-f911-465f-bb4c-bc667ca70d8d恢复revision2检查点后技术成功，已完成15张表候选、保存完整MinIO提案和工程草稿revision3。首轮资料理解结论PARTIAL / NOT ACCEPTED，见SOURCE-UNDERSTANDING-REVIEW-20261101.txt；任务技术状态由ultra-repair-test-20261101.json登记，技术成功不等于语义通过。上一版v35草稿revision2的3118条问题是历史记录，不能当本轮问题数，也不能当纯AI错误数。

v38–v40已修复区域分类反馈、身份/附加字段冲突和待修草案跨表/重启持久化，相关定向检查与构建通过。当前仍缺可靠的独立资料理解成果；主要残留为模块依据、空间/跨表关系、数量描述及摘要合并一致性。本轮停止增加工程字段校验，不再启动重复分析；保留来源与结果，没有正式发布CMDB或拓扑。

非阻断待办：model API大规模evidence分页；本轮不扩展UI/API分页。后续业务验收、跨客户泛化和完整参考重算未通过或未执行，不能根据P0-P5代码存在宣称完成。

## 已完成里程碑事件：按工程域纠错与非法输出恢复

- 真实Lightning任务在两个Sheet后返回INVALID_JSON；此前通过全局字段词表的部分映射也不符合领域含义，不能当成成功。现按实际提取器能力限制每个区域的字段，恢复时重新校验旧检查点并退回不合法的Sheet。
- 复用现有通用表头词表提供候选提示，按可识别表头证据优先处理清晰表格；没有固定Sheet名、列号、设备型号或业务常量。复杂图表保留来源和疑问，不强行变成设备行。
- 模型非法JSON/截断输出最多要求纠正两次，原响应写MinIO取证，仍执行严格Schema。完整internal/ai与internal/engineering定向包检查通过，新增跨域错误映射拒绝和非法工具参数纠错用例。真实整本理解仍待本轮在线复测。

## 当前复测切换：免费Lightning模型与模型版本记录

- Ultra免费模型的响应正文超时已通过新诊断确认（response_timeout，120秒）。仅将独立测试Provider `59c504bf-a8c8-42db-9afc-b89f70a0f2d5`更新为`nvidia/nemotron-3.5-lightning:free`、版本2、名称TEST-Agent-Free-Nemotron-Lightning；原test/DeepSeek及默认选择不变。新模型原生工具检查HTTP200、费用0。
- 工程作业现在在持久步骤记录实际模型和Provider版本，响应中断保留明确错误码。Go构建通过；从同一来源的MinIO检查点继续真实测试，不把旧模型结果混记为新模型验收。

## 已完成里程碑事件：阻止无新增证据的读取循环

- 完整小表已读完，或同一区域被重复读取时，首轮理解只允许提交提案/明确不确定项；提案仍必须通过Schema和确定性来源校验，不能用假数据绕过。
- 真实免费模型还两次遇到响应正文中断（PROVIDER_RESPONSE_FAILED），现在区分响应超时/中断并作有界重试；取消任务仍立即退出，永久额度错误不重试。
- 完整internal/ai包检查通过（含原生调用、阶段工具、字段词表、未完成Sheet恢复、上游错误/中断），Go API构建通过。正在恢复真实任务；此前没有有效整本模型，不把100余次读取当理解成功。

## 已完成里程碑事件：资料分析焦点与证据上下文

真实免费模型一次读取多个Sheet，而最近六步上下文丢掉先前返回的证据，导致重复整批读取（v6作业已超过100步仍无有效Sheet提交）。现将首轮资料理解限定为一个active_sheet，并持续提供该Sheet的有界来源预览；读取后只暴露该阶段所需的来源/公式依赖/提案工具，保存后推进下一Sheet。完整工具实现仍保留，模块适用性和工程依赖不按Sheet顺序硬编码。已有原生工具、Schema限制和中断恢复定向测试通过，Go构建通过；正以既有MinIO进度继续实测。

## 已完成里程碑事件：原生工具字段词表与工程模型统一

真实免费模型连续输出不支持的cable_type，说明仅靠提示词列字段不充分。工具参数的Column/Constant/CellBinding.field及Module.module_id现在直接使用现有工程字段/模块注册表枚举，批准的类型扩展按项目版本加入；不建立另一套业务词表。HTTP工具契约定向测试确认cable_model可选、cable_type不被宣告，Go构建通过。继续用已有MinIO检查点验证真实纠错，不放宽本地来源校验。

## 当前继续路线：独立免费真实模型复测（不修改原默认Provider）

- 在同一OpenRouter服务发现并实际验证`nvidia/nemotron-3-ultra-550b-a55b:free`原生工具调用HTTP200且费用0；新建非默认Provider `TEST-Agent-Free-Nemotron`，ID `59c504bf-a8c8-42db-9afc-b89f70a0f2d5`。原test/DeepSeek配置完全保留，结果不能混记。
- 独立项目`TEST-Agent-MM-Free-01` / `proj-2754da3a-cc48-4464-bbf7-ec483cfa4e06`；来源`a5c5d704-9cbd-4b6e-a7d4-15c7fca51222`；作业`69c70880-5861-4ab9-830c-ae8ab0db4931`运行中。15次Sheet读取已经完成，首次提案因未知字段cable_type被拒绝；正在按真实错误继续修复，不宣称已理解成功。
- 本机资料/证据仍在`/home/ubuntu/.local/state/keel-tests/mm-agent-repair`，新manifest为`live-free.json`。原DeepSeek额度问题保留；异步充值问题仍待答复，但使用该独立免费模型继续推进当前目标。

## 当前验收状态：修复已上线，真实理解尚未通过

- 当前API代码`f98c8ea`，分支dev；逐工具MinIO检查点与网格纠错反馈已部署。原5180只更新上传分流，主分支和其他功能开发未改动。
- 最后一轮真实MM任务因Provider HTTP 402结束，不能宣称识别完整。项目保留以供充值后继续及用户清理；无CMDB资产和发布记录。详见[真实复测记录](docs/intelligent-deployment/agent-repair-test-20261101.json)。
- 已完成代码/存储/契约定向验证；未完成整本语义、关键设备/端接覆盖、参考计算和跨客户验收。用户充值/切换有额度Provider的异步问题仍待答复，不擅自更换模型或伪造成功。

## 已完成里程碑事件：未完成Sheet的逐步恢复与明确映射纠错

- 每个成功工具步骤和Schema纠错反馈均保存检查点，包含有界工具历史、已读取Sheet和已完成Sheet；失败/重启后连未完成Sheet也可恢复，不需要重读整段资料。
- 真实模型曾把网格多列重复映射为label；继续严格拒绝，现明确反馈重复字段与越界列的具体原因及拆区域/records的修正方式。提示词强调每个对象的实体类型，避免将机柜行当成机房；没有修改Excel或放宽事实校验。
- 定向协议/错误分类检查、未完成Sheet中断恢复新用例、工程域现有检查和Go构建通过。前端工程Adapter定向TypeScript及88项接口登记一致性检查通过。真实MM整本理解仍未通过，Provider额度问题待用户处理。

## 已完成里程碑事件：分析产物恢复及清理验证

隔离PostgreSQL与合成S3验证通过：新的Service实例恢复MinIO检查点，跨项目/旧版本拒绝，SHA256篡改检测，丢失claim时孤立对象清理，以及来源删除级联清理产物索引。真实MinIO已有工具产物写入/读取证据。没有运行全平台回归，也未用这些检查替代真实MM理解验收。

## 当前外部阻塞：真实Provider账户额度（已通知用户补充）

- OpenRouter只读账户查询证实：API Key的limit_remaining约45.14是Key限额，账户total_credits=410、total_usage约409.9931，实际仅余约0.0069美元；不是可用45美元。真实多轮调用持续返回402/in_flight_budget_exhausted。已异步请用户充值当前账户或提供其他可用Provider，用户尚未答复。
- 当前独立测试项目`proj-bbf16ed0-8dcd-4d95-bdb0-8a4eea3b3a3d`，工程来源`f821c09d-c4c3-4284-8a2c-859e0b8eacfc`，v4任务`d906ba9b-1f23-490b-90dc-9a8f6a40a218`已因不可重试HTTP 402结束；最新账户剩余约0.0031美元，不能覆盖当前请求。未发布资产。不要把已修复协议和存储当整本理解成功。
- 当前部署API代码f98c8ea；[部署记录](docs/intelligent-deployment/deployment-agent-repair-20261101.json)记录二进制摘要、进程、迁移038和回退入口。前端是原4064a69加工程上传Adapter，不含其他分支开发。

# Keel 整合交接 · 2026-10-31

## 已完成里程碑事件：工具返回消息与上下文修正

- 原生工具结果现在使用标准assistant/tool消息配对，避免把历史结果当普通用户数据而反复读取；最近上下文按64KiB和六步限制，完整产物仍在MinIO。
- 原生调用、恢复检查点、禁止正式审批工具和上游错误分类的定向测试通过；API v4已上线。真实MM分析仍未完成；上游两次返回临时额度占用，需要按120秒要求等待。
- 本阶段文档台账检查曾因新部署标识未同步部署证据而失败；现已补齐独立部署记录，需以本次实际检查输出为准。

## 当前任务：真实资料 → Agent 理解修复（dev，用户授权在线修改及复测）

- **已完成里程碑事件：定位失败根因并部署首批修复。** 实际复现上游HTTP 402，metadata.reason=`in_flight_budget_exhausted`、Retry-After=120；旧实现丢弃原因且直接终止。不是Excel解析失败，也不把合成Provider连通性当资料理解验收。
- 工程Agent改用原生工具定义，保留严格本地Schema与来源校验；支持一轮多个工具按序执行、逐Sheet检查点、恢复时跳过已完成Sheet。暂时性402/429/502/503/504按Retry-After等待重试，永久402不伪装可重试。
- 迁移038已应用；工具输入/输出、检查点、最终提案压缩写MinIO项目专属前缀，流式经过私有临时文件；数据库仅登记对象键/大小/摘要/作业关系，工程事实继续使用现有关系表。失败不发布半成品模型；已有检查点可恢复。旧eng_job.proposal不再重复写大提案。
- 上传新增`import_mode=engineering`，仅解析原始资料，不启动旧BOM Agent；5180维持4064a69前端基线，只更新工程上传Adapter。API来自当前dev修复；没有部署作业平台新UI、合并main或修改真实资产。
- 运行/回退：`/home/ubuntu/.local/state/keel-deployments/engineering-agent-repair`（私有）；原`engineering-5abb94f`回退版本保留；仍使用原8081/5180、配置、数据库、凭据和MinIO。代码工作树不用于存放在线部署补丁。
- 已验证：Go构建、原生工具/断点恢复/上游错误分类定向测试通过；真实新上传没有旧bom_job，工具产物在MinIO实际写入且能解压读取。真实MM资料仍在分析中，尚未完成理解验收，不宣称设备/线缆/公式正确。独立项目`TEST-Agent-MM-Repair-01`，本机私有证据`/home/ubuntu/.local/state/keel-tests/mm-agent-repair`。
- 当前恢复入口：`dev/backend/internal/ai/engineering_agent.go`、`cmd/api/engineering_job.go`、`cmd/api/engineering_artifacts.go`；只推进理解闭环，不扩展CMDB发布/整个平台回归。


## 最新指令：恢复作业平台UI独立开发（2026-11-01）

作业平台UI从最新dev同步到 `feat/jobs-kestra-ui` 后继续，工作区 `/home/ubuntu/keel-kestra-ui-dev`；其他业务继续dev。此前“作业平台也统一dev”和暂停要求被本次指令覆盖。仅开发，不发布、不合并业务代码至dev/main。具体进度继续更新作业平台实施记录。

## 当前开发约定：dev统一入口，旧Demo归档（2026-11-01）

- 后续开发统一Git `dev`、代码目录 `dev/`，工作区 `/home/ubuntu/keel-dev`。main保留稳定版本；不再从旧作业平台或部署分支继续堆补丁。作业平台仍约20%且暂停，下一次接续按下方清单在最新dev完成。
- 旧Demo历史Tag `archive/keel-mock-first-20261101` 保留4492819及其347个不在当前main祖先链的提交；旧应用完整快照Tag `archive/pre-dev-cleanup-20261101` 保留fc07c70。Tag已推送后才删除本地/远端demo分支。
- 本次移除keel-demo/及旧web/，根Compose只保留PostgreSQL/Redis/MinIO；dev脚本不再依赖旧Demo的node_modules或向旧目录复制契约。保留现有fixtures、原型、业务代码和真实数据。
- 本次是代码/Git整理，不执行docker up/down、服务切换、数据库迁移、AI调用或业务测试。现有5180/8081/5181及其他工作区原样保留，不把新dev工作区冒称当前运行环境。
- 实际验证：归档Tag已推送；更新的JS/Python脚本语法检查通过；Compose配置仅返回minio/postgres/redis；88项真实API登记/423项设计操作的文档引用检查通过；暂存和未暂存差异检查通过。没有运行完整构建、业务回归或浏览器检查。


作业平台继续开发先读 [2026-11-01接续检查点](docs/flows-redesign/implementation.md#2026-11-01-作业平台接续检查点)：原生UI仍约20%，保持暂停；已明确下步顺序和保护最新工程部署的边界。

## 当前交接：用户认可进度，合并智能开局工程至 main

- 本次将智能开局工程`4064a69`与远程main已有的作业平台整合`e265628`合并，保留双方实现；只有部署文档追加位置冲突，两个历史部署记录均保留。
- 本地主工作目录`/home/ubuntu/keel`使用main并同步GitHub。原始`TX B300 Layout-new.xlsx`仅本地忽略，不删除、不提交。其他开发工作树保持独立。
- 当前已部署工程代码仍为5abb94f（部署记录4064a69），迁移037；前端继续由`/home/ubuntu/keel-engineering-p0/dev`提供。Git合并不额外发布main中的原生作业平台，也不改变数据库。
- 合并检查：Go API构建、88项接口文档一致性及暂存区空白检查通过；未重复完整前端构建、业务回归或浏览器验收。
- 用户认可当前开发进度；真实AI/资料/Ground Truth/浏览器与跨客户验收仍按实际记录，不能由Git合并推导为已通过。
- 下方此前整合和暂停记录是历史快照；工程P1–P6开发与部署的当前状态见本次工程记录及`docs/intelligent-deployment/implementation.json`。


## 最新操作：Git 分支整合

- 用户授权整合 `enzoOvOyu/keel-new` 当前工程的开发分支；工作区 `/home/ubuntu/keel-integration`，分支 `integrate/all-keel-branches`。只合并和推送源码，不部署、不修改数据库、不切换其他工作区。
- BOM/项目管理以 `audit/intelligent-deployment-p0`（314126d）为基线，加入原生作业平台 `feat/jobs-kestra-ui`（327b953），保留验收记录并合并重复 BOM 修复的历史。重复修复采用当前已演进代码，未恢复旧来源/部件推断规则。
- 原生作业平台开发仍暂停，约20%功能完成；BOM工程P0仍等待用户审核。Git整合不是功能开发或验收完成。
- `demo/keel-mock-first`、`design/prd-v3.0.3-20261020`、`dev/p1-network-bom` 是与当前工程没有共同祖先的旧工程历史，远端分支保留，不把其代码混入当前目录。
- 当前运行环境以实际进程为准；以下分支快照中的“当前部署”均为历史记录，本次不重新确认或改变服务。详细整合与检查结果见 [整合记录](docs/integration-20261031.md)。

## 作业平台接续快照

## 当前任务接续入口：Kestra原生作业平台（已按用户要求暂停）

- **状态**：用户最新要求“记录handoff然后暂停”。已停止后续开发；当前整体完成度约20%（功能估算），不是完整交付。恢复时从本节继续，不重扫全部代码。
- **分支/目录**：`feat/jobs-kestra-ui` / `/home/ubuntu/keel-kestra-ui-dev`。原生列表/时间线`a9ae943`；原生代码/多面板壳层`3df73d7`；QuickRun/模板修复`564a1e5`。本次暂停提交还包含下述Outputs增量。已推送到`enzoOvOyu/keel-new`对应分支；未合并main、未部署。
- **Git推送认证已核实**：使用原项目私钥路径`/home/ubuntu/keel/deploy_key`（公钥`deploy_key.pub`），不是`~/.ssh/id_ed25519`或`keel_github_ed25519`。本仓库local `core.sshCommand`已固定该IdentityFile和IdentitiesOnly；不要打印/复制私钥到Git。当前功能分支及其余本地已提交分支均已成功推送origin，remote仍为`git@github.com:enzoOvOyu/keel-new.git`，main未合并/覆盖。开发仍按用户要求暂停。
- **唯一方案**：[KESTRA-UI-REPLACEMENT.md](docs/flows-redesign/KESTRA-UI-REPLACEMENT.md)，包括用户新增的LC-01…14生命周期范围。直接复用固定Kestra commit `49b62a2c820d37e98a1c7b63aa35fd83f960d4ec` 的Vue组件；Keel保留React外壳、项目/权限/CMDB/审批/Agent与SSH边界。
- **实际完成**：原生design-system与React/Vue桥、样式隔离；两列表基础接线、搜索/状态URL恢复；RunTimeline接线；KsEditor/Monaco与原生多面板壳层（YAML+只读结构）；Outputs原生SidebarList搜索、选择、预览、复制/下载接线。结构摘要不是No-code，Outputs侧栏不是完整变量/表达式调试器。
- **实际验证**：列表构建与三个浏览器检查通过；编辑器构建与三个浏览器检查通过（挂载、键盘输入同步、Visual/YAML切换保留）；Outputs构建与三个浏览器检查通过（挂载、搜索选择更新详情、JSON下载）。Outputs检查仅为浏览器注入的明确synthetic执行，未写业务库，不能当真实引擎产物验收。证据：[native-ui目录](docs/flows-redesign/native-ui/)。
- **已修故障/限制**：编辑器ASI初始化错误已修；一次完整构建137退出，裁减为本期YAML所需generic Worker后通过。JSON/TypeScript语义Worker未启用。编辑器结构面板间距样式在截图后定向修正，完整最终视觉及拖动/合并/最大化/布局恢复仍待验证。旧React详情/Visual仍在过渡中。
- **立即接续事项**：优先完善原生No-code节点/属性与Topology、Files/Docs，接入完整执行详情；同时补列表服务器分页/total/最后执行摘要，避免永远停留在本地分页。已检查上游FlowController/search、QueryFilterFormat源码位置，但尚未编写新目录查询接口。
- **其余未完成**：完整KSFilter、列管理、批量动作；原生Gantt/TaskRun/Attempt/日志/完整变量/指标/依赖；LC并发/排队/补跑/跨执行检索/源码搜索；QuickRun/Agents替换；证书轮换/持久批量安装、设备事件触发、ZTP消费者及真实API契约与联调。不要把软件缺口归类为只等真机。
- **代码入口**：`dev/src/features/jobs-native/`（新Vue模块）；`dev/src/vendor/kestra/`（来源与许可证）；`features/flows/FlowsWorkspace.tsx`（React消费者）；`features/flows/Builder.tsx`（YAML入口）。`dev/scripts/import-kestra-components.py`只从显式提供且commit匹配的上游checkout复制依赖。临时参考checkout `/tmp/keel-kestra-ui-source`不是交付运行依赖。
- **验证脚本/日志**：本机`/tmp/keel-native-ui-check.mjs`、`/tmp/keel-native-editor-check.mjs`、`/tmp/keel-native-outputs-check.mjs`用浏览器拦截提供分支dist，不开新端口；脚本登录读取本机私有环境引用，不得输出/提交秘密。最新构建日志`/tmp/keel-native-outputs-build.log`。恢复时无代码变动不重复已过检查。
- **环境保护**：5180仍由`/home/ubuntu/keel-flows-dev/dev`运行；本分支未切换服务。其他窗口持续更新8081，下面BOM v5段落只是本分支历史，不得当当前二进制。部署前核验负责窗口最新运行版本，不能用本分支旧后端基线覆盖BOM修复。迁移025未由本任务应用，恢复时先检查跨分支迁移编号是否占用。不要停止其他窗口服务、导入资产或恢复旧合成Agent。


---

## BOM / 项目管理接续快照（运行版本描述为历史）

# Keel 当前开发交接

## 已完成里程碑事件：智能开局工程部署

- 当前环境已运行代码`5abb94f`，分支`audit/intelligent-deployment-p0`；原5180前端与8081 API均已更新。入口：项目 → BOM 开局配置（`/p/:projectId/deploy/bom`）。本节覆盖下方历史部署版本。
- 数据库`keel`已应用027–037；完整备份及可读取的备份目录已保留，迁移前后项目/导入/资产/用户数量不变。没有创建工程测试任务、导入资料或发布资产。
- 原API环境逐项保持，仅增加固定镜像摘要的`KEEL_CALC_IMAGE`；原AI Provider、受控凭据路径、对象存储及Flows配置沿用。隔离Calc和Pydantic校验器可用，未调用外部AI。
- 实际检查：Go构建、前端打包通过；首页/健康/新工作台模块200；登录后工程任务、规则、属性、物理版本四个GET及Provider选项200，检查会话已注销。启动日志无panic/fatal。没有浏览器或完整业务验收，没有全量冒烟/回归。
- 启动：tmux `keel-cmdb-api`/`keel-dev-5180`；私有启动器、完整备份和环境快照位于`/home/ubuntu/.local/state/keel-deployments/engineering-5abb94f`，不打印或提交其内容。后端保留原工作目录以复用秘密引用；前端源目录为本工作树的`dev/`。
- 部署证据、二进制摘要及回退入口见[deployment-20261101](docs/intelligent-deployment/deployment-20261101.json)。真实XLSX/真实AI/Ground Truth/参考计算/跨客户/浏览器验收仍待用户安排。


## 已完成里程碑事件：P6 工作台与计算输入收口

- 完成：相关落位/路由输入变化使旧线长计算失效；估算/计算/确认长度改变使沿用的规则选型失效。无关机柜修改不清空该线缆结果，原始规格、确认值和实际安装证据保留在对应状态/版本。
- 编辑与提示：属性按其实际批准版本展示类型/单位，排队计算显示排队状态，分析及审核/计算错误提供可执行的中文提示；人工领域修订关联来源字段，仍执行确定性校验。
- 实际检查：`TestCableCalculationTracksRelevantInputs`通过，包含相关/无关输入、三种长度输入、派生证据失效与原始规格保留；增量TypeScript及88项已登记接口的文档一致性检查通过；不是HTTP/浏览器验收。
- 当前阶段：P1已完成，P2–P6代码已收口；未部署，真实Provider/XLSX/Ground Truth/参考计算/跨客户/浏览器验收未完成。下方旧里程碑中的“剩余开发”仅表示当时状态。

## 本轮里程碑时间线（UTC，2026-11-01）

- 02:23:42：继续首批纵向链开发。已起草结构化存储迁移027、工作台/任务/区域映射类型；尚未完成集成，未应用迁移、未部署。用户要求：每个里程碑记录真实时间与检查结果，持续推进至开发完成。
- 02:51:42：首批后端闭环定向检查通过。隔离数据库 `keel_engineering_dbf4e8ad88` 的合成资料完成类型化草案、人工审核、分域Outbox投影、幂等重放、现有机柜图查询；规划U位修改保留Observed原payload及无关域发布状态。没有真实AI/原始XLSX入库、没有当前环境迁移或部署。工作台已接接口，正在完成类型与Agent协议检查，尚未提交本阶段。

- 02:59:31：S1开发里程碑。新增合成检查验证在途重复发布拒绝、已确认值保护、结构变体和单位换算；隔离数据库链、AI协议、TS、文档一致性均通过。真实资料与当前环境未验证/未部署。记录后提交并继续计算与后续工程域。

- 03:05:15：P3计算内核检查通过。隔离Calc镜像构建完成；合成工作簿跨表公式独立重算为20，除零保留为错误、原件与缓存未被覆盖。公式范围/命名区域/循环/动态引用及单位/规则审批门禁检查通过。计算持久化与工作台仍在接入，未部署。

- 03:23:14：P3计算链里程碑。规则版本/人工审批、持久计算任务、独立结果/依赖/错误、公式字段证据关联已接入。隔离PostgreSQL检查通过，人工无关修改保留来源冲突；8个接口文档校验通过。计算工作台及案例切换最终增量类型检查通过。尚未部署。

- 03:26:24：P3开发完成，提交`fa2fc37`已推送。最终增量TypeScript通过；新增8操作/13DTO并扩展4个相关Schema。继续连接/线缆，真实资料和当前环境仍未验收/部署。


- 03:54:24：S3连接/线缆里程碑。已完成任意区域端接提取、父/Breakout端口、配线前后端口对、分域审核/发布、类型化持久化与工作台修订。隔离库验证外部两段规划线缆可由现有拓扑查询读取；未把估算线长/采购光模块变为实际资产。核心校验、AI协议、TS与75操作文档检查通过。原有Trace仍为单段接口，多跳不冒充已验证。继续电力/冷却及后续域，未部署。

- 04:03:54：S4电力/冷却里程碑。类型化功率依据/容量/冷却与内置单位计算、持久计算运行、批量结果绑定、独立审核发布已实现。合成2kW负荷/1kW余量及隔离DB闭环通过，拒绝过期/篡改结果，修改输入清除旧值；计算结果也可应用线缆计算/标准长度。AI严格Schema及TS检查通过。继续采购/施工与网络域，未部署。

- 04:23:40：S5采购/施工里程碑。物料需求/已采购/备件/缺口及工程量/标准长度/工时分别结构化保存，支持来源提取、人工修订、规则结果绑定、独立发布。隔离DB验证工程量3→4更新、旧数量阻断与重算替换、无采购实物资产；核心单位/缺口不补造/标准长度分布及TS检查通过。采购规则未固化；缺少逐物理段标准规格保持partial。继续网络域及统一编排/重导入，未部署。

- 04:47:04：S6网络规划里程碑。十类逻辑规划、跨来源已发布物理版本引用、独立审核/发布及Golden输入门禁已实现。隔离DB验证完整往返、发布依赖及失效阻断、无新增CMDB资产；跨表顺序合成检查与TS通过。Golden仅提供审核后的规划输入，未实现厂商配置渲染/下发。继续动态ImportPlan、重导入差异和工作台收口；未部署。

## 已完成里程碑事件：S7 资料理解与模块计划闭环

- 完成：ImportPlan、模块实例/输入、区域绑定、语义事实和生产者溯源结构化保存；状态来自实际审核/作业记录，适用模块自动排队内置计算。Agent支持横向参数和跨表单元格记录。
- 前端：统一工程总览与来源证据浏览，各领域编辑冻结版本，避免旧表单覆盖。保留现有机柜图。
- 实际验证：合成记录提取/独立模块、隔离DB持久化与计算选择、AI合成协议、TypeScript通过。无真实资料/真实Provider/浏览器验收，未部署；迁移034仅隔离库。
- 剩余：跨文件重新导入/差异决策、已移除规划关系同步清理、有类型扩展落地、线缆分段规格及工作台最终收口。
- 用户更正：后续在里程碑事件完成时记录完成内容、验证结果和剩余事项；不是持续记录时间。以前的记录保持不动。

## 已完成里程碑事件：P5 新版资料差异与规划撤回

- 完成：独立新版资料关联、精确基准、按域差异处理（采用新版/保留基准/确认保留已有值）、审核审计及原有发布门禁。基准之后发生域变化时拒绝旧版本覆盖。
- CMDB：同资料版本链/同域的旧规划证据转历史、旧关系结束有效期、撤回纯规划资产退役；保留Observed、其他来源和审计。现有拓扑排除退役对象；祖先相同发布目标失效。
- 实际验证：隔离合成库的旧版发布→新版差异→人工决定→线缆撤回→空目标端口撤回通过，现有拓扑读取无残留线缆，Observed payload未变；TypeScript通过。没有真实资料、真实AI或浏览器验收，未部署。迁移035仅隔离库。
- 剩余开发：P4分段线缆规格与类型化扩展、P6交互收口。后续按用户的P0–P6阶段汇报，不再使用S编号代替阶段完成度。

## 已完成里程碑事件：P4 分段线缆规格

- 完成：CablePlan支持明确外部物理段序号；带配线节点的每段独立规格、长度状态、来源和计算依据。施工量按逐段标准长度汇总，缺失段不套用总长。
- 实际验证：合成两段3m/5m持久化、独立规格分布及原拓扑两段查询通过；缺少段规格/序号保持不完整，TypeScript通过。迁移036仅隔离库，未部署、无真实文件/浏览器验收。
- 剩余开发：P4类型化扩展与P6最后交互收口。

## 已完成里程碑事件：P4 类型化扩展

- 完成：项目属性定义版本、人工批准、类型/单位/枚举/必填约束与工程对象值；使用明确关系表，核心身份不被扩展覆盖。Agent只能引用已批准定义并通过来源提取；物理对象扩展关联CMDB planned证据。
- 实际验证：合成来源80GiB属性、未批准定义拒绝、false值保存/读取/发布、必填缺失阻断、合成Agent协议及TypeScript通过。迁移037仅隔离库；未部署、无真实资料/浏览器验收。
- 剩余开发：P6最终交互与计算输入变化的收口；真实样本Ground Truth、真实Provider与跨客户验收不冒充已完成。

## 当前任务：通用智能开局工程重构（已部署，待业务验收）

分支`audit/intelligent-deployment-p0`，工作树`/home/ubuntu/keel-engineering-p0`，目标仓库`enzoOvOyu/keel-new`。后续按用户定义的P0–P6汇报；在里程碑事件完成时记录内容、实际检查与缺口，历史记录保留。

- P0：现状/样本分析报告已提交（314126d）。
- P1：三层架构、通用/领域模型与模块契约已完成（582b19e、b25915e）。
- P2：多轮Agent工具、列/跨表单元格建议、确定性提取、区域/事实/来源持久化及动态ImportPlan已实现；真实AI识别验收未运行。
- P3：可替换计算Provider、隔离Calc、公式依赖/错误/缓存分离、项目规则版本/案例/审核及计算记录已实现；真实样本参考结果对比未运行。
- P4：五批领域模型、编辑、校验和发布链已接入；包括分段线缆规格和批准后的类型化扩展。NetworkPlan只提供审核后的Golden输入，不包含厂商配置生成/下发。
- P5：分域审核/事务Outbox、跨文件版本差异决策、保留确认值、同版本链旧规划撤回已实现；Observed与审计保留。
- P6：统一资料理解、工程总览、来源证据、计算、问题、差异、审核发布工作台已接入；本次完成计算输入失效、旧属性版本编辑及操作提示收口。未新建机柜图。

本轮开发收口；没有把代码实现等同完整验收。剩余交付事项：真实XLSX与真实Provider联合测试、人工Ground Truth、可靠参考计算对比、第二份真实不同结构资料的泛化验证，以及浏览器交互验收。完整闭环和集中测试由用户安排，不自行继续全平台冒烟/回归。

实际检查包括核心合成用例、隔离PostgreSQL审核/发布/Outbox/既有机柜与拓扑查询、多轮AI合成协议、隔离Calc合成参考公式、TypeScript和接口文档一致性；逐项范围见`docs/intelligent-deployment/implementation.json`，接口与限制见`docs/api/engineering.md`。这些检查不能证明真实识别质量或跨客户泛化。

部署状态：代码5abb94f及新增33个HTTP操作已发布到原5180/8081，迁移027–037已用于业务库；原始工作簿未修改。此前合成隔离库`keel_engineering_dbf4e8ad88`仍独立保留，若清理仅删除该明确命名隔离库。私有启动、备份和检查材料不入Git。

## 用户决定：停止旧Agent补丁并重新开发开局配置

已按用户指令清理13个测试/验收项目，关联BOM导入、项目资产及CMDB专属数据已删除，13项对象清理均完成；保留审计、审批、共享型号、3个固定演示项目及工作目录原始XLSX。清单见 `docs/topology/test-project-cleanup-20261031.json`。不要继续对旧测试批次重试或恢复旧Agent修补任务；开局配置模块按用户后续新需求重新开发。本次仅实际清理及结果核对，未运行冒烟/回归。


## 当前前端热修复：删除项目不重载页面

前端代码 `0deb3a6`，当前5180源目录未变，通过Vite热更新生效；后端继续994bb1c。删除后在页内更新授权快照、项目列表与项目缓存，后台刷新Session，不再清空整个QueryClient或强制刷新页面。修正启动占位/真实删除Modal说明，拒绝旧授权资产请求回写，空授权清空资产。未运行冒烟、回归或实际删除，详情见项目删除记录。


## 当前部署：项目删除（2026-10-31，覆盖下方历史环境）

用户已授权部署，运行代码 `994bb1c`，集成项目删除及 Flows `95df50f`，保留此前BOM/拓扑修复。8081运行 `/tmp/keel-api-project-delete-1031`，5180前端源目录改为 `/home/ubuntu/keel-project-delete-deploy/dev`；API原配置逐字节保留。启动器及私有部署记录 `/tmp/keel-project-delete-deployment-1031`，不得打印环境文件。迁移025 Flows游标、026项目删除已应用。项目列表提供删除确认；没有实际删除任何项目，没有运行冒烟/回归。Go构建及受限内存前端打包通过，合并后的完整TypeScript构建被终止，不能声称类型检查通过。全库备份失败，完成结构备份；部署详情见 `docs/topology/project-deletion-deployment-20261031.json`。


## 项目删除功能开发（未部署）

独立工作树 `/home/ubuntu/keel-project-delete-dev`，分支 `feat/project-delete`。实现项目列表删除确认、Session/CSRF及project:delete校验、项目专属数据事务清理和上传对象Outbox重试；保留审计、审批和共享型号/资产。迁移026尚未执行。用户要求停止测试后不再运行检查；共享版本引用修正后未复测，实际限制及检查见 `docs/topology/project-deletion-20261031.md`。此节不改变下方当前运行版本。

## 当前运行环境 · BOM 新文件根因修复（2026-10-31）

此节覆盖下方旧运行版本。后端 `7a49bca`，分支 `fix/bom-source-closure`，工作树 `/home/ubuntu/keel-bom-closure-dev`；8081 运行 `/tmp/keel-api-bom-v6p-flows-1031`，启动器 `/tmp/keel-bom-v6p-run-api.py`。前端5180、Flows及原配置未替换。部署证据 `/tmp/keel-bom-v6p-20261031/deployment.json`，秘密环境文件不得打印或入Git。

新文件33/33提取完成，216/216显式类型核对通过；整批确认/提交曾422，完整闭环仍未通过。用户已批准并执行一个空能力的测试型号登记。实际分批入库：30台交换机、14个机柜、8个端口，共52个唯一资产；两次Outbox均已发布，8/8端口归属正确。项目资产58条是版本历史，不是58个唯一资产。此前口头40交换机/4机柜统计不准确，以此次实际API/数据库结果为准。

本次补修CMDB尺寸类型/空值导致投影失败、分批提交候选ID未映射正式资产ID导致依赖失败；部署后同批重试通过。层级/Fabric返回52资产，物理线缆仍0，四类连接矩阵均空，Trace为no_record。线缆速率证据、端点/占用冲突及其他候选审核尚未闭环，不能把HTTP200或资产入库当作完整拓扑通过。不猜测源文件缺失事实，不继续重复全量识别。

独立项目 `proj-d8a90883-3361-465f-8793-7b18346cf0b8`，导入 `fd0f48a6-32e0-440f-a958-9c9c1d545527`。全局测试型号revision `2417e33c-afb1-4414-9e53-2022af410b72`需单独记录清理，删除项目不会删除它。原文件未修改。实测/清理记录见 `docs/topology/bom-real-subset-acceptance-20261031.json`，修复清单见 `docs/topology/bom-source-closure-fixes-20261031.json`。私有原始证据 `/tmp/keel-tx-layout-v6-20261031` 不入Git。

## 当前运行环境 · BOM v5 与 Flows 合并发布（2026-10-31）

用户已授权发布本次 BOM 根因修复。当前后端源码 **fdffbcb**（在 Flows 部署基线 `6884439` 上集成 `67c7f1d`），独立分支 `deploy/bom-v5-flows-1031`、工作树 `/tmp/keel-bom-v5-deploy`。运行 `/tmp/keel-api-bom-v5-flows-1031`，API `127.0.0.1:8081`；5180前端仍从 `/home/ubuntu/keel-flows-dev/dev` 运行。没有替换前端、Flows依赖、数据库或配置。该节覆盖下方旧运行版本及“未部署”的历史记录。

- 已确认运行二进制的源码版本/摘要，进程环境与发布前逐字节一致，工作目录保持 `/home/ubuntu/keel/dev/backend`。
- 集成版 API/AI/BOM/CMDB/Flows 定向测试与构建通过，18 项启动/只读检查通过（含登录、BOM进度、拓扑矩阵/物理追踪及Flows）。无活动任务，正式提交/资产数量前后不变。
- 本次未运行新模型识别、真实文件入库或浏览器绘图验收。已有批次的旧候选不会因更换二进制自动重算；需从工作台按用户验收安排继续分析。
- 启动器 `/tmp/keel-bom-v5-run-api.py`；回退启动器 `/tmp/keel-bom-v5-rollback-api.py` 恢复原 `/tmp/keel-api-flows-combined-1031`。私有运行环境快照位于 `/tmp/keel-bom-v5-deployment-20261031/`，不得打印或入Git。
- 后续构建/发布必须包含 `67c7f1d` 或等价集成 `fdffbcb`；不要直接用未集成这项修复的旧工作树后端覆盖当前8081。

发布证据：[bom-v5-deployment-20261031.json](docs/topology/bom-v5-deployment-20261031.json)。


## 2026-10-31 · 五轮 BOM 验收缺陷根因修复（当前）

用户已重新授权彻底修复已发现问题，覆盖下方历史暂停开发记录。代码在独立分支 `fix/bom-acceptance-root-causes`，工作树 `/tmp/keel-bom-root-fix`，基于已部署的 Agent+Topology 集成代码；未修改其他开发者分支，未部署、未重启服务。

- 修复整批/局部识别状态混淆、表头修正后的行资格丢失、错误字段语义、重复光模块/候选 ID、合并后的引用悬空及第一次关联不稳定。诊断版本 `workbook-agent/v5`。
- 两份原始 XLSX 离线回放各得到 189 条交换机、7,154 条布线候选，其中 7,150 条关联两端物理端口；4 条来源依赖缺失仍阻断。162,715 个源单元格全部保留，NS 第 3 行保持一条线缆，图候选集合重放稳定。
- 现有 GPU/NIC/BMC/PSU 等物理部件可以拥有来源明确的端口；拓扑保留独立部件归属，有上级设备时优先聚合到设备，歧义和跨项目访问继续阻断。没有把部件改成服务器。
- P2P-18 旧方案不满足物理端口契约，被拒绝作为线缆；估算/数量信息完整保留，仍需正确参考/物料方案和人工处置。型号审批、单位、尺寸及原始缺失信息未被补造。
- Go API/AI/BOM/CMDB 定向检查、两文件本地回放、PostgreSQL 投影幂等/完整拓扑读取/部件归属隔离检查通过。新增合成测试位于独立项目的回滚事务中，无持久化测试资产。
- 本轮无新外部模型调用、无真实文件正式提交、无浏览器绘图验收。不能把离线恢复结果当作新模型准确率或上线验收。

逐问题记录：[bom-root-fixes-20261031.json](docs/topology/bom-root-fixes-20261031.json)。到此结束开发，后续部署/集中验收按用户安排。

更新：2026-10-31 UTC。先读本文件，再按下列索引定位；无需重扫全仓。历史交接可用 `git show 6601444:HANDOFF.md` 查阅。

## 当前已发布环境 · 2026-10-31（覆盖下方开发阶段记录）

- 用户现已授权发布；入口恢复统一为 **http://localhost:5180/flows**，API仍`127.0.0.1:8081`。临时5190/8096与合成本地Agent已关闭；不要重新启动它们。
- 运行源码 **9f16065**，分支`feat/flows-redesign`、目录`/home/ubuntu/keel-flows-dev`，未合并或推送main。4403c63/9f16065保留原8081的两项BOM Agent修复，相关BOM/AI文件与原部署分支比对无差异。
- 前端tmux`keel-dev-5180`从本工作区`dev/`启动；API tmux`keel-cmdb-api`运行`/tmp/keel-api-flows-combined-1031`，工作目录继续为`/home/ubuntu/keel/dev/backend`，保留原数据库、对象存储、凭据路径和账号。
- 原业务库新增迁移021…024，未装载任何隔离合成资产。备份位置见`/tmp/keel-flows-deploy-backup-current`，私有目录含部署前完整pg_dump与原进程环境，不得打印或入Git。
- API启动器`/tmp/keel-flows-run-deployed-api.py`读取私有原环境备份和`dev/deploy/flows/.env.deployed`覆盖；后者只配置引擎/Agent/SSH受控路径，不覆盖原DATABASE_URL或密码。Worker经Docker bridge调用现有5180代理，不新增用户入口。
- Kestra8095与Agent网关8097/TLS注册8098是本机后端依赖，不是新增前端入口；真实设备公网TLS/Bootstrap仍需部署条件。
- [发布检查](docs/flows-redesign/deployment-validation.json)：原admin登录、5个Flows只读API、3页面渲染共9项通过；容器到5180健康200、无票据worker回调403。没有设备执行、BOM AI调用或真实数据导入。
- 应用回退：停当前两个tmux服务，使用备份中原API环境恢复`/tmp/keel-api-agent-topology-1031`（同原cwd），前端从`/home/ubuntu/keel-visualization-dev/dev`恢复原5180命令。新增表不自动drop，数据库无需为了旧应用读取而破坏性回滚；完整dump只用于独立授权的数据恢复。

以下Flows独立环境描述为部署前记录，当前端口和运行版本以上述发布节为准。

## 2026-10-31 当前任务：Flows 全栈功能分支（先读）

- 工作区 **`/home/ubuntu/keel-flows-dev`**，分支 **`feat/flows-redesign`**；代码提交`63ca2c9`、`826b964`、开发补齐`afe8cb9`。没有合并/推送main，也没有更改原5180/8081运行服务。
- 任务包原文及八页draw.io在`docs/flows-redesign/`；优先读[实施记录](docs/flows-redesign/implementation.md)与[真实API契约](docs/api/flows.md)，无需重扫旧Flows或全仓。
- 独立前端 **http://127.0.0.1:5190/flows**，API `172.22.0.1:8096`，Kestra `127.0.0.1:8095`，gRPC `localhost:8097`，TLS注册`https://localhost:8098`。远程浏览需端口转发；没有发布到原5180。
- Docker项目`keel-flows`，Postgres中分开`kestra`与`keel_flows_dev`。后者仅原库schema+合成项目/资产，未复制原业务行；迁移021…024只应用这里。不要运行写死原容器的`backend/migrate.sh`。
- 私有配置/新隔离admin密码：`dev/deploy/flows/.env.local`，0600且忽略；原账号密码未改。PKI在同目录`pki/`。API只复用原`.env`的对象存储配置，DATABASE_URL由隔离配置覆盖，禁止输出秘密。
- API `/tmp/keel-flows-api`，PID `/tmp/keel-flows-api.pid`，日志同名`.log`；Agent `/tmp/keel-flows-agent`，PID同名`.pid`，state `/tmp/keel-flows-agent-state`。不要删除state或重复注册。重启前核验PID的exe，不要kill原8081。
- 实际通过：Go定向测试/构建、TypeScript+生产构建、真实Kestra+双合成SSH协议失败重试/幂等、实际非rootAgent/mTLS实时输出、原生Pause/Resume、文件读写、版本冲突、游标与跨项目拒绝、8项浏览器交互。
- 本轮已补齐：Kestra驱动的作用域批次调度、每目标多步骤与输出传递、二进制版本/分发、持久事件游标重放、既有审批与单次执行绑定、实际任务取消。新增API/迁移/测试证据见实施记录。
- 能力边界：每目标32个串行依赖步骤、步骤结果32KiB、二进制资源1MiB；Agent0.2.0支持job.pipeline。待验收仅生产Bootstrap/NOS/真实8目标混合/规模与长期运行，不得宣称生产通过。测试Cron已停用。
- 修改定位：`dev/src/features/flows/`、`dev/src/lib/api/flows.ts`、`dev/backend/internal/flows/`、`cmd/{api/flows.go,agent/}`、迁移021…024、OpenAPI FlowsReal。规则和运行证据链接均见实施记录。

以下为原BOM/拓扑环境的保留历史，勿把运行版本与当前Flows分支混用。

## 2026-10-31 后端 Agent 部署更新（优先于下方历史版本）

当前8081已切换为 `/tmp/keel-api-agent-topology-1031`，源码 `ad8918b`，来自独立集成分支 `deploy/bom-agent-topology-1031`（工作区 `/tmp/keel-agent-deploy`）。该版本保留拓扑基线 b184645，并集成 Astra 的 e3f6cd3、74ab238 Agent 修复。启动配置仍使用 `/home/ubuntu/keel/dev/backend/.env`，前端5180仍使用本工作区。本分支没有合入这些后端代码，后续重新部署时必须保留集成修复，不能直接从旧后端源码重建覆盖。

定向 Go 检查、构建和部署启动检查通过；没有新启动真实识别/入库验收。证据见 `docs/topology/agent-deployment-20261031.json`；旧二进制 `/tmp/keel-api-4541850` 保留用于回退。下方4541850部署描述是历史状态。

## 当前分支与运行环境

- 仓库目标：`enzoOvOyu/keel-new`。工作区 `/home/ubuntu/keel-visualization-dev`，分支 `feat/datacenter-visualization-approved`；未合并 main。
- 已部署业务代码 `4541850`；后续交接/契约文档提交不改变运行代码。
- 前端 http://localhost:5180 ，tmux `keel-dev-5180`，目录为本工作区 `dev/`。
- 后端 `127.0.0.1:8081`，tmux `keel-cmdb-api`，运行 `/tmp/keel-api-4541850`。配置和存储工作目录仍为 `/home/ubuntu/keel/dev/backend`，不要复制或输出 `.env`。
- 原目录 `/home/ubuntu/keel` 的源码仍在 `keel-new-main` / `978db7f`。5181 批准原型未调整，额外 5182/8082 已停止。
- 用户已授权本次部署；本轮没有数据库迁移、资产导入或主分支合并。后续文档维护不隐含再次部署授权。
- 启动与回退命令见 [DEPLOYMENT](docs/DEPLOYMENT.md) 末节；不要为了接续开发重复启动或重启。

## 已完成及入口

| 页面 | 当前入口 | 实现位置 |
|---|---|---|
| 全局拓扑 | `/topology?view=global` | `dev/src/features/infrastructure/approved/pages/global-topology.tsx` |
| 网络架构 | `/topology?view=network` | 同目录 `network-architecture.tsx` |
| 物理布线 | `/topology?view=cabling` | 同目录 `physical-cabling.tsx` |
| 链路追踪 | `/topology?view=trace` | 同目录 `link-tracing.tsx` |
| 机柜可视化 | `/racks?view=gallery` | 同目录 `rack-detail.tsx` 及 `approved/rack/` |

页面共享范围、URL、缓存由 `approved/ApprovedTopologyWorkspace.tsx` 维护，组件不能各自创造资产身份。服务器与交换机详情继续复用现有抽屉。

## API 与代码定位

首先查 [真实 API 契约总入口](docs/REAL-API-CONTRACTS.md) → [可视化使用契约](docs/api/visualization.md) → [机器注册表](docs/api/real-api-registry.json)。字段权威为 [OpenAPI](docs/openapi.yaml)，不在交接里重复定义 DTO。

- 真实接口：GET subgraph、matrix、connections，POST trace；兼容 GET visualization。完整路径、operationId、参数及错误见上述契约。
- Go 路由：`dev/backend/cmd/api/cmdb.go`；查询 `dev/backend/internal/cmdb/visual_queries.go`，兼容投影 `visualization.go`，位置校验 `visual_placement.go`。
- 前端 Adapter：`dev/src/lib/api/infrastructure.ts`；生成类型 `dev/src/lib/schemas/openapi.ts`。
- CMDB 是唯一资产事实源。布局草稿不修改安装事实；实际位置变更走现有 ChangeSet。物理/逻辑/运行观测分层，缺数据不能伪造连接或正常状态。

## 实际检查与限制

- 已有构建/类型生成、Go 编译及定向纯逻辑检查通过，见 [代码检查记录](docs/topology/code-validation.json)。纯文档收尾不重复运行。
- 已部署四个新接口启动 HTTP 200，五页面加载，无 pageerror 和失败可视化请求；兼容旧接口不在本次四接口探测范围。见 [部署证据](docs/topology/deployment-validation.json)。
- groups 查询瓶颈已修复：单次观察 6568ms → 447ms；未修改超时或数据库配置。这不是生产性能 SLO。
- 未完成真实导入验收、全部关键交互闭环或千卡/大型生产规模基准。真实数据用户后续统一联调，不能把页面加载当作完整验收。
- 逻辑/历史 Trace 缺能力时明确拒绝；物理路径存在不等于已验收通过。规划快照、精确面板、实时监控和 PDU 缺少事实时不制造展示数据。
- BOM 完整自主工具调用 Agent 的历史未完成状态保持，不因本次可视化交付宣称已解决；相关现状查 [BOM 实施状态](docs/BOM-REAL-IMPLEMENTATION-STATUS.md) 和 [工作台设计](docs/BOM-WORKBENCH-V2.md)。

## 后续修改顺序

1. 用户指出要调整的页面/行为后，先查模块契约与 [实施报告](docs/topology/implementation-report.md)，再定向读取涉及的代码。
2. 契约变化同步 OpenAPI、生成类型、Go/Adapter 和注册表；UI 调整保护批准原型及既有身份/权限模型。
3. 只运行受影响的必要检查，更新本文件与证据；不重复全仓 review，不提前导入或合并 main。
4. 文档/登记一致性：`node dev/scripts/check-real-api-docs.mjs`（只读，不调用服务）。

## 2026-10-31 用户要求暂停后的提交

- 仅调整机柜页现有视图按钮，区分“3D 机房 / 机柜阵列 / 单柜工作台”；3D 仍使用已有 `view=gallery&overlay=capacity`，没有新增独立路由、Sidebar 入口或渲染引擎。
- 用户随后纠正方向并要求停止开发、提交；未继续实现独立 3D 页面，后续需求需重新确认。变更位置 `dev/src/features/infrastructure/approved/pages/rack-detail.tsx`。
- TypeScript `tsc --noEmit` 与 `git diff --check` 通过；本次没有浏览器交互验证，没有重启服务或合并 main。5180 使用该 worktree 的 Vite，源文件修改可能已被热更新读取，不应将未重启等同于界面未变化。

## 当前 Flows 重构分支（2026-10-31）

最新任务包见 `docs/flows-redesign/`，实施索引 `docs/flows-redesign/implementation.md`。本任务已隔离到 `/home/ubuntu/keel-flows-dev` / `feat/flows-redesign`，基线 d8a91b4；不要在运行5180的可视化 worktree 继续写本任务。包内指定真实 Kestra/Go/Agent/SSH 落地，覆盖旧 Mock-only 的 Flows 阶段限制；其他业务不扩展。新底座 `keel-flows-*` 独立容器，Kestra 仅127.0.0.1:8095，未替换现有5180/8081，未修改原数据库。


历史五轮验收见 [原始报告](docs/topology/bom-import-acceptance-20261031.json)；其他旧分支交接原文由Git提交 `2f5ed92`、`664f4a5`、`327b953` 保留，不在本文件重复拼接。
