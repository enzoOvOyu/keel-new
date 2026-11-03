import type { Domain, TableDefinition, ModelField } from './types';

export const domains: { id: Domain; title: string; short: string; description: string; outcome: string }[] = [
  { id: 'facility', title: '机房与机柜', short: '空间', description: '关联机房、机柜和设备位置，逐步补齐空间信息。', outcome: '机柜立面 · 空间布局 · 容量统计' },
  { id: 'network', title: '网络与接入', short: '网络', description: '明确设备接入方式与网络意图，为 Fabric 提供统一输入。', outcome: '设备接入 · Fabric Blueprint · 配置生成' },
  { id: 'provisioning', title: '装机准备', short: '装机', description: '按设备组关联启动身份、镜像和安装策略。', outcome: '待装机清单 · 安装计划 · 兼容性预检' },
  { id: 'cabling', title: '布线与计算', short: '布线', description: '复用已识别连接，按明确路径和规则计算长度与用量。', outcome: '路径规划 · 线缆长度 · 采购清单' },
  { id: 'power', title: '电力与冷却', short: '电力', description: '区分规划负荷、铭牌功率和实际观测，按机柜核算容量。', outcome: '规划功率 · 供电关系 · 容量余量' },
];

const fields = (spec: string): ModelField[] => spec.split('|').map(v => {
  const [key, label, kind] = v.split(':');
  return { key, label, ...(kind ? { kind: kind as ModelField['kind'] } : {}) };
});
const table = (id: string, domain: Domain, name: string, description: string, consumer: string, spec: string): TableDefinition =>
  ({ id, domain, name, description, consumer, fields: fields(spec) });

export const tables: TableDefinition[] = [
  table('rooms','facility','机房管理','维护机房名称与归属；地址可后补，平面位置通过机房布局设置。','机房空间图','room_id:机房引用|name:机房名称|site:场地|address:地理地址|frame:坐标系|unit:坐标单位'),
  table('racks','facility','机柜规格与位置','维护机柜归属和规格；可从机柜清单批量设置机房布局。','机柜图与空间布局','rack_id:机柜引用|room_id:机房引用|capacity_u:容量U:positive|width_mm:宽mm:positive|depth_mm:深mm:positive|x:坐标X:number|y:坐标Y:number|orientation:朝向度:number|frame:坐标系|unit:坐标单位'),
  table('equipment_specs','facility','设备型号与尺寸','型号参数只补充明确匹配的设备，不能替代实际组件清单。','U位占用与安装剖面','model:型号|height_u:高度U:positive|width_mm:宽mm:positive|depth_mm:深mm:positive|face:安装面|panel_ref:面板版本引用'),
  table('placements','facility','设备落位','已识别机柜与U位自动关联；未知位置保留为空。','机柜立面与未定位清单','device_id:设备引用|rack_id:机柜引用|start_u:起始U:positive|height_u:占用U:positive|face:安装面|position:非U位置'),
  table('management','network','管理接入','管理地址、BMC与业务网卡分开；这里只保存凭据引用。','ZTP与管理接入','device_id:设备引用|management_ip:管理地址|bmc_ip:BMC地址|management_port:管理端口|credential_ref:凭据引用|method:接入方式'),
  table('network_planes','network','Fabric与Plane','由明确规划定义，不能从物理连线推测逻辑平面。','Fabric Blueprint','fabric:Fabric|plane:Plane|role:用途|architecture:网络架构|profile:策略版本引用'),
  table('network_roles','network','设备角色','引用统一设备身份，支持同一设备的明确多Plane归属。','角色与目标分组','device_id:设备引用|fabric:Fabric|plane:Plane|role:设备角色'),
  table('network_interfaces','network','接口与网络域','物理端口关联逻辑接口，保留父端口与Breakout依据。','接口配置','device_id:设备引用|port:物理端口|interface:逻辑接口|plane:Plane|vlan:VLAN|vrf:VRF|parent_port:父端口'),
  table('network_addresses','network','地址与ASN','导入明确分配结果，或引用已批准的地址池与分配规则。','地址与路由规划','device_id:设备引用|interface:接口|address:地址及前缀|asn:ASN:positive|pool_ref:批准地址池|source:规划依据'),
  table('network_bgp','network','邻居与协议','邻居、BFD和路由策略必须有规划依据。','协议配置生成','device_id:本端设备|interface:接口|peer:对端设备|peer_address:对端地址|local_asn:本端ASN:positive|peer_asn:对端ASN:positive|family:地址族|bfd_ref:BFD策略'),
  table('network_policies','network','网络策略','按实际适用范围选择RoCE/QoS、EVPN或其他已批准策略。','Fabric配置集','scope:设备或分组|profile:配置集版本|roce:RoCE/QoS策略|ar:路由策略|evpn:EVPN/VXLAN策略|source:依据'),
  table('boot_interfaces','provisioning','启动身份','明确启动网卡及其MAC，不把任意业务端口MAC当成启动身份。','PXE目标识别','device_id:设备引用|port:启动端口|mac:启动MAC|boot_mode:启动方式|network:安装网络|address_policy:地址策略'),
  table('install_profiles','provisioning','镜像与安装策略','按设备组选择一次策略，实际执行前再校验镜像和设备兼容性。','安装计划','scope:设备或分组|architecture:架构|image_ref:镜像版本引用|disk_policy:磁盘策略|bios_ref:BIOS配置引用|firmware_ref:固件基线|credential_ref:凭据引用'),
  table('route_nodes','cabling','通道节点','登记允许走线的通道节点，名称和坐标均可来自布局资料。','布线路径','code:节点编号|room_id:机房引用|x:坐标X:number|y:坐标Y:number|z:坐标Z:number|unit:坐标单位'),
  table('route_edges','cabling','允许路段','计算仅使用明确路段；路段长度以米为单位。','确定性路径计算','code:路段编号|from:起点节点|to:终点节点|distance_m:长度m:positive|category:线路类别|source:测量或规划依据'),
  table('rack_access','cabling','机柜接入点','将机柜关联到通道图，缺少接入点只影响该柜线长计算。','双端路径定位','rack_id:机柜引用|node:接入节点|lead_m:柜内引线m:number'),
  table('cable_specs','cabling','线缆与标准长度','按规格逐行登记可采购长度，计算不改变实装长度。','标准长度匹配','code:规格引用|medium:介质|connector_a:本端连接器|connector_b:对端连接器|rate:速率|length_m:标准长度m:positive|optic_mode:模块方式'),
  table('optic_specs','cabling','光模块与匹配','按物理端口统计模块需求；没有依据时不默认每端一只。','光模块需求','model:模块型号|port_type:端口类型|medium:介质|rate:速率|reach_m:适用距离m:positive|source:匹配依据'),
  table('cabling_rules','cabling','布线规则','通道最短路径＋双端引线＋明确余量，再匹配同规格标准长度。','确定性布线计算','code:规则编号|version:版本:positive|strategy:路径算法|slack_m:固定余量m:number|slack_percent:比例余量%:number|cable_spec:线缆规格引用|source:规则依据'),
  table('power_profiles','power','设备功率依据','规划、铭牌与实测分开；本页负荷计算只采用明确规划功率。','机柜负荷','device_id:设备引用|power_kw:功率kW:positive|basis:功率依据|cooling:冷却方式|source:来源'),
  table('rack_capacity','power','机柜容量','容量未知不按零值处理，也不影响设备和连接入库。','容量与冷却核算','rack_id:机柜引用|capacity_kw:可用容量kW:positive|cooling:冷却方式|cooling_kw:冷却容量kW:positive|source:容量依据'),
  table('power_circuits','power','PDU与回路','复用已有设备身份，明确供电对象的上游关系。','供电拓扑','code:供电对象|rack_id:机柜引用|upstream:上游引用|circuit:回路|capacity_kw:容量kW:positive|group:A/B分组'),
  table('power_connections','power','PSU供电连接','必须有明确的电源口与回路关联，不能从A/B标签推测冗余成立。','供电关系与冗余','device_id:设备引用|psu:电源口|circuit:回路|group:A/B分组|allocation:分摊规则引用'),
];

export const tableById = (id: string) => tables.find(t => t.id === id)!;
