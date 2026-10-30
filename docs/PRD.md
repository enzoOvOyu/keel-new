# keel PRD v3.2 — GPU 集群交付平台

> 本文是开发执行版需求。只定义目标、行为、规则和验收边界；实现结构见 `ARCHITECTURE.md`，领域模型与流程见 `SYSTEM-DESIGN.md`，页面规范见 `DESIGN-SYSTEM.md`，开发顺序见 `IMPLEMENTATION-PLAN.md`，接口唯一契约见 `API-CONVENTIONS.md`。遇到冲突先按本文业务判据执行，并登记冲突，不自行猜测。

## 1. 产品目标与边界

`keel` 面向 B300/GB300 千卡/万卡集群交付，统一编排网络底座、裸金属装机、验机、网络线序、配置、连通性、NCCL、退场和 CMDB 交接。产品原则：交付即代码、物料唯一事实源、阶段门禁、可追溯、失败可恢复。

**范围内**：网络底座（`keel-fabric`）、BOM 与智能开局资料导入、PXE、Redfish/IPMI、六阶段交付主链路 验机门禁、Mapping、批量配置、NCCL、Flow、AI 辅助排障、优雅退出、RBAC/审计、项目模板、监控告警、型号库、CMDB、BMC/固件管理、端到端交付编排。

**范围外**：平台替客户决定网络设计方案、地址/VLAN/ASN/VNI/QoS/无损参数/端口 breakout 等配置内容；模型训练；机房动环；财务资产系统内部实现。网络方案由客户与网络工程师商定后导入。平台负责承接、版本化、渲染、校验、审批、下发、回读和漂移闭环。

## 2. 角色

以下为业务职责称谓；默认认证角色与映射以本文件“身份认证与项目授权实施补充”和 IDENTITY-RBAC.md 为准。业务签署职责不自动授予平台权限。

- `DE` 交付工程师：装机、验机、配置、测试和异常处理。
- `DL` 交付组长：门禁审核、豁免、项目放行。
- `NE` 网络工程师：网络资料、配置基线、网络故障处置。
- `NDE` 网络交付工程师：网络设备建模、开通、线缆校验和网络交付。
- `NH` 网络负责人：网络模板与 Critical 变更审批。
- `Admin/SRE`：用户、项目模板、介质、参数、运行环境与审批。
- `Viewer`：只读查看与导出。

权限按“模块 × 动作 × 项目资源范围”实施。普通网络变更由 NE+DE 双签，Critical 变更由 NE+DE+NH 三签。

## 3. 端到端主流程

唯一活动主链：**开局配置 → 网络就绪 → 装机 → 验机 → NCCL测试 → 优雅退场**。

设备/机柜/批次/项目均按相同六阶段记录统计；前序未通过不解锁后序。验机中的硬件、GPU/NIC、PCIe、网卡/MAC、LLDP、DDM、配置一致性和必要RoCE测试是子任务，不是主阶段。BOM是独立部署菜单，不属于Provision。网络就绪必须先于PXE/OS；NP-1…NP-6仍为网络内部断言，不是六个交付阶段。

详细状态机、放行条件与验收见[DELIVERY-LIFECYCLE.md](DELIVERY-LIFECYCLE.md)，历史编号迁移见[DELIVERY-MIGRATION.md](DELIVERY-MIGRATION.md)。

## 4. 核心功能需求

### FR-0 网络底座 `keel-fabric`

1. 交换机、交换机端口（含 breakout 子口）、线缆、光模块进入统一 BOM；不得另建网络专属物料事实源。
2. 维护项目级已商定网络基线、模板、地址与命名规则、VLAN/ASN/VNI、QoS/无损参数、breakout 与端口角色。模板按版本发布，使用 pongo2 v6 沙箱，禁止模板文件/网络访问和绝对路径。
3. 支持 Cumulus ZTP/SZTP。设备身份优先级 `SN > 管理MAC > 基板MAC > 其他`；身份冲突拒绝业务配置，只允许隔离/管理引导。首次 ZTP 不下发生产业务配置。
4. 网络配置变更唯一流程：意图承接与渲染 → Doublecheck（语法/语义/一致性）→ 备份 → diff → 预检/仿真 → 审批 → 下发 → 复采验证。审批前禁止下发；调优建议不得自动改写设备配置。
5. 设备状态、LLDP、FDB/ARP、端口、光模块 DDM、syslog、BGP/PTP、PFC/ECN/CNP/队列/缓冲等按能力采集。缺少能力返回 `unknown` 并标注降级，不能将无数据当 PASS。
6. 线序校验使用 BOM 期望、服务器侧 LLDP、交换机侧 LLDP 三方比对。差异映射至既有 T1–T7 工单，不新建平行故障分类。
7. 网络规则、阈值和模板版本化、审批、dry-run、回滚；所有结论记录规则版本。
8. 网络证据通过统一契约供服务器域消费；第三方网络控制器只能通过同一适配器契约接入，不支持的能力返回 `unknown`。

### FR-1 作业平台

- Flow → Task → Runnable；YAML 声明式，支持输入、变量、子流程、重试、超时、并发、失败分支、触发器。
- 支持 Agent 与 Agentless（SSH）。门禁判据在两种模式下相同；Agentless 的持续监控和空闲日志流可降级展示。
- 执行记录包含目标、日志、产物、参数快照、版本引用和审计。数据库为权威状态；事件采用 Outbox 与幂等消费者。副作用任务超时/断连进入 `result_unknown`，核实真实状态后才可重试。
- 目标组支持按机柜/rail/BOM/网络角色分组，支持暂停、终止、重试失败目标和熔断。

### FR-2 BOM、开局导入与 PXE

**BOM 唯一权威定义**：统一 `bom_item`。`category` 至少支持 `server/nic/gpu/psu/bmc/switch/port/optic_module/cable/other`。核心字段包含 `bom_import_batch_id`、`bom_line_no`、不可变 `item_uid`、类别、型号引用、序列号、位置、父引用、线缆两端端口引用、端口信息、rail/pod/role、MAC、凭据批次引用、期望版本、来源和状态。业务展示码不作为唯一键。

**FR-2.1 智能开局资料导入（BOM 起点）**

1. 网络团队可上传原始 `.xlsx/.xlsm`（仅允许受控格式）、多 Sheet、多行表头、合并单元格及不同人员编写的非统一表格。原始文件不可变保存，记录 SHA-256、上传人、项目、时间和模板版本。
2. 解析器读取工作簿结构、Sheet、单元格、合并区域、公式和值，保存 `文件/Sheet/单元格` 来源位置；不先转图片做 OCR。宏不执行；外部链接不访问。
3. 字段映射顺序：项目模板中已审批的别名/规则 → 确定性解析规则 → AI 语义映射候选。AI 只给候选字段、归一值、来源位置、理由与置信度，不得直接写入正式 BOM，也不得决定网络方案。
4. 映射对象包含 `source_field`、`target_field`、转换规则、适用范围、版本、状态、来源和审批记录。未知列不得静默丢弃；进入 WARN 问题清单。AI 新发现的映射需人工确认后才能沉淀为模板规则。
5. 标准化流程：上传 → 解析 → 映射 → 生成候选数据 → 三段校验 → 人工修正/确认 → 生成标准 BOM 导入包 → 只读预览 → 确认提交。导入和发布必须显式操作，禁止上传即落正式 BOM。
6. 三段校验：格式/类型/必填；跨表引用、身份、重复、型号与 MAC/IP 冲突；工程规则（型号能力、端口 breakout、光模块速率、线缆两端、rail/拓扑等）。级别 `BLOCK/WARN/INFO`。BLOCK 未清零不得提交；WARN 必须显示并按模板策略确认；关键身份、端口与链路歧义不得自动放行。
7. 审核界面必须可回看原始单元格、标准字段、映射理由、校验规则、修改前后值；支持批量确认低风险映射、逐项处理歧义、导出 CSV 问题清单和修正后重导。
8. 审核通过后生成符合附录 A 导入包契约的标准数据。服务器与网络物料一次性提交为同一 `bom_import_batch_id`，同一事务内落统一 BOM；跨域线缆引用必须唯一解析。
9. 同批次同摘要幂等；同批次异摘要拒绝覆盖，须新 seq/新版本。所有映射、校验、修正、审批、提交动作可审计。
10. 单文件服务器宽表兼容入口可保留；该入口不能声明网络物料齐备，也不能解锁网络开通或跨域线序校验。
11. 网络配置方案仍由客户与网络工程师商定；智能导入仅提取和标准化已提供信息，不自动补造 IP、VLAN、链路、端口或 QoS 决策。

**BOM 标准导入包**：`manifest.json` + `server/servers.csv`、`server/server_components.csv`、`server/server_ports.csv`、`fabric/switches.csv`、`fabric/switch_ports.csv`、`fabric/optic_modules.csv`、`fabric/cables.csv`。可选 `network_baseline/`、`address_plan.csv`、`credential_batches.csv`。标准目录包需要声明全部七个 CSV，允许零行文件，但必须通过项目完整性校验。UTF-8 无 BOM；压缩包仅 ZIP/TAR.GZ，拒绝路径穿越、符号链接、明文凭据、私钥和 token。完整列契约以 `API-CONVENTIONS.md` 中的 Schema/附录为实现依据；原始资料字段由模板映射到这些标准字段。

PXE：UEFI HTTP Boot 优先、Legacy PXE 兜底；ProxyDHCP+iPXE+HTTP；按 SN/MAC 幂等识别；装机完成自动验证 SSH 互信。

### FR-3 资产采集

以 Redfish 为主、IPMI 为补充，采集 BMC、CPU、内存、存储、网卡、供电、温度、BIOS、启动项等；原始 JSON 存对象存储，结构化数据入 PostgreSQL。记录来源、采集时间、版本、质量和 freshness。BMC/交换机凭据仅存 Vault 引用，不存明文。

### FR-4 验机、Mapping 与工单

验机在装机阶段有效通过之后启动。硬件在位/健康、GPU/NIC及默认GPU1–8↔NIC1–8配对、PCIe拓扑、网卡状态/MAC、双侧LLDP与BOM三方线序、DDM、配置一致性、RoCE等分别作为内部检查任务，独立记录结果和证据；不得用数字门禁再次组成交付主链。

FAIL/BLOCKED必须自动生成或关联统一Issues，绑定项目、设备、端口、check_key、原始证据及Execution。整改与受影响范围定向复验完成前，对应设备不能判验机通过。MappingRevision仍不可变、可追溯，配置实施与一致性复核分开。全量批次必须分别显示成功、失败、阻断及待处理；不能用单设备成功掩盖其他失败。

### FR-5 服务器批量配置

ConfigSet 版本化；驱动、固件、OS、CUDA、NCCL、网卡分割、GPU-NIC 绑定和 RoCE 参数来自项目模板/参数目录。版本选择使用已审批介质下拉项，不自由输入版本字符串。变更必须有目标范围、预览、校验、审批、执行和回读。

### FR-6 NCCL 与网络连通性

NCCL属于第五主阶段，只能对验机通过或合法限定豁免、且验机工单已整改复验关闭、网络测试前置满足的目标运行。NP-5/NP-6继续由网络域提供证据。NCCL结果只更新本阶段，不反向更改验机。报告绑定Flow/Execution、BOM、Mapping、配置和真实覆盖范围。NetTest/RoCE保留为网络/验机支撑功能，不增加主阶段。

### FR-7 自定义作业

提供 YAML Flow 编辑/校验/版本发布、运行历史、日志流、目标矩阵、输出物和失败重跑。危险动作必须经权限、租约和门禁检查。

### FR-8 AI 辅助排障

通过可配置 API 端点调用模型；输入限于当前用户有权查看的日志、作业、资产和证据。回答必须引用来源，未知信息明确标记，不得自动执行生产变更。

### FR-9 优雅退出

清理本次交付创建且登记归属的临时文件、账号、Agent、缓存和作业残留；清理步骤可恢复、可审计。完成后设备进入 CMDB `maintained`，仅保留批准的 IPMI-only 带外能力；SSH、Agent、Redfish 不得因后续运维自动恢复。

### FR-10 平台基础能力

RBAC、项目/批次、审计、通知、配置管理、操作历史、导出、健康检查。敏感值脱敏；秘密值不进入参数快照、日志或导出。

### FR-11 项目模板与参数目录

模板是项目/机型差异的唯一承载体：型号、服务器基线、网络基线、BOM 字段映射、参数默认值、Flow 蓝图和门禁策略。模板版本不可变；发布需校验和审批。参数目录提供已批准、未退役的版本与阈值选项。

### FR-12 监控告警

统一采集来源、时间戳、质量、能力和 freshness；规则数据化、版本化、审批和 dry-run。无法采集返回 `unknown`，不得默认为正常。原始数据、趋势、告警、日志和工单互相链接。

### FR-13 型号库

统一管理服务器、NIC、GPU、BMC、交换机、端口和光模块型号版本与能力矩阵；BOM 工程校验引用已发布型号版本。

### FR-14 CMDB

管理稳定资产身份、项目资产关系、BOM/配置/Mapping/交付证据版本与维护态；导入、RMA、位置变化保留历史，不复用稳定 ID。

### FR-15 BMC 与固件

Redfish/IPMI 带外操作、固件升级、凭据批次轮换；危险操作必须使用设备独占租约、审批和验证。

### FR-16 端到端项目编排

统一展示网络阶段 N-S0..N-S9、服务器 六阶段交付主链路、Flow、工单和交付状态。所有阶段状态来源于持久化证据，不由前端推断。

### FR-17 自动机柜图生成

- 基于已确认 BOM/CMDB、型号库能力、机柜/U 位及已审批布局规则自动生成版本化机柜图；机柜图是派生视图，不是第二套资产事实源。
- 支持多机柜、正面/背面、U 位刻度、设备详情、端口/线缆追溯、冲突提示、未分配设备、布局 diff、拖放调整、撤销/重做及 PNG/SVG/PDF 导出。
- 设备高度优先使用已发布型号库能力；机柜/U 位优先使用已确认 BOM/布局数据。缺失或歧义时只生成 planned 候选/未分配项，不得推断为实际落位。
- 生成先预览、校验、diff，再人工发布。重生成默认保留人工覆盖；不得静默覆盖已发布版本。
- 校验 U 位越界/重叠、重复设备、未知高度、机柜容量、full-depth 正反面一致性和线缆端点引用。缺失的电源/重量/散热参数标 `unknown`。
- 图纸版本、布局调整、发布与导出关联 BOM revision、型号库 revision、生成器版本、审计和 CMDB 时间线。详细规则/API/验收见 `RACK-DIAGRAM.md`。

## 5. 网络前置断言

- `NP-1`：目标链路两端交换机与端口在统一 BOM 中建模，breakout 子口独立成行。
- `NP-2`：交换机 ZTP 完成，最近 Golden 比对无 Critical 漂移。
- `NP-3`：验机内部LLDP三方线序检查通过并写入PortModel；不作为网络就绪/装机前置。
- `NP-4`：网络就绪证据在 `gate.readiness_ttl=30s` 内刷新且 LLDP 可读。
- `NP-5`：全网健康通过（BGP 邻居 100% Up、光功率在阈值内、要求 PTP 的项目已锁定）。
- `NP-6`：无损基线可用；不可采集的能力标 `unknown`/降级，证据不足为 BLOCKED。

结论统一使用 `PASS/FAIL/BLOCKED/WAIVED`。`WAIVED` 必须有授权人、范围、原因、有效期与审计；不得伪装为 PASS。

## 6. 核心数据与状态规则

- BOM 物料身份：有序列号实体 `(category, serial)`；端口 `(parent_uid, port_index)`；线缆 `(category, sorted(endpoint_a_uid, endpoint_b_uid))`。`bom_line_no` 仅用于溯源，不是身份键。
- 导入状态：`uploaded → parsing → mapping → validating → review_required|ready_to_commit → committed|failed|cancelled`。修正后重新校验；提交前检查数据版本未变化。
- GateEvidence：`PASS/FAIL/BLOCKED/WAIVED`；证据必须含对象、规则版本、采集时间、有效期、来源与引用。
- PushJob 状态：`CREATED/PRECHECKING/BLOCKED/READY/APPROVED/REJECTED/EXECUTING/VERIFYING/COMPLETED/PARTIAL/ROLLING_BACK/ROLLED_BACK/ARCHIVED`。未审批不得执行。
- Flow 状态权威以数据库持久化状态为准；事件至少一次投递，消费者幂等。
- 标准包一次导入同时承载服务器和网络物料；不得先后分开提交再人工拼接。

## 7. 非功能要求

1. 规模：支持 10,000+ 目标、批量异步执行和虚拟化列表；高耗时工作不得占用 HTTP 请求直到完成。
2. 可靠性：幂等、重试、断点恢复、结果对账、设备租约和 fencing token；副作用未知时不盲目重放。
3. 安全：RBAC、项目隔离、密钥 Vault、上传文件安全检查、审计、敏感信息脱敏、生产变更审批。
4. 可追溯：所有 BOM、模板、Flow、ConfigSet、Mapping、规则、介质和执行均有不可变版本引用。
5. 可观测：结构化日志、指标、追踪、任务进度、错误分类与关联 ID。
6. 测试：单元测试覆盖规则和状态机；集成测试覆盖 API/数据库/队列；仿真契约 golden-file 固化设备交互；真机验证仅按批准 PoC 执行。
7. 数据正确性优先于自动化率。关键关联/配置不能仅凭 AI 置信度放行。

## 8. MVP 顺序

P0：项目/RBAC、型号库最小能力、统一 BOM、原始开局导入/预览/审核/校验/提交、审计、仿真器与 API 契约。
P1：PXE、Redfish、验机硬件检查/验机LLDP检查/Mapping发布、工单与证据链。
P2：网络 ZTP/配置闭环、装机基础配置/RoCE子任务、ConfigSet/Flow。
P3：NCCL、监控告警、AI 排障、优雅退出、CMDB 完整交接。

## 9. 验收原则

- 原始文件可追溯；未知列不静默丢弃；AI 结果不直接落正式 BOM。
- BLOCK 未清零不得提交；关键设备/端口/线缆引用必须唯一解析。
- 相同批次同摘要幂等；同批次异摘要拒绝覆盖。
- 标准导入包服务器与网络物料同批次、同事务提交。
- 每个 G/NP 门禁由真实可追溯证据驱动；缺数据不得判 PASS。
- API、数据模型、页面与测试使用相同状态枚举和字段定义。
- 各项阈值与硬件能力必须来自项目模板、参数目录或已发布型号能力，不得由实现者臆造。

## 7. 工作流与 CMDB 全链约束（全局修订）

### FR-7.1 Flow 的首期核心定位
Flow 首期面向**已纳管服务器和交换机的批量脚本/命令执行**，不是单纯的流程画布。必须支持目标筛选/冻结、脚本与参数、一次执行/定时频率、timeout/retry/concurrency/failure policy、版本、运行生命周期、目标状态矩阵、日志、产物、暂停/继续/终止/重试失败目标。执行详情参考 Kestra 的 Overview/Gantt、Topology、Targets、Logs、Outputs 和关联视图。前端先用全量 mock 交互实现，后端按模块接入。

### FR-7.2 CMDB 全链自动生成
从 BOM 导入任务创建开始，平台所有资产、端口、线缆、采集、Flow/Execution、门禁、Mapping、配置、PushJob、测试报告、工单、审批、交付与维护态数据必须自动生成 CMDB 记录或关系。CMDB 是统一资产/关系/生命周期投影，不是要求各模块手工重复录入的第二事实源。业务事务写 Outbox，CMDB projector 幂等消费、可重试/重放；投影延迟必须显示，不能伪称完成。

### FR-7.3 前端先行与 API 契约
所有主菜单页面先完成 React + TypeScript + shadcn/ui 的可交互 mock 版本；所有页面 API 必须先定义在 OpenAPI，mock 与 Go 后端共享同一 schema。不得在页面组件内硬编码业务 mock，不得在后端接入时重做 UI。

### FR-7.4 真实 Excel 解析依据
真实样本与 Sheet 分型见 `EXCEL-IMPORT-PROFILE.md`。该文件是实现解析器的专用规格。原工作簿包含敏感凭据列，禁止把原文件或秘密值放入 Git、mock、日志、CMDB 普通字段或外部模型请求。

## OPS-2026-10-29 · 交付后运维设计增补

### FR-18 服务器交付后运维
交付完成后持续提供已纳管服务器列表及独立设备详情。按项目/机柜/型号/交付/健康/告警/维护筛选。概览、CPU/内存/GPU/NIC/磁盘/PCIe、OS/内核/驱动/CUDA/NCCL、基线与实测diff、硬件趋势、BMC/SEL/启动项、固件、端口/LLDP/Mapping、作业/门禁/告警/工单/审计均为详情子页面。用户可从异常下钻证据、创建工单、发起受控采集、维修和定向复验。不可采集显示unsupported/unknown，不伪装实时健康。

### FR-19 交换机交付后运维
独立已纳管交换机列表，按Fabric/Pod/机柜/角色/型号/NOS/配置/健康/告警筛选。详情覆盖概览、approved baseline/rendered intended/running或NVUE observed及结构化diff、物理与breakout端口、LLDP、FDB、ARP、DDM趋势、BGP/系统健康、无损网络、ZTP/PushJob/审批/回滚/采集/工单/审计。所有字段带来源、采集时间、保鲜、能力、质量和适用阈值引用。配置写入仍只走FR-0。

### FR-20 持续配置与固件管理
统一批准基线、意图、实测、版本、差异三方关联；不把规划值当作观测。沿用NetworkBaselineRevision、ConfigSetRevision、PushJob、MediaArtifact、Lease与ApprovalRecord。固件库存引用具体部件和型号；升级必须校验兼容性、已批准介质、权限、通道、维护窗口、独占租约、审批、回执及升级后复验。未知兼容性或禁用通道阻断执行。

### FR-21 持续采集、观测与保留
支持定时策略、手动刷新、任务进度、超时、安全重试、历史范围、快照对比和版本化保鲜策略。MetricSample、Telemetry/LLDP/FDB/ARP/DDM快照与LogEvent通过稳定资产ID关联CMDB；时序、日志和原文使用专用查询/存储边界。失败不覆盖上次成功证据为健康，null与0不同。保留窗口及阈值引用批准策略，不臆造数值。

### FR-22 维护、维修复验与生命周期退役
完整链路：BOM→CMDB→实施→验收→运维纳管→持续监控/配置→告警/工单→维修/变更→定向重新验证→持续维护→退役。复用T1–T7工单和证据四值；维修状态不改变稳定身份或放宽访问。RMA新实物新UID。退役必须批准、停止任务、撤销凭据并保留历史。

### FR-9 / FR-14 / FR-15 的明确化
maintained不是功能终点。CMDB是统一资产身份、有效关系与生命周期事实的权威入口；业务域仍持有各自写入事实，通过Outbox投影，禁止另一份手工资产真相。按本次明确要求，maintained设备仅允许批准的IPMI-only直连；不得恢复SSH、Agent或Redfish。审批、运维页面、重试或适配器fallback均不能绕过限制；能力不足只读历史/批准的独立外部来源，不承诺不可获取的实时数据。

### 运维用户故事与验收引用
- DE/SRE从服务器异常定位GPU/NIC/配置证据，再关联工单与限定范围复验。
- NE从交换机端口下钻LLDP/FDB/ARP/DDM和漂移，仍经原PushJob审批链变更。
- DL审核维护/退役范围与历史可追溯性，Viewer只读历史；都不能重开已撤销通道。
具体页面、字段与链路见OPERATIONS-DESIGN.md；验收OPS-001…OPS-016见ACCEPTANCE.md。上述FR是设计要求，不是已实现声明。

## IA v2.3 · 双平面与统一抽屉

新增 **FR-23 IA v2.3 双平面与设备详情**：部署平面以项目为作用域，运维平面以授权项目集合为作用域，系统管理按资源权限控制。导航、页面和职责见 IA-V2.3.md §1、§5；ProjectContext、两类设备抽屉和加载协议见 §2–4。保留 FR-0…22、G/NP/PushJob和 maintained/IPMI-only。权限由服务端 permissions[] 和各次请求共同校验，前端菜单不是安全边界。

### FR-24 IA v2.4 与全局 RackView
部署以项目作用域，运维跨授权项目，系统管理独立。/projects 为项目目录；/dashboard、/fleet、/racks、/issues、/monitor、/flows、/cmdb、/ai为运维入口；/admin为系统入口。全局机房视角展示机柜分布、容量、健康与异常，下钻独立单柜编辑器，数据来自BOM/CMDB与明确来源。Fabric/Fleet/Flows均为单一一级入口，子页面见IA-V2.4.md。maintained冲突见DECISIONS IA24-01。


## IA v2.4 Mock 前端实施记录

导航树、完整页面清单、路由/权限/作用域/加载策略、五/八Tab抽屉与API对照统一见 [IA24-IMPLEMENTATION-MATRIX.md](IA24-IMPLEMENTATION-MATRIX.md)。原FR-0…24、门禁、BOM与维护态规则保留。当前实现为前端Mock，不表示已实现Go/真实设备/真实AI。权威种子为 `fixtures/demo-full-64-b300/demo-seed.json`；原包83集合，补齐与规范化后集合数量以当前manifest为准，主项目硬件数量不变。结构检查、数据修复、契约与交互结果以IA24-*报告为准，无法采集/缺少证据保持unknown或阻断。


## IA24-05 · 用户确认的GPU-NIC默认映射
用户于本轮明确：没有单独说明时，GPU1–8分别对应NIC1–8。对当前B300切片按此规则确认512条GPU-NIC关系，NIC9/10保留存储角色；不再把已有LLDP链路显示成全部unknown。矩阵行GPU、列NIC，点击显示物理端口、线缆、交换机、来源与观测；配对关系与线序/新鲜度判据分开，异常不能被配对规则覆盖。


### FR-25 六阶段交付状态机与交接
KICKOFF → NETWORK_READINESS → PROVISIONING → VALIDATION → NCCL_VALIDATION → CLEANUP_HANDOVER为唯一顺序。阶段状态NOT_STARTED/READY/RUNNING/BLOCKED/FAILED/PASSED/WAIVED。WAIVED记录审批人、理由、范围与有效期，不等于PASSED且不绕过前置。Cleanup包含执行停止、临时资源、Agent/SSH撤销、凭据处理、日志证据归档及客户签署；安全要求不可豁免，全部完成后才进入maintained/IPMI-only。旧编号只保留历史追溯。


## 当前六阶段主链路修正

本轮用户明确废弃数字门禁作为主链。当前唯一业务顺序、状态机、逐目标放行、安全交接、独立BOM菜单与API约束以[DELIVERY-LIFECYCLE.md](DELIVERY-LIFECYCLE.md)为准。历史编号只按[DELIVERY-MIGRATION.md](DELIVERY-MIGRATION.md)追溯，不得用于解锁。验机失败关联统一Issues并定向复验；NCCL不反写验机；清理/归档/客户签署齐全后才进入维护态。NP仍是网络内部断言。当前仅前端Mock与契约实现，没有真实设备后端。


## 单柜详情工作区增补

固定单柜窄画布、左侧层级导航和右侧八Tab为本轮布局要求；交互、API、元数据、Mock来源及待执行验收见[RACK-WORKSPACE.md](RACK-WORKSPACE.md)。资产/位置和既有安全模型不变。右侧按需加载，不以缺失指标显示正常。当前构建/浏览器验证按用户安排等待统一执行。


## Fabric Topology 增补

FR-26 Fabric Topology：Fabric独立Topology入口；物理布线与逻辑网络使用独立模型共享CMDB稳定ID。双端线缆/端口、逻辑层级与规划/实际Diff、双视图定位、局部展开与历史查询均按FABRIC-TOPOLOGY实现。观察页面不承担审批或生产变更。原FR与六阶段依赖保持不变。

完整导航、模型、交互、API表与实施边界见[FABRIC-TOPOLOGY.md](FABRIC-TOPOLOGY.md)。


## 无损网络分层证据 v2

FR-27：无损网络分层证据。每项绑定型号/ASIC/精确NOS/模式/批准基线；版本化新规则代替旧UI标签。PFC非零不直接故障，ECN按absolute/relative单位，watchdog按补丁实际条件。网络就绪不等待主机；验机补齐双端；NCCL消费当前目标有效证据。确认异常关联统一Issue及定向复验。

规则/官方章节/命令/版本/判据/证据对照、历史迁移、真实缺口见[LOSSLESS-EVIDENCE.md](LOSSLESS-EVIDENCE.md)。


## CMDB 全链路资产档案增补

FR-28：CMDB升级全链路资产档案。保留跨项目授权与统一设备抽屉；左侧资源树，目录/关系/配置来源/生命周期/质量五视图，项目资料辅助入口保留。搜索/过滤/保存视图/URL上下文；当前资产、规划与候选分别呈现，维护态不恢复带内通道。

唯一详细设计与模型边界见[CMDB-DOSSIER.md](CMDB-DOSSIER.md)。

## BOM 真实开发增补

用户已授权在 `dev/` 真实实现 BOM 模块。数据与权限边界、版本/审核/Outbox、真实六步工作台及运行验收统一见 [BOM-REAL-DESIGN.md](BOM-REAL-DESIGN.md)。FR-2.1 和唯一 BOM/资产身份保持；其他模块的 Mock 状态不因本模块接入而视为后端已完成。实际通过范围以 BOM-REAL-ACCEPTANCE.json 为准。

## 2026-10-30 · 身份认证与项目授权实施补充

默认身份角色固定为 viewer、delivery_engineer、delivery_lead、developer_sre、admin、network_engineer；DE/DL/NE/SRE/Viewer 为业务称谓映射，研发与管理员分离。RBAC + Project/Resource Scope 默认拒绝；权限与审批分开，申请人不得自批；NE 无服务器重装/清理执行权限。NH 业务职责保留，资格绑定待决，不映射 Admin，Critical 变更阻断。用户故事、真实范围与验收见 IDENTITY-RBAC.md。此补充细化现有角色/安全要求，不重编号或删除既有 FR。

## FR-29 CMDB 真实资产事实服务

在保留FR-14/FR-28工作台、唯一bom_item及六阶段交付链路的基础上，实现版本化Schema Registry、类型化关系、字段/关系来源、Revision与ChangeSet。CMDB统一当前资产身份与有效关系；BOM、配置、采集及作业仍持有各自领域原文。人工身份/关系变更经独立审核发布，换件新身份、旧历史保留。前端通过同形Adapter接入，禁止因后端重做UI。

接口对照、模型与实现边界统一见 [CMDB-REAL-DESIGN.md](CMDB-REAL-DESIGN.md)，具体HTTP Schema以openapi.yaml为准。

## 项目与 AI 配置真实落地

项目创建与AI配置（FR-2.1补充）：可创建真实空白项目并直接进入开局配置；系统管理提供真实Provider配置、受控凭据、启停/默认及主动连接测试；BOM使用同一配置源。Rules/人工流程不依赖AI，候选不自动提交。

唯一详细说明与API对照见 [PROJECT-AI-CONFIGURATION.md](PROJECT-AI-CONFIGURATION.md)。


## BOM Intelligence Agent 当前实施（2026-10-30）

本轮仅 AI 智能识别：上传→真实分析进度→分类审核→确认入库。交换机/端接优先，单主编排器+Workbook IR 工具+独立 Schema/来源/覆盖校验。当前范围替代此前 BOM 双模式/六步 UI 描述，历史数据不删除；详细字段、状态、API、安全和验证边界统一见 [BOM-WORKBENCH-V2.md](BOM-WORKBENCH-V2.md)。保留 FR-2.1、项目权限、唯一资产身份和原事务 Outbox。完整导入入库闭环由用户验收；不以旧报告宣称当前通过。
