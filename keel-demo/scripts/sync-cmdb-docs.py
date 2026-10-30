from pathlib import Path
import shutil
root=Path(__file__).resolve().parents[1]
sections={
'PRD.md':'FR-28：CMDB升级全链路资产档案。保留跨项目授权与统一设备抽屉；左侧资源树，目录/关系/配置来源/生命周期/质量五视图，项目资料辅助入口保留。搜索/过滤/保存视图/URL上下文；当前资产、规划与候选分别呈现，维护态不恢复带内通道。',
'SYSTEM-DESIGN.md':'CMDB档案只聚合既有领域事实与稳定ID。关系按position/composition/physical/logical/business分类，planned/observed独立；一跳/渐进展开且节点有上限。配置基线、Intended、actual、operational分开，未知不代填。生命周期六阶段后继续维护/换件/退役，历史实体不当作当前库存重复项。',
'DATA-DICTIONARY.md':'CmdbDossierAsset是既有八类物理资源投影（服务器/交换机/GPU/NIC/端口/光模块/Cable/Rack），不是新资产表。CmdbDossierRelation保留kind/fact/source_ref/revision；Source含字段值JSON、Sheet/单元格/批次/采集引用及freshness/support；Quality是数据问题，不能自动等同硬件故障。只新增演示字段来源/质量证据与保存视图，历史实际配置仍在原配置域集合。',
'DESIGN-SYSTEM.md':'CMDB左树+主工作区，五视图共享asset/filter URL。目录按类型字段呈现独立交付/维护/健康/完整性；关系采用React Flow+右侧来源/逐层展开面板，不重建拓扑引擎。设备沿用唯一五/八Tab抽屉，原始证据用Dialog。质量条目直接定位来源，不堆统计卡。',
'UI-FLOWS.md':'资产搜索→统一抽屉；目录/树选中→关系展开NIC→端口→Cable→对端交换机→既有八Tab抽屉。配置版本Diff→字段来源→原领域快照/采集/Execution/Issue；生命周期→验机/NCCL/历史退场归档；质量→待审核导入或采集；人工备注白名单/版本校验→审计。保存视图恢复过滤与选中上下文。',
'API-CONVENTIONS.md':'新增/cmdb/dossier专用catalog/tree/relations/provenance/configuration/lifecycle/quality/manual-fields/saved-views/evidence接口。project_id可清除，仅授权项目；目录cursor+limit 1..100+排序；关系limit最多120、expanded最多10；写入冻结项目/资产/版本且拒绝白名单外字段，403/404/409/422及request_id统一。',
'DOMAIN-API-CONTRACTS.md':'CmdbDossier*资源Schema明确目录状态维度、关系类别与规划/观测、字段来源、配置版本、生命周期和质量。raw evidence为允许领域资源的专用schema union，禁止任意集合访问。实际浏览器返回的九类response另用JSON Schema逐项验证，结果见CMDB-DOSSIER-VALIDATION.json。',
'MOCK-DATA-CONTRACT.md':'沿用原64服务器/512GPU/24交换机及稳定端口/Cable/Rack ID；只补cmdb_field_provenance、cmdb_quality_findings、cmdb_saved_views及原领域配置历史/待审核BOM/生命周期/报告。Sheet/Cell映射明确synthetic，非真实工作簿解析。草稿U冲突不覆盖现场位置，pending缺端点不覆盖正式Cable，换件事件保存旧身份与不同的新NIC身份，退场归档示例不解锁当前阶段。',
'ACCEPTANCE.md':'CMDB-D-01…06：服务器→部件→端口→线缆→对端；Diff→来源→作业/工单；六阶段生命周期到验机/NCCL/退场证据；质量定位导入/采集；维护/unknown/stale/权限；跨列表/抽屉/机柜/拓扑同稳定ID。另验保存视图、人工备注审计/版本与只读权限。真机和正式Go仍NOT RUN。',
'TEST-STRATEGY.md':'定向CMDB构建、浏览器六条验收链与API九类响应JSON Schema验证。浏览器脚本cmdb-dossier-check.mjs，补充保存视图/Viewer/分页唯一性cmdb-contract-final-check.mjs；响应校验check-cmdb-response-contracts.py。不做无关全仓回归；实际结果记录CMDB-DOSSIER-VALIDATION.json。',
'IMPLEMENTATION-PLAN.md':'无损证据改造提交1631460后实施CMDB档案：现有域/契约读取→五视图与只读关系投影→统一Provider/来源/历史/质量→稳定ID演示补齐→相关构建/契约/交互检查→独立提交keel-new。未实现新CMDB后端、图数据库、通用低代码或真实采集。'}
for name,body in sections.items():
 p=root/'docs'/name;s=p.read_text();marker='## CMDB 全链路资产档案增补';s=s.split('\n'+marker)[0]+'\n\n'+marker+'\n\n'+body+'\n\n唯一详细设计与模型边界见[CMDB-DOSSIER.md](CMDB-DOSSIER.md)。\n';p.write_text(s);shutil.copy2(p,root.parent/'docs'/name)
shutil.copy2(root/'docs/CMDB-DOSSIER.md',root.parent/'docs/CMDB-DOSSIER.md')
