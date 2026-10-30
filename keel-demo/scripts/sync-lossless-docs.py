from pathlib import Path
import shutil
root=Path(__file__).resolve().parents[1]
sections={
'PRD.md':'FR-27：无损网络分层证据。每项绑定型号/ASIC/精确NOS/模式/批准基线；版本化新规则代替旧UI标签。PFC非零不直接故障，ECN按absolute/relative单位，watchdog按补丁实际条件。网络就绪不等待主机；验机补齐双端；NCCL消费当前目标有效证据。确认异常关联统一Issue及定向复验。',
'SYSTEM-DESIGN.md':'批准baseline→applied→operational→counter/test分别存储，NVUE列分层解释。Verdict与capability/freshness/applicability分开。设备/端口/方向/SP/TC/PG/pool/路径/流量/窗口不可混同；同窗口增量处理重启/清零/回绕/缺测。配置/拓扑版本变化失效，最新有效窗口不删除历史失败；同时间冲突证据BLOCKED。',
'ARCHITECTURE.md':'集中LosslessProvider/typed envelope及纯规则器，四处UI共用判据；Go和真实collector尚未接入。当前分层帧解析只认证合成格式，不声明原生NVUE字段解析通过；只读command allowlist不包含clear/set/action。历史拓扑不显示当前无损结果冒充历史。',
'DATA-DICTIONARY.md':'LosslessRule@version（历史映射）、LosslessProfile、LosslessBaseline、LosslessEvidence、LosslessRawOutput、LosslessHostEvidence、LosslessRetest引用既有项目/Device/PhysicalPort/Issue/Execution。计数十进制字符串保精度；boot/epoch/width/wrap、窗口、单位和来源显式记录；无原始值不补零。raw/旧窗口保留，新增窗口独立ID。',
'API-CONVENTIONS.md':'新增项目显式lossless evidence/conclusion/collect/retests/raw资源专用Schema和LosslessProvider。查询on-demand+AbortSignal；复验冻结config/topology revision与幂等键；403/409/422独立；raw只返回授权设备合成帧，后续生产raw必须脱敏。',
'DOMAIN-API-CONTRACTS.md':'Lossless* Schema定义scope、适配范围、四层证据、原值/单位/窗口delta、有效期、判据原因、Issue与Execution。list默认当前端口/规则最新观测，历史raw保留；证据冲突或缺口不可PASS。collect仅消费只读Mock帧，不重写采集时间；retest不自动修复、不自动调参。',
'DESIGN-SYSTEM.md':'Fabric无损、交换机无损抽屉、Topology健康详情、机柜Network共用LosslessEvidencePanel。规则与端口可选，五层Tab展示baseline/applied/operational/counters-test/provenance；显示缺口、来源、有效期及工单，配置匹配与运行测试结论分开。缺测/不适用不可用绿色替代。',
'UI-FLOWS.md':'Fabric无损选择设备/反例→端口→rule@version→五层证据→读取固定Mock快照并自动关联确认异常→整改产生新证据→定向复验→当前证据与绑定版本有效才关闭Issue。人工关闭不恢复PASS，过期/配置变动必须重验。',
'UI-REFERENCE.md':'无损页面以规则/证据工作区承载，物理健康与无损verdict独立，保留拓扑和机柜原交互；不把NVUE配置表复制成全网通过大表，不用计数卡片颜色证明RoCE性能。',
'MOCK-DATA-CONTRACT.md':'fixture集中增加lossless_rules/profiles/baselines/evidence/raw_outputs/host_evidence/retests，覆盖正常PFC、无流量、relative、合法精确硬件差异、watchdog触发/处置/恢复、reset、stale、双端缺失、lossy/single-ipool、未知/旧补丁。部分端口覆盖保持BLOCKED；明确合成身份/参数不是官方默认，不修改原资产数量。',
'ACCEPTANCE.md':'LL-EV-01…12验收：适用范围；版本/历史迁移；分层；模式单位；PFC不误报；精确硬件案例；watchdog版本/事件；窗口与reset/wrap；缺测与能力；六阶段无循环依赖；工单当前定向复验；四处消费共享结论。真机/生产阈值/原生parser未验证保持NOT RUN。',
'TEST-STRATEGY.md':'本轮仅运行相关无损规则/契约测试及契约TypeScript一致性检查，不做全平台回归/真实设备验证。反例断言见scripts/lossless-rules.test.mjs；OpenAPI枚举/作用域/稳定引用/历史迁移/host窗口见lossless-contract.test.mjs；实际结果见LOSSLESS-VALIDATION.json。',
'IMPLEMENTATION-PLAN.md':'先官方章节核对/迁移表→规则/证据专用契约→集中Mock反例与只读适配计划→共享组件及六阶段/Issues联动→相关规则/契约检查→提交keel-new。后续需取得真实脱敏NVUE输出、单位/能力/采样误差和主机响应证据后认证collector，不提前实现Go或自动调参。'}
for name,body in sections.items():
 p=root/'docs'/name;s=p.read_text();marker='## 无损网络分层证据 v2';s=s.split('\n'+marker)[0]+'\n\n'+marker+'\n\n'+body+'\n\n规则/官方章节/命令/版本/判据/证据对照、历史迁移、真实缺口见[LOSSLESS-EVIDENCE.md](LOSSLESS-EVIDENCE.md)。\n';p.write_text(s);shutil.copy2(p,root.parent/'docs'/name)
shutil.copy2(root/'docs/LOSSLESS-EVIDENCE.md',root.parent/'docs/LOSSLESS-EVIDENCE.md')
