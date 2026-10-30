from pathlib import Path
import re,shutil,json
repo=Path(__file__).resolve().parents[2];app=repo/'keel-demo';docs=repo/'docs'
for name in ['DELIVERY-LIFECYCLE.md','DELIVERY-MIGRATION.md','WORK-QUEUE.md']:
 shutil.copy2(app/'docs'/name,docs/name)
# Update the active business sections; retain all unrelated FRs and OPS rules.
p=docs/'PRD.md';s=p.read_text();a=s.index('## 3. 端到端主流程');b=s.index('## 4.',a);s=s[:a]+'''## 3. 端到端主流程

唯一活动主链：**开局配置 → 网络就绪 → 装机 → 验机 → NCCL测试 → 优雅退场**。

设备/机柜/批次/项目均按相同六阶段记录统计；前序未通过不解锁后序。验机中的硬件、GPU/NIC、PCIe、网卡/MAC、LLDP、DDM、配置一致性和必要RoCE测试是子任务，不是主阶段。BOM是独立部署菜单，不属于Provision。网络就绪必须先于PXE/OS；NP-1…NP-6仍为网络内部断言，不是六个交付阶段。

详细状态机、放行条件与验收见[DELIVERY-LIFECYCLE.md](DELIVERY-LIFECYCLE.md)，历史编号迁移见[DELIVERY-MIGRATION.md](DELIVERY-MIGRATION.md)。

'''+s[b:]
a=s.index('### FR-4 ');b=s.index('### FR-5 ',a);s=s[:a]+'''### FR-4 验机、Mapping 与工单

验机在装机阶段有效通过之后启动。硬件在位/健康、GPU/NIC及默认GPU1–8↔NIC1–8配对、PCIe拓扑、网卡状态/MAC、双侧LLDP与BOM三方线序、DDM、配置一致性、RoCE等分别作为内部检查任务，独立记录结果和证据；不得用数字门禁再次组成交付主链。

FAIL/BLOCKED必须自动生成或关联统一Issues，绑定项目、设备、端口、check_key、原始证据及Execution。整改与受影响范围定向复验完成前，对应设备不能判验机通过。MappingRevision仍不可变、可追溯，配置实施与一致性复核分开。全量批次必须分别显示成功、失败、阻断及待处理；不能用单设备成功掩盖其他失败。

'''+s[b:]
a=s.index('### FR-6 ');b=s.index('### FR-7 ',a);s=s[:a]+'''### FR-6 NCCL 与网络连通性

NCCL属于第五主阶段，只能对验机通过或合法限定豁免、且验机工单已整改复验关闭、网络测试前置满足的目标运行。NP-5/NP-6继续由网络域提供证据。NCCL结果只更新本阶段，不反向更改验机。报告绑定Flow/Execution、BOM、Mapping、配置和真实覆盖范围。NetTest/RoCE保留为网络/验机支撑功能，不增加主阶段。

'''+s[b:]
s=s.replace('NP-3`：G2 三方线序比对通过并写入 PortModel；由 G2 产生。','NP-3`：验机内部LLDP三方线序检查通过并写入PortModel；不作为网络就绪/装机前置。')
if '### FR-25 ' not in s:s+='\n\n### FR-25 六阶段交付状态机与交接\nKICKOFF → NETWORK_READINESS → PROVISIONING → VALIDATION → NCCL_VALIDATION → CLEANUP_HANDOVER为唯一顺序。阶段状态NOT_STARTED/READY/RUNNING/BLOCKED/FAILED/PASSED/WAIVED。WAIVED记录审批人、理由、范围与有效期，不等于PASSED且不绕过前置。Cleanup包含执行停止、临时资源、Agent/SSH撤销、凭据处理、日志证据归档及客户签署；安全要求不可豁免，全部完成后才进入maintained/IPMI-only。旧编号只保留历史追溯。\n'
p.write_text(s)
# Replace old numeric labels in active documents with their actual responsibilities.
files=['PRD.md','ARCHITECTURE.md','SYSTEM-DESIGN.md','DATA-DICTIONARY.md','API-CONVENTIONS.md','DOMAIN-API-CONTRACTS.md','MOCK-DATA-CONTRACT.md','IMPLEMENTATION-PLAN.md','ACCEPTANCE.md','TEST-STRATEGY.md','DESIGN-SYSTEM.md','UI-FLOWS.md','UI-REFERENCE.md','RACK-DIAGRAM.md','IA-V2.4.md','SECURITY.md','DEPLOYMENT.md','README.md','EXCEL-IMPORT-PROFILE.md','OPERATIONS-DESIGN.md','OPERATIONS-IMPACT.md']
terms={'G1':'验机硬件检查','G2':'验机LLDP检查','G3':'Mapping发布','G4':'装机基础配置','G5':'RoCE子任务','G6':'NCCL测试'}
for name in files:
 p=docs/name
 if not p.exists():continue
 text=p.read_text();text=re.sub(r'G1\s*[–—~～-]\s*G6|G1\.\.G6|G1…G6','六阶段交付主链路',text)
 for old,new in terms.items():text=re.sub(r'\b'+old+r'\b',new,text)
 text=text.replace('GateEvidence统一四值','检查结果统一四值').replace('GateEvidence 统一四值','检查结果统一四值')
 if name=='SYSTEM-DESIGN.md':
  text=text.replace('NP-1..NP-6 见 PRD §5。验机LLDP检查 启动只检查 NP-1/NP-2/NP-4；NP-3 由 验机LLDP检查 三方比对产出；NP-5/NP-6 是 RoCE子任务/NCCL测试 前置。不可采集时为 BLOCKED/unknown，不能判 PASS。','网络就绪在开局配置之后、装机之前，检查NP-1/NP-2/NP-4；NP-3由装机后的验机线序任务产生，不得形成装机循环依赖。NP-5/NP-6为NCCL网络测试前置。缺失或不可采集为BLOCKED/unknown。')
 if name=='DATA-DICTIONARY.md':text+='\n\n六阶段新增/迁移实体：DeliveryStageRecord（逐项目/设备/阶段）、DeliveryStageEvent（追加审计证据）、ValidationCheck（验机内部子任务）、DeliveryBatch（冻结目标与混合结果）、ProvisioningReceipt、ValidationRemediation、HandoverRecord。复用Device/Port/BOM/Issue/Execution，不建立平行资产。旧门禁和旧StepReport仅在legacy_*集合查询。\n'
 marker='## 当前六阶段主链路修正'
 if marker not in text:text+='\n\n'+marker+'\n\n本轮用户明确废弃数字门禁作为主链。当前唯一业务顺序、状态机、逐目标放行、安全交接、独立BOM菜单与API约束以[DELIVERY-LIFECYCLE.md](DELIVERY-LIFECYCLE.md)为准。历史编号只按[DELIVERY-MIGRATION.md](DELIVERY-MIGRATION.md)追溯，不得用于解锁。验机失败关联统一Issues并定向复验；NCCL不反写验机；清理/归档/客户签署齐全后才进入维护态。NP仍是网络内部断言。当前仅前端Mock与契约实现，没有真实设备后端。\n'
 if name in ['ACCEPTANCE.md','TEST-STRATEGY.md']:text+='\n本轮新增验收：装机前网络阻断、验机前装机阻断、验机问题自动工单/端口/Execution关联、未整改或未复验不放行、NCCL不改验机、混合批次分项计数、WAIVED期限/范围、未知副作用清理阻断、撤销/归档/客户签署完整性、旧编号无活动解锁、BOM独立菜单。按用户安排尚未执行本轮统一测试，状态NOT RUN；不得沿用旧报告宣称通过。\n'
 p.write_text(text)
p=repo/'AGENTS.md';s=p.read_text();s=re.sub(r'G1\s*[–—~～-]\s*G6|G1\.\.G6|G1…G6','六阶段交付主链路',s)
for old,new in terms.items():s=re.sub(r'\b'+old+r'\b',new,s)
s+='\n\n## 当前优先约束：六阶段主链路\n唯一主链为开局配置→网络就绪→装机→验机→NCCL测试→优雅退场。BOM独立菜单。旧门禁仅历史查询，禁止作为当前阶段；网络NP断言不是主阶段。验机检查按内部子任务组织，异常必须关联项目/设备/端口/检查/Execution到统一Issues；未整改复验不放行。WAIVED不等于PASSED且不能跳过安全清理和客户签署。先完成此项实现，再机柜布局，再Fabric Topology。用户指定时间点前暂停重复回归与构建，验证标NOT RUN。\n';p.write_text(s)
# Keep historical IA source identifiable instead of silently treating it as active business rules.
p=docs/'IA-V2.3.md'
if p.exists() and not p.read_text().startswith('> 历史'):p.write_text('> 历史IA设计记录：导航原则可追溯；原数字门禁主链已失效，现行规则见DELIVERY-LIFECYCLE.md。\n\n'+p.read_text())
for name in files+['DELIVERY-LIFECYCLE.md','DELIVERY-MIGRATION.md','WORK-QUEUE.md','IA-V2.3.md']:
 if (docs/name).exists():shutil.copy2(docs/name,app/'docs'/name)
shutil.copy2(repo/'AGENTS.md',app/'AGENTS.md')
# No test result is manufactured here. Consistency validation waits for the user's test window.
status={'phase':'six-stage implementation synchronization','verification_status':'NOT RUN — user requested a unified test window','documents_updated':files+['AGENTS.md','DELIVERY-LIFECYCLE.md','DELIVERY-MIGRATION.md'],'active_stage_ids':['KICKOFF','NETWORK_READINESS','PROVISIONING','VALIDATION','NCCL_VALIDATION','CLEANUP_HANDOVER'],'legacy_storage':['legacy_gate_evidence','legacy_step_reports','legacy_gate_summary'],'next_tasks':['rack three-panel detail/monitoring layout','Fabric physical/logical topology']}
(docs/'DELIVERY-CONSISTENCY-STATUS.json').write_text(json.dumps(status,ensure_ascii=False,indent=2)+'\n');shutil.copy2(docs/'DELIVERY-CONSISTENCY-STATUS.json',app/'docs/DELIVERY-CONSISTENCY-STATUS.json')
print('Updated active documents and migration references. Unified verification remains NOT RUN.')
