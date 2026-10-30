# Keel 交付后运维域设计

标识：OPS-2026-10-29；状态：设计契约，尚未实现新增运维能力。对应 PRD FR-18…FR-22，不替代 FR-0…FR-17。

## 1. 全生命周期与页面导航

`BOM导入 → CMDB建模 → 网络/服务器实施 → 验收交付 → 运维纳管 → 持续监控/配置管理 → 告警/工单 → 维修或变更 → 重新验证 → maintained持续运行 → 生命周期退役`。

交付记录、原始BOM、基线、实际配置、物理端口、拓扑、监控、告警、工单和执行记录在退场后持续可查，原项目授权与稳定身份不丢失。

```
交付工作区（保留现有全部入口）
运维管理
 ├─ 已纳管服务器：项目/机柜/型号/交付/健康/告警/维护状态
 │   └─ 设备详情：概览 | 基础配置 | 基线与实际 | 硬件监控 | BMC | 固件 | 网络端口 | 作业/证据/时间线
 ├─ 已纳管交换机：Fabric/Pod/机柜/角色/型号/NOS/配置/健康/告警
 │   └─ 设备详情：概览 | 配置差异 | 端口 | LLDP | FDB | ARP | DDM | 健康 | 无损网络 | 变更/采集/审计
 ├─ 配置漂移与变更：baseline → intended → observed → diff → 既有审批/PushJob → 复采
 ├─ 采集中心：周期策略/手动任务/失败重试/历史/来源能力
 └─ 维护与退役：维修窗口/变更工单/定向复验/退役审批
共享：监控/告警/工单/Flow/介质/权限/审计，按稳定资产ID联动，不建平行系统。
```

## 2. 页面与详情契约

| 页面 | 主信息与交互 | 专属约束 |
|---|---|---|
| 服务器列表 | 项目、机柜、型号、交付、健康、告警、维护筛选；搜索、分页、批量选择；详情独立抽屉/路由 | 批量刷新先列可采集/被策略拒绝/不支持的目标数，禁止静默跳过 |
| 服务器概览/基础配置 | SN、型号、rack/U、GPU数量、项目、交付/维护；CPU/内存/GPU/NIC/磁盘/PCIe、OS/内核/驱动/CUDA/NCCL | 每字段可追溯来源/时间/版本；交付快照不能冒充当前实测 |
| 服务器配置/硬件监控 | 批准基线、intended与observed差异；CPU/GPU温度、利用率、显存、内存/磁盘、功耗/电源/风扇、健康趋势 | unsupported/unknown/null与真实0区别；批准阈值和单位随指标显示 |
| BMC | 地址、Redfish/IPMI能力及策略允许性、可达性、传感器、SEL、启动项/固件；受控电源操作 | maintained只允许IPMI；Redfish支持但禁用时单独标policy_denied；电源操作冻结目标/动作/理由并审批确认、租约、审计 |
| 固件 | BIOS/BMC/NIC/GPU库存、目标基线、兼容性、已批准介质、升级历史/审批/复验 | 兼容性未知或通道禁用必须BLOCKED，不刷写；固件升级不是普通采集重试 |
| 服务器网络/端口 | PhysicalPort、OS接口、永久MAC、速率/链路、DOM、LLDP、GPU-NIC Mapping | 永久身份与可变OS接口名分开；版本化Mapping引用不被最新观测覆盖 |
| 交换机概览/配置 | SN/Hostname/IP/rack/U/角色/NOS/ZTP/运行；模板、approved baseline、rendered intended、running/NVUE observed、diff/漂移/历史 | planned、intended、observed、approved四者分栏；任何写入复用原PushJob和NE+DE（Critical加NH） |
| 端口/LLDP | 全物理端口与breakout；角色/速率/MTU/VLAN/PFC/ECN、链路/错误；邻居设备/端口/Chassis ID/Port ID/Sysname/Sysdesc | 父子端口关联、期望/实测差异、采集时间；缺一侧LLDP不得判一致 |
| FDB/ARP | MAC/VLAN/学习端口/静动态/更新时间；IP/MAC/VLAN/VRF/接口/状态/采集时间 | 观测记录不是资产身份变更；允许历史时间范围和快照间比较 |
| DDM | Rx/Tx dBm、温度°C、电压V、偏置mA、模块型号/厂商/SN、阈值、历史趋势 | 每型号/通道/方向阈值独立引用；不把unsupported填0 |
| 健康/无损 | BGP、CPU/内存/温度/电源/风扇、syslog、接口错误/flap；PFC/ECN/CNP、队列/缓冲/丢包/QoS一致性 | 能力/保鲜/质量/规则结论分开；counter reset与缺采样不解释为恢复 |
| 维护/退役 | 维修工单、维护窗口、RMA关系、配置变更、复验、退役审批 | 维护状态与lifecycle正交；维修不恢复已撤销通道；RMA新实物新ID |

## 3. 事实与所有权

- 资产身份以BOM item_uid / Device.id对应的稳定资产ID为锚；Machine是server Device的视图，Switch是switch Device的视图，Component是GPU/NIC/PSU/BMC等BOM子物料，不另建平行身份。PhysicalPort/Rack/RackPlacement/Cable继续复用。
- CMDB持有统一身份、有效关系和生命周期查询权威；Inventory/BOM负责各自可审查的资产变更，配置域负责配置版本，采集域负责观测和Job，监控负责指标索引和Alert，工单域负责Issue。事务Outbox投影，不能让前端直接补写第二份事实。
- ConfigBaseline复用NetworkBaselineRevision（网络）/ConfigSetRevision（服务器）的统一契约视图。IntendedConfig引用已批准基线和模板渲染版本；ObservedConfig是不可变实测快照。ConfigRevision是这三类版本的通用引用，不是第四份配置正文；ConfigDiff绑定精确版本/规则，目标任一版本变化则旧审批失效。
- PostgreSQL保存身份、关系、版本元数据、策略、任务、审批和索引；对象存储保存不可变配置/采集原文；MetricStore、LogStore以适配器承载时序/日志查询。首期可用现有PostgreSQL分区/对象存储，专用产品选择不在本次变更中决定。CMDB只保存关联/摘要，不存全量时序与日志。

## 4. 观测与通道约束

ObservationContext统一包含project_id、asset_id、source、adapter_version、collected_at、received_at、capability_status、freshness_status、quality_status、threshold_revision_id、retention_policy_id、policy_allowed、policy_reason。指标、快照、日志必须带context或通过snapshot_id解析到它。

- capability：supported/unsupported/unknown；freshness：fresh/aging/stale/unknown；quality：good/degraded/failed/unknown；规则结论仍仅PASS/FAIL/BLOCKED/WAIVED。健康、维护状态和Job状态另行表示，不混用。
- value=null表示无数据并附原因；数值0只表示一次成功的零值观测。采集失败保留上次成功观测并标stale/degraded，不刷新成功时间。counter需带reset标记。
- maintained访问不因维修/审批/刷新自动扩大：Keel直连只有IPMI-only允许集合；SSH/Agent/Redfish一律不允许。外部批准的观测来源可继续读取/接入，但不能用其作为Keel恢复带内访问的代理。交换机缺IPMI时不假装有实时数据；依赖禁用通道的任务拒绝，历史仍可查。
- CollectionJob创建、worker执行、每次重试/换适配器、推送接入和恢复任务均检查project/RBAC/maintenance/access policy；租约和授权吊销即时生效；禁止隐式fallback到SSH或Agent。

## 5. 变更、维修与退役闭环

Alert/配置漂移 → 复用T1–T7 Issue → 冻结资产/版本/影响范围 → 维护窗口与审批 → 允许通道内的变更Job → ObservedConfig/Telemetry/FirmwareInventory复采 → 精确范围GateEvidence/验证报告 → 工单关闭与维护状态恢复。缺失/过期/unsupported证据为BLOCKED，不使用工单“已关闭”推导PASS。

生命周期进入maintained后仍保留全部关联历史。维护状态 normal → planned → in_maintenance/repairing → revalidating → normal；失败留在维修或复验并保留Issue。retirement_pending → retired必须有资产范围、批准人、保留/法律冻结策略、采集停止与凭据撤销回执；不删除历史。RMA以新UID和replaces/replaced_by有效期关系连接旧实体。

## 6. 保留、阈值和实现边界

RetentionPolicy按数据类别、项目/资产范围、热存储窗口、归档/删除时间、legal_hold、审批revision定义；具体天数、采集间隔、型号阈值、RPO/RTO待负责人批准。被证据/审批/历史执行引用的数据不得提前删除；允许删除原始payload时必须保留元数据、hash、来源与expired标记，界面不能把“过期已清理”显示为“从未采集”。

本次只冻结页面和OpenAPI/Mock数据契约；不实现采集器、后端、时序数据库、真实BMC/设备或新增运维页面代码。开发顺序见IMPLEMENTATION-PLAN的OPS增补。
