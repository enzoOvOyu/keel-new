# IA v2.4 Mock Coverage Matrix

本文件把对话中确认的 IA v2.3 双平面导航、BOM 智能导入、机柜全局视角、服务器/交换机运维抽屉、Kestra 风格 Flow/Execution 统一映射到演示数据集合。

## 导航到数据集合

| 页面/区域 | 数据集合 | 关键交互状态 |
|---|---|---|
| ProjectSwitcher / 项目列表 | projects, project_members, project_timelines | 当前项目、跨项目切换、无权限、无项目 |
| Dashboard | dashboard_snapshots, fleet_summaries, fabric_summaries, alerts, issues, executions | KPI、阻塞、告警、活动执行 |
| Provision / BOM 智能导入 | bom_imports, bom_sheet_regions, bom_mapping_candidates, bom_issues, ai_providers, ai_tasks, ai_evaluations | 解析、映射候选、待审核、BLOCK/WARN、提交演示 |
| Fleet / 服务器抽屉五 Tab | servers, gpus, nics, ports, server_runtime, bmc_inventory, bmc_sensors, bmc_sel_events, firmware_inventory, server_config_baselines, server_observed_configs, mapping_revisions | healthy/warning/critical/maintained/agentless/stale |
| Fabric / 交换机抽屉八 Tab | switches, switch_runtime, switch_config_baselines, switch_running_configs, switch_config_diffs, switch_interfaces, lldp_neighbors, fdb_entries, arp_entries, ddm_samples, switch_bgp_neighbors, switch_logs | IN_SYNC/DRIFT、接口 up/down/flapping、DDM warning、BGP 状态 |
| Rack 全局机房视图 | locations, racks, rack_global_maps, rack_power_circuits, rack_environment_sensors, rack_issues | 机房/楼层/行/柜、健康、容量、电力、温度 |
| Rack 单柜编辑器 | rack_diagrams, rack_diagram_revisions, rack_placements, device_models, servers, switches | 正反面、U 位、自动生成、手动覆盖、冲突、版本差异 |
| Issues | issues, alerts, lifecycle_events, execution_logs, gate_evidence | open/assigned/in_progress/waiting_verification/closed |
| Monitor | metric_series, metrics, monitoring_sources, collection_jobs, alerts, bmc_inventory, bmc_sensors | 趋势、fresh/stale/unknown、采集失败、BMC-only |
| Flows | flows, flow_revisions, flow_blueprints, flow_target_groups, schedules | draft/published/disabled、Cron、目标快照 |
| Execution | executions, execution_targets, task_runs, execution_logs, execution_outputs | queued/running/success/partial/failed/killed/result_unknown |
| Validation / NetTest / NCCL | gate_evidence, delivery_runs, mapping_revisions, executions, task_runs, issues | G1-G6、PASS/FAIL/BLOCKED/WAIVED |
| Config / Fabric PushJob | network_baselines, switch_config_baselines, switch_running_configs, switch_config_diffs, push_jobs | 预检、Doublecheck、审批、灰度、下发、复采 |
| Cleanup / 维护态 | maintenance_records, bmc_inventory, collection_jobs, lifecycle_events | maintained、BMC-only、带内禁用 |
| 系统管理 | users, project_members, templates, device_models, parameters, media, audit_events | 权限、模板、型号、参数、介质、审计 |

## 数据事实边界

- `servers/switches/gpus/nics/ports/racks/cables` 是演示资产主记录。
- `cmdb_entities/cmdb_relationships` 是 CMDB 关系视图；`lifecycle_events` 是生命周期事件。
- `*_config_baselines` 是计划/批准基线；`*_observed_configs` 或 `*_running_configs` 是设备实际观察值；两者不可合并。
- `metric_series` 是时序观测；`*_runtime`、`*_sensors` 是当前快照。不得将快照当成完整历史时序。
- `lldp_neighbors/fdb_entries/arp_entries/ddm_samples/switch_interfaces` 是交换机侧采集视图，均带采集时间、来源和演示标记。
- AI 识别与评测数据是合成样本，不代表真实模型调用或准确率承诺。
- 所有执行、变更、BMC 控制和下发只模拟，不触达真实设备。

## 已知演示边界

- 主项目是 64 台 B300 / 512 GPU / 24 交换机 / 16 机柜。
- 另外两个项目只提供 ProjectSwitcher 和跨项目运维上下文，不伪造额外服务器明细。
- 机柜功耗、环境、DDM、FDB、ARP、LLDP 和性能趋势均为合成数据。
- 数据集保留原始 Excel 的结构维度作为导入场景依据，但不复制源文件中的真实凭据或敏感资产标识。
