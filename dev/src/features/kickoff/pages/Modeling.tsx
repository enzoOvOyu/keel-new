import { useEffect, useState } from 'react';
import { useParams, useSearchParams } from 'react-router-dom';
import { ArrowRight, Box, Check, Download, Edit3, FileSpreadsheet, Layers, Network, Plus, Server, SlidersHorizontal, Upload, Zap } from 'lucide-react';
import { useWorkspace } from '../context';
import { domains, tables } from '../lib/api';
import { exportWorkbook } from '../lib/api/workbook';
import { Badge, Button, DataTable, Empty, Heading, Notice, Section, formatDate } from '../components/ui';
import { ModelEditor } from '../components/ModelEditor';
import { CalculationPanel } from '../components/CalculationPanel';
import { FacilityPanel } from '../components/facility/FacilityPanel';
import { entityOptions, resolveEntity } from '../components/facility/FacilityEntities';

export default function Modeling(){
  const {domain:routeDomain}=useParams();
  const domain=domains.find(d=>d.id===routeDomain);
  const {state:s,go,inspect,can}=useWorkspace();
  const [params,setParams]=useSearchParams();
  const defs=tables.filter(t=>t.domain===domain?.id);
  const def=defs.find(t=>t.id===params.get('table'))??defs[0];
  const input=s.models.find(m=>m.tableId===def?.id);
  const [editor,setEditor]=useState<string>();
  useEffect(()=>setEditor(undefined),[routeDomain]);
  if(!domain||!def)return <Empty title="工程域不存在" description="请从左侧导航选择一个工作区。"/>;
  const known=defs.filter(t=>s.models.some(m=>m.tableId===t.id&&m.rows.length)).length;
  const labelFor=(key:string,value:string)=>{const options=entityOptions(s,key);return options.find(o=>o.id===resolveEntity(options,value))?.label??value;};
  return <>
    <Heading eyebrow={`ENGINEERING / ${domain.id.toUpperCase()}`} title={domain.title} description={domain.description} action={<><Button onClick={()=>go('publish')}>查看入库版本<ArrowRight size={14}/></Button><Button kind="primary" disabled={!can('bom:edit')} onClick={()=>setEditor(def.id)}><Plus size={15}/>补充工程输入</Button></>}/>
    <div className="domain-banner"><span className={'domain-icon '+domain.id}>{domain.id==='facility'?<Server size={25}/>:domain.id==='network'?<Network size={25}/>:domain.id==='provisioning'?<Box size={25}/>:domain.id==='power'?<Zap size={25}/>:<SlidersHorizontal size={25}/>}</span><div><b>{domain.outcome}</b><p>{domain.id==='facility'?'四类输入独立维护 · 字段与关系覆盖率见下方，可点击筛选缺口':`${known} / ${defs.length} 类输入已有资料 · 可继续导入、编辑或按型号批量补充`}</p></div><Badge state={domain.id==='facility'?'partial':known===defs.length?'ready':known?'partial':'needs_input'}>{domain.id==='facility'?'工程输入 · 待发布核对':known?'已有可用输入':'等待工程输入'}</Badge></div>
    {domain.id==='facility'&&<FacilityPanel activeRack={params.get('rack')} selectRack={id=>setParams({table:'placements',rack:id})} edit={setEditor}/>}
    {domain.id==='network'&&<div className="scope-strip"><div><Network size={19}/><span><b>{s.devices.length} 台设备可纳入规划</b><small>物理连接已知，不代表逻辑协议已配置</small></span></div><div><Layers size={19}/><span><b>{s.models.find(m=>m.tableId==='network_planes')?.rows.length??0} 个明确Plane</b><small>配置生成按所选目标检查参数</small></span></div><button className="text-link" onClick={()=>setParams({table:'network_addresses'})}>补充地址与ASN<ArrowRight size={14}/></button></div>}
    {domain.id==='provisioning'&&<Section title="服务器与启动身份" caption="设备入库后可进入待装机清单；执行前再检查启动身份、镜像和网络就绪。" action={<Button onClick={()=>setEditor('boot_interfaces')}>补充启动网卡<ArrowRight size={14}/></Button>}><DataTable data={s.devices.filter(d=>d.type==='server')} columns={[{accessorKey:'name',header:'服务器',cell:({row})=><button className="text-link" onClick={()=>inspect('device',row.original.id)}>{row.original.name}<ArrowRight size={12}/></button>},{accessorKey:'serial',header:'SN'},{accessorKey:'model',header:'型号'},{id:'boot',header:'启动身份',cell:({row})=>{const b=s.models.find(m=>m.tableId==='boot_interfaces')?.rows.find(r=>r.device_id===row.original.id);return b?.mac?<code>{b.mac}</code>:<Badge state="needs_input">启动MAC待补充</Badge>;}},{id:'profile',header:'安装策略',cell:({row})=>{const p=s.models.find(m=>m.tableId==='install_profiles')?.rows.find(r=>r.scope===row.original.id||r.scope==='servers');return p?.image_ref??'按设备组关联';}}]} pageSize={4}/></Section>}
    {(domain.id==='cabling'||domain.id==='power')&&<CalculationPanel domain={domain.id} edit={setEditor}/>}
    <div className="modeling-layout"><aside className="model-rail"><div className="rail-heading"><b>工程输入</b><span>{defs.length}</span></div>{defs.map(t=>{const m=s.models.find(v=>v.tableId===t.id);return <button key={t.id} className={t.id===def.id?'active':''} onClick={()=>setParams({table:t.id})}><FileSpreadsheet size={16}/><span><b>{t.name}</b><small>{m?.rows.length?`${m.rows.length} 行 · v${m.version}`:'待补充'}</small></span>{m?.rows.length?<Check size={13}/>:<Plus size={13}/>}</button>;})}<div className="model-rail-note">各表可独立补充。留空表示未知，不等于已通过后续校验。</div></aside><section className="section model-content"><div className="model-content-heading"><div><div className="eyebrow">{def.id}</div><h2>{def.name}</h2><p>{def.description}</p></div><div className="actions"><Button kind="ghost" onClick={()=>exportWorkbook(def,input?.rows??[],!input?.rows.length)}><Download size={14}/>{input?.rows.length?'导出XLSX':'下载模板'}</Button><Button disabled={!can('bom:edit')} onClick={()=>setEditor(def.id)}><Upload size={14}/>导入</Button><Button kind="primary" disabled={!can('bom:edit')} onClick={()=>setEditor(def.id)}><Edit3 size={14}/>编辑输入</Button></div></div><div className="model-metadata"><span><Layers size={13}/>用于：{def.consumer}</span><span>{input?`${input.source} · ${formatDate(input.updatedAt)}`:'尚未提供此类输入'}</span></div>{input?.rows.length?<DataTable key={def.id+params.get('rack')} data={input.rows.filter(r=>!params.get('rack')||def.id!=='placements'||r.rack_id===params.get('rack'))} columns={def.fields.map(f=>({accessorKey:f.key,header:f.label,cell:({getValue}:any)=><span className={getValue()?'':'missing-value'}>{getValue()?labelFor(f.key,String(getValue())):'待补充'}</span>}))} pageSize={8} onRow={()=>{if(can('bom:edit'))setEditor(def.id);}}/>:<Empty title={`补充${def.name}`} description="使用标准XLSX导入，或直接在页面编辑。没有依据的字段可以保留为空。" action={<Button disabled={!can('bom:edit')} onClick={()=>setEditor(def.id)}><Plus size={14}/>开始填写</Button>}/>}<div className="section-foot"><span>变更保留版本与来源，已入库身份不改变。</span><button className="text-link" onClick={()=>go('publish')}>核对增补差异<ArrowRight size={13}/></button></div></section></div>
    {editor&&<ModelEditor key={editor} tableId={editor} close={()=>setEditor(undefined)}/>}
  </>;
}
