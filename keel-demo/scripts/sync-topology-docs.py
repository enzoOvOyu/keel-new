from pathlib import Path
import re
root=Path(__file__).resolve().parents[1]
files=['PRD.md','ARCHITECTURE.md','SYSTEM-DESIGN.md','DESIGN-SYSTEM.md','UI-FLOWS.md','UI-REFERENCE.md','DATA-DICTIONARY.md','API-CONVENTIONS.md','DOMAIN-API-CONTRACTS.md','MOCK-DATA-CONTRACT.md','ACCEPTANCE.md','TEST-STRATEGY.md','IMPLEMENTATION-PLAN.md']
sections={
'PRD.md':'''FR-26 Fabric Topology：Fabric独立Topology入口；物理布线与逻辑网络使用独立模型共享CMDB稳定ID。双端线缆/端口、逻辑层级与规划/实际Diff、双视图定位、局部展开与历史查询均按FABRIC-TOPOLOGY实现。观察页面不承担审批或生产变更。原FR与六阶段依赖保持不变。''',
'ARCHITECTURE.md':'''Topology查询投影不持有第二套Device事实。物理视图组合BOM/Cable/PhysicalPort与LLDP/接口/DDM；逻辑视图组合批准规划、独立observed配置、BGP。Typed Mock Provider隔离数据源；按需Query支持AbortSignal、作用域key、分组及cursor渐进展开。Go接入在用户确认前端后逐模块实施。''',
'SYSTEM-DESIGN.md':'''物理Cable边和逻辑membership/BGP/containment边不能合并；关联只由显式稳定设备/端口ID建立。planned/observed两种边样式，health五值独立于交付阶段PASS/FAIL。逻辑异常映射至同一端口/Cable/Issue，不能反向改变验机结果。历史快照不可变，找不到at版本不使用当前数据代替。''',
'DESIGN-SYSTEM.md':'''Topology采用左侧主画布+右侧详情。Physical/Logical双视图切换保留筛选、设备与线缆上下文。规划虚线，观测实线；healthy绿、warning黄、critical红、unknown灰、planned蓝，配合文本。选中线缆高亮双端/路径，无关对象降低亮度；端口详情内联，设备详情使用唯一共享Drawer。''',
'UI-FLOWS.md':'''Fabric→Topology→物理全局分组→机柜/Pod展开→设备/端口/线缆→双端证据；线缆→明确逻辑网络→配置Diff/BGP异常→物理设备/端口→统一Issues→处置/定向复验。逻辑网络无关联时unknown，不能凭物理连线补造。切换视图保留scope/filter/focus，URL支持返回。''',
'UI-REFERENCE.md':'''React Flow承载拓扑的缩放、平移、节点拖动、Minimap与自动布局；域模型由Keel定义。不能把网络拓扑做成RackView的替代品或一张端口巨表。详情遵循共享Drawer与紧凑DataTable，规划/观测的图例及证据状态始终可见。''',
'DATA-DICTIONARY.md':'''复用Device/PhysicalPort/Cable/Rack稳定ID，不新增Machine/Switch身份。TopologyFabric、TopologyNetwork、TopologyMembership为版本化逻辑规划投影；TopologyObservedNetwork、TopologyBGPSession、TopologyPortObservation为独立观测；TopologySnapshot保存不可变来源切片。network_id、device_id、port_ids、baseline_id、source/time/freshness/support及revision必须可追溯。专用时序/日志仍通过资产关联，不塞入CMDB。''',
'API-CONVENTIONS.md':'''新增/projects/{project_id}/fabric/topology/{physical|logical|filters}与/details/{kind}/{id}，资源专用FabricTopology* schemas。显式项目ID、范围/分组/过滤/cursor/limit(10..120)/at；local必须device_id。权限失败403、无对象/历史404、参数422。只读查询可重试，不产生审批或设备副作用；懒加载协议见FABRIC-TOPOLOGY。''',
'DOMAIN-API-CONTRACTS.md':'''FabricTopologyGraph返回nodes/edges/total_nodes/total_edges/next_cursor/warnings/snapshot_at/request_id。node/edge引用CMDB UID，物理边必须source_port_id/target_port_id/cable_id。detail返回双端、显式network关联、逐字段Diff、LLDP/FDB/ARP/DDM/BGP及统一Issue/Execution。物理/逻辑查询与节点/边详情均具专用schema，不能退化GenericObject。''',
'MOCK-DATA-CONTRACT.md':'''权威fixture增加topology_fabrics/networks/memberships/observed_networks/bgp_sessions/port_observations/snapshots；VNI/VRF和路由对端是explicit_synthetic_topology_v1独立演示规划/观测，不是从Cable推断。保留原包数据与source_original，双侧LLDP修复报告见TOPOLOGY-DATA-NORMALIZATION.json。覆盖known fault、降级、stale/unknown和rail-8仅规划。64B300/512GPU/24交换机/16机柜/704Cable不增加设备副本。''',
'ACCEPTANCE.md':'''TOPO-01…09：独立双视图；双端稳定引用；Cable选中高亮；Fabric/Pod/Rail/VLAN/VRF/VNI/BGP；双向定位；规划/实际/观测分离；端口/设备/Issue下钻；分组/局部/cursor；Mock与CMDB一致。当前NOT RUN，待用户指定集中测试时间；不得用契约生成替代浏览器验收。''',
'TEST-STRATEGY.md':'''待执行关键检查：双端引用完整性与资产数量；逻辑关系不依赖Cable推断；healthy需充分新鲜证据、陈旧/缺失不能健康；选择/双视图保留上下文；规划/实际Diff；Issue/Drawer链接；权限/历史缺失；大图初始分组+局部与cursor。只测试受影响交付链/Rack/Topology；当前按用户要求未执行构建和浏览器回归。''',
'IMPLEMENTATION-PLAN.md':'''执行顺序：六阶段+BOM独立入口→Rack三区布局与八Tab→Fabric Topology物理/逻辑交互及显式关联→契约/Mock/文档同步。前三部分只做前端Mock；集中构建/浏览器由用户安排，未验证保持NOT RUN，之后逐模块考虑Go。'''
}
for name in files:
 p=root/'docs'/name;s=p.read_text();marker='## Fabric Topology 增补';s=s.split('\n'+marker)[0]+'\n\n'+marker+'\n\n'+sections[name]+'\n\n完整导航、模型、交互、API表与实施边界见[FABRIC-TOPOLOGY.md](FABRIC-TOPOLOGY.md)。\n';p.write_text(s)
p=root/'AGENTS.md';s=p.read_text();marker='## Fabric Topology 执行约束';s=s.split('\n'+marker)[0]+'''\n\n## Fabric Topology 执行约束
- Fabric下独立Topology，物理Cable与逻辑网络分模型，禁止从物理线缆推断VLAN/VRF/VNI/BGP。
- 稳定资产/端口ID来自CMDB，显式演示扩展只在统一fixture，不复制资产。
- 先完成交互、契约、Mock和文档；Go/真实设备不在本轮范围。
- 用户要求集中测试前不重复构建/浏览器回归；当前三项变更保持NOT RUN。
''';p.write_text(s)
