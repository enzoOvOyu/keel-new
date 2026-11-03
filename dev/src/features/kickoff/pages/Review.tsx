import { useEffect, useState } from 'react';
import { useSearchParams } from 'react-router-dom';
import { ArrowRight, Check, CheckCheck, Download, FileSpreadsheet, Link2, ShieldCheck } from 'lucide-react';
import type { ColumnDef } from '@tanstack/react-table';
import { useWorkspace } from '../context';
import { eligibleDevices, placementReady, type Issue, type Evidence } from '../lib/api';
import { Badge, Button, DataTable, Field, Heading, Modal, Notice, Tabs } from '../components/ui';
import { downloadFile } from '../lib/api/workbook';

interface ReviewRow { id:string; name:string; description:string; reference:string; location:string; state:string; evidence:Evidence }
export default function Review(){
  const {api,state:s,go,inspect,run,can}=useWorkspace();
  const [params,setParams]=useSearchParams();
  const category=params.get('category')??'devices';
  const issuesOnly=params.get('filter')==='issues';
  const [issue,setIssue]=useState<Issue>(),[answer,setAnswer]=useState('');
  const issueDevice=s.devices.find(d=>d.id===issue?.entityId);
  const matches=issueDevice?.serial?s.devices.filter(d=>d.id!==issueDevice.id&&d.serial===issueDevice.serial&&d.type===issueDevice.type):[];
  const [issueLimit,setIssueLimit]=useState(20);
  const [selected,setSelected]=useState<string[]>([]);
  const eligible=new Set(eligibleDevices(s).map(d=>d.id));
  const published=new Set(s.publications.filter(p=>p.state==='published').flatMap(p=>p.deviceIds));
  const tabs=[{id:'devices',label:'设备',count:s.devices.length},{id:'components',label:'组件',count:s.components.length},{id:'ports',label:'端口',count:s.totals?.ports??s.ports.length},{id:'connections',label:'双端连接',count:s.totals?.connections??s.connections.length},{id:'placements',label:'落位',count:s.devices.length}];
  const [paging,setPaging]=useState({page:0,search:'',sorting:[] as {id:string;desc:boolean}[]});
  const [page,setPage]=useState<{rows:ReviewRow[];total:number}>({rows:[],total:0});
  const [pageError,setPageError]=useState('');
  useEffect(()=>setPaging({page:0,search:'',sorting:[]}),[category,issuesOnly]);
  useEffect(()=>{let active=true;setPageError('');const timer=setTimeout(()=>{const q=new URLSearchParams({category,offset:String(paging.page*10),limit:'10',q:paging.search,issues:String(issuesOnly),sort:paging.sorting[0]?.id??'name',desc:String(paging.sorting[0]?.desc??false)});void api.reviewPage!(q).then(value=>{if(active)setPage(value);}).catch(e=>{if(active){setPage({rows:[],total:0});setPageError(e.message);}});},150);return()=>{active=false;clearTimeout(timer);};},[api,category,issuesOnly,paging,s.revision]);
  const rows=page.rows;
  const columns:ColumnDef<ReviewRow,any>[]=[
    ...(category==='devices'?[{id:'select',header:()=> <input aria-label="选择全部可入库设备" type="checkbox" checked={eligible.size>0&&selected.length===eligible.size} onChange={e=>setSelected(e.target.checked?[...eligible]:[])}/>,cell:({row}:any)=><input aria-label={`选择 ${row.original.name}`} type="checkbox" checked={selected.includes(row.original.id)} disabled={!eligible.has(row.original.id)} onClick={e=>e.stopPropagation()} onChange={e=>setSelected(e.target.checked?[...selected,row.original.id]:selected.filter(id=>id!==row.original.id))}/>,enableSorting:false,size:40}]:[]),
    {accessorKey:'name',header:category==='connections'?'本端设备 / 端口':category==='components'?'组件 / 型号':category==='ports'?'端口 / 速率':'设备 / 型号',cell:({row})=><div className="table-name"><b>{row.original.name}</b><small>{row.original.description}</small></div>},
    {accessorKey:'reference',header:category==='devices'?'设备SN':category==='connections'?'对端设备 / 端口':category==='placements'?'机柜':'所属设备',cell:({getValue})=><code>{getValue()}</code>},
    {accessorKey:'location',header:category==='ports'?'MAC':category==='connections'?'线缆规格':category==='components'?'槽位':category==='placements'?'U位 / 高度':'机柜'},
    {accessorKey:'state',header:'核对状态',cell:({getValue})=><Badge state={getValue()}/>},
    {id:'source',header:'依据',cell:({row})=><button className="source-link" onClick={e=>{e.stopPropagation();inspect('evidence',row.original.id,row.original.evidence);}}><FileSpreadsheet size={13}/>{row.original.evidence.sheet}<ArrowRight size={12}/></button>},
  ];
  const edit=(i:Issue)=>{setIssue(i);setAnswer(i.scope==='identity'?i.entityId:String(s.models.find(m=>m.tableId==='equipment_specs')?.rows.find(r=>r.model===s.devices.find(d=>d.id===i.entityId)?.model)?.height_u??''));};
  return <>
    <Heading eyebrow="REVIEW & RECONCILE" title="把注意力放在需要确认的地方" description="批量事实自动关联；只处理身份冲突和真正影响下一步的缺口。" action={<Button kind="primary" onClick={()=>go('publish'+(selected.length?'?targets='+selected.join(','):''))}>预览入库范围<ArrowRight size={15}/></Button>}/>
    <div className="review-summary"><span className="summary-icon"><ShieldCheck size={23}/></span><div><b>{eligible.size} 台设备身份明确，可以先入库</b><p>组件、端口与连接随对应设备一同确认，未完成的落位和工程配置继续保留。</p></div><Badge state="partial">{s.issues.filter(i=>!i.resolved&&i.scope!=='configuration').length} 项待确认</Badge></div>
    <div className="review-layout"><div className="review-table section"><div className="review-tabs"><Tabs items={tabs} value={category} onChange={v=>{const next=new URLSearchParams(params);next.set('category',v);setParams(next);}}/></div><>{pageError&&<Notice tone="warning">{pageError}</Notice>}<DataTable remote={{...paging,total:page.total,change:setPaging}} data={rows} columns={columns} onRow={r=>inspect(category==='devices'||category==='placements'?'device':category==='connections'?'connection':category==='components'?'component':'port',r.id)} toolbar={<><label className="filter-check"><input type="checkbox" checked={issuesOnly} onChange={e=>{const next=new URLSearchParams(params);if(e.target.checked)next.set('filter','issues');else next.delete('filter');setParams(next);}}/>仅待确认</label><Button kind="ghost" onClick={()=>run('核对结果已导出',async()=>{const result=await api.exportReview!(new URLSearchParams({category,q:paging.search,issues:String(issuesOnly),sort:paging.sorting[0]?.id??'name',desc:String(paging.sorting[0]?.desc??false)}));downloadFile(JSON.stringify(result,null,2),'核对结果-'+category+'.json','application/json');})}><Download size={14}/>导出</Button></>}/></>{selected.length>0&&<div className="selection-bar"><CheckCheck size={16}/><b>已选 {selected.length} 台设备</b><Button kind="ghost" onClick={()=>setSelected([])}>清空选择</Button><Button kind="primary" onClick={()=>go('publish?targets='+selected.join(','))}>确认所选范围<ArrowRight size={14}/></Button></div>}</div>
      <aside className="review-issues"><div className="issues-heading"><h2>集中处理</h2><span>{s.issues.filter(i=>!i.resolved).length}</span></div>{s.issues.filter(i=>!i.resolved).slice(0,issueLimit).map(i=><div className={'issue-card '+i.scope} key={i.id}><div><Badge state={i.scope==='identity'?'blocked':'needs_input'}>{i.scope==='identity'?'身份关联':i.scope==='placement'?'位置核对':'后续配置'}</Badge><small>{i.scope==='identity'?'影响局部入库':'不影响设备入库'}</small></div><h3>{i.title}</h3><p>{i.description}</p><span className="issue-impact">{i.impact}</span><Button kind="ghost" disabled={!can('bom:edit')&& !i.action} onClick={()=>i.action?go(i.action):i.scope==='configuration'?go('models/cabling'):edit(i)}>{i.scope==='configuration'?'补充建模':'处理此项'}<ArrowRight size={14}/></Button></div>)}{s.issues.filter(i=>i.resolved).slice(0,issueLimit).map(i=><div className="resolved-issue" key={i.id}><Check size={15}/><span>{i.title}</span><small>已确认</small></div>)}{s.issues.length>issueLimit&&<Button kind="ghost" onClick={()=>setIssueLimit(n=>n+20)}>显示更多事项</Button>}</aside>
    </div>
    {issue&&<Modal title={issue.title} description={issue.description} close={()=>setIssue(undefined)}><Notice>{issue.impact}。确认后保留原始来源和本次人工决定。</Notice>{issue.scope==='identity'?<Field label="匹配到的设备" hint={matches.length?'仅显示相同SN和类型的身份，确认后保留所选身份。':'当前无可关联身份，请修正来源后重新识别；该对象保持待确认。'}><select value={answer} onChange={e=>setAnswer(e.target.value)}><option value="">选择相同SN的明确身份</option>{matches.map(d=><option key={d.id} value={d.id}>{d.name} · {d.serial}</option>)}</select></Field>:<Field label="占用高度 U" hint="建议值来自明确匹配的设备型号，确认后应用于此设备。"><input type="number" min="1" step="1" value={answer} onChange={e=>setAnswer(e.target.value)}/></Field>}<div className="modal-actions"><Button onClick={()=>setIssue(undefined)}>稍后处理</Button><Button kind="primary" disabled={issue.scope==='identity'&&!matches.some(d=>d.id===answer)} onClick={async()=>{if(await run('关联已确认，相关数据已更新',()=>api.resolveIssue(issue.id,answer)))setIssue(undefined);}}><Check size={14}/>确认关联</Button></div></Modal>}
  </>;
}
