# 旧门禁编号迁移表

旧编号全部退出活动主链路；仅在legacy_*字段、历史证据与本迁移表中保留原值。

| 旧编号 | 原职责 | 新归属 | 新标识 / 说明 |
|---|---|---|---|
| G1 | 预收集、硬件清单 | VALIDATION内部 | hardware_inventory / gpu_nic_inventory / pcie_topology / nic_mac |
| G2 | 双侧LLDP和三方线序 | VALIDATION内部 | lldp_line_order；产出网络内部NP-3 |
| G3 | Mapping发布 | VALIDATION关系/证据 | gpu_nic_inventory与MappingRevision；不是阶段 |
| G4 | 基础配置实施 | PROVISIONING | baseline_apply；实际一致性复核另属VALIDATION.config_consistency |
| G5 | RoCE连通性 | VALIDATION / NetTest支撑 | roce_connectivity；不是NCCL前的另一个主阶段 |
| G6 | NCCL测试 | NCCL_VALIDATION | nccl_test；只引用已通过验机的目标 |

旧gate_evidence、step_reports转入legacy_gate_evidence、legacy_step_reports，并标historical_only；新模型使用delivery_stage_records、validation_checks、test_reports和handover_records。历史GateEvidence与历史JSON不会用于新阶段解锁。旧项目汇总保留在legacy_gate_summary，活动汇总仅从六阶段逐设备记录派生。

数据迁移不把原“通过”机械换成六阶段“通过”。先根据阶段前置、可用证据、实际覆盖和未关闭工单建立状态；无法确认时BLOCKED/NOT_STARTED，原始记录保持可追溯。
