import { useEffect, useState } from 'react';
import { ArrowRight, FileSpreadsheet, Link2, MapPin, Server } from 'lucide-react';
import { useWorkspace } from '../context';
import type { Evidence } from '../lib/api';
import { Badge, Button, KV, Modal } from './ui';

export function EvidenceDrawer({kind,id,evidence:given,close}:{kind:'device'|'port'|'connection'|'component'|'evidence';id:string;evidence?:Evidence;close:()=>void}) {
  const {api,state,inspect,go}=useWorkspace();
  const sourceOnly=kind==='evidence'&&!!given;
  const key=id+':'+state.revision;
  const [request,setRequest]=useState(0);
  const [context,setContext]=useState<{key:string;status:'loading'|'ready'|'error';error?:string}>({key:'',status:'loading'});
  useEffect(()=>{if(sourceOnly){setContext({key,status:'ready'});return;}let active=true;setContext({key,status:'loading'});void api.loadContext!(id).then(()=>{if(active)setContext({key,status:'ready'});}).catch(e=>{if(active)setContext({key,status:'error',error:e.message});});return()=>{active=false;};},[api,id,state.revision,request,sourceOnly,key]);
  const device=sourceOnly?undefined:state.devices.find(d=>d.id===id),port=sourceOnly?undefined:state.ports.find(p=>p.id===id),link=sourceOnly?undefined:state.connections.find(c=>c.id===id),component=sourceOnly?undefined:state.components.find(c=>c.id===id);
  const evidence=given??device?.evidence??port?.evidence??link?.evidence??component?.evidence;
  const source=state.sources.find(s=>s.id===evidence?.sourceId);
  const title=(device?.name||device?.serial)||port?.name||component?.name||(link?'物理连接':'来源证据');
  const a=state.ports.find(p=>p.id===link?.a),b=state.ports.find(p=>p.id===link?.b);
  const rack=state.racks.find(r=>r.id===device?.rackId);
  const ownPorts=state.ports.filter(p=>p.deviceId===id),portIds=new Set(ownPorts.map(p=>p.id));
  const ownLinks=state.connections.filter(c=>portIds.has(c.a)||portIds.has(c.b));
  const findings=sourceOnly?[]:state.issues.filter(i=>!i.resolved&&i.entityId===id).map(i=>({title:i.title,impact:i.impact,action:i.action??(i.scope==='identity'?'review?category=devices&filter=blocked':i.scope==='configuration'?'models/cabling':'models/facility'),block:i.scope==='identity'}));
  if(device){
    if(!rack)findings.push({title:'机柜未关联',impact:'设备可先入库；无法归入机柜图。',action:'models/facility',block:false});
    if(device.startU===null)findings.push({title:'缺 U 位',impact:'设备可先入库；无法定位柜内位置。',action:'models/facility',block:false});
    if(device.heightU===null&&!findings.some(v=>v.title.includes('高度')))findings.push({title:'缺设备高度',impact:'无法确认完整占位；不影响设备身份入库。',action:'models/facility',block:false});
    if(rack&&rack.capacityU===null)findings.push({title:'缺机柜容量',impact:'无法检查 U 位是否越界。',action:'models/facility',block:false});
  }
  if(port&&!state.devices.some(d=>d.id===port.deviceId))findings.push({title:'所属设备缺失',impact:'无法确认端口归属。',action:'review?category=devices',block:true});
  if(link&&(!a||!b))findings.push({title:'连接端点缺失',impact:'此连接不能作为完整双端连接入库。',action:'review?category=ports',block:true});
  findings.sort((a,b)=>Number(b.block)-Number(a.block));
  const related=(category:string)=>{go(`review?category=${category}&q=${encodeURIComponent(device?.name||device?.serial||'')}`);close();};
  if(context.key!==key||context.status!=='ready')return <Modal title={title} description="资产、关系与字段依据使用同一引用。" close={close} drawer>
    {context.key===key&&context.status==='error'?<div className="notice error" role="alert"><span>{context.error||'对象详情暂时无法读取'}</span><Button onClick={()=>setRequest(n=>n+1)}>重新加载</Button></div>:<div className="empty" aria-busy="true" role="status">正在读取对象与关联信息…</div>}
  </Modal>;
  return <Modal title={title} description="资产、关系与字段依据使用同一引用。" close={close} drawer>
    {!!findings.length&&<section className="detail-findings" aria-label="核对事项">{findings.map((finding,i)=><div className={'detail-finding '+(finding.block?'block':'')} key={i}><b>{finding.title}</b><p>{finding.impact}</p><button className="text-link" onClick={()=>{go(finding.action);close();}}>{finding.action.startsWith('models/')?'补充工程信息':'查看待核对对象'}<ArrowRight size={12}/></button></div>)}</section>}
    {device&&<><div className="detail-hero"><span><Server size={23}/></span><div><b>{device.type==='server'?'服务器':device.type==='switch'?'交换机':'设备类型待补'}</b><small>{device.role||'规划设备'}</small></div><Badge state={findings.some(f=>f.block)?'blocked':state.publications.some(p=>p.state==='published'&&p.deviceIds.includes(device.id))?'published':'ready'}/></div><KV values={[...(device.serial&&device.serial!==title?[["SN",device.serial] as [string,string]]:[]),["型号",device.model],["管理地址",device.managementAddress],["机房",rack?.room],["机柜",rack?.name??'待关联'],["起始U",device.startU?`U${device.startU}`:''],["占用高度",device.heightU?`${device.heightU}U`:'待核对']]}/><div className="detail-section"><h3>关联对象</h3><div className="detail-numbers"><button onClick={()=>related('components')}><b>{state.components.filter(c=>c.deviceId===id).length}</b> 组件</button><button onClick={()=>related('ports')}><b>{ownPorts.length}</b> 端口</button><button onClick={()=>related('connections')}><b>{ownLinks.length}</b> 规划连接</button></div><table className="detail-port-list"><thead><tr><th>端口名称</th><th>速率</th><th>角色</th><th/></tr></thead><tbody>{ownPorts.slice(0,6).map(p=><tr key={p.id}><td><code>{p.name}</code></td><td>{p.speed||'未提供'}</td><td>{p.role||'—'}</td><td><button aria-label={`查看端口 ${p.name}`} onClick={()=>inspect('port',p.id)}><ArrowRight size={13}/></button></td></tr>)}</tbody></table>{ownPorts.length>6&&<Button kind="ghost" onClick={()=>related('ports')}>查看全部 {ownPorts.length} 个端口</Button>}</div></>}
    {port&&<><KV values={[["所属设备",state.devices.find(d=>d.id===port.deviceId)?.name],["MAC",port.mac],["速率",port.speed],["端口角色",port.role]]}/><h3 className="detail-title">关联连接</h3>{state.connections.filter(c=>c.a===id||c.b===id).map(c=><button className="relationship-row" key={c.id} onClick={()=>inspect('connection',c.id)}><Link2 size={15}/><span>{(()=>{const peer=state.ports.find(p=>p.id===(c.a===id?c.b:c.a));return `${state.devices.find(d=>d.id===peer?.deviceId)?.name||'设备待关联'} / ${peer?.name||'端口待关联'}`;})()}</span><ArrowRight size={14}/></button>)}<Button onClick={()=>inspect('device',port.deviceId)}>查看所属设备</Button></>}
    {component&&<><KV values={[["型号",component.model],["类型",component.type.toUpperCase()],["槽位/位置",component.slot],["父设备",state.devices.find(d=>d.id===component.deviceId)?.name]]}/><Button onClick={()=>inspect('device',component.deviceId)}>查看父设备</Button></>}
    {link&&<><div className="connection-detail">{[a,b].map((p,i)=><div key={i}><small>{i===0?'本端':'对端'}</small><b>{state.devices.find(d=>d.id===p?.deviceId)?.name??'待关联'}</b><button onClick={()=>p&&inspect('port',p.id)}>{p?.name}<ArrowRight size={13}/></button></div>)}</div><KV values={[["介质",link.medium],["连接状态","规划连接"],["实际安装长度",'待现场确认']]}/><Button onClick={()=>{go('models/cabling');close();}}>查看布线与计算</Button></>}
    {evidence&&<div className="evidence-card"><div><FileSpreadsheet size={18}/><h3>字段来源</h3><Badge state={evidence.method}/></div><KV values={[["资料",source?.name],["工作表",evidence.sheet],["单元格范围",evidence.cell],["来源版本",source?`v${source.version}`:''],["摘要",source?source.hash.slice(0,18)+'…':'']]}/>{evidence.note&&<p className="source-note">{evidence.note}</p>}<Button kind="ghost" disabled={!source} onClick={()=>{go(`sources?source=${encodeURIComponent(evidence.sourceId)}`);close();}}>查看资料上下文 <ArrowRight size={14}/></Button></div>}
    {!sourceOnly&&<details className="detail-identifiers"><summary>内部标识</summary><code>{id}</code></details>}
    <div className="detail-foot"><MapPin size={14}/><span>规划记录与实际观测分别保留</span></div>
  </Modal>;
}
