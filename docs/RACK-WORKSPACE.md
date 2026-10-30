# 单柜工作区布局增补

当前实现任务：固定单柜画布、扩大右侧详情；全局 `/racks` 机房层与 `/racks/:rackId` 单柜层独立。设备身份与现场位置仍以 BOM/CMDB 为准；拖动只产生可审核布局草稿，不创建资产副本，不覆盖发布版本。

## 布局与交互

桌面三列 180 / 315 / 剩余宽度 px；宽屏 195 / 335 / 剩余宽度。中间始终一个机柜，保留正反面、缩放/平移、U刻度、拖动、方向键移动、撤销/重做、冲突检查、生成差异、审核发布、SVG/PNG/PDF导出。窄屏将详情区移至下一行，不扩大单柜为机房全局图。

左侧机房/Pod/机柜导航支持搜索 hostname/SN；展示设备、健康与关联告警。右侧八 Tab：Overview、Environment、Devices、Selected Device、Network、Topology、Alerts & Events、History。设备点击/设备行点击同步选中并定位U位，切换Selected Device；右侧可打开唯一统一设备抽屉，禁止抽屉叠抽屉。端口对端定位同时切换机柜与选中设备。

`rack`、`selected`、`rack_tab` 存URL，详情Tab保存滚动。当前Tab首次打开才调用 `GET /projects/{project_id}/racks/{rack_id}/workspace/{tab}?device_id=...`，TanStack Query key绑定项目、机柜、Tab、设备、身份与场景；请求可取消。布局编辑历史独立于实时观测刷新。

## API 与事实

| Tab | 返回数据 | 规则 |
|---|---|---|
| Overview | Rack、BOM/CMDB设备与位置 | 草稿占用与现场位置分开 |
| Environment | 环境传感器、PDU、逐指标时序 | 1h/24h/7d按固定演示时间窗口；null断线；液冷unsupported、电压unknown |
| Devices | 当前柜设备 | 搜索/排序/分页，行点击定位画布 |
| Selected Device | 资产、BMC、固件、批准基线、observed配置 | 单一资产ID；维护态IPMI-only，历史Redfish不授权 |
| Network | 端口、Cable双端、LLDP/FDB/ARP、DDM | 规划速率与实际接口区分；单样本DDM无虚构趋势 |
| Topology | GPU-NIC矩阵、BDF、归属、Cable | 1:1配对不代表PCIe/NVLink桥接，缺证据unknown |
| Alerts & Events | 同一告警与Issues | 深链跳转，不创建第二工单系统 |
| History | 图版本、采集、Execution、审计 | 保留BOM来源版本，不改写历史 |

每项观测展示source/observed_at/freshness/support及单位。机柜Pod分组和新环境时序是`explicit_synthetic_demo_group_v1`/`explicit_synthetic_rack_timeseries_v1`，集中在权威fixture；不代表客户现场或生产阈值。保留健康、异常、陈旧、未知、维护及无数据场景。液冷/电压能力缺失明确显示，不填零。

## 验收状态

本次实现已写入代码、Mock与契约。最小启动与交互检查已实际执行并通过，见下节；完整八Tab/导出/权限等集中验收仍 **NOT RUN**。不得沿用历史报告声称本轮完整验收通过。


### 实际最小可用性检查

2026-10-30：生产构建首次通过；Chromium实际打开机柜与Topology，验证单柜/详情宽度、设备选择、环境懒加载/7d范围、精确线缆双端高亮、物理→逻辑与配置Diff，page_errors为空。详见TOPOLOGY-RACK-SMOKE.json。完整业务/权限/历史/导出回归仍NOT RUN，等待用户安排；不把本次最小检查等同全量验收。


### Rack Mapping：所选设备关联追溯（当前定义）

Mapping恢复重构前三列工作区中“设备与连接追溯”的内容，不再作为GPU-NIC矩阵页面。支持服务器和交换机：设备型号/SN/稳定ID/来源BOM、草稿与实际位置、“连接/端口”切换、线缆A/B两端设备/端口/介质/长度、MAC与状态、对端定位和统一设备详情抽屉。切换画布设备或定位对端保留Mapping上下文，清除旧设备的关联内容；连接和端口支持搜索、分批加载，缺失引用显示unknown。GPU-NIC矩阵继续由Topology/Validation承载。

workspace/mapping按需返回ports/cables及授权peer_ports/peer_devices；关联来自原有BOM/CMDB稳定ID，不创建资产副本。Cable双端与端口peer引用不同要明确提示冲突，不以端口状态或历史线序结果伪造当前验证通过。
