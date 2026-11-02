import type {ModelingInput} from '../lib/api/engineering';

// The approved studio groups engineering objects, not workbook sheets.
export const studioModels = [
 {id:'rooms',name:'机房与坐标系',group:'facility',step:2,steps:[2,3,5],tables:['rooms'],requires:[],purpose:'明确机房边界、坐标原点与单位，所有机柜引用同一坐标系。'},
 {id:'racks',name:'机柜与尺寸',group:'facility',step:2,steps:[2,3,5],tables:['racks'],requires:['rooms'],purpose:'宽深高、容量、坐标及朝向形成明确的空间约束。'},
 {id:'placements',name:'设备落位',group:'facility',step:2,steps:[2,3,5],tables:['placements','equipment_specs'],requires:['racks'],purpose:'复用设备身份，明确 U 位、设备高度及安装面。规划位置与实际位置分开。'},
 {id:'route_nodes',name:'通道节点与机柜接入',group:'cabling',step:3,steps:[3],tables:['route_nodes','rack_access'],requires:['racks','placements'],purpose:'机柜坐标不等于可走线路；显式定义接入点与走线节点。'},
 {id:'route_edges',name:'允许布线路径',group:'cabling',step:3,steps:[3],tables:['route_edges','patch_paths'],requires:['route_nodes'],purpose:'用已测量或已审核的通道长度计算，不把直线距离当作布线长度。'},
 {id:'cable_specs',name:'线缆与光模块规格',group:'cabling',step:3,steps:[3],tables:['cable_specs','port_specs','optic_specs'],requires:[],purpose:'标准线长和两端规格由项目选择，光模块按物理端口计数。'},
 {id:'cabling_rules',name:'布线计算规则',group:'cabling',step:3,steps:[3],tables:['cabling_rules'],requires:['route_edges','cable_specs'],purpose:'选择有版本的算法与参数，通过独立参考算例后才可采用。'},
 {id:'network',name:'逻辑网络依据',group:'network_power',step:4,steps:[4],tables:['network_addresses','network_interfaces','network_planes','network_roles','network_asns','network_bgp'],requires:[],purpose:'明确导入端口、IP、ASN 与邻居，引用已发布物理版本。'},
 {id:'power',name:'供电与冷却输入',group:'network_power',step:5,steps:[5],tables:['rack_capacity','power_profiles','power_scope','power_circuits','power_connections','power_rules'],requires:['placements'],purpose:'规划负载、机柜容量与冷却方式分别导入，按机柜汇总容量和余量。'},
];
export type StudioModel = typeof studioModels[number];
export const modelCategories=[['all','全部'],['facility','机房与机柜'],['cabling','路径与布线规则'],['network_power','网络与电力']] as const;
export const modelForTable=(table:string)=>studioModels.find(m=>m.tables.includes(table));
export function modelInputState(model:StudioModel,inputs:ModelingInput[]){
 const items=inputs.filter(i=>model.tables.includes(i.table_id));
 if(!items.some(i=>i.rows>0))return 'missing';
 if(items.some(i=>i.state==='stale'))return 'stale';
 if(items.some(i=>i.issues.length))return 'invalid';
 return items.every(i=>i.state==='adopted')?'adopted':'draft';
}
export const modelStateLabels:Record<string,string>={missing:'待补充',stale:'待重新核对',invalid:'需要修正',adopted:'已核对',draft:'待核对'};
export function downloadEngineeringFile(name:string,content:string,type='text/plain;charset=utf-8'){
 const url=URL.createObjectURL(new Blob([content],{type}));
 const a=document.createElement('a');a.href=url;a.download=name;a.click();URL.revokeObjectURL(url);
}
