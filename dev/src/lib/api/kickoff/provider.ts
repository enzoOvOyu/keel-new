import { rowsFor } from './calculations';
import type { Capability, Device, PublishPreview, Workspace } from './types';
export function eligibleDevices(s: Workspace) {
  return s.devices.filter(d => !s.issues.some(i => !i.resolved && i.scope === 'identity' && i.entityId === d.id));
}
export function publishPreview(s: Workspace, ids: string[]): PublishPreview {
  const devices = eligibleDevices(s).filter(d => ids.includes(d.id));
  const keys = new Set(devices.map(d => d.id));
  const ports = s.ports.filter(p => keys.has(p.deviceId));
  const portIds = new Set(ports.map(p => p.id));
  const connections = s.connections.filter(c => portIds.has(c.a) && portIds.has(c.b));
  const components = s.components.filter(c => keys.has(c.deviceId));
  const placements = devices.filter(d => placementReady(s, d));
  return { devices, ports, connections, components, placements, held: s.devices.filter(d => !keys.has(d.id)),
    counts: { devices: devices.length, ports: ports.length, connections: connections.length, components: components.length, placements: placements.length } };
}
export function placementReady(s: Workspace, d: Device) {
  const rack = s.racks.find(r => r.id === d.rackId);
  return !!rack && rack.capacityU !== null && !!d.startU && !!d.heightU && d.startU + d.heightU - 1 <= rack.capacityU &&
    !s.issues.some(i => !i.resolved && i.scope === 'placement' && i.entityId === d.id);
}
export function capabilities(s: Workspace): Capability[] {
  const publications=s.publications.filter(p=>p.state==='published');
  const deviceMap=new Map<string,{device:Device;publication:typeof publications[number]}>();
  for(const publication of publications)for(const device of publication.deviceSnapshot)deviceMap.set(device.id,{device,publication});
  const current=[...deviceMap.values()].map(v=>v.device);
  const confirmedState=(device:Device):Workspace=>{const publication=deviceMap.get(device.id)!.publication;return {...s,models:publication.modelSnapshot,racks:publication.rackSnapshot};};
  const count=(n:number,total:number)=>total>0&&n===total?'ready':n>0?'partial':'needs_input';
  const located=current.filter(d=>placementReady(confirmedState(d),d)).length;
  const hosts=current.filter(d=>d.type==='server');
  const installation=hosts.filter(d=>{const publishedState=confirmedState(d);return rowsFor(publishedState,'boot_interfaces').some(b=>b.device_id===d.id&&b.port&&b.mac&&b.boot_mode&&b.network)&&rowsFor(publishedState,'install_profiles').some(p=>(p.scope===d.id||p.scope==='servers')&&p.image_ref&&p.disk_policy&&p.architecture);}).length;
  const network=current.filter(d=>{const publishedState=confirmedState(d);return rowsFor(publishedState,'network_roles').some(r=>r.device_id===d.id)&&rowsFor(publishedState,'network_addresses').some(a=>a.device_id===d.id&&a.address&&a.asn)&&rowsFor(publishedState,'network_policies').some(p=>(p.scope===d.id||p.scope===d.type+'s')&&p.profile);}).length;
  const ready = eligibleDevices(s).length;
  return [
    {id:'cmdb',title:'CMDB 资产与关系',detail:current.length ? '统一身份已可供下游读取' : '确认后即可建立统一资产视图',state:count(current.length,s.devices.length),available:current.length,total:s.devices.length,action:current.length?'查看入库版本':`确认 ${ready} 台可用设备`,href:'publish'},
    {id:'rack',title:'机柜图与空间',detail:'立面按已知U位绘制，空间参数可后补',state:count(located,s.devices.length),available:located,total:s.devices.length,action:'补充空间与落位',href:'models/facility'},
    {id:'fabric',title:'Fabric 网络交付',detail:current.length?'物理清单可用，配置生成按目标检查意图':'先接入资产，再逐步形成网络计划',state:count(network,s.devices.length),available:network,total:s.devices.length,action:'完善网络意图',href:'models/network'},
    {id:'install',title:'装机管理',detail:hosts.length?`${hosts.length} 台服务器可进入待装机清单`:'设备入库后即可建立待装机清单',state:count(installation,s.devices.filter(d=>d.type==='server').length),available:installation,total:s.devices.filter(d=>d.type==='server').length,action:'关联启动与安装策略',href:'models/provisioning'},
  ];
}
