# MOCK-DATA-CONTRACT.md — API 契约驱动的全平台演示数据规范

## 1. 目标与硬约束

先让前端获得完整、可交互、数据自洽的控制台，再逐模块接入 Go 后端。Mock 是 API 契约的可执行实现，不是散落在 React 组件中的静态对象。

- 唯一演示数据集：`fixtures/demo-full-64-b300/demo-seed.json`；PostgreSQL 装载脚本：`fixtures/demo-full-64-b300/seed-demo.sql`。
- 64 台 B300 为**合成演示切片**，不是源 Excel 实际数量的声明。按每台 8 GPU、10 个网络适配器建模，共 512 GPU、640 NIC、16 个 42U 机柜、24 台交换机、704 条线缆；另含 4,600+ CMDB 实体和 5,900+ 关系记录。所有标识和地址均为虚构值。
- 禁止把源 Excel 的真实 SN、MAC、IP、账号、密码或完整原始行复制进 fixture。演示 hostname 使用 `.invalid`，IPv4 使用文档保留网段，MAC 使用本地管理地址。
- 所有执行、下发、采集、监控和审批均显示 `execution_mode=mock` / `SIMULATION_ONLY`；Mock 不得连接真实设备，不得伪装真实成功。
- `openapi.yaml` 是 HTTP wire contract 的唯一来源；字段、枚举、错误结构必须与 OpenAPI 一致。修改契约时同步 fixture、mock provider、TS 类型和 Go handler。

## 2. 数据装载方式

### 2.1 推荐的第一步

1. 建一个**专用演示数据库**，不复用生产库凭据。
2. 执行 `fixtures/demo-full-64-b300/seed-demo.sql`，数据写入隔离的 `keel_demo` schema。
3. Mock API 从 `keel_demo.mock_resource`、`mock_relation`、`mock_event` 读取 JSONB payload，按 OpenAPI 响应 envelope 返回；业务动作写入同一 schema 的 mock store/audit，避免只弹 toast。
4. 前端默认 `VITE_API_MODE=mock`，页面只依赖 typed API services；不直接导入 fixture。后续将 `VITE_API_MODE=http` 指向 Go API，页面和 TypeScript 资源类型不变。

### 2.2 演示数据库表

- `keel_demo.mock_resource`: `(resource_type, resource_id)` 主键，保存与 API schema 对齐的单资源 payload。
- `keel_demo.mock_relation`: CMDB 关系投影，保存 from/to、关系类型及来源。
- `keel_demo.mock_event`: 生命周期事件，按实体与时间排序。
- `keel_demo.mock_dataset_meta`: 数据集版本、范围、数量与演示标记。

这是用于先做界面的隔离演示存储，不等同于生产领域表设计。逐步接入后端时，应把业务写入迁移到正式领域服务，再通过 Outbox/幂等投影更新 CMDB。禁止将 JSONB demo 表当作生产数据库架构。

## 3. Provider 接口

```ts
export interface ApiProvider {
  projects: ProjectsApi; bomImports: BomImportsApi; cmdb: CmdbApi;
  inventory: InventoryApi; racks: RacksApi; rackDiagrams: RackDiagramsApi;
  flows: FlowsApi; executions: ExecutionsApi; fabric: FabricApi;
  delivery: DeliveryApi; issues: IssuesApi; monitoring: MonitoringApi;
  templates: TemplatesApi; media: MediaApi; parameters: ParametersApi;
  deviceModels: DeviceModelsApi; users: UsersApi; audit: AuditApi;
}
```

页面仅使用 hooks/services。Mock 与 HTTP provider 的方法签名、状态枚举、响应 envelope、错误结构必须相同。

## 4. 64 台 B300 数据覆盖

| 数据域 | fixture 集合 | 覆盖要求 |
|---|---|---|
| 项目/总览 | `projects`, `overview`, `templates` | 64 台节点、512 GPU、640 NIC、16 机柜、交换机、链路、健康/告警汇总 |
| BOM 智能导入 | `bom_imports`, `bom_sheet_regions`, `bom_mapping_candidates`, `bom_issues` | 33 Sheet 画像摘要、映射候选、待审核、未知列、敏感列阻断、校验与提交状态 |
| 资产/硬件 | `servers`, `gpus`, `nics`, `ports`, `switches`, `switch_ports`, `optics`, `cables`, `device_models`, `project_assets` | 设备/端口/线缆两端引用自洽，支持搜索、过滤、详情与关系追溯 |
| CMDB | `cmdb_entities`, `cmdb_relationships`, `lifecycle_events` | BOM 起点、设备关系、执行、工单、机柜图事件；支持详情、关系图和时间线 |
| 工作流/脚本执行 | `flows`, `schedules`, `executions`, `task_runs`, `execution_logs`, `execution_outputs` | 目标快照、参数、定时频率、运行/成功/失败/部分成功/终止/结果未知、日志 cursor、重试与输出物 |
| 网络底座 | `network_baselines`, `push_jobs`, `switches`, `switch_ports`, `optics`, `cables` | ZTP、IN_SYNC/DRIFT、预检、审批、下发、回滚状态；模拟数据不能触发真实网络操作 |
| 交付门禁 | `gate_evidence`, `delivery_runs`, `mapping_revisions` | 六阶段交付主链路、NP-1..NP-6 相关展示，PASS/FAIL/BLOCKED/WAIVED、证据有效期与豁免信息 |
| 工单/告警/监控 | `issues`, `alerts`, `metrics`, `monitoring_sources` | T1-T7、严重级别、趋势、来源、unknown/degraded 能力、定向复验状态 |
| 机柜图 | `rack_diagrams`, `rack_diagram_revisions`, `rack_placements`, `racks` | 16 个 42U 机柜、设备位置、U 位占用、自动生成/人工覆盖、冲突提示、发布版本 |
| 平台治理 | `media`, `parameters`, `users`, `audit_events` | 介质版本/审批、参数目录、角色权限展示、审计前后差异脱敏 |

## 5. 数据完整性规则

1. 所有 `project_id` 指向 `proj-b300-demo`；所有外键型引用必须存在。
2. 64 台服务器编号固定为 `srv-0001`…`srv-0064`；每台 8 GPU、10 NIC；物理端口和线缆端点引用必须可解析。
3. 机柜 16 个，每柜 4 台服务器；42U 机柜中每台服务器 6U，U 位不能重叠。交换机位置必须与服务器占用区间不冲突。
4. `oper_state=down`、失败执行、BLOCKED 门禁和告警是**有意注入的演示异常**，不能被总览错误地聚合为全绿。
5. 数据集使用固定种子/固定值，不得每次请求随机生成不同的设备状态。时间序列是固定 fixture；需要动态交互时只推进预定义状态机。
6. CMDB 投影记录必须能追溯 `source_event_id`、`source_refs[]`、revision 和时间戳。重复事件按 event ID 幂等。
7. API 对敏感字段只返回脱敏占位和 secret reference；绝不返回密码、token、私钥。
8. 列表分页、筛选、排序、搜索、空状态、加载状态、错误状态均应在 Mock 中实现，不能只展示静态首页。

## 6. 交互状态机要求

- BOM：`uploaded → parsing → mapping_review → validating → preview_ready → committed`；存在 BLOCK 时不能 commit；同 batch/hash 重复提交幂等，同 batch 不同 hash 返回 409。
- Flow：draft/active/disabled/archived；运行前预览目标并冻结 `target_snapshot_id`。
- Execution：queued/running/success/warning/partial/failed/killed/result_unknown；Pause/Resume/Kill/Retry-failed 必须真实改变 Mock store、日志和审计时间线。`result_unknown` 必须先 reconciliation，不能直接标成功。
- PushJob：严格遵循 PRD 的 13 态状态机；未审批不得执行；失败回滚与结果核对状态明确。
- GateEvidence：只允许 `PASS|FAIL|BLOCKED|WAIVED`；WAIVED 必须有审批人、原因、范围、有效期、整改计划。
- Rack diagram：生成新 draft revision；编辑需 revision/If-Match；U 位冲突阻断发布；发布后不覆盖历史 revision。
- CMDB：先显示投影状态 pending，再按固定场景转 complete/failed/retry；不可因为源事件创建成功就提前显示投影已完成。

## 7. Mock 场景切换器

开发模式提供固定 Scenario Switcher：`happy-path`、`bom-unknown-columns`、`bom-blocked`、`cmdb-projection-pending`、`execution-running`、`execution-partial`、`execution-result-unknown`、`fabric-unknown`、`rack-u-conflict`、`rack-generation-failed`、`permission-denied`、`api-error`。生产构建不显示切换器。

场景切换必须重置到可重复 fixture，不允许场景之间泄漏编辑状态。Mock 操作应可 reset。

## 8. API 响应/错误

成功 envelope 以 `API-CONVENTIONS.md` 和 OpenAPI 为准。错误统一包含 `error.code`、`error.message`、`error.details[]`、`request_id`。至少覆盖 400/401/403/404/409/422/429/500/503。不得返回原始敏感行。

## 9. 前端全量 Mock 完成定义

- 所有导航主页面都能加载数据；每个页面至少一个 API contract、固定 fixture 和错误/空/加载态。
- 关键交互可执行：过滤/搜索/排序/分页、详情抽屉、创建/编辑、审批、BOM 映射审核、机柜图拖放、Flow 运行与日志查看、Pause/Resume/Kill/Retry、工单处置、版本 diff。
- Mock 模式下不需要 Go 后端即可完成演示；刷新后恢复基准 fixture，开发操作可在当前会话内保持。
- `openapi.yaml` 校验、TypeScript 类型检查和前端 build 通过。只执行本次变更相关测试，不做无关全仓重构。

## 10. 加载命令

```bash
# 仅在 disposable demo PostgreSQL 上执行
PGOPTIONS="-c keel.allow_demo_seed=true" psql "$KEEL_DEMO_DATABASE_URL" -v ON_ERROR_STOP=1 -f fixtures/demo-full-64-b300/seed-demo.sql

# 通过 API_MODE 切换，不要让浏览器组件自行读取 SQL/JSON fixture
VITE_API_MODE=mock
```

`seed-demo.sql` 使用隔离的 `keel_demo` schema，重复执行可幂等更新演示记录；默认会拒绝执行，必须显式设置 `PGOPTIONS="-c keel.allow_demo_seed=true"`。不要将演示 schema 暴露给生产执行器或生产网络凭据。

## OPS-2026-10-29 · 交付后运维设计增补

### 运维Mock扩展范围（设计契约，未生成/未实现）
保留64台B300、512GPU、640NIC、16机柜、24交换机和既有BOM/端口/线缆ID。新增运维数据只关联已有稳定资产，不另造一套设备。

| 集合 | 关联与覆盖要求 |
|---|---|
| operations_device_details / maintenance_events | 覆盖64服务器+24交换机；交付/维护/退役维度、ipmi_only、权限与策略拒绝；历史ID仍可查询 |
| configuration_revisions / config_diffs | baseline/intended/observed明确fact_kind、版本/hash/来源；漂移、缺失实测、审批失效；复用既有基线与PushJob引用 |
| telemetry_snapshots / metric_samples / log_events | CPU/GPU/内存/磁盘/功耗/风扇/电源、OS/软件、BGP/syslog/接口/无损；成功0/null/unknown/unsupported/stale/degraded/失败分别构造 |
| lldp_snapshots / fdb_snapshots / arp_snapshots | 端口/线缆真实fixture引用，完整邻居字段、MAC/VLAN/VRF/接口、采集与学习时间，包含漂移/缺侧/过期 |
| ddm_snapshots | 关联optic和port；Rx/Tx dBm、温度°C、电压V、偏置mA、通道/阈值revision；不支持字段为null+原因 |
| firmware_inventory / firmware_upgrade_jobs | BIOS/BMC/NIC/GPU组件版本、基线、兼容性、批准介质与审批、租约、升级前后证据；禁止自动刷写与通道绕过 |
| collection_policies / collection_jobs | 周期/手动、timeout/retry/history、任务和last_success、退休/策略吊销/换适配器仍拒绝 |
| maintenance_events / retirement_jobs | 维修→复验→恢复维护；退役审批、停止任务、凭据撤销、历史保留，RMA新旧UID关系 |

每条记录符合openapi.yaml Ops* schema并含ObservationContext或可解析snapshot引用。retention_policy_id与threshold_revision_id只指合成且明确标注的策略，不能伪称生产已批准阈值。

固定新场景：ops-maintained-ipmi-only、ops-unsupported-gpu-utilization、ops-collection-timeout、ops-observed-config-missing、ops-critical-drift、ops-ddm-threshold-crossed、ops-lldp-missing-side、ops-firmware-incompatible、ops-firmware-result-unknown、ops-retention-expired、ops-repair-revalidate、ops-retired-history。通道拒绝必须先于任何模拟成功；不能靠“刷新”把unsupported变为PASS。页面正常/空/加载/权限/错误/历史过期场景均需明确。

实施时先生成覆盖报告：物理资产计数不变、全部FK可解析、配置三类事实不混用、单位/时间/阈值合法、样本0与null分离、任务/审批状态一致、维护态无禁用通道、秘密值为零。当前既有expanded-seed交付孵化数据不自动满足本运维契约，需单独实现和验收。

## IA v2.3 · 双平面与统一抽屉

新增 IA Mock 需要至少两个获授权合成项目和一个未授权项目，项目/设备/端口/告警/工单/事件均使用稳定引用；保留原64台B300种子规模和完整关联，不将未授权资源暴露给跨项目查询。固定场景覆盖延迟项目切换、权限撤销、SSE断线重连、空数据、stale、unsupported及maintained。需要新增完整 fixture 与 provider 契约测试；当前尚未实现。

## IA v2.4 实施增补

固定64台B300与512GPU；跨项目使用独立合成项目和明确授权，提供失败、阻断、未知、陈旧、维护与拒绝场景，记录字段来源、时间和能力。


## 2026-10-30 全量数据包接入
最新权威演示数据为 `fixtures/demo-full-64-b300/demo-seed.json`（来源 `keel-ia-v2.4-full-demo-pack.zip`），覆盖83个集合和33个工作簿结构Sheet。停止在运行时合并旧扩展种子或额外A100资产；其他两个项目仅上下文，不虚构额外设备。导入检查见 `docs/IA24-DATA-IMPORT-REPORT.json`，覆盖见 `docs/IA-V2.4-MOCK-COVERAGE.md`。真实Excel仍未提供；workbook-profile只作为提供的结构元数据，不等于本轮实际检查原工作簿。新包的旧PRD不能删去FR-18…24或放宽既有IPMI-only；不可解析项目成员不得自动授权。


## IA v2.4 Mock 前端实施记录

导航树、完整页面清单、路由/权限/作用域/加载策略、五/八Tab抽屉与API对照统一见 [IA24-IMPLEMENTATION-MATRIX.md](IA24-IMPLEMENTATION-MATRIX.md)。原FR-0…24、门禁、BOM与维护态规则保留。当前实现为前端Mock，不表示已实现Go/真实设备/真实AI。权威种子为 `fixtures/demo-full-64-b300/demo-seed.json`；原包83集合，补齐与规范化后集合数量以当前manifest为准，主项目硬件数量不变。结构检查、数据修复、契约与交互结果以IA24-*报告为准，无法采集/缺少证据保持unknown或阻断。


## IA24-05 · 用户确认的GPU-NIC默认映射
用户于本轮明确：没有单独说明时，GPU1–8分别对应NIC1–8。对当前B300切片按此规则确认512条GPU-NIC关系，NIC9/10保留存储角色；不再把已有LLDP链路显示成全部unknown。矩阵行GPU、列NIC，点击显示物理端口、线缆、交换机、来源与观测；配对关系与线序/新鲜度判据分开，异常不能被配对规则覆盖。


## 当前六阶段主链路修正

本轮用户明确废弃数字门禁作为主链。当前唯一业务顺序、状态机、逐目标放行、安全交接、独立BOM菜单与API约束以[DELIVERY-LIFECYCLE.md](DELIVERY-LIFECYCLE.md)为准。历史编号只按[DELIVERY-MIGRATION.md](DELIVERY-MIGRATION.md)追溯，不得用于解锁。验机失败关联统一Issues并定向复验；NCCL不反写验机；清理/归档/客户签署齐全后才进入维护态。NP仍是网络内部断言。当前仅前端Mock与契约实现，没有真实设备后端。


## 单柜详情工作区增补

固定单柜窄画布、左侧层级导航和右侧八Tab为本轮布局要求；交互、API、元数据、Mock来源及待执行验收见[RACK-WORKSPACE.md](RACK-WORKSPACE.md)。资产/位置和既有安全模型不变。右侧按需加载，不以缺失指标显示正常。当前构建/浏览器验证按用户安排等待统一执行。


## Fabric Topology 增补

权威fixture增加topology_fabrics/networks/memberships/observed_networks/bgp_sessions/port_observations/snapshots；VNI/VRF和路由对端是explicit_synthetic_topology_v1独立演示规划/观测，不是从Cable推断。保留原包数据与source_original，双侧LLDP修复报告见TOPOLOGY-DATA-NORMALIZATION.json。覆盖known fault、降级、stale/unknown和rail-8仅规划。64B300/512GPU/24交换机/16机柜/704Cable不增加设备副本。

完整导航、模型、交互、API表与实施边界见[FABRIC-TOPOLOGY.md](FABRIC-TOPOLOGY.md)。
