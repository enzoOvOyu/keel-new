import { useEffect, useState } from 'react';
import { ArrowRight, FileSpreadsheet, Link2, MapPin, Network, Server } from 'lucide-react';
import { useWorkspace } from '../context';
import type { Evidence } from '../lib/api';
import { Badge, Button, KV, Modal } from './ui';

export function EvidenceDrawer({kind,id,evidence:given,close}:{kind:'device'|'port'|'connection'|'component'|'evidence';id:string;evidence?:Evidence;close:()=>void}) {
  const {api,state,inspect,go}=useWorkspace();
  const key=id+':'+state.revision;
  const [request,setRequest]=useState(0);
  const [context,setContext]=useState<{key:string;status:'loading'|'ready'|'error';error?:string}>({key:'',status:'loading'});
  useEffect(()=>{let active=true;setContext({key,status:'loading'});void api.loadContext!(id).then(()=>{if(active)setContext({key,status:'ready'});}).catch(e=>{if(active)setContext({key,status:'error',error:e.message});});return()=>{active=false;};},[api,id,state.revision,request]);
  const device=state.devices.find(d=>d.id===id),port=state.ports.find(p=>p.id===id),link=state.connections.find(c=>c.id===id),component=state.components.find(c=>c.id===id);
  const evidence=given??device?.evidence??port?.evidence??link?.evidence??component?.evidence;
  const source=state.sources.find(s=>s.id===evidence?.sourceId);
  const title=device?.name??port?.name??component?.name??(link?'物理连接':'来源证据');
  const a=state.ports.find(p=>p.id===link?.a),b=state.ports.find(p=>p.id===link?.b);
  if(context.key!==key||context.status!=='ready')return <Modal title={title} description="资产、关系与字段依据使用同一引用。" close={close} drawer>
    {context.key===key&&context.status==='error'?<div className="notice error" role="alert"><span>{context.error||'对象详情暂时无法读取'}</span><Button onClick={()=>setRequest(n=>n+1)}>重新加载</Button></div>:<div className="empty" aria-busy="true" role="status">正在读取对象与关联信息…</div>}
  </Modal>;
  return <Modal title={title} description="资产、关系与字段依据使用同一引用。" close={close} drawer>
    {device&&<><div className="detail-hero"><span><Server size={23}/></span><div><b>{device.model}</b><small>{device.id}</small></div><Badge state={state.publications.some(p=>p.state==='published'&&p.deviceIds.includes(device.id))?'published':'ready'}/></div><KV values={[["设备SN",device.serial],["设备类型",device.type==='server'?'服务器':device.type==='switch'?'交换机':'设备类型待补充'],["型号",device.model],["管理地址",device.managementAddress],["机柜",state.racks.find(r=>r.id===device.rackId)?.name],["起始U",device.startU?`U${device.startU}`:''],["占用高度",device.heightU?`${device.heightU}U`:'待核对']]}/><div className="detail-section"><h3>组件与连接</h3><div className="detail-numbers"><span><b>{state.components.filter(c=>c.deviceId===id).length}</b>组件</span><span><b>{state.ports.filter(p=>p.deviceId===id).length}</b>端口</span></div><div className="detail-list">{state.ports.filter(p=>p.deviceId===id).slice(0,6).map(p=><button key={p.id} onClick={()=>inspect('port',p.id)}><Network size={14}/><span>{p.name}</span><small>{p.speed}</small><ArrowRight size={13}/></button>)}</div></div></>}
    {port&&<><KV values={[["稳定ID",port.id],["所属设备",state.devices.find(d=>d.id===port.deviceId)?.name],["端口名称",port.name],["MAC",port.mac],["速率",port.speed],["端口角色",port.role]]}/><h3 className="detail-title">关联连接</h3>{state.connections.filter(c=>c.a===id||c.b===id).map(c=><button className="relationship-row" key={c.id} onClick={()=>inspect('connection',c.id)}><Link2 size={15}/><span>{c.id}</span><ArrowRight size={14}/></button>)}<Button onClick={()=>inspect('device',port.deviceId)}>查看所属设备</Button></>}
    {component&&<><KV values={[["组件ID",component.id],["型号",component.model],["类型",component.type.toUpperCase()],["槽位/位置",component.slot],["父设备",state.devices.find(d=>d.id===component.deviceId)?.name]]}/><Button onClick={()=>inspect('device',component.deviceId)}>查看父设备</Button></>}
    {link&&<><div className="connection-detail">{[a,b].map((p,i)=><div key={i}><small>{i===0?'本端':'对端'}</small><b>{state.devices.find(d=>d.id===p?.deviceId)?.name??'待关联'}</b><button onClick={()=>p&&inspect('port',p.id)}>{p?.name}<ArrowRight size={13}/></button></div>)}</div><KV values={[["连接引用",link.id],["介质",link.medium],["连接状态","规划连接"],["计算长度",'待计算'],["实际安装长度",'待现场确认']]}/><Button onClick={()=>{go('models/cabling');close();}}>查看布线与计算</Button></>}
    {evidence&&<div className="evidence-card"><div><FileSpreadsheet size={18}/><h3>字段来源</h3><Badge state={evidence.method}/></div><KV values={[["资料",source?.name],["工作表",evidence.sheet],["单元格范围",evidence.cell],["来源版本",source?`v${source.version}`:''],["摘要",source?.hash.slice(0,18)+'…']]}/><Button kind="ghost" onClick={()=>{go(`sources?source=${evidence.sourceId}`);close();}}>查看资料上下文 <ArrowRight size={14}/></Button></div>}
    <div className="detail-foot"><MapPin size={14}/><span>规划记录与实际观测分别保留</span></div>
  </Modal>;
}
