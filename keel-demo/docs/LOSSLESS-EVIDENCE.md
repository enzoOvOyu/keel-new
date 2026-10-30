# 无损网络检查与分层证据（Mock v2）

本轮定向改造，保留六阶段、NP编号与历史LL记录；没有真实设备连接、自动调参或计数清零。所有演示参数是显式项目合成基线，不是NVIDIA默认值。资料核对日期2026-10-30。

## 官方依据与适配矩阵

- [Quality of Service / PFC Watchdog](https://docs.nvidia.com/networking-ethernet-software/cumulus-linux-518/Layer-1-and-Switch-Ports/Quality-of-Service/#pfc-watchdog)
- [Dynamic ECN](https://docs.nvidia.com/networking-ethernet-software/cumulus-linux-518/Layer-1-and-Switch-Ports/Quality-of-Service/#dynamic-ecn)
- [Lossless Headroom Based on Small Packet Probability](https://docs.nvidia.com/networking-ethernet-software/cumulus-linux-518/Layer-1-and-Switch-Ports/Quality-of-Service/#lossless-headroom-based-on-small-packet-probability)
- [RoCE及NVUE各列/计数](https://docs.nvidia.com/networking-ethernet-software/cumulus-linux-518/Layer-1-and-Switch-Ports/Quality-of-Service/RDMA-over-Converged-Ethernet-RoCE/)
- [What's New](https://docs.nvidia.com/networking-ethernet-software/cumulus-linux-518/Whats-New/)、[Release Notes](https://docs.nvidia.com/networking-ethernet-software/cumulus-linux-518/Whats-New/rn/)

| 检查项@2.0.0 | 官方章节 | 只读命令/字段族 | 适用范围及Keel判据 | 证据 |
|---|---|---|---|---|
| LL-CLASSIFICATION | QoS Classification / RoCE | `nv show qos roce`，trust、PCP/DSCP→SP、SP→TC、ETS；`nv show interface <port> qos roce status` | 绑定已审批型号/ASIC/精确NOS/模式/版本；逐表比对基线，不假设SP/TC/PG/pool同号 | 独立baseline、applied、operational |
| LL-PFC-FLOW | QoS PFC / RoCE | `nv show interface <port> qos pfc` + `... qos roce counters` rx/tx pause-packets | lossless/single-ipool：双方向启用/窗口；PAUSE非零不是死锁。lossy基线不适用项说明原因而非PASS | Rx/Tx与SP作用域、当前增量 |
| LL-ECN-MODE | Dynamic ECN / RoCE硬件说明 | `nv show interface <port> qos congestion-control` + status/counters | relative使用percent，absolute使用bytes；动态能力按ASIC/补丁配置。Spectrum-4只接受文档150000→159744 bytes精确案例，不推广任意容差 | 模式/单位/应用与硬件值；流量窗口 |
| LL-CNP-HOST | RoCE scheduling/counters | `nv show qos roce` / status，`... counters` tx-cnp-stats | 按项目CNP TC/调度；交换机CNP不足证明主机响应，需独立同窗口/作用域主机证据 | 调度、发送增量、主机响应、Execution |
| LL-BUFFER-SCOPE | RoCE pool / Headroom | `nv show interface <port> qos roce status pool-map` | 各模式独立pool/PG/headroom及小包概率基线；占用与历史峰值不能推导ECN失败 | pool/PG/TC各自身份、单位、来源 |
| LL-DROP-WINDOW | RoCE counters | `nv show interface <port> qos roce counters` rx/tx discards | 同窗口RoCE丢弃增量；无流量零值不证明测试通过，不用累计值证明当前故障 | 同scope采样对、受控测试 |
| LL-WATCHDOG | QoS watchdog / 5.18.2更新 | `nv show interface <port> qos pfc-watchdog`；`nv show qos pfc-watchdog` | 5.18.2/3读取实际启用条件；旧补丁不强求新字段。当前窗口触发/处置/恢复保留，恢复不抹除丢弃 | TC状态/窗口deadlock增量/事件链 |

只接受明确的5.18.0/1/2/3适配配置；未知精确NOS不能按5.18.x猜测。relative能力需匹配Spectrum-4及以上适配，不代表所有型号/补丁字段均已真机认证。生产采样周期、PFC拥塞持续时长阈值、pause-duration单位、Headroom公式、通用硬件归一化均待项目/设备验证，不内置生产默认。发行说明中的已知问题需要现场版本/SDK关联，不自动执行修复建议。

## 历史规则迁移

旧实现只有UI标签和配置存在性检查，没有可复用的窗口判定器，因此语义变化使用新ID：LL-01→LL-PFC-FLOW，LL-02→LL-ECN-MODE，LL-03→LL-CNP-HOST，LL-04/05→LL-BUFFER-SCOPE，LL-06→LL-DROP-WINDOW，LL-07→LL-CLASSIFICATION；LL-WATCHDOG新增。旧@1.0.0保留historical_only，禁止拿旧绿色状态作为v2证明。

## 数据、采样与安全

LosslessEvidence统一记录project/device/port、direction、SP、TC、PG、pool、path/flow_scope/traffic_type、model/ASIC/NOS/mode、baseline/config/topology revision、rule@version、source/command/raw_output_ref/parser_version、observed_at/window/valid_until、capability/freshness/coverage、原值/单位/增量及Execution。批准基线、applied、operational、counter/test分层；`nv show`不是统一配置态。

计数使用十进制字符串保留64位精度；同窗口/方向/流量范围计算。boot或epoch变化、缺测、采样错位、下降且无法确认回绕、未知位宽/单位均BLOCKED，不补零、不自动清零。确认位宽和回绕才计算精确增量。比例分子/分母须同窗口/范围/方向/单位且分母非零，无固定5%异常规则。历史watchdog事件和峰值保留但不用于当前窗口失败。

PASS/FAIL/BLOCKED/WAIVED与capability、freshness、applicability独立。not_applicable不伪造PASS，汇总仅排除有模式依据的不适用项。WAIVED不由此规则器自动创建，不替代配置或安全审批。任何PASS只覆盖列明设备/端口/路径/流量/窗口。

## 六阶段与工单

网络就绪只检查本阶段的交换机适用配置/运行证据，不等待主机LLDP/RDMA。idle ECN仅可给出“网络配置相符”结论，不给出运行测试通过。验机补齐双端与受控RoCE证据；NCCL读取当前验机和当前无损证据，不能反改验机。

配置基线、running revision或Cable关系版本变化使旧证据失效；新增端口进入必需覆盖，不能用单口PASS覆盖设备。确认FAIL自动关联统一Issue（项目/设备/端口/rule@version/原始证据/Execution）。整改需新的有效观测；定向复验不自动修复、不改写旧窗口，人工关闭不恢复PASS，当前证据和绑定配置/拓扑版本均有效才允许关闭。

## 页面和接口

Fabric“无损”、交换机抽屉无损Tab、Topology“PFC/ECN/BGP”、机柜Network共用LosslessEvidencePanel/同一规则器；拓扑节点明确显示独立无损verdict，不把物理链路健康当作无损证明。列表端口过滤及规则选择、五层证据切换、读取固定Mock快照、关联工单、定向复验可操作。读取历史快照不会伪造新采样时间。

新增 `/projects/{project_id}/fabric/devices/{device_id}/lossless/{evidence|conclusion|collect|retests|raw/{evidence_id}}`，见Lossless* OpenAPI资源Schema。查询on-demand，Query key含项目/设备/端口/用户，支持AbortSignal；写操作权限与冻结revision/幂等作用域校验。Mock与未来Go共用LosslessProvider，未实施Go handler或设备连接。

## Mock与真实缺口

fixture增加lossless_rules/profiles/baselines/evidence/raw_outputs/host_evidence/retests。覆盖正常PFC、idle ECN、relative、合法精确硬件差异、watchdog链、reset、stale、双端缺失、lossy、single-ipool、未知NOS及旧补丁。只为部分端口提供证据，其余覆盖缺口必须BLOCKED。新增身份profile是明确合成观测，不把原包5.x历史身份冒充精确现场版本。

当前适配器认证范围仅`keel-normalized-nvue-fixture/v1`分层帧，不声称原生NVUE文本/JSON解析、真实计数单位/能力覆盖或完整设备验证已通过。真实raw输出、采样间隔/抖动、主机拥塞响应、生产阈值及非示例硬件归一化仍需现场认证。
